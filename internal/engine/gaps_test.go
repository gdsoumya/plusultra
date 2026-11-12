package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

// printed runs the progression of squat for done sets and returns its print output.
func printed(t *testing.T, p *model.Program, st *model.UserState, key model.ProgKey, sets ...model.SetResult) []string {
	t.Helper()
	_, out := progress(t, p, st, result(key, sets...))
	return out
}

func TestProgressPersistsTimersAndFlags(t *testing.T) {
	body := "    ctx.work += 15\n    ctx.amrap[3] = True\n    ctx.ask = True\n    ctx.log_rpe = 1\n    ctx.at(week=2).work = 90"
	p := evalSrc(t, customSquat(body, "[s(3, 5, kg(100), work=30)]", ""))
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.Work[model.SetAddr{Key: squat}]; o.Abs != nil || o.Delta != 15 {
		t.Errorf("delta work %+v, want +15", o)
	}
	if o := d.Amrap[model.SetAddr{Key: squat, Set: 3}]; o.Abs == nil || !*o.Abs {
		t.Errorf("delta amrap %+v, want set 3 on", o)
	}
	for week, work := range map[int]int{1: 45, 2: 90} {
		for i, s := range workSets(t, p, st, week, 1) {
			if s.Work != work || s.Amrap != (i == 2) || !s.Ask || !s.LogRPE {
				t.Errorf("week %d set %d: %+v, want work %d, amrap on set 3, ask and log_rpe", week, i+1, s, work)
			}
		}
	}
}

func TestSetCountPerVariation(t *testing.T) {
	src := squatDay + `def f(ctx):
    ctx.at(variation=2).add_sets(1)
    print(ctx.at(variation=2).number_of_sets, ctx.number_of_sets)
ex("Squat", warmup=[], variations=[[s(3, 5, kg(100))], [s(2, 3, kg(110))]], progress=custom(f))
`
	p := evalSrc(t, src)
	st := newState()
	d, out := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.NumSets[model.DayAddr{Key: squat, Variation: 2}]; o.Abs != nil || o.Delta != 1 {
		t.Errorf("delta sets %+v, want +1 at variation 2", d.NumSets)
	}
	if !slices.Equal(out, []string{"3 3"}) {
		t.Errorf("printed %q, want variation 2 at 3 sets and this workout at 3", out)
	}
	if n := len(workSets(t, p, st, 2, 1)); n != 3 {
		t.Errorf("variation 1 has %d sets, want 3", n)
	}
	st.Overlay.SetVariation = map[model.ProgKey]int{squat: 2}
	if n := len(workSets(t, p, st, 2, 1)); n != 3 {
		t.Errorf("variation 2 has %d sets, want 3", n)
	}
	p = evalSrc(t, squatDay+"def f(ctx):\n    ctx.at(variation=2).number_of_sets = 5\n"+
		`ex("Squat", warmup=[], variations=[[s(3, 5, kg(100))], [s(2, 3, kg(110))]], set_variation=2, progress=custom(f))`)
	st = newState()
	progress(t, p, st, result(squat, done(3, 3)...))
	if n := len(workSets(t, p, st, 1, 1)); n != 5 {
		t.Errorf("absolute: variation 2 has %d sets, want 5", n)
	}
}

func TestRPEInfersWeight(t *testing.T) {
	src := squatDay + `ex("Squat", warmup=[], sets=[s(1, 5, rpe=8), s(1, 5, rpe=8, ask=True)])`
	st := newState()
	e := resolveEx(t, src, st, 1, 1, 0)
	assertWeights(t, e.Sets, 0, 0) // no 1RM: the lifter enters it
	if !e.Sets[0].Ask || !e.Sets[1].Ask {
		t.Errorf("sets %+v, want ask sets without a 1RM", e.Sets)
	}
	st.RM1["Squat"] = kgW(140)
	e = resolveEx(t, src, st, 1, 1, 0)
	assertWeights(t, e.Sets, 112.5, 112.5) // 0.811 x 140 = 113.5
	if e.Sets[0].Ask || !e.Sets[1].Ask {
		t.Errorf("sets %+v, want ask only where asked", e.Sets)
	}
	rpe := 10.0
	st.Overlay.RPE = map[model.SetAddr]model.FloatOverride{{Key: squat}: {Abs: &rpe}}
	assertWeights(t, resolveEx(t, src, st, 1, 1, 0).Sets, 120, 120) // 0.863 x 140 = 120.8
}

