package programs_test

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/model"

	"github.com/gdsoumya/plusultra/internal/programs"
)

const mealScript = `meal_program("Eat", weeks=8)
oats = food("Oats", per100g={"kcal": 380, "protein": 13})
day(1, kind="training", name="Train", weeks="1-8")
meal("Breakfast", oats(100))
day(1, kind="rest", name="Rest", weeks="1-8")
meal("Breakfast", oats(50))
day(1, 7, "Sunday feast")
meal("Brunch", oats(200))
`

func TestResolvedDaysRange(t *testing.T) {
	e := setup(t)
	p := e.activate(e.create(programs.KindWorkout, eightWeeks).ID, &today)
	days := e.days(p.ID, programs.Range{From: date(2026, 11, 12), To: date(2026, 11, 16)})
	if len(days) != 5 || !days[0].Date.Equal(date(2026, 11, 12)) || days[4].Week != 2 || days[4].Day != 1 {
		t.Fatalf("days %+v", days)
	}
	if days[4].Kind != "training" || days[4].Workout.Exercises[0].Name != "Squat" {
		t.Fatalf("monday of week 2 %+v", days[4])
	}
	if got := e.days(p.ID, programs.Range{FromWeek: 8}); len(got) != 7 || got[0].Week != 8 {
		t.Fatalf("week 8 of anchored: %d days", len(got))
	}

	f := e.create(programs.KindWorkout, twoWeeks)
	if _, err := e.s.Finish(t.Context(), e.user, p.ID); err != nil {
		t.Fatal(err)
	}
	e.activate(f.ID, nil)
	days = e.days(f.ID, programs.Range{FromWeek: 2, ToWeek: 2})
	if len(days) != 2 || days[0].Week != 2 || days[0].Day != 1 || days[1].Day != 2 {
		t.Fatalf("floating week 2 %+v", days)
	}
}

func TestMealProgramActivateAnchored(t *testing.T) {
	e := setup(t)
	start := date(2026, 11, 10)
	w := e.activate(e.create(programs.KindWorkout, eightWeeks).ID, &start)
	m := e.activate(e.create(programs.KindMeal, mealScript).ID, &start)
	if !m.EndDate.Equal(date(2027, 1, 3)) {
		t.Fatalf("meal end %v", m.EndDate)
	}
	days := e.days(m.ID, programs.Range{To: date(2026, 11, 16)})
	if len(days) != 7 {
		t.Fatalf("%d meal days, want Tuesday to Monday", len(days))
	}
	for i, want := range []struct {
		kind, name string
		kcal       float64
	}{
		{"training", "Train", 380}, {"rest", "Rest", 190}, {"rest", "Rest", 190}, {"rest", "Rest", 190},
		{"rest", "Rest", 190}, {"rest", "Sunday feast", 760}, {"training", "Train", 380},
	} {
		d := days[i]
		if d.Kind != want.kind || d.Meal == nil || d.Meal.Name != want.name || d.Meal.Target.Kcal != want.kcal {
			t.Errorf("day %d (%v) = kind %q %+v, want %+v", i, d.Date, d.Kind, d.Meal, want)
		}
	}

	_, err := e.s.Activate(t.Context(), e.user, e.create(programs.KindMeal, mealScript).ID, nil)
	assertCode(t, err, connect.CodeFailedPrecondition)

	// A new workout revision trains on Wednesdays; meal days follow from today on.
	if _, err := e.s.Update(t.Context(), e.user, w.ID, eightWeeks+`day(1, 3, "Extra", weeks="1-8")
ex("Deadlift", warmup=[], sets=[s(1, 5, kg(140))])
`); err != nil {
		t.Fatal(err)
	}
	days = e.days(m.ID, programs.Range{To: date(2026, 11, 12)})
	if len(days) != 3 || days[0].Kind != "training" || days[1].Kind != "training" || days[2].Kind != "rest" {
		t.Fatalf("meal kinds after the workout update: %+v", days)
	}

	// Without an anchored workout program every date is a rest date.
	if _, err := e.s.Finish(t.Context(), e.user, w.ID); err != nil {
		t.Fatal(err)
	}
	if days = e.days(m.ID, programs.Range{From: today, To: today}); len(days) != 1 || days[0].Kind != "rest" {
		t.Fatalf("meal day after finishing the workout: %+v", days)
	}

	// So is every date after the active workout program is deleted.
	w = e.activate(e.create(programs.KindWorkout, eightWeeks).ID, &start)
	if days = e.days(m.ID, programs.Range{From: date(2026, 11, 16), To: date(2026, 11, 16)}); days[0].Kind != "training" {
		t.Fatalf("monday with a workout program: %+v", days)
	}
	if err := e.s.Delete(t.Context(), e.user, w.ID); err != nil {
		t.Fatal(err)
	}
	if days = e.days(m.ID, programs.Range{From: date(2026, 11, 16), To: date(2026, 11, 16)}); days[0].Kind != "rest" {
		t.Fatalf("monday after deleting the workout program: %+v", days)
	}
}

