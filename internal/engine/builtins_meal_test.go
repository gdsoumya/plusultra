package engine

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

func readMeal(t *testing.T, file string) string {
	t.Helper()
	src, err := fs.ReadFile(os.DirFS("testdata/meals"), file)
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

func loadMeal(t *testing.T, file string) *model.MealProgram {
	t.Helper()
	p, err := EvaluateMeal(readMeal(t, file), file)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mealDayFor(t *testing.T, p *model.MealProgram, week, num int, kind string) *model.MealDay {
	t.Helper()
	d := p.DayFor(week, num, kind)
	if d == nil || d.Target == nil {
		t.Fatalf("DayFor(%d, %d, %q) = %+v, want a day with a target", week, num, kind, d)
	}
	return d
}

func TestFoodQtyScalesMacros(t *testing.T) {
	p := loadMeal(t, "chicken.star")
	if p.Name != "Cut" || p.Weeks != 4 || p.Loop || p.Notes != "High protein" || len(p.Days) != 1 {
		t.Fatalf("got %+v", p)
	}
	d := p.Days[0]
	if d.Week != 1 || d.Num != 1 || d.Kind != "training" || d.Name != "Training" || d.Desc != "Lift day" || len(d.Meals) != 2 {
		t.Fatalf("day = %+v", d)
	}
	lunch := d.Meals[0]
	it := lunch.Items[0]
	if lunch.Name != "Lunch" || lunch.Time != "12:30" || it.Qty != 200 || it.Unit != "g" || it.Note != "grilled" ||
		it.Food.Name != "Chicken breast" {
		t.Fatalf("lunch = %+v", lunch)
	}
	if want := (model.Macros{Kcal: 240, Protein: 46, Fat: 5}); it.Macros != want {
		t.Errorf("lunch macros = %+v, want %+v", it.Macros, want)
	}
	if got := d.Meals[1].Items[0].Macros.Kcal; got != 120 {
		t.Errorf("food(qty) sugar kcal = %v, want 120", got)
	}
}

func TestPerUnitFood(t *testing.T) {
	items := loadMeal(t, "egg.star").Days[0].Meals[0].Items
	eggs, grams := items[0], items[1]
	if eggs.Unit != "egg" || eggs.Macros != (model.Macros{Kcal: 210, Protein: 18}) {
		t.Errorf("3 eggs = %+v, want 210 kcal, 18 g protein", eggs)
	}
	if grams.Unit != "g" || grams.Macros.Kcal != 71.5 {
		t.Errorf("50 g of egg = %+v, want 71.5 kcal", grams)
	}
}

func TestTargetDefaultsToSum(t *testing.T) {
	d := loadMeal(t, "targets.star").Days[0]
	want := model.Macros{Kcal: 570, Protein: 34.5, Carbs: 84}
	if d.Totals() != want || d.Target == nil || *d.Target != want {
		t.Fatalf("totals = %+v, target = %+v, want both %+v", d.Totals(), d.Target, want)
	}
}

func TestExplicitTargetOverrides(t *testing.T) {
	d := loadMeal(t, "targets.star").Days[1]
	if want := (model.Macros{Kcal: 2200, Protein: 180, WaterML: 3000}); d.Target == nil || *d.Target != want {
		t.Fatalf("target = %+v, want %+v", d.Target, want)
	}
	if d.Totals().Kcal != 260 {
		t.Errorf("totals = %+v, want 260 kcal", d.Totals())
	}
}

func TestKindDayLookup(t *testing.T) {
	p := loadMeal(t, "kinds.star")
	if d := mealDayFor(t, p, 1, 1, "training"); d.Num != 0 || d.Target.Kcal != 2600 || len(d.Meals) != 1 {
		t.Errorf("training = %+v", d)
	}
	if d := mealDayFor(t, p, 1, 1, "rest"); d.Num != 0 || d.Target.Kcal != 2100 {
		t.Errorf("rest = %+v", d)
	}
}

func TestNumberedDayWins(t *testing.T) {
	p := loadMeal(t, "kinds.star")
	for _, kind := range []string{"training", "rest"} {
		if d := mealDayFor(t, p, 1, 2, kind); d.Name != "Heavy" || d.Target.Kcal != 3000 {
			t.Errorf("day 2 as %s = %+v, want the numbered day", kind, d)
		}
	}
	if d := mealDayFor(t, p, 2, 2, "training"); d.Target.Kcal != 2600 {
		t.Errorf("week 2 day 2 = %+v, want the week 1 training day", d)
	}
}

func TestNearestEarlierWeekFallback(t *testing.T) {
	p := loadMeal(t, "fallback.star")
	for week, want := range map[int]float64{1: 2000, 2: 2000, 3: 1900, 4: 1900, 6: 1900} {
		if d := mealDayFor(t, p, week, 1, "rest"); d.Target.Kcal != want {
			t.Errorf("week %d rest = %v kcal, want %v", week, d.Target.Kcal, want)
		}
	}
	if d := p.DayFor(1, 1, "training"); d != nil {
		t.Errorf("week 1 training = %+v, want nil", d)
	}
	if d := mealDayFor(t, p, 5, 1, "training"); d.Target.Kcal != 2500 {
		t.Errorf("week 5 training = %+v", d)
	}
}

func TestFoodRefTakesSnapshotMacros(t *testing.T) {
	src := readMeal(t, "ref.star")
	p, err := EvaluateMeal(src, "ref.star")
	if err != nil || p.Days[0].Meals[0].Items[0].Macros != (model.Macros{}) {
		t.Fatalf("without a snapshot: %v, %+v; want no macros yet", err, p)
	}
	snap := `catalog_snapshot({"usda:173904": {"kcal": 380, "protein": 13, "carbs": 68, "fat": 7, "fiber": 10}, "usda:173944": {"kcal": 1}})`
	p, err = EvaluateMeal(src+snap+"\n", "ref.star")
	if err != nil {
		t.Fatal(err)
	}
	items := p.Days[0].Meals[0].Items
	if want := (model.Macros{Kcal: 190, Protein: 6.5, Carbs: 34, Fat: 3.5, Fiber: 5}); items[0].Macros != want {
		t.Errorf("oats = %+v, want %+v", items[0].Macros, want)
	}
	if items[0].Food.Ref != "usda:173904" || items[0].Food.Per100g.Kcal != 380 {
		t.Errorf("oats food = %+v, want snapshot macros frozen in", items[0].Food)
	}
	if items[1].Macros.Kcal != 89 {
		t.Errorf("banana = %+v, want its declared macros kept", items[1].Macros)
	}
}

func TestLoopTableGeneratesWeeks(t *testing.T) {
	p := loadMeal(t, "loop.star")
	if len(p.Days) != 12 {
		t.Fatalf("got %d days, want 12", len(p.Days))
	}
	for w := 1; w <= 6; w++ {
		step := 0.0
		if w > 3 {
			step = 250
		}
		for kind, kcal := range map[string]float64{"training": 2600, "rest": 2200} {
			if d := mealDayFor(t, p, w, 1, kind); d.Target.Kcal != kcal-step || d.Totals().Kcal != 240 {
				t.Errorf("week %d %s = %+v, want %v kcal target", w, kind, d, kcal-step)
			}
		}
	}
}

func TestMealErrors(t *testing.T) {
	const pre = "meal_program(\"M\", weeks=2)\nf = food(\"F\", per100g={\"kcal\": 100})\n"
	cases := map[string]string{
		"day(1, 1)\n": "meal_program() first",
		"x = 1\n":     "meal_program() was never called",
		"meal_program(\"M\", weeks=1)\nmeal_program(\"N\", weeks=1)\n": "more than once",
		"meal_program(\"M\", weeks=0)\n":                               "weeks must be 1-104",
		"meal_program(\"M\", weeks=105)\n":                             "weeks must be 1-104",
		pre + "meal(\"L\", f(1))\n":                                    "day()",
		pre + "day(3, 1)\n":                                            "week 3 out of range 1-2",
		pre + "day(1, 1, weeks=\"1-3\")\n":                             "week 3 out of range 1-2",
		pre + "day(1, kind=\"cardio\")\n":                              "kind",
		pre + "day(1, -1)\n":                                           "omit n for a kind day",
		pre + "day(1, 0)\n":                                            "omit n for a kind day",
		pre + "day(1, 1)\nmeal(\"L\", f(100001))\n":                    "qty value too large: the limit is 100000",
		pre + "day(1, 1)\nmeal(\"L\", f(1e308))\n":                     "qty value too large",
		pre + "food(\"G\", per100g={\"kcal\": 1e308})\n":               "kcal value too large",
		pre + "food(\"G\", per_unit={\"fat\": 100001}, unit=\"x\")\n":  "fat value too large",
		pre + "target(water_ml=100001)\n":                              "water_ml value too large",
		pre + "catalog_snapshot({\"r\": {\"kcal\": 1e308}})\n":         "kcal value too large",
		pre + "food(\"G\", per_unit={\"kcal\": 1}, unit=\"g\")\n":      "use per100g",
		pre + "day(1, 1)\nmeal(\"A\", *[f(1) for _ in range(600)])\nmeal(\"B\", *[f(1) for _ in range(600)])\n": "more than 1000 items on this day",
		pre + "day(1, 1)\nmeal(\"L\", f(-1))\n":                        "qty must be a finite non-negative number",
		pre + "day(1, 1)\nmeal(\"L\", f(float(\"nan\")))\n":            "qty must be a finite non-negative number",
		pre + "food(\"G\", per100g={\"kcal\": -1})\n":                  "kcal must be a finite non-negative number",
		pre + "food(\"G\", per100g={\"sugar\": 1})\n":                  "unknown macro \"sugar\"",
		pre + "target(kcal=float(\"inf\"))\n":                          "kcal must be a finite non-negative number",
		pre + "target(2000)\n":                                         "keyword arguments only",
		pre + "food(\"G\", per_unit={\"kcal\": 1})\n":                  "per_unit and unit go together",
		pre + "day(1, 1)\nmeal(\"L\", f(1, unit=\"cup\"))\n":           "unit must be",
		pre + "day(1, 1)\nmeal(\"L\", 1)\n":                            "want item",
		pre + "day(1, 1)\nmeal(\"L\", *[f(1) for _ in range(1001)])\n": "more than 1000 items",
		pre + "e = food(\"E\", per_unit={\"kcal\": 70}, unit=\"egg\")\nday(1, 1)\nmeal(\"L\", e(50, unit=\"g\"))\n": "no per100g",
	}
	for src, want := range cases {
		_, err := EvaluateMeal(src, "test.star")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want error containing %q", src, err, want)
		}
	}
}

func TestMealValueTooLargeAtCallingLine(t *testing.T) {
	_, err := EvaluateMeal("meal_program(\"M\", weeks=1)\nf = food(\"F\",\n  per100g={\"kcal\": 1e308})\n", "big.star")
	ee, ok := errors.AsType[*EvalError](err)
	if !ok || ee.Line != 2 || !strings.Contains(ee.Msg, "value too large") {
		t.Fatalf("got %v, want value too large at line 2", err)
	}
}

func TestMaximalItemsSumFinitely(t *testing.T) {
	p := loadMeal(t, "maximal.star")
	want := 1e13 // 1000 items of 100000 units at 100000 per unit
	tot := p.Days[0].Totals()
	for _, v := range []float64{tot.Kcal, tot.Protein, tot.Carbs, tot.Fat, tot.Fiber, tot.WaterML} {
		if v != want {
			t.Fatalf("totals = %+v, want every field %g", tot, want)
		}
	}
}

func TestExplicitZeroMacrosKept(t *testing.T) {
	items := loadMeal(t, "zero.star").Days[0].Meals[0].Items
	if coffee := items[0]; coffee.Food.Per100g != (model.Macros{}) || coffee.Macros != (model.Macros{}) {
		t.Errorf("coffee = %+v, want its declared zero macros kept over the snapshot", coffee)
	}
	if pill := items[1]; pill.Unit != "g" || pill.Macros != (model.Macros{}) {
		t.Errorf("pill in grams = %+v, want 0 kcal from its explicit per100g", pill)
	}
	if oats := items[2]; oats.Macros.Kcal != 190 {
		t.Errorf("oats = %+v, want snapshot macros for a ref-only food", oats)
	}
}

func TestSameWeekKindDayTieBreak(t *testing.T) {
	p := loadMeal(t, "tie.star")
	for week, want := range map[int]float64{1: 2100, 2: 2100, 3: 1800, 4: 2000} {
		if d := mealDayFor(t, p, week, 1, "rest"); d.Target.Kcal != want {
			t.Errorf("week %d rest = %v kcal, want %v", week, d.Target.Kcal, want)
		}
	}
}

func TestFoodRecordsItsLine(t *testing.T) {
	p := loadMeal(t, "chicken.star")
	if got := p.Days[0].Meals[0].Items[0].Food.Line; got != 2 {
		t.Errorf("food line = %d, want 2", got)
	}
}