func TestStartingSetVariation(t *testing.T) {
	src := squatDay + `def f(ctx):
    print(ctx.set_variation)
    ctx.set_variation += 1
ex("Squat", warmup=[], variations=[[s(3, 5, kg(100))], [s(2, 3, kg(110))]], set_variation=2, progress=custom(f))
`
	p := evalSrc(t, src)
	st := newState()
	assertWeights(t, workSets(t, p, st, 1, 1), 110, 110)
	if out := printed(t, p, st, squat, done(3, 3)...); !slices.Equal(out, []string{"2"}) {
		t.Errorf("printed %q, want 2", out)
	}
	assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
	for _, bad := range []string{"variations=[[s(1, 5, kg(1))]], set_variation=2", "sets=[s(1, 5, kg(1))], set_variation=1"} {
		_, err := Evaluate(squatDay+"ex(\"Squat\", "+bad+")\n", "test.star")
		assertContains(t, err, "set_variation")
	}
}

func TestLpFailDecrement(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5), fail_after=2, fail_dec=kg(5)))`)
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 4)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 100, 100, 100)
	d, _ := progress(t, p, st, result(squat, done(5, 4, 4)...))
	if o := d.Weights[model.SetAddr{Key: squat}]; o.Abs != nil || o.Delta != kgW(-5) {
		t.Errorf("delta weight %+v, want -5 kg", o)
	}
	assertWeights(t, workSets(t, p, st, 2, 1), 95, 95, 95)
	progress(t, p, st, result(squat, done(5, 5, 5)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 97.5, 97.5, 97.5)
}

func TestLpFailAfterResets(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5), after=3, fail_after=1, fail_reset=0.9))`)
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 4)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 90, 90, 90)
}

func TestLpPercentIncrement(t *testing.T) {
	p := evalSrc(t, twoWeeks("", "[s(3, 5, kg(100))]", "[s(3, 5, kg(80))]", "lp(pct(5))"))
	st := newState()
	d, _ := progress(t, p, st, result(squat, done(5, 5, 5)...))
	if o := d.Weights[model.SetAddr{Key: squat}]; o.Abs != nil || o.Scale != 1.05 {
		t.Errorf("delta weight %+v, want x1.05", o)
	}
	assertWeights(t, workSets(t, p, st, 1, 1), 105, 105, 105)
	assertWeights(t, workSets(t, p, st, 2, 1), 85, 85, 85) // 84 rounds to the plates
}

func TestLpArgErrors(t *testing.T) {
	for bad, want := range map[string]string{
		"lp(kg(1), fail_after=-1, fail_dec=kg(1))":           "fail_after",
		"lp(kg(1), fail_after=0, fail_dec=kg(1))":            "fail_after",
		"lp(kg(1), fail_dec=kg(1), fail_reset=0.9)":          "fail_dec or fail_reset",
		"lp(kg(1), fail_after=2)":                            "fail_after",
		"lp(pct(-5))":                                        "finite non-negative",
		"lp(kg(1), fail_dec=kg(-1))":                         "finite non-negative",
		"dp(pct(5), 8, 12)":                                  "want kg or lb",
		"lp(kg(1), fail_after=2, fail_dec=pct(5))":           "want kg or lb",
		"lp(kg(1), fail_after=2, fail_reset=1.5)":            "fail_reset",
		"lp(kg(1), after=2, fail_reset=0.9, fail_dec=kg(1))": "fail_dec or fail_reset",
	} {
		_, err := Evaluate(squatDay+"ex(\"Squat\", sets=[s(1, 5, kg(1))], progress="+bad+")\n", "test.star")
		assertContains(t, err, want)
	}
}

