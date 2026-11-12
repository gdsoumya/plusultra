package engine

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

var squat = model.ProgKey{Exercise: "Squat"}

// done lists completed sets of 100 kg with the given reps.
func done(reps ...int) []model.SetResult {
	out := make([]model.SetResult, len(reps))
	for i, r := range reps {
		out[i] = model.SetResult{Reps: r, Weight: kgW(100), Completed: true}
	}
	return out
}

func result(key model.ProgKey, sets ...model.SetResult) model.ExerciseResult {
	return model.ExerciseResult{Key: key, Week: 1, Day: 1, Sets: sets}
}

// progress runs a progression and merges its delta into st.
func progress(t *testing.T, p *model.Program, st *model.UserState, res model.ExerciseResult) (model.Overlay, []string) {
	t.Helper()
	d, out, err := RunProgress(p, st, res)
	if err != nil {
		t.Fatal(err)
	}
	st.Overlay.Merge(d)
	return d, out
}

// workSets are the working sets of the first exercise of week, day.
func workSets(t *testing.T, p *model.Program, st *model.UserState, week, day int) []model.ResolvedSet {
	t.Helper()
	d, err := ResolveDay(p, st, week, day)
	if err != nil {
		t.Fatal(err)
	}
	return slices.DeleteFunc(d.Exercises[0].Sets, func(s model.ResolvedSet) bool { return s.Warmup })
}

func customSquat(body, sets, state string) string {
	return squatDay + "def f(ctx):\n" + body + "\nex(\"Squat\", warmup=[], sets=" + sets + ", progress=custom(f" + state + "))\n"
}

func TestLpSuccessIncrements(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))`)
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.Weights[model.SetAddr{Key: squat}]; o.Abs != nil || o.Delta != kgW(2.5) {
		t.Fatalf("delta weight %+v, want +2.5 kg for all sets", o)
	}
	for week := 1; week <= 2; week++ {
		assertWeights(t, workSets(t, p, st, week, 1), 102.5, 102.5, 102.5)
	}
}

func TestLpFailureResets(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5), after=2, fail_after=2, fail_reset=0.9))`)
	st := newState()
	for _, reps := range [][]int{{5, 5, 5}, {5, 5, 3}, {5, 5, 5}, {5, 4, 5}} { // a failure resets the success count
		progress(t, p, st, result(squat, done(reps...)...))
		assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
	}
	progress(t, p, st, result(squat, done(5, 4, 5)...))
	assertWeights(t, workSets(t, p, st, 2, 1), 90, 90, 90)
}

func TestDpAddsRepsThenWeight(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 8, kg(50))], progress=dp(kg(5), 8, 12))`)
	st := newState()
	for want := 9; want <= 12; want++ {
		r := workSets(t, p, st, 1, 1)[0].Reps
		progress(t, p, st, result(squat, done(r, r, r)...))
		if s := workSets(t, p, st, 2, 1)[2]; s.Reps != want || s.Weight != kgW(50) {
			t.Fatalf("got %d x %v, want %d x 50 kg", s.Reps, s.Weight, want)
		}
	}
	progress(t, p, st, result(squat, done(12, 12, 11)...)) // a miss changes nothing
	progress(t, p, st, result(squat, done(12, 12, 12)...))
	if s := workSets(t, p, st, 2, 1)[2]; s.Reps != 8 || s.Weight != kgW(55) {
		t.Fatalf("got %d x %v, want 8 x 55 kg", s.Reps, s.Weight)
	}
}

func TestSumRepsTarget(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 10, kg(50))], progress=sum_reps(kg(5), 30))`)
	st := newState()
	progress(t, p, st, result(squat, append(done(10, 10), model.SetResult{Reps: 10})...)) // not completed: 20
	progress(t, p, st, result(squat, done(10, 10, 9)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 50, 50, 50)
	progress(t, p, st, result(squat, done(12, 10, 8)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 55, 55, 55)
}

func TestCustomWeightsBroadcast(t *testing.T) {
	p := evalSrc(t, customSquat("    ctx.weights += kg(5)", "[s(3, 5, kg(100))]", ""))
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.Weights[model.SetAddr{Key: squat}]; len(d.Weights) != 1 || o.Abs != nil || o.Delta != kgW(5) {
		t.Fatalf("delta %v, want one relative +5 kg for all sets", d.Weights)
	}
	assertWeights(t, workSets(t, p, st, 2, 1), 105, 105, 105)
}

func TestBroadcastReplacesPerSetWrites(t *testing.T) {
	body := `    if ctx.state.n == 0:
        ctx.weights += kg(5)
    else:
        ctx.weights = kg(120)
    ctx.state.n += 1`
	p := evalSrc(t, customSquat(body, "[s(1, 5, kg(100)), s(2, 5, kg(90))]", ", n=0"))
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 2, 1), 105, 95, 95)
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 2, 1), 120, 120, 120)
}

