package engine

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

func evaluateFile(t *testing.T, file string) (*model.Program, error) {
	t.Helper()
	src, err := fs.ReadFile(os.DirFS("testdata/structure"), file)
	if err != nil {
		t.Fatal(err)
	}
	return Evaluate(string(src), file)
}

func loadProgram(t *testing.T, file string) *model.Program {
	t.Helper()
	p, err := evaluateFile(t, file)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func loadProgramErr(t *testing.T, file string) *EvalError {
	t.Helper()
	_, err := evaluateFile(t, file)
	ee, ok := errors.AsType[*EvalError](err)
	if !ok {
		t.Fatalf("got %v, want *EvalError", err)
	}
	return ee
}

func dayAt(t *testing.T, p *model.Program, week, num int) *model.Day {
	t.Helper()
	d := p.DayAt(week, num)
	if d == nil {
		t.Fatalf("DayAt(%d, %d) = nil", week, num)
	}
	return d
}

func exNames(d *model.Day) []string {
	var names []string
	for _, e := range d.Exercises {
		names = append(names, e.Names[0])
	}
	return names
}

func kgLoad(v float64) model.Load {
	return model.Load{Kind: model.LoadAbsolute, Weight: model.Weight{Value: v, Unit: model.UnitKg}}
}

var bwLoad = model.Load{Kind: model.LoadBodyweight, Delta: model.Weight{Unit: model.UnitKg}}

func TestProgramHeader(t *testing.T) {
	p := loadProgram(t, "header.star")
	if p.Name != "Strength 8" || p.Weeks != 8 || p.Units != model.UnitLb || !p.Loop || p.Notes != "Three days a week" {
		t.Fatalf("got %+v", p)
	}
}

func TestExBeforeDayErrors(t *testing.T) {
	err := loadProgramErr(t, "ex_before_day.star")
	if err.Line != 3 || !strings.Contains(err.Msg, "day()") {
		t.Fatalf("got %v, want error at line 3 naming day()", err)
	}
}

func TestDayWeekRangeRepeats(t *testing.T) {
	p := loadProgram(t, "day_week_range.star")
	d := dayAt(t, p, 5, 2)
	if d.Week != 5 || d.Name != "Upper" || !slices.Equal(exNames(d), []string{"Bench Press"}) {
		t.Fatalf("DayAt(5, 2) = %+v", d)
	}
	if p.DayAt(8, 2) != nil || p.DayAt(5, 1) != nil {
		t.Fatal("day placed outside its weeks")
	}
}

func TestExerciseWeeksRange(t *testing.T) {
	p := loadProgram(t, "ex_weeks_range.star")
	for w := 1; w <= 7; w++ {
		if !slices.Contains(exNames(dayAt(t, p, w, 1)), "Squat") {
			t.Errorf("week %d has no Squat", w)
		}
	}
	if got := exNames(dayAt(t, p, 2, 1)); !slices.Equal(got, []string{"Squat"}) {
		t.Errorf("week 2 = %v, want only Squat", got)
	}
	if d := p.DayAt(8, 1); d != nil {
		t.Errorf("week 8 = %+v, want nil", d)
	}
}

func TestLaterWeekOverridesExercise(t *testing.T) {
	p := loadProgram(t, "override.star")
	w2, w3 := dayAt(t, p, 2, 2), dayAt(t, p, 3, 2)
	if got := exNames(w2); !slices.Equal(got, []string{"Bench Press", "Row"}) {
		t.Fatalf("week 2 = %v", got)
	}
	if got, want := w2.Exercises[0].Sets[0], (model.SetGroup{Count: 5, MinReps: 3, MaxReps: 3, Load: kgLoad(85)}); got != want {
		t.Errorf("week 2 bench = %+v, want %+v", got, want)
	}
	if got, want := w3.Exercises[0].Sets[0], (model.SetGroup{Count: 3, MinReps: 5, MaxReps: 5, Load: kgLoad(80)}); got != want {
		t.Errorf("week 3 bench = %+v, want %+v", got, want)
	}
	if w2.Name != "Upper heavy" || w3.Name != "Upper" {
		t.Errorf("names = %q, %q", w2.Name, w3.Name)
	}
}

func TestDescriptionInheritsAndStops(t *testing.T) {
	p := loadProgram(t, "desc.star")
	cases := []struct {
		week, day int
		want      []string
	}{
		{1, 1, []string{"Brace hard"}},
		{2, 1, []string{"Brace hard"}},
		{3, 1, nil},
		{4, 1, nil},
		{1, 2, []string{"Pause first rep", "Touch and go"}},
		{2, 2, []string{"Pause first rep", "Touch and go"}},
		{3, 2, nil},
		{4, 2, nil},
	}
	for _, c := range cases {
		if got := dayAt(t, p, c.week, c.day).Exercises[0].Desc; !slices.Equal(got, c.want) {
			t.Errorf("week %d day %d desc = %q, want %q", c.week, c.day, got, c.want)
		}
	}
}

func TestAlternatesAndCurrent(t *testing.T) {
	ex := dayAt(t, loadProgram(t, "alts.star"), 1, 1).Exercises
	if !slices.Equal(ex[0].Names, []string{"Bench Press", "Bench Press, Dumbbell"}) || ex[0].Current != 2 {
		t.Errorf("alts = %v current %d", ex[0].Names, ex[0].Current)
	}
	if !slices.Equal(ex[1].Names, []string{"Pull Up", "Chin Up"}) || ex[1].Current != 1 {
		t.Errorf("name+alts = %v current %d", ex[1].Names, ex[1].Current)
	}
}

func TestSetGroupParsing(t *testing.T) {
	ex := dayAt(t, loadProgram(t, "sets.star"), 1, 1).Exercises
	want := [][]model.SetGroup{
		{
			{Count: 4, MinReps: 5, MaxReps: 5, Load: kgLoad(75), Rest: 180, Label: "Work"},
			{Count: 3, MinReps: 8, MaxReps: 12, Load: model.Load{Kind: model.LoadPercent, Pct: 70}},
			{Count: 1, MinReps: 5, MaxReps: 5, Load: kgLoad(60), Amrap: true},
		},
		{{Count: 3, MinReps: 1, MaxReps: 1, Load: bwLoad, Work: 60, Rest: 30, Auto: true}},
		{{Count: 1, MinReps: 1, MaxReps: 1, Load: bwLoad, Work: 30, CountUp: true}},
		{{Count: 3, MinReps: 10, MaxReps: 15, Load: model.Load{Kind: model.LoadNone}, RPE: 8, Ask: true, LogRPE: true}},
	}
	for i, w := range want {
		if !slices.Equal(ex[i].Sets, w) {
			t.Errorf("%s sets = %+v, want %+v", ex[i].Names[0], ex[i].Sets, w)
		}
	}
}

func TestWarmupNoneVsDefault(t *testing.T) {
	ex := dayAt(t, loadProgram(t, "warmup.star"), 1, 1).Exercises
	if !ex[0].WarmupSet || len(ex[0].Warmup) != 0 {
		t.Errorf("warmup=[]: set %v len %d", ex[0].WarmupSet, len(ex[0].Warmup))
	}
	if ex[1].WarmupSet {
		t.Error("omitted warmup is marked set")
	}
	want := []model.SetGroup{
		{Count: 1, MinReps: 8, MaxReps: 8, Load: model.Load{Kind: model.LoadPercent, Pct: 50}},
		{Count: 1, MinReps: 5, MaxReps: 5, Load: kgLoad(60)},
	}
	if !ex[2].WarmupSet || !slices.Equal(ex[2].Warmup, want) {
		t.Errorf("warmup = %+v, want %+v", ex[2].Warmup, want)
	}
}

func TestMissingWeightErrors(t *testing.T) {
	err := loadProgramErr(t, "missing_weight.star")
	if err.Line != 3 || !strings.Contains(err.Msg, "weight required") {
		t.Fatalf("got %v, want weight required at line 3", err)
	}
}

func TestLabelsSupersetsTags(t *testing.T) {
	ex := dayAt(t, loadProgram(t, "labels.star"), 1, 1).Exercises
	if len(ex) != 4 || ex[0].Label != "heavy" || ex[1].Label != "light" {
		t.Fatalf("labels: %+v", ex)
	}
	if !slices.Equal(ex[0].Tags, []int{1, 101}) {
		t.Errorf("tags = %v", ex[0].Tags)
	}
	if ex[2].Superset != "A" || ex[3].Superset != "A" || ex[2].Equipment != "cable" {
		t.Errorf("superset/eq = %q %q %q", ex[2].Superset, ex[3].Superset, ex[2].Equipment)
	}
}

func TestVariations(t *testing.T) {
	e := dayAt(t, loadProgram(t, "variations.star"), 1, 1).Exercises[0]
	if len(e.Sets) != 0 || len(e.Variations) != 2 || len(e.Variations[1]) != 2 || !e.Variations[1][1].Amrap {
		t.Fatalf("got sets %+v variations %+v", e.Sets, e.Variations)
	}
}

func TestTemplateSpread(t *testing.T) {
	ex := dayAt(t, loadProgram(t, "template.star"), 1, 1).Exercises
	for _, e := range ex {
		if len(e.Sets) != 1 || e.Sets[0].Count != 3 || !e.WarmupSet || e.Progress == nil || e.Progress.Kind != model.ProgressCustom {
			t.Errorf("%s = %+v", e.Names[0], e)
		}
	}
	a, b := ex[0].Progress.State, ex[1].Progress.State
	if len(a) != 1 || !maps.Equal(a, b) {
		t.Fatalf("states = %+v, %+v", a, b)
	}
	a["probe"] = model.StateVar{}
	if _, shared := b["probe"]; shared {
		t.Error("exercises share one state map")
	}
}

func TestLaterWeekWinsOverSourceOrder(t *testing.T) {
	p := loadProgram(t, "reversed_override.star")
	w2, w3 := dayAt(t, p, 2, 1), dayAt(t, p, 3, 1)
	if w2.Name != "Heavy" || w2.Exercises[0].Sets[0].Load != kgLoad(120) {
		t.Errorf("week 2 = %q %+v, want Heavy at 120 kg", w2.Name, w2.Exercises[0].Sets)
	}
	if w3.Name != "Base" || w3.Exercises[0].Sets[0].Load != kgLoad(100) {
		t.Errorf("week 3 = %q %+v, want Base at 100 kg", w3.Name, w3.Exercises[0].Sets)
	}
}

func TestLoopThenRangedAccessories(t *testing.T) {
	p := loadProgram(t, "loop_then_accessories.star")
	w4 := dayAt(t, p, 4, 1)
	if w4.Desc != "Deload" || !slices.Equal(exNames(w4), []string{"Squat", "Leg Curl"}) || w4.Exercises[0].Sets[0].Load != kgLoad(120) {
		t.Errorf("week 4 = %q %v %+v", w4.Desc, exNames(w4), w4.Exercises[0].Sets)
	}
	if got := exNames(dayAt(t, p, 2, 1)); !slices.Equal(got, []string{"Squat", "Leg Curl"}) {
		t.Errorf("week 2 = %v", got)
	}
}

func TestStateValuesFrozen(t *testing.T) {
	pre := "program(\"P\", weeks=1)\ndef f(ctx):\n    pass\nday(1, 1, \"A\")\nl = [1]\n"
	for _, arg := range []string{"h=l", "h=prompt(l)"} {
		src := pre + "ex(\"Squat\", sets=[s(1, 1, kg(1))], progress=custom(f, " + arg + "))\nl.append(2)\n"
		if _, err := Evaluate(src, "test.star"); err == nil || !strings.Contains(err.Error(), "frozen") {
			t.Errorf("%s: got %v, want frozen list error", arg, err)
		}
	}
}

func TestDayNumberAbove7AllowedHere(t *testing.T) {
	if d := dayAt(t, loadProgram(t, "day_above7.star"), 1, 9); d.Name != "Ninth" {
		t.Fatalf("got %+v", d)
	}
}

func TestProgressionBuiltins(t *testing.T) {
	ex := dayAt(t, loadProgram(t, "progress.star"), 1, 1).Exercises
	kg := func(v float64) model.Weight { return model.Weight{Value: v, Unit: model.UnitKg} }
	want := []model.Progression{
		{Kind: model.ProgressLP, Inc: kg(2.5), After: 1},
		{Kind: model.ProgressLP, Inc: kg(5), After: 2, FailReset: 0.9},
		{Kind: model.ProgressDP, Inc: kg(2.5), Lo: 6, Hi: 8},
		{Kind: model.ProgressSumReps, Inc: kg(5), Target: 25},
	}
	for i, w := range want {
		if got := *ex[i].Progress; got.Kind != w.Kind || got.Inc != w.Inc || got.After != w.After ||
			got.FailReset != w.FailReset || got.Lo != w.Lo || got.Hi != w.Hi || got.Target != w.Target {
			t.Errorf("%s progress = %+v, want %+v", ex[i].Names[0], got, w)
		}
	}
	c := ex[4]
	if c.Progress.Kind != model.ProgressCustom || c.Progress.Fn == nil || c.Progress.Fn.Name() != "bump" {
		t.Fatalf("custom = %+v", c.Progress)
	}
	if st := c.Progress.State; len(st) != 2 || st["attempt"] != (model.StateVar{Value: starlark.MakeInt(0)}) ||
		st["top"] != (model.StateVar{Value: starlark.MakeInt(0), Prompt: true}) {
		t.Errorf("state = %+v", st)
	}
	if c.Update == nil || c.Update.Fn.Name() != "bump" {
		t.Errorf("update = %+v", c.Update)
	}
}

func TestStructureErrors(t *testing.T) {
	const pre = "program(\"P\", weeks=2)\nday(1, 1, \"A\")\n"
	cases := map[string]string{
		"day(1, 1, \"A\")\n": "program() first",
		"program(\"P\", weeks=1)\nprogram(\"Q\", weeks=1)\n": "more than once",
		"x = 1\n": "program() was never called",
		pre + "week(2)\nex(\"Squat\", sets=[s(3, 5, kg(100))])\n":                                    "day()",
		pre + "day(3, 1, \"B\")\n":                                                                   "week 3 out of range 1-2",
		pre + "ex(\"Squat\", weeks=\"1-3\", sets=[s(1, 1, kg(1))])\n":                                "week 3 out of range 1-2",
		pre + "ex(\"Squat\", sets=[s(3, 5, kg(-5))])\n":                                              "weight must be a finite non-negative number",
		pre + "ex(\"Squat\", sets=[s(3, 5, kg(float(\"nan\")))])\n":                                  "weight must be a finite non-negative number",
		pre + "ex(\"Squat\", warmup=[w(1, 5, lb(1) * float(\"inf\"))], sets=[s(1, 1, kg(1))])\n":     "weight must be a finite non-negative number",
		pre + "ex(\"Squat\", sets=[s(3, (12, 8), kg(1))])\n":                                         "reps",
		pre + "ex(sets=[s(1, 1, kg(1))])\n":                                                          "name or alts required",
		pre + "ex(\"A\", alts=[\"B\"], current=3, sets=[s(1, 1, kg(1))])\n":                          "current 3 out of range 1-2",
		pre + "ex(\"Squat\")\n":                                                                      "sets or variations required",
		pre + "ex(\"Squat\", sets=[s(1, 1, kg(1))], variations=[[s(1, 1, kg(1))]])\n":                "not both",
		pre + "ex(\"Squat\", warmup=[w(1, 5)], sets=[s(1, 1, kg(1))])\n":                             "weight required",
		pre + "ex(\"Squat\", sets=[s(1, 1, kg(1))])\nex(\"Squat\", sets=[s(1, 1, kg(1))])\n":         "already on this day",
		pre + "ex(\"Squat\", sets=[s(1, 1, kg(1))], state={\"x\": 1})\n":                             "state needs progress",
		pre + "def f(ctx):\n    pass\nex(\"Squat\", sets=[s(1, 1, kg(1))], update=custom(f, x=1))\n": "update",
		pre + "ex(\"Squat\", sets=[s(1, 1, kg(1))], update=lp(kg(1)))\n":                             "update",
		"program(\"P\", weeks=1)\nday(1, 1, \"A\", kind=\"cardio\")\n":                               "kind",
		pre + "ex(\"Squat\", sets=[s(1, 1, kg(1), rpe=11)])\n":                                       "rpe",
		pre + "ex(\"Squat\", sets=[s(1000000000, 1, kg(1))])\n":                                      "n must be 1-1000",
		pre + "ex(\"Squat\", sets=[s(1, 1, kg(2000) + kg(1))])\n":                                    "weight too large",
		pre + "ex(\"Squat\", warmup=[w(1, 5, lb(4400) * 2)], sets=[s(1, 1, kg(1))])\n":               "weight too large",
	}
	for src, want := range cases {
		_, err := Evaluate(src, "test.star")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want error containing %q", src, err, want)
		}
	}
}

func TestExStateMergesIntoProgress(t *testing.T) {
	src := "program(\"P\", weeks=1)\ndef f(ctx):\n    pass\nday(1, 1, \"A\")\n" +
		"ex(\"Squat\", sets=[s(1, 1, kg(1))], progress=custom(f, a=1), state={\"b\": prompt(2)})\n"
	p, err := Evaluate(src, "test.star")
	if err != nil {
		t.Fatal(err)
	}
	st := p.Days[0].Exercises[0].Progress.State
	if st["a"] != (model.StateVar{Value: starlark.MakeInt(1)}) || st["b"] != (model.StateVar{Value: starlark.MakeInt(2), Prompt: true}) {
		t.Fatalf("state = %+v", st)
	}
}