func TestProgramNumberOfSets(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    print(ctx.number_of_sets, ctx.program_number_of_sets)
ex("Squat", warmup=[], variations=[[s(3, 5, kg(100))], [s(1, 3, kg(110)), s(1, 2, kg(120))]], set_variation=2, progress=custom(f))
`)
	st := newState()
	st.Overlay.NumSets = map[model.DayAddr]model.IntOverride{{Key: squat}: absN(4)}
	if out := printed(t, p, st, squat, done(3, 3, 3, 3)...); !slices.Equal(out, []string{"4 4"}) {
		t.Errorf("printed %q, want 4 sets done of the 4 the program plans", out)
	}
}

func TestMathFunctions(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    print(floor(3.7), ceil(3.2), round(2.5), round(-2.5), floor(4), round(0.49999999999999994))
    print(floor(kg(101.6)), ceil(lb(101.2)), round(kg(101.5)))
    print(sum(ctx.completed_reps), sum(ctx.completed_weights), sum([]), sum([1.5, 2]))
    print(zero_or_gte(ctx.completed_reps, ctx.reps), zero_or_gte([0, 5], [3, 5]), zero_or_gte([2, 5], [3, 5]), zero_or_gte(ctx.completed_reps, 6))
ex("Squat", warmup=[], sets=[s(3, 5, kg(floor(100.9)))], progress=custom(f))
`)
	want := []string{"3 4 3 -2 4 0", "kg(101) lb(102) kg(102)", "15 kg(300) 0 3.5", "True True False False"}
	if out := printed(t, p, newState(), squat, done(5, 5, 5)...); !slices.Equal(out, want) {
		t.Errorf("printed %q, want %q", out, want)
	}
	for _, bad := range []string{"floor(\"x\")", "sum(1)", "zero_or_gte([1], [1, 2])", "round(float(\"nan\"))"} {
		_, err := Evaluate(squatDay+"x = "+bad+"\n", "test.star")
		if err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

const inheritSrc = `program("P", weeks=4)
def hook(ctx):
    ctx.weights = kg(1)
day(1, 1, "A")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)), update=custom(hook))
day(2, 1, "A")
ex("Squat", warmup=[], sets=[s(3, 3, kg(110))])
day(3, 1, "A")
ex("Squat", warmup=[], sets=[s(3, 5, kg(90))], progress="none", update="none")
day(4, 1, "A")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))])
`

func TestProgressInheritsAcrossWeeks(t *testing.T) {
	p := evalSrc(t, inheritSrc)
	for week, want := range map[int]bool{1: true, 2: true, 3: false, 4: true} {
		e := dayAt(t, p, week, 1).Exercises[0]
		if (e.Progress != nil) != want || (e.Update != nil) != want {
			t.Errorf("week %d: progress %v update %v, want %v", week, e.Progress, e.Update, want)
		}
		if want && e.Progress.Kind != model.ProgressLP {
			t.Errorf("week %d: progress %+v, want the week 1 lp", week, e.Progress)
		}
	}
	st := newState()
	progress(t, p, st, model.ExerciseResult{Key: squat, Week: 2, Day: 1, Sets: done(3, 3, 3)})
	assertWeights(t, workSets(t, p, st, 2, 1), 112.5, 112.5, 112.5)
	d, _ := progress(t, p, st, model.ExerciseResult{Key: squat, Week: 3, Day: 1, Sets: done(5, 5, 5)})
	if len(d.Weights) != 0 {
		t.Errorf("progress=\"none\" wrote %v", d.Weights)
	}
	for _, bad := range []string{`progress="lp"`, `update="no"`, `update=lp(kg(1))`} {
		_, err := Evaluate(squatDay+"ex(\"Squat\", sets=[s(1, 5, kg(1))], "+bad+")\n", "test.star")
		assertContains(t, err, "none")
	}
}

func TestDayDescInherits(t *testing.T) {
	p := evalSrc(t, `program("P", weeks=4)
day(1, 1, "A", desc="Legs")
ex("Squat", sets=[s(3, 5, kg(100))])
day(2, 1, "A")
ex("Squat", sets=[s(3, 5, kg(100))])
day(3, 1, "A", desc="")
ex("Squat", sets=[s(3, 5, kg(100))])
day(4, 1, "A", desc="Test day")
`)
	for week, want := range map[int]string{1: "Legs", 2: "Legs", 3: "", 4: "Test day"} {
		d, err := ResolveDay(p, newState(), week, 1)
		if err != nil {
			t.Fatal(err)
		}
		if d.Desc != want {
			t.Errorf("week %d: desc %q, want %q", week, d.Desc, want)
		}
	}
}

func TestRPEMultiplierPrintsClean(t *testing.T) {
	p := evalSrc(t, customSquat("    print(rpe_multiplier(5, 8), rpe_multiplier(5, 7.75))", "[s(1, 5, kg(100))]", ""))
	if out := printed(t, p, newState(), squat, done(5)...); !slices.Equal(out, []string{"0.811 0.805"}) {
		t.Errorf("printed %q, want 4-decimal multipliers", out)
	}
}

func TestSharedFromUntagged(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    ctx.shared(3).top = 9
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))], progress=custom(f))
`)
	st := newState()
	progress(t, p, st, result(squat, done(5)...))
	if got := st.Overlay.State[model.ProgKey{Exercise: "tag:3"}]["top"].String(); got != "9" {
		t.Errorf("tag state %s, want 9", got)
	}
}

func TestProgramExerciseAndOriginalWeights(t *testing.T) {
	p := evalSrc(t, squatDay+`def f(ctx):
    print(ctx.exercise, "|", ctx.program_exercise, "|", ctx.weights[1], ctx.original_weights[1])