func TestCustomIndexed(t *testing.T) {
	p := evalSrc(t, customSquat("    ctx.weights[2] = kg(60)", "[s(3, 5, kg(100))]", ""))
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.Weights[model.SetAddr{Key: squat, Set: 2}]; len(d.Weights) != 1 || o.Abs == nil || *o.Abs != kgW(60) {
		t.Fatalf("delta %v, want set 2 = 60 kg", d.Weights)
	}
	assertWeights(t, workSets(t, p, st, 2, 1), 100, 60, 100)

	p = evalSrc(t, customSquat("    ctx.weights[0] = kg(60)", "[s(3, 5, kg(100))]", ""))
	_, _, err := RunProgress(p, newState(), result(squat, done(5, 5, 5)...))
	assertContains(t, err, "out of range 1-3")
}

func TestCustomAt(t *testing.T) {
	p := evalSrc(t, `program("P", weeks=2)
def f(ctx):
    ctx.at(week=2, day=3).weights[1] = kg(60)
    ctx.at(week=1, day=3).weights += kg(5)
    print(ctx.at(week=2, day=3).weights, ctx.at(week=1, day=3).weights, ctx.weights)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(f))
day(1, 3, "C", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(f))
`)
	st := newState()
	_, out := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if want := "[kg(60), kg(100), kg(100)] [kg(105), kg(105), kg(105)] [kg(100), kg(100), kg(100)]"; !slices.Equal(out, []string{want}) {
		t.Fatalf("printed %q, want %q", out, want)
	}
	assertWeights(t, workSets(t, p, st, 2, 3), 60, 100, 100)
	assertWeights(t, workSets(t, p, st, 1, 3), 105, 105, 105)
	assertWeights(t, workSets(t, p, st, 2, 1), 100, 100, 100)
	assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
}

