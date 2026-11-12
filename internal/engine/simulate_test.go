package engine

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

func simulate(t *testing.T, src string, st *model.UserState, opts SimOptions) *SimResult {
	t.Helper()
	res, err := Simulate(t.Context(), evalSrc(t, src), st, opts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSimulateCompletesThePlanByDefault(t *testing.T) {
	src := squatDay + `ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
def f(ctx):
    print(ctx.week)
ex("Curl", warmup=[], sets=[s(1, 5, kg(10))], progress=custom(f))`
	res := simulate(t, src, nil, SimOptions{Weeks: 3})
	if len(res.Days) != 3 || res.Days[2].Week != 1 { // two program weeks, looped once
		t.Fatalf("days %+v", res.Days)
	}
	for i, want := range []float64{100, 102.5, 105} {
		assertWeights(t, res.Days[i].Exercises[0].Sets, want, want, want)
	}
	if !slices.Equal(res.Output, []string{"1", "2", "1"}) {
		t.Fatalf("output %q", res.Output)
	}

	res = simulate(t, src, nil, SimOptions{Weeks: 2, Reps: map[string]int{"Squat": 3}})
	assertWeights(t, res.Days[1].Exercises[0].Sets, 100, 100, 100) // missed reps fail lp
}

func TestSimulateScenarioShorthands(t *testing.T) {
	src := squatDay + `ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
def lighter(ctx):
    if ctx.completed_weights[1] < ctx.weights[1]:
        ctx.weights -= kg(5)
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))], progress=custom(lighter))
def by_feel(ctx):
    if ctx.state.rating >= 4:
        ctx.weights += kg(2.5)
ex("Curl", warmup=[], sets=[s(1, 5, kg(10))], progress=custom(by_feel, rating=prompt(3)))`
	res := simulate(t, src, nil, SimOptions{Weeks: 2, Scenario: []Logged{
		{Week: 1, Day: 1, Exercise: "squat", MissLastSetBy: 2},
		{Week: 1, Day: 1, Exercise: "Bench Press", WeightDelta: &model.Weight{Value: -10, Unit: model.UnitKg}},
		{Week: 1, Day: 1, Exercise: "Curl", Prompts: map[string]starlark.Value{"rating": starlark.MakeInt(5)}},
	}})
	w2 := res.Days[1].Exercises
	assertWeights(t, w2[0].Sets, 100, 100, 100) // 5, 5, 3 fails lp
	assertWeights(t, w2[1].Sets, 75, 75, 75)    // lifted 70 against a plan of 80
	assertWeights(t, w2[2].Sets, 12.5)

	res = simulate(t, src, nil, SimOptions{Weeks: 2, Scenario: []Logged{
		{Week: 1, Day: 1, Exercise: "Squat", Skip: true},
		{Week: 1, Day: 1, Exercise: "Bench Press", Weight: &model.Weight{Value: 80, Unit: model.UnitKg}},
		{Week: 1, Day: 1, Exercise: "Curl", Sets: []model.SetResult{{Reps: 5, Weight: kgW(10)}}}, // not completed
	}})
	w2 = res.Days[1].Exercises
	assertWeights(t, w2[0].Sets, 100, 100, 100)
	assertWeights(t, w2[1].Sets, 80, 80, 80)
	assertWeights(t, w2[2].Sets, 10)
}

func TestSimulateStartsAtAPosition(t *testing.T) {
	src := `program("P", weeks=2)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 2, "B", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(1, 5, kg(80))])`
	res := simulate(t, src, nil, SimOptions{Weeks: 2, FromWeek: 2, FromDay: 2})
	if len(res.Days) != 3 || res.Days[0].Week != 2 || res.Days[0].Day != 2 || res.Days[1].Week != 1 || res.Days[2].Day != 2 {
		t.Fatalf("days %+v", res.Days)
	}
	if _, err := Simulate(t.Context(), evalSrc(t, src), nil, SimOptions{Weeks: 1, FromWeek: 3}); err == nil {
		t.Fatal("week 3 of a 2-week program ran")
	}
}

func TestSimulateTrace(t *testing.T) {
	src := squatDay + `ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
def f(ctx):
    ctx.weights = kg(50)
    ctx.at(week=2).reps += 1
    ctx.rm1 = kg(120)
    ctx.state.n += 1
    print("hi")
ex("Bench Press", warmup=[], sets=[s(2, 5, kg(40))], progress=custom(f, n=1))
ex("Curl", warmup=[], sets=[s(1, 5, kg(10))])`
	res := simulate(t, src, nil, SimOptions{Weeks: 1, Trace: true})
	if len(res.Trace) != 2 { // Curl has no progression
		t.Fatalf("trace %+v", res.Trace)
	}
	lp := res.Trace[0]
	if lp.Week != 1 || lp.Day != 1 || lp.Exercise != "Squat" || lp.Function != "lp" || len(lp.StateBefore) != 0 ||
		lp.StateAfter["successes"] != "0" || lp.StateAfter["failures"] != "0" {
		t.Fatalf("lp trace %+v", lp)
	}
	if want := []Write{{"weights Squat", "relative", "+2.5 kg"}}; !slices.Equal(lp.Writes, want) {
		t.Fatalf("lp writes %+v, want %+v", lp.Writes, want)
	}
	f := res.Trace[1]
	want := []Write{{"reps Bench Press[w2]", "relative", "+1"}, {"rm1 Bench Press", "absolute", "120 kg"}, {"weights Bench Press", "absolute", "50 kg"}}
	if f.Function != "f" || f.StateBefore["n"] != "1" || f.StateAfter["n"] != "2" || !slices.Equal(f.Writes, want) || !slices.Equal(f.Prints, []string{"hi"}) {
		t.Fatalf("custom trace %+v, want writes %+v", f, want)
	}
}

func codes(ws []Warning, code string) []Warning {
	return slices.DeleteFunc(slices.Clone(ws), func(w Warning) bool { return w.Code != code })
}

func TestSimulateWarnings(t *testing.T) {
	st := newState()
	st.Rounding = map[string]model.Rounding{"Squat": {Kind: model.RoundStack, Increment: 5}}
	src := `program("P", weeks=3)
day(1, 1, "A", weeks="1-3")
ex("Squat", warmup=[], sets=[s(3, 5, kg(77.5))], progress=lp(kg(2)))
ex("Bench Press", warmup=[], sets=[s(1, 5, kg(60))], progress=lp(kg(2.5)))
day(1, 3, "B", weeks="1-3")
ex("Bench Press", warmup=[], sets=[s(1, 5, kg(60))])`
	res := simulate(t, src, st, SimOptions{Weeks: 3})
	moved := codes(res.Warnings, "rounding_moved") // 77.5, 79.5 and 81.5 all round to 80
	if len(moved) != 3 || moved[0].Exercise != "Squat" || moved[0].Week != 1 || !strings.Contains(moved[0].Message, "77.5 kg rounds to 80 kg") {
		t.Errorf("rounding_moved %+v", moved)
	}
	src2 := codes(res.Warnings, "rounding_source")
	if len(src2) != 2 || !strings.Contains(src2[0].Message, `exercise "Squat"`) || !strings.Contains(src2[0].Message, "steps of 5 kg") ||
		!strings.Contains(src2[1].Message, "default rounding") {
		t.Errorf("rounding_source %+v", src2)
	}
	if stalled := codes(res.Warnings, "stalled_progression"); len(stalled) != 1 || stalled[0].Exercise != "Squat" || !strings.Contains(stalled[0].Message, "80 kg") {
		t.Errorf("stalled_progression %+v", stalled)
	}
	if twice := codes(res.Warnings, "double_progression_in_week"); len(twice) != 1 || twice[0].Exercise != "Bench Press" || !strings.Contains(twice[0].Message, "days 1 and 3") {
		t.Errorf("double_progression_in_week %+v", twice)
	}
	if ws := codes(res.Warnings, "pct_without_rm1"); len(ws) != 0 {
		t.Errorf("pct_without_rm1 %+v", ws)
	}

	res, err := Simulate(t.Context(), evalSrc(t, squatDay+`ex("Squat", sets=[s(1, 5, pct(80))])`), newState(), SimOptions{Weeks: 1})
	ws := codes(res.Warnings, "pct_without_rm1")
	if err == nil || len(ws) != 1 || ws[0].Exercise != "Squat" {
		t.Fatalf("pct without 1RM: err %v, warnings %+v", err, ws)
	}
}

func TestSimulateUsesUserStateWithoutChangingIt(t *testing.T) {
	st := newState()
	st.RM1["Squat"] = kgW(200)
	src := squatDay + `ex("Squat", warmup=[], sets=[s(1, 5, pct(50))], progress=lp(kg(5)))`
	res := simulate(t, src, st, SimOptions{Weeks: 2})
	assertWeights(t, res.Days[1].Exercises[0].Sets, 105)
	if len(st.Overlay.Weights) != 0 {
		t.Fatalf("simulate changed the caller's overlay: %+v", st.Overlay.Weights)
	}
}

func TestSimulateStopsWithoutLoop(t *testing.T) {
	src := `program("P", weeks=1, loop=False)
day(1, 1, "A")
ex("Squat", sets=[s(1, 5, kg(100))])`
	res := simulate(t, src, nil, SimOptions{Weeks: 4})
	if len(res.Days) != 1 {
		t.Fatalf("days %d", len(res.Days))
	}
}

func TestSimulateCapsOutput(t *testing.T) {
	src := `program("P", weeks=1)
day(1, 1, "A")
def f(ctx):
    print("x" * 60000)
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], progress=custom(f))
ex("Bench Press", warmup=[], sets=[s(1, 5, kg(60))], progress=custom(f))`
	res := simulate(t, src, nil, SimOptions{Weeks: maxWeeks})
	size := 0
	for _, l := range res.Output {
		size += len(l)
	}
	out := res.Output
	if len(res.Days) != maxWeeks || size > maxSimulateOutput || out[len(out)-1] != "output truncated" || len(out) != 5 {
		t.Fatalf("%d days, %d lines of %d bytes, last %.20q", len(res.Days), len(out), size, out[len(out)-1])
	}
}

func TestSimulateStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	src := squatDay + `ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))`
	_, err := Simulate(ctx, evalSrc(t, src), nil, SimOptions{Weeks: 2})
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "week 1") {
		t.Fatalf("err = %v, want a cancelled simulation", err)
	}
}