ex("Squat", label="heavy", alts=["Front Squat"], current=2, warmup=[], sets=[s(1, 5, pct(73))], progress=custom(f))
ex("Squat", warmup=[], sets=[s(1, 5, pct(73))], progress=custom(f))
`)
	st := newState()
	st.RM1["Front Squat"], st.RM1["Squat"] = kgW(140), kgW(140)
	heavy := model.ProgKey{Exercise: "Squat", Label: "heavy"}
	if out := printed(t, p, st, heavy, done(5)...); !slices.Equal(out, []string{"Front Squat | heavy: Squat | kg(102.5) kg(102.2)"}) {
		t.Errorf("printed %q", out)
	}
	if out := printed(t, p, st, squat, done(5)...); !slices.Equal(out, []string{"Squat | Squat | kg(102.5) kg(102.2)"}) {
		t.Errorf("printed %q", out)
	}
}

func TestEquipmentFromName(t *testing.T) {
	st := newState()
	st.Rounding = map[string]model.Rounding{
		"dumbbell": {Kind: model.RoundFixed, Plates: []float64{10, 12.5, 15}},
		"cable":    {Kind: model.RoundStack, Increment: 3},
	}
	src := squatDay + `def f(ctx):
    print(round_weight(kg(11)))
ex("Bench Press, Dumbbell", warmup=[], sets=[s(1, 8, kg(12.4))], progress=custom(f))
ex("Curl, Dumbbell", eq="cable", warmup=[], sets=[s(1, 8, kg(12.4))])
ex("Pushdown", eq="CABLE", warmup=[], sets=[s(1, 8, kg(12.4))])
ex("Row", alts=["Row, Dumbbell"], current=2, warmup=[], sets=[s(1, 8, kg(14))])
ex("Lateral Raise", alts=["Lateral Raise, Cable"], current=2, eq="dumbbell", warmup=[], sets=[s(1, 8, kg(11))])
ex("Fly", alts=["Pec Deck"], current=2, eq="dumbbell", warmup=[], sets=[s(1, 8, kg(11))])
`
	p := evalSrc(t, src)
	d, err := ResolveDay(p, st, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if bench := d.Exercises[0]; bench.Alternates[0] != "Bench Press, Dumbbell" {
		t.Errorf("bench named %q, want the full name", bench.Alternates[0])
	}
	for i, want := range []struct {
		eq string
		kg float64
	}{
		{"Dumbbell", 12.5}, // the suffix finds the "dumbbell" rounding
		{"Dumbbell", 12.5}, // the name's suffix wins over eq=
		{"CABLE", 12},      // eq= matches "cable" in any case
		{"Dumbbell", 15},   // an alternate's own suffix
		{"Cable", 12},      // an alternate's suffix wins over eq=
		{"dumbbell", 10},   // an alternate without one falls back to eq=
	} {
		e := d.Exercises[i]
		if e.Equipment != want.eq {
			t.Errorf("%s on %q, want %q", e.Name, e.Equipment, want.eq)
		}
		assertWeights(t, e.Sets, want.kg)
	}
	key := model.ProgKey{Exercise: "Bench Press, Dumbbell"}
	if out := printed(t, p, st, key, done(8)...); !slices.Equal(out, []string{"kg(10)"}) {
		t.Errorf("round_weight printed %q, want the dumbbell rounding", out)
	}
}

const programWideSrc = `program("P", weeks=2)
def f(ctx):
    pass
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)), update=custom(f))
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))], progress=lp(kg(2.5)))
day(1, 3, "C", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(80))])
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(60))], progress="none", update="none")
day(2, 3, "C")
ex("Squat", warmup=[], sets=[s(3, 5, kg(85))])
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(65))])
`

func TestProgressInheritsProgramWide(t *testing.T) {
	p := evalSrc(t, programWideSrc)
	for _, c := range []struct {
		week, day, ex int
		want          bool
	}{
		{1, 3, 0, true}, {2, 3, 0, true}, // Squat: day 1's lp and hook reach day 3
		{1, 3, 1, false}, {1, 1, 1, true}, {2, 1, 1, true}, {2, 3, 1, true}, // Bench: "none" is day 3 of week 1 only
	} {
		e := dayAt(t, p, c.week, c.day).Exercises[c.ex]
		if (e.Progress != nil) != c.want {
			t.Errorf("week %d day %d %s: progress %v, want %v", c.week, c.day, e.Names[0], e.Progress, c.want)
		}
	}
	if e := dayAt(t, p, 2, 3).Exercises[0]; e.Update == nil {
		t.Errorf("week 2 day 3 Squat has no update hook, want day 1's")
	}
	st := newState()
	progress(t, p, st, model.ExerciseResult{Key: squat, Week: 1, Day: 3, Sets: done(5, 5, 5)})
	assertWeights(t, workSets(t, p, st, 1, 1), 102.5, 102.5, 102.5)
}

func TestProgressNotInheritedBackwards(t *testing.T) {
	p := evalSrc(t, `program("P", weeks=2)