func TestCompletedRepsGteReps(t *testing.T) {
	body := `    if ctx.completed_reps >= ctx.reps:
        ctx.weights += kg(5)`
	p := evalSrc(t, customSquat(body, "[s(3, 5, kg(100))]", ""))
	st := newState()
	progress(t, p, st, result(squat, done(5, 6, 4)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
	progress(t, p, st, result(squat, done(5, 6, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 105, 105, 105)
}

func TestStateAcrossRuns(t *testing.T) {
	body := `    ctx.state.attempt += 1
    ctx.state.hist.append(ctx.state.attempt)
    if ctx.state.attempt == 3:
        ctx.weights += kg(5)
        ctx.state.attempt = 0`
	p := evalSrc(t, customSquat(body, "[s(3, 5, kg(100))]", ", attempt=0, hist=[]"))
	st := newState()
	for range 2 {
		progress(t, p, st, result(squat, done(5, 5, 5)...))
	}
	assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
	if got := st.Overlay.State[squat]; got["attempt"] != starlark.MakeInt(2) || got["hist"].String() != "[1, 2]" {
		t.Fatalf("state %v, want attempt 2 and hist [1, 2]", got)
	}
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 105, 105, 105)
	if got := st.Overlay.State[squat]["attempt"]; got != starlark.MakeInt(0) {
		t.Fatalf("attempt %v, want 0", got)
	}
	if v := p.Days[0].Exercises[0].Progress.State["hist"].Value.String(); v != "[]" {
		t.Fatalf("program initial state changed to %s", v)
	}
}

func TestSharedTagState(t *testing.T) {
	p := evalSrc(t, squatDay+`def give(ctx):
    ctx.shared(1).top = ctx.completed_reps[1]
def take(ctx):
    print(getattr(ctx.shared(1), "top", 0))
def other(ctx):
    ctx.shared(2).top = 1
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], tags=[1], progress=custom(give))
ex("Bench", warmup=[], sets=[s(1, 5, kg(60))], tags=[1], progress=custom(take))
ex("Row", warmup=[], sets=[s(1, 5, kg(60))], tags=[1], progress=custom(other))
`)
	st := newState()
	bench := model.ProgKey{Exercise: "Bench"}
	if _, out := progress(t, p, st, result(bench, done(5)...)); !slices.Equal(out, []string{"0"}) {
		t.Fatalf("before: printed %q, want 0", out)
	}
	progress(t, p, st, result(squat, done(7)...))
	if _, out := progress(t, p, st, result(bench, done(5)...)); !slices.Equal(out, []string{"7"}) {
		t.Fatalf("after: printed %q, want 7", out)
	}
	if got := st.Overlay.State[model.ProgKey{Exercise: "tag:1"}]["top"]; got != starlark.MakeInt(7) {
		t.Fatalf("tag state %v, want 7", got)
	}
	progress(t, p, st, result(model.ProgKey{Exercise: "Row"}, done(5)...)) // any exercise reaches any tag
	if got := st.Overlay.State[model.ProgKey{Exercise: "tag:2"}]["top"]; got != starlark.MakeInt(1) {
		t.Fatalf("tag 2 state %v, want 1", got)
	}
}

func TestPromptValueInjected(t *testing.T) {
	p := evalSrc(t, customSquat("    ctx.weights += kg(ctx.state.add)", "[s(3, 5, kg(100))]", ", add=prompt(2)"))
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 102.5, 102.5, 102.5) // 102 rounds to the plates
	res := result(squat, done(5, 5, 5)...)
	res.PromptValues = map[string]starlark.Value{"add": starlark.MakeInt(5)}
	progress(t, p, st, res)
	assertWeights(t, workSets(t, p, st, 1, 1), 107.5, 107.5, 107.5)
}

func TestRm1Write(t *testing.T) {
	p := evalSrc(t, customSquat("    ctx.rm1 += kg(2.5)", "[s(3, 5, pct(80))]", ""))
	st := newState()
	st.RM1["Squat"] = kgW(100)
	assertWeights(t, workSets(t, p, st, 1, 1), 80, 80, 80)
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if d.RM1["Squat"] != kgW(102.5) {
		t.Fatalf("delta 1RM %v, want 102.5 kg", d.RM1)
	}
	assertWeights(t, workSets(t, p, st, 1, 1), 82.5, 82.5, 82.5)
}

func TestSetVariationSwitch(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    ctx.set_variation += 1
ex("Squat", warmup=[], variations=[[s(3, 5, kg(100))], [s(2, 3, kg(110))]], progress=custom(f))
`)
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if d.SetVariation[squat] != 2 {
		t.Fatalf("delta set variation %v, want 2", d.SetVariation)
	}
	assertWeights(t, workSets(t, p, st, 1, 1), 110, 110)
}

func TestExerciseVariationWraps(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    ctx.exercise_variation += 1
ex(name="Squat", alts=["Front Squat"], warmup=[], sets=[s(3, 5, kg(100))], progress=custom(f))
`)
	st := newState()
	for _, want := range []string{"Front Squat", "Squat"} {
		progress(t, p, st, result(squat, done(5, 5, 5)...))
		d, err := ResolveDay(p, st, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := d.Exercises[0].Name; got != want {
			t.Fatalf("exercise %s, want %s", got, want)
		}
	}
	if got := st.Overlay.ExerciseVariation[squat]; got != 1 {
		t.Fatalf("stored variation %d, want 1 after wrapping", got)
	}
}

func TestNumberOfSetsProgression(t *testing.T) {
	p := evalSrc(t, twoWeeks("def f(ctx):\n    ctx.add_sets(1)\n", "[s(3, 5, kg(100))]", "[s(5, 5, kg(100))]", "custom(f)"))
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.NumSets[model.DayAddr{Key: squat}]; o.Abs != nil || o.Delta != 1 {
		t.Fatalf("delta sets %+v, want +1", o)
	}
	if n1, n2 := len(workSets(t, p, st, 1, 1)), len(workSets(t, p, st, 2, 1)); n1 != 4 || n2 != 6 {
		t.Fatalf("sets %d and %d, want 4 and 6", n1, n2)
	}
	p = evalSrc(t, twoWeeks("def f(ctx):\n    ctx.at(week=2).add_sets(1)\n", "[s(3, 5, kg(100))]", "[s(5, 5, kg(100))]", "custom(f)"))
	st = newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	if n1, n2 := len(workSets(t, p, st, 1, 1)), len(workSets(t, p, st, 2, 1)); n1 != 3 || n2 != 6 {
		t.Fatalf("view: sets %d and %d, want 3 and 6", n1, n2)
	}
}

func TestOverlayWritesClamped(t *testing.T) {
	body := `    ctx.weights = kg(1500) * 3
    ctx.reps = -2
    ctx.number_of_sets = 100000
    ctx.rm1 = lb(4000) * 2`
	p := evalSrc(t, customSquat(body, "[s(3, 5, kg(100))]", ""))
	d, _, err := RunProgress(p, newState(), result(squat, done(5, 5, 5)...))
	if err != nil {
		t.Fatal(err)
	}
	all := model.SetAddr{Key: squat}
	if *d.Weights[all].Abs != kgW(2000) || *d.Reps[all].Abs != 0 || *d.NumSets[model.DayAddr{Key: squat}].Abs != maxListLen ||
		d.RM1["Squat"] != (model.Weight{Value: 4400, Unit: model.UnitLb}) {
		t.Fatalf("got %+v, want weights, reps, sets and 1RM clamped", d)
	}
	p = evalSrc(t, customSquat("    ctx.weights *= 1e9\n    ctx.reps += 5000", "[s(3, 5, kg(100))]", ""))
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	if s := workSets(t, p, st, 2, 1)[0]; s.Weight != kgW(2000) || s.Reps != maxListLen {
		t.Fatalf("got %d x %v, want relative writes clamped to 1000 x 2000 kg", s.Reps, s.Weight)
	}
	p = evalSrc(t, customSquat(`    ctx.weights = kg(float("nan"))`, "[s(3, 5, kg(100))]", ""))
	_, _, err = RunProgress(p, newState(), result(squat, done(5, 5, 5)...))
	assertContains(t, err, "finite")
}

func TestBuiltinFunctions(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    print(calc_1rm(kg(100), 5) / kg(1), training_max(kg(100), 5) / kg(1), rpe_multiplier(5, 8), rpe_multiplier(5, 7.75))
    print(round_weight(kg(101.4)), increment(kg(100)), decrement(kg(100)))
    print(is_exercise("squat"), is_exercise("Bench"), is_program_exercise("heavy: Squat"), is_program_exercise("Squat"))
ex("Squat", label="heavy", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(f))
`)
	_, out := progress(t, p, newState(), result(model.ProgKey{Exercise: "Squat", Label: "heavy"}, done(5, 5, 5)...))
	if len(out) != 3 {
		t.Fatalf("printed %q, want 3 lines", out)
	}
	nums := strings.Fields(out[0])
	for i, want := range []float64{100 * (1 + 5.0/30), 0.9 * 100 * (1 + 5.0/30), 0.811, 0.805} { // Epley; RTS chart, interpolated
		if got, err := strconv.ParseFloat(nums[i], 64); err != nil || math.Abs(got-want) > 1e-9 {
			t.Errorf("value %d = %s, want %g", i, nums[i], want)
		}
	}
	if want := []string{"kg(102.5) kg(102.5) kg(97.5)", "True False True False"}; !slices.Equal(out[1:], want) {
		t.Fatalf("printed %q, want %q", out[1:], want)
	}
	_, err := Evaluate(squatDay+"x = round_weight(kg(100))\n", "test.star")
	assertContains(t, err, "only inside a progression or update")
}

func TestPrintCappedByBytes(t *testing.T) {
	p := evalSrc(t, customSquat("    s = \"x\" * 40000\n    print(s)\n    print(s)\n    print(\"end\")", "[s(1, 5, kg(100))]", ""))
	if _, out := progress(t, p, newState(), result(squat, done(5)...)); len(out) != 1 {
		t.Fatalf("kept %d lines, want output to stop at 64 KiB", len(out))
	}
}

func TestPrintCaptured(t *testing.T) {
	p := evalSrc(t, customSquat(`    print("hello", ctx.exercise, ctx.completed_reps)`, "[s(2, 5, kg(100))]", ""))
	_, out := progress(t, p, newState(), result(squat, done(5, 4)...))
	if !slices.Equal(out, []string{"hello Squat [5, 4]"}) {
		t.Fatalf("printed %q", out)
	}
}

func TestUpdateHookAddsSets(t *testing.T) {
	p := evalSrc(t, squatDay+`def hook(ctx):
    if ctx.set_index == 1:
        ctx.sets(2, 4, 3, 5, False, kg(90), 120, 8, False)
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], update=custom(hook))
`)
	e, err := RunUpdate(p, newState(), result(squat, done(5)...), 1)
	if err != nil {
		t.Fatal(err)
	}
	assertWeights(t, e.Sets, 100, 90, 90, 90)
	if s := e.Sets[3]; s.MinReps != 3 || s.Reps != 5 || s.Rest != 120 || s.RPE != 8 {
		t.Fatalf("added set %+v, want 3-5 reps, rest 120, rpe 8", s)
	}
}

func TestUpdateCannotTouchCompleted(t *testing.T) {
	p := evalSrc(t, squatDay+`def hook(ctx):
    ctx.weights = kg(50)
    ctx.weights[1] = kg(10)
    ctx.reps[1] = 1
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], update=custom(hook))
`)
	pending := model.SetResult{Reps: 5, Weight: kgW(100)}
	st := newState()
	e, err := RunUpdate(p, st, result(squat, append(done(5), pending, pending)...), 1)
	if err != nil {
		t.Fatal(err)
	}
	assertWeights(t, e.Sets, 100, 50, 50)
	if e.Sets[0].Reps != 5 || e.Sets[1].Reps != 5 {
		t.Fatalf("reps %d, %d, want 5, 5", e.Sets[0].Reps, e.Sets[1].Reps)
	}
	if len(st.Overlay.Weights) != 0 {
		t.Fatalf("update wrote the overlay: %v", st.Overlay.Weights)
	}
}

func TestScriptErrorHasLine(t *testing.T) {
	p := evalSrc(t, customSquat("    x = 1\n    ctx.weights = \"heavy\"", "[s(3, 5, kg(100))]", ""))
	_, _, err := RunProgress(p, newState(), result(squat, done(5, 5, 5)...))
	ee, ok := errors.AsType[*EvalError](err)
	if !ok || ee.File != "test.star" || ee.Line != 5 || !strings.Contains(ee.Msg, "want kg or lb") {
		t.Fatalf("got %v, want test.star:5 weight error", err)
	}
}

func TestProgressWithoutPlannedSets(t *testing.T) {
	for _, prog := range []string{"lp(kg(2.5))", "dp(kg(5), 8, 12)"} {
		p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=`+prog+`)`)
		st := newState()
		st.Overlay.NumSets = map[model.DayAddr]model.IntOverride{{Key: squat}: absN(0)}
		if _, _, err := RunProgress(p, st, result(squat, done(5, 5, 5)...)); err != nil {
			t.Fatalf("%s: %v", prog, err)
		}
	}
}