func TestMealProgramFloatingDays(t *testing.T) {
	e := setup(t)
	m := e.activate(e.create(programs.KindMeal, mealScript).ID, &today)
	if _, err := e.s.Finish(t.Context(), e.user, m.ID); err != nil {
		t.Fatal(err)
	}
	m = e.activate(m.ID, nil)
	if rows, err := e.s.Q.ListResolvedDays(t.Context(), m.ID); err != nil || len(rows) != 0 {
		t.Fatalf("%d stored days left from the anchored run, err %v", len(rows), err)
	}
	days := e.days(m.ID, programs.Range{FromWeek: 1, ToWeek: 1})
	if len(days) != 3 || days[0].Day != 7 || days[1].Day != 0 || days[1].Kind != "training" || days[2].Kind != "rest" || days[0].Date != nil {
		t.Fatalf("floating meal days %+v", days)
	}
	if got := e.days(m.ID, programs.Range{FromWeek: 2, ToWeek: 2}); len(got) != 2 {
		t.Fatalf("week 2 %+v, want the two kind days", got)
	}
}

func TestWorkoutActivationKeepsPastMealDays(t *testing.T) {
	e := setup(t)
	start := date(2026, 11, 10)
	a := e.activate(e.create(programs.KindWorkout, eightWeeks).ID, &start)
	m := e.activate(e.create(programs.KindMeal, mealScript).ID, &start)
	if _, err := e.s.Finish(t.Context(), e.user, a.ID); err != nil {
		t.Fatal(err)
	}
	e.activate(e.create(programs.KindWorkout, eightWeeks).ID, &today)
	if days := e.days(m.ID, programs.Range{To: start}); len(days) != 1 || days[0].Kind != "training" {
		t.Fatalf("yesterday's meal day %+v, want it kept as planned", days)
	}
}

func TestMealUnknownFoodHasLineAndSuggestions(t *testing.T) {
	e := setup(t)
	customFood(t, e, "Oats", 380)
	src := `meal_program("Eat", weeks=1)
oats = food("Oatz")
rice = food("Rice", ref="usda_sr:1")
day(1, kind="rest")
meal("Breakfast", oats(100), oats(50), rice(10))
`
	v, err := e.s.Validate(t.Context(), e.user, programs.KindMeal, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.NameErrors) != 2 || v.NameErrors[0].Line != 2 || v.NameErrors[0].Name != "Oatz" || !slices.Contains(v.NameErrors[0].Suggestions, "Oats") ||
		v.NameErrors[1].Line != 3 || v.NameErrors[1].Name != "usda_sr:1" {
		t.Fatalf("name errors %+v, want one per food line", v.NameErrors)
	}
	_, err = e.s.Create(t.Context(), e.user, programs.KindMeal, src)
	assertCode(t, err, connect.CodeInvalidArgument)
	if !strings.Contains(err.Error(), `line 2: unknown food "Oatz"; did you mean Oats?; add a custom food or remove the item`) {
		t.Fatalf("err = %v", err)
	}
}