day(1, 3, "C")
ex("Squat", sets=[s(3, 5, kg(80))])
day(2, 1, "A")
ex("Squat", sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
`)
	if e := dayAt(t, p, 1, 3).Exercises[0]; e.Progress != nil {
		t.Errorf("week 1 day 3 took a progression first declared in week 2: %+v", e.Progress)
	}
}

func TestProgressDeclaredTwiceErrors(t *testing.T) {
	for src, want := range map[string]string{
		`ex("Squat", sets=[s(1, 5, kg(1))], progress=lp(kg(2.5)))
day(1, 3, "C")
ex("Squat", sets=[s(1, 5, kg(1))], progress=lp(kg(5)))`: "progression for Squat declared more than once; declare it once (or use a label)",
		`def f(ctx):
    return
def g(ctx):
    return
ex("Squat", label="t1", sets=[s(1, 5, kg(1))], update=custom(f))
day(1, 3, "C")
ex("Squat", label="t1", sets=[s(1, 5, kg(1))], update=custom(g))`: "update for t1: Squat declared more than once",
	} {
		_, err := Evaluate("program(\"P\", weeks=1)\nday(1, 1, \"A\")\n"+src+"\n", "test.star")
		ee, ok := errors.AsType[*EvalError](err)
		if !ok || !strings.Contains(ee.Msg, want) || ee.Line != strings.Count(src, "\n")+3 {
			t.Errorf("got %v, want %q at the second declaration", err, want)
		}
	}
	_, err := Evaluate(`program("P", weeks=1)
T = template(sets=[s(1, 5, kg(1))], progress=lp(kg(2.5)))
day(1, 1, "A")
ex("Squat", **T)
day(1, 3, "C")
ex("Squat", **T)
day(1, 5, "E")
ex("Squat", sets=[s(1, 5, kg(1))], progress=lp(kg(2.5)))
`, "test.star")
	if err != nil {
		t.Errorf("the same progression twice: %v", err)
	}
}

func TestInheritedProgressTakesState(t *testing.T) {
	p := evalSrc(t, `program("P", weeks=1)
def f(ctx):
    pass
day(1, 1, "A")
ex("Squat", sets=[s(1, 5, kg(1))], progress=custom(f, a=1))
day(1, 3, "C")
ex("Squat", sets=[s(1, 5, kg(1))], state={"b": prompt(2)})
`)
	if st := dayAt(t, p, 1, 3).Exercises[0].Progress.State; len(st) != 2 || !st["b"].Prompt {
		t.Errorf("day 3 state %v, want a and the prompted b", st)
	}
	if st := dayAt(t, p, 1, 1).Exercises[0].Progress.State; len(st) != 1 {
		t.Errorf("day 1 state %v, want a only", st)
	}
}

func TestLpFailAfterDefaultsToOne(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5), after=3, fail_reset=0.9))`)
	st := newState()
	progress(t, p, st, result(squat, done(5, 5, 4)...))
	assertWeights(t, workSets(t, p, st, 1, 1), 90, 90, 90)
}
