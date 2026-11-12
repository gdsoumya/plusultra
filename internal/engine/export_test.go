package engine

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

const stateScript = "program(\"x\", weeks=2)\nday(1, 1, \"A\")\nex(\"Squat\", sets=[s(3, 5, kg(100))])\n"

func sampleOverlay() model.Overlay {
	abs, reps, on := model.Weight{Value: 100, Unit: model.UnitKg}, 6, true
	squat := model.ProgKey{Exercise: "Squat"}
	heavy := model.ProgKey{Exercise: "Bench Press", Label: "heavy"}
	return model.Overlay{
		Weights: map[model.SetAddr]model.WeightOverride{
			{Key: squat}:                           {Delta: model.Weight{Value: 2.5, Unit: model.UnitKg}, Scale: 1},
			{Key: heavy, Week: 2, Set: 1}:          {Abs: &abs},
			{Key: tagKey(1), Day: 3, Variation: 2}: {Scale: 0.9},
		},
		Reps:         map[model.SetAddr]model.IntOverride{{Key: squat, Set: 2}: {Abs: &reps}},
		RPE:          map[model.SetAddr]model.FloatOverride{{Key: squat}: {Delta: -0.5}},
		Amrap:        map[model.SetAddr]model.BoolOverride{{Key: squat, Set: 3}: {Abs: &on}},
		NumSets:      map[model.DayAddr]model.IntOverride{{Key: squat, Week: 2}: {Delta: 1}},
		SetVariation: map[model.ProgKey]int{squat: 2},
		DescIndex:    map[model.ProgKey]int{heavy: 3},
		State: map[model.ProgKey]map[string]starlark.Value{
			squat: {"misses": starlark.MakeInt(2), "last": &weightValue{quantity{97.5, model.UnitKg}}, "none": starlark.None,
				"xs": starlark.NewList([]starlark.Value{starlark.Float(1.5), starlark.String("a\"b")})},
			tagKey(1): {"rating": starlark.MakeInt(5)},
		},
		RM1: map[string]model.Weight{"Squat": {Value: 140, Unit: model.UnitKg}},
	}
}

func overlayText(t *testing.T, o model.Overlay) string {
	t.Helper()
	b, err := MarshalOverlay(o)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStateBlockRoundTrip(t *testing.T) {
	want := sampleOverlay()
	block, err := FormatState(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(block, "state({\n") || !strings.HasSuffix(block, "\n})\n") || strings.Contains(block, "null") ||
		strings.Contains(block, "\"Work\"") { // nil maps are left out
		t.Fatalf("block:\n%s", block)
	}
	src := stateScript + "\n" + block
	p, st, err := EvaluateState(src, "t.star")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "x" || st == nil || st.Line != 5 {
		t.Fatalf("program %v, state %+v", p, st)
	}
	if got, want := overlayText(t, st.Overlay), overlayText(t, want); got != want {
		t.Fatalf("overlay changed:\n%s\n%s", got, want)
	}
	if _, err := Evaluate(src, "t.star"); err != nil {
		t.Fatalf("Evaluate accepts the block: %v", err)
	}
	if _, st, err := EvaluateState(stateScript, "t.star"); err != nil || st != nil {
		t.Fatalf("no block: %+v, %v", st, err)
	}
}

func TestStateBlockErrors(t *testing.T) {
	for _, bad := range []string{
		"state({})\nstate({})\n",
		"state(\"x\")\n",
		"state({\"Weights\": 1})\n",
		"state({1: 2})\n",
	} {
		if _, _, err := EvaluateState(stateScript+bad, "t.star"); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSplitExport(t *testing.T) {
	block, err := FormatState(sampleOverlay())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		src, script string
		meal        bool
	}{
		{stateScript + "\n" + block, stateScript, false},
		{stateScript + "\n\n" + block, stateScript + "\n", false},
		{stateScript, stateScript, false},
		{"meal_program(\"m\", weeks=1)\n\ncatalog_snapshot({})\nstate({})\n", "meal_program(\"m\", weeks=1)\n", true},
		{"catalog_snapshot({})\nmeal_program(\"m\", weeks=1)\n", "catalog_snapshot({})\nmeal_program(\"m\", weeks=1)\n", true},
		{"meal_program(\"m\", weeks=1)\ncatalog_snapshot({})\n\ncatalog_snapshot({})\n", "meal_program(\"m\", weeks=1)\ncatalog_snapshot({})\n", true},
		{"program(\"x\",\n", "program(\"x\",\n", false},
	} {
		script, meal := SplitExport(tc.src)
		if script != tc.script || meal != tc.meal {
			t.Errorf("SplitExport(%q) = %q, %v; want %q, %v", tc.src, script, meal, tc.script, tc.meal)
		}
	}
}

func TestSnapshotPerUnit(t *testing.T) {
	egg := model.Macros{Kcal: 72, Protein: 6.3}
	foods := map[string]model.Food{
		"usda_sr:169705": {Name: "Oats", Ref: "usda_sr:169705", Per100g: model.Macros{Kcal: 389, Protein: 16.9}},
		"Egg, whole":     {Name: "Egg, whole", Per100g: model.Macros{Kcal: 143}, PerUnit: &egg, Unit: "egg"},
	}
	block := FormatSnapshot(foods)
	want := "catalog_snapshot({\n" +
		"    \"Egg, whole\": {\"per100g\": {\"kcal\": 143}, \"per_unit\": {\"kcal\": 72, \"protein\": 6.3}, \"unit\": \"egg\"},\n" +
		"    \"usda_sr:169705\": {\"kcal\": 389, \"protein\": 16.9},\n})\n"
	if block != want {
		t.Fatalf("block:\n%s\nwant:\n%s", block, want)
	}
	src := "meal_program(\"m\", weeks=1)\noats = food(\"Oats\", ref=\"usda_sr:169705\")\negg = food(\"Egg, whole\")\n" +
		"day(1, kind=\"training\")\nmeal(\"B\", oats(100), egg(2, unit=\"egg\"), item(egg, 50, unit=\"g\"), egg(100))\n\n" + block
	m, snap, err := EvaluateSnapshot(src, "t.star")
	if err != nil {
		t.Fatal(err)
	}
	items := m.Days[0].Meals[0].Items
	if items[0].Macros.Kcal != 389 || items[1].Unit != "egg" || items[1].Macros.Kcal != 144 || items[2].Unit != "g" || items[2].Macros.Kcal != 71.5 ||
		items[1].Food.PerUnit == nil || items[1].Food.Unit != "egg" || items[3].Unit != "g" || items[3].Macros.Kcal != 143 { // grams, not 100 eggs
		t.Fatalf("items %+v", items)
	}
	if len(snap) != 2 || snap["Egg, whole"].PerUnit.Kcal != 72 || snap["Egg, whole"].Unit != "egg" || snap["usda_sr:169705"].Per100g.Kcal != 389 {
		t.Fatalf("snapshot %+v", snap)
	}
	if _, _, err := EvaluateSnapshot(strings.Replace(src, "item(egg, 50, unit=\"g\")", "item(oats, 1, unit=\"cup\")", 1), "t.star"); err == nil ||
		!strings.Contains(err.Error(), "cup") {
		t.Fatalf("unit the snapshot lacks: %v", err)
	}
	for _, bad := range []string{
		"catalog_snapshot({\"a\": {\"per100g\": {}, \"per_unit\": {\"kcal\": 1}}})\n",
		"catalog_snapshot({\"a\": {\"per100g\": 1}})\n",
	} {
		if _, err := EvaluateMeal("meal_program(\"m\", weeks=1)\n"+bad, "t.star"); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