// twoWeeks is a program where Squat on day 1 has sets w1 in week 1 and w2 in week 2.
func twoWeeks(defs, w1, w2, prog string) string {
	return "program(\"P\", weeks=2)\n" + defs +
		"day(1, 1, \"A\")\nex(\"Squat\", warmup=[], sets=" + w1 + ", progress=" + prog + ")\n" +
		"day(2, 1, \"A\")\nex(\"Squat\", warmup=[], sets=" + w2 + ", progress=" + prog + ")\n"
}

func TestLpKeepsPerWeekWeights(t *testing.T) {
	p := evalSrc(t, twoWeeks("", "[s(3, 5, kg(100))]", "[s(3, 5, kg(105))]", "lp(kg(2.5))"))
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 102.5, 102.5, 102.5)
	assertWeights(t, workSets(t, p, st, 2, 1), 107.5, 107.5, 107.5)
}

func TestLpKeepsHeavyLightDays(t *testing.T) {
	p := evalSrc(t, `program("P", weeks=1)
day(1, 1, "Heavy")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 3, "Light")
ex("Squat", warmup=[], sets=[s(3, 5, kg(80))], progress=lp(kg(2.5)))
`)
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 102.5, 102.5, 102.5)
	assertWeights(t, workSets(t, p, st, 1, 3), 82.5, 82.5, 82.5)
}