func TestMealNameOnlyFoodFilledFromCatalog(t *testing.T) {
	e := setup(t)
	oats := customFood(t, e, "Oats", 380)
	rice := customFood(t, e, "Rice", 130)
	src := `meal_program("Eat", weeks=1)
oats = food("Oats")
rice = food("White rice", ref="custom:` + rice.SourceID + `")
day(1, kind="rest", name="Rest")
meal("Lunch", oats(50), rice(200))
`
	m := e.activate(e.create(programs.KindMeal, src).ID, nil)
	// Catalog edits after saving do not reach the program: its macros are frozen.
	if _, err := catalog.UpdateCustomFood(t.Context(), e.s.Q, e.user, oats.ID, catalog.CustomFood{Name: "Oats", Per100g: model.Macros{Kcal: 400}}); err != nil {
		t.Fatal(err)
	}
	days := e.days(m.ID, programs.Range{})
	if len(days) != 1 || days[0].Meal.Target.Kcal != 190+260 {
		t.Fatalf("days %+v", days)
	}
	items := days[0].Meal.Meals[0].Items
	if f := items[0].Food; items[0].Macros.Kcal != 190 || f.Name != "Oats" || f.Ref != "custom:"+oats.SourceID || f.Unresolved {
		t.Errorf("oats %+v", items[0])
	}
	if f := items[1].Food; items[1].Macros.Kcal != 260 || f.Name != "White rice" || f.Ref != "custom:"+rice.SourceID {
		t.Errorf("rice %+v", items[1])
	}

	stored, err := e.s.Get(t.Context(), e.user, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	mp, err := programs.LoadMeal(stored)
	if err != nil || mp.Days[0].Meals[0].Items[0].Macros.Kcal != 190 {
		t.Fatalf("LoadMeal = %+v, %v; want the frozen oats", mp, err)
	}
}

func TestMealExplicitMacrosUntouched(t *testing.T) {
	e := setup(t)
	customFood(t, e, "Banana", 1)
	oats := customFood(t, e, "Oats", 380)
	src := `meal_program("Eat", weeks=1)
banana = food("Banana", per100g={"kcal": 89})
oats = food("Oats", ref="custom:` + oats.SourceID + `", per100g={"kcal": 10})
day(1, kind="rest")
meal("Snack", banana(100), oats(100))
`
	m := e.activate(e.create(programs.KindMeal, src).ID, nil)
	items := e.days(m.ID, programs.Range{})[0].Meal.Meals[0].Items
	if items[0].Macros.Kcal != 89 || items[0].Food.Ref != "" || items[1].Macros.Kcal != 10 {
		t.Fatalf("items %+v, want the script's macros", items)
	}
}

func customFood(t *testing.T, e env, name string, kcal float64) catalog.Food {
	t.Helper()
	f, err := catalog.CreateCustomFood(t.Context(), e.s.Q, e.user, catalog.CustomFood{Name: name, Per100g: model.Macros{Kcal: kcal}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestMealUnknownUnitIsAValidationError(t *testing.T) {
	e := setup(t)
	if _, err := catalog.CreateCustomFood(t.Context(), e.s.Q, e.user, catalog.CustomFood{Name: "Milk", Per100g: model.Macros{Kcal: 60}, Units: []catalog.Unit{{Name: "cup", Grams: 240}}}); err != nil {
		t.Fatal(err)
	}
	src := `meal_program("Eat", weeks=1)
milk = food("Milk")
day(1, kind="rest")
meal("Breakfast", item(milk, 200, unit="ml"))
`
	v, err := e.s.Validate(t.Context(), e.user, programs.KindMeal, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Errors) != 1 || v.Errors[0].Line != 2 || !strings.Contains(v.Errors[0].Msg, `Milk has no unit "ml"`) || !strings.Contains(v.Errors[0].Msg, "cup") {
		t.Fatalf("errors %+v, want the unit error with the units Milk has", v.Errors)
	}
	_, err = e.s.Create(t.Context(), e.user, programs.KindMeal, src)
	assertCode(t, err, connect.CodeInvalidArgument)
}