func TestLpFailResetScalesPerWeek(t *testing.T) {
	p := evalSrc(t, twoWeeks("", "[s(3, 5, kg(100))]", "[s(3, 5, kg(150))]", "lp(kg(2.5), fail_reset=0.9)"))
	st := newState()
	progress(t, p, st, result(squat, done(5, 4, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 90, 90, 90)
	assertWeights(t, workSets(t, p, st, 2, 1), 135, 135, 135)
}

func TestLpWithoutResultsSkips(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5), after=2))`)
	st := newState()
	for _, sets := range [][]model.SetResult{done(5, 5, 5), nil, done(5, 5, 5)} { // no results is not a failure
		progress(t, p, st, result(squat, sets...))
	}
	assertWeights(t, workSets(t, p, st, 1, 1), 102.5, 102.5, 102.5)
}

func TestCustomRelativeKeepsSetDifferences(t *testing.T) {
	p := evalSrc(t, twoWeeks("def f(ctx):\n    ctx.weights += kg(5)\n",
		"[s(1, 5, kg(100)), s(1, 5, kg(90))]", "[s(1, 5, kg(110)), s(1, 5, kg(95))]", "custom(f)"))
	st := newState()
	progress(t, p, st, result(squat, done(5, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 105, 95)
	assertWeights(t, workSets(t, p, st, 2, 1), 115, 100)
}

func TestCustomScaleIsRelative(t *testing.T) {
	p := evalSrc(t, twoWeeks("def f(ctx):\n    ctx.weights *= 1.1\n", "[s(1, 5, kg(100))]", "[s(1, 5, kg(150))]", "custom(f)"))
	st := newState()
	progress(t, p, st, result(squat, done(5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 110)
	assertWeights(t, workSets(t, p, st, 2, 1), 165)
}

func TestAbsoluteBroadcastFlattens(t *testing.T) {
	p := evalSrc(t, twoWeeks("def f(ctx):\n    ctx.weights = kg(120)\n", "[s(1, 5, kg(100))]", "[s(1, 5, kg(150))]", "custom(f)"))
	st := newState()
	progress(t, p, st, result(squat, done(5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 120)
	assertWeights(t, workSets(t, p, st, 2, 1), 120)
}

func TestDpRepsDeltaPerWeek(t *testing.T) {
	p := evalSrc(t, twoWeeks("", "[s(3, 8, kg(50))]", "[s(3, 10, kg(50))]", "dp(kg(5), 8, 12)"))
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(8, 8, 8)...))
	if o := d.Reps[model.SetAddr{Key: squat}]; o.Abs != nil || o.Delta != 1 {
		t.Fatalf("delta reps %+v, want +1", o)
	}
	if r1, r2 := workSets(t, p, st, 1, 1)[0].Reps, workSets(t, p, st, 2, 1)[0].Reps; r1 != 9 || r2 != 11 {
		t.Fatalf("reps %d and %d, want 9 and 11", r1, r2)
	}
}

func TestProgressPersistsSetFields(t *testing.T) {
	body := "    ctx.min_reps += 1\n    ctx.rpe = 8\n    ctx.rest += 30"
	p := evalSrc(t, customSquat(body, "[s(3, (5, 8), kg(100), rest=90, rpe=7)]", ""))
	st := newState()
	progress(t, p, st, result(squat, done(8, 8, 8)...))
	if s := workSets(t, p, st, 2, 1)[2]; s.MinReps != 6 || s.Reps != 8 || s.RPE != 8 || s.Rest != 120 {
		t.Fatalf("set %+v, want 6-8 reps at rpe 8, rest 120", s)
	}
}

func TestUpdateRejectsProgressFields(t *testing.T) {
	for _, body := range []string{"ctx.rm1 = kg(1)", "ctx.state.n = 1", "ctx.set_variation = 2", "ctx.at(week=1)"} {
		p := evalSrc(t, squatDay+"def hook(ctx):\n    "+body+"\n"+
			`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(hook, n=0), update=custom(hook))`)
		_, err := RunUpdate(p, newState(), result(squat, done(5)...), 1)
		assertContains(t, err, "only changeable in progress")
	}
}

func TestUpdateChangesPersistAcrossCalls(t *testing.T) {
	p := evalSrc(t, squatDay+`def hook(ctx):
    if ctx.set_index == 1:
        ctx.weights = kg(90)
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], update=custom(hook))
`)
	pending := func(w float64) model.SetResult { return model.SetResult{Reps: 5, Weight: kgW(w)} }
	e, err := RunUpdate(p, newState(), result(squat, done(5)[0], pending(100), pending(100)), 1)
	if err != nil {
		t.Fatal(err)
	}
	assertWeights(t, e.Sets, 100, 90, 90)
	second := model.SetResult{Reps: 5, Weight: kgW(90), Completed: true}
	if e, err = RunUpdate(p, newState(), result(squat, done(5)[0], second, pending(90)), 2); err != nil {
		t.Fatal(err)
	}
	assertWeights(t, e.Sets, 100, 100, 90) // the pending set keeps 90; a completed set shows its plan

}

func TestSpecificWriteKeepsBroaderOverride(t *testing.T) {
	same := "def f(ctx):\n    ctx.weights += kg(2.5)\n    ctx.at(week=2).weights += kg(5)\n"
	across := "def f(ctx):\n    if ctx.state.n == 0:\n        ctx.weights += kg(2.5)\n    else:\n        ctx.at(week=2).weights += kg(5)\n    ctx.state.n += 1\n"
	for name, defs := range map[string]string{"same run": same, "across runs": across} {
		p := evalSrc(t, twoWeeks(defs, "[s(1, 5, kg(100))]", "[s(1, 5, kg(100))]", "custom(f, n=0)"))
		st := newState()
		for range 2 {
			progress(t, p, st, result(squat, done(5)...))
			if name == "same run" {
				break
			}
		}
		if w1, w2 := workSets(t, p, st, 1, 1)[0].Weight, workSets(t, p, st, 2, 1)[0].Weight; w1 != kgW(102.5) || w2 != kgW(107.5) {
			t.Errorf("%s: weeks %v and %v, want 102.5 and 107.5 kg", name, w1, w2)
		}
	}
}

func TestViewArithmeticDoesNotWrite(t *testing.T) {
	body := "    x = ctx.at(week=2).weights + kg(5)\n    if ctx.at(week=2).weights * 1.05 > ctx.weights:\n        pass"
	p := evalSrc(t, twoWeeks("def f(ctx):\n"+body+"\n", "[s(1, 5, kg(100))]", "[s(1, 5, kg(100))]", "custom(f)"))
	d, _, err := RunProgress(p, newState(), result(squat, done(5)...))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Weights) != 0 {
		t.Fatalf("arithmetic wrote %v", d.Weights)
	}
}

func TestVectorCompareLengthsErrors(t *testing.T) {
	eq := customSquat("    ctx.number_of_sets = 2\n    print(ctx.completed_reps == ctx.reps, ctx.completed_reps != ctx.reps)", "[s(3, 5, kg(100))]", "")
	if _, out := progress(t, evalSrc(t, eq), newState(), result(squat, done(5, 5, 5)...)); !slices.Equal(out, []string{"False True"}) {
		t.Fatalf("printed %q, want False True", out)
	}
	ordered := customSquat("    ctx.number_of_sets = 2\n    x = ctx.completed_reps >= ctx.reps", "[s(3, 5, kg(100))]", "")
	_, _, err := RunProgress(evalSrc(t, ordered), newState(), result(squat, done(5, 5, 5)...))
	if err == nil {
		t.Fatal("ordered comparison of vectors of different lengths succeeded")
	}
	assertContains(t, err, "vector lengths differ")
}

func TestUpdateKeepsCompletedPlannedWeight(t *testing.T) {
	p := evalSrc(t, squatDay+`def hook(ctx):
    ctx.weights[2] = ctx.weights[1]
    ctx.weights[3] = ctx.completed_weights[1]
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], update=custom(hook))
`)
	doneAt95 := model.SetResult{Reps: 5, Weight: kgW(95), Completed: true}
	pending := model.SetResult{Reps: 5, Weight: kgW(100)}
	e, err := RunUpdate(p, newState(), result(squat, doneAt95, pending, pending), 1)
	if err != nil {
		t.Fatal(err)
	}
	assertWeights(t, e.Sets, 100, 100, 95) // ctx.weights[1] is planned, completed_weights[1] is done
}

func TestResultsRejectNonFinite(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], progress=lp(kg(2.5)), update=custom(lambda ctx: None))`)
	cases := map[string]model.SetResult{
		"finite":   {Reps: 5, Weight: kgW(math.NaN()), Completed: true},
		"finite ":  {Reps: 5, Weight: kgW(math.Inf(1))},
		"finite  ": {Reps: 5, Weight: kgW(100), RPE: math.NaN()},
		"negative": {Reps: -1, Weight: kgW(100)},
	}
	for want, r := range cases {
		_, _, err := RunProgress(p, newState(), result(squat, r))
		if err == nil {
			t.Fatalf("progress accepted %+v", r)
		}
		assertContains(t, err, strings.TrimSpace(want))
		if _, err = RunUpdate(p, newState(), result(squat, r), 0); err == nil {
			t.Fatalf("update accepted %+v", r)
		}
	}
}

func TestStaleRelativeBaseIsAbsolute(t *testing.T) {
	body := "    w = ctx.weights\n    ctx.weights = kg(100)\n    ctx.weights = w + kg(5)"
	p := evalSrc(t, customSquat(body, "[s(3, 5, kg(90))]", ""))
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 2, 1), 95, 95, 95) // w's values plus 5, not 100 plus 5
}

func TestPartialOverlapWrites(t *testing.T) {
	week2 := "[s(2, 5, kg(100))]"
	same := "def f(ctx):\n    ctx.at(week=2).weights += kg(5)\n    ctx.weights[1] = kg(60)\n"
	across := "def f(ctx):\n    if ctx.state.n == 0:\n        ctx.at(week=2).weights += kg(5)\n    else:\n        ctx.weights[1] = kg(60)\n    ctx.state.n += 1\n"
	for name, defs := range map[string]string{"same run": same, "across runs": across} {
		p := evalSrc(t, twoWeeks(defs, week2, week2, "custom(f, n=0)"))
		st := newState()
		for range 2 {
			progress(t, p, st, result(squat, done(5, 5)...))
			if name == "same run" {
				break
			}
		}
		if got := weights(workSets(t, p, st, 2, 1)); !slices.Equal(got, []float64{60, 105}) {
			t.Errorf("%s: week 2 %v, want set 1 at 60 over the week 2 +5", name, got)
		}
		if got := weights(workSets(t, p, st, 1, 1)); !slices.Equal(got, []float64{60, 100}) {
			t.Errorf("%s: week 1 %v, want [60 100]", name, got)
		}
	}

	p := evalSrc(t, `program("P", weeks=2)
def f(ctx):
    ctx.at(week=2).weights += kg(5)
    ctx.at(day=3).weights += kg(5)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], progress=custom(f))
day(1, 3, "C", weeks="1-2")
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], progress=custom(f))
`)
	st := newState()
	progress(t, p, st, result(squat, done(5)...))
	nested := `program("P", weeks=2)
def f(ctx):
    ctx.at(week=2, day=3).weights += kg(5)
    ctx.at(week=2).weights[1] = kg(60)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(2, 5, kg(100))], progress=custom(f))
day(1, 3, "C", weeks="1-2")
ex("Squat", warmup=[], sets=[s(2, 5, kg(100))], progress=custom(f))
`
	sn := newState()
	progress(t, evalSrc(t, nested), sn, result(squat, done(5, 5)...))
	if got := weights(workSets(t, evalSrc(t, nested), sn, 2, 3)); !slices.Equal(got, []float64{60, 105}) {
		t.Errorf("{W2,S1} over {W2,D3}: week 2 day 3 %v, want [60 105]", got)
	}
	for _, c := range []struct {
		week, day int
		want      float64
	}{{1, 1, 100}, {2, 1, 105}, {1, 3, 105}, {2, 3, 110}} {
		if got := workSets(t, p, st, c.week, c.day)[0].Weight.Value; got != c.want {
			t.Errorf("week %d day %d: %v, want %v", c.week, c.day, got, c.want)
		}
	}
}

func TestCompletedRepsLeftRead(t *testing.T) {
	body := `    if ctx.completed_reps_left >= ctx.reps:
        ctx.weights += kg(5)`
	p := evalSrc(t, customSquat(body, "[s(3, 5, kg(100))]", ""))
	st := newState()
	left := func(n int) []model.SetResult {
		sets := done(5, 5, 5)
		for i := range sets {
			sets[i].RepsLeft = n
		}
		return sets
	}
	progress(t, p, st, result(squat, left(4)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
	progress(t, p, st, result(squat, left(5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 105, 105, 105)
}
