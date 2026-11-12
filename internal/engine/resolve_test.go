package engine

import (
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

func evalSrc(t *testing.T, src string) *model.Program {
	t.Helper()
	p, err := Evaluate(src, "test.star")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newState() *model.UserState {
	return &model.UserState{Units: model.UnitKg, Bodyweight: kgW(80), RM1: map[string]model.Weight{}, DefaultRounding: plates20}
}

func resolveEx(t *testing.T, src string, st *model.UserState, week, day, ex int) model.ResolvedExercise {
	t.Helper()
	d, err := ResolveDay(evalSrc(t, src), st, week, day)
	if err != nil {
		t.Fatal(err)
	}
	return d.Exercises[ex]
}

func weights(sets []model.ResolvedSet) []float64 {
	var out []float64
	for _, s := range sets {
		out = append(out, s.Weight.Value)
	}
	return out
}

func assertWeights(t *testing.T, sets []model.ResolvedSet, want ...float64) {
	t.Helper()
	got := weights(sets)
	if len(got) != len(want) {
		t.Fatalf("weights %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] || sets[i].Weight.Unit != model.UnitKg {
			t.Fatalf("weights %v, want %v kg", sets, want)
		}
	}
}

const squatDay = `program("P", weeks=2)
day(1, 1, "Lower", weeks="1-2")
`

func TestPercentOf1RM(t *testing.T) {
	st := newState()
	st.RM1["Squat"] = kgW(100)
	e := resolveEx(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, pct(75))])`, st, 1, 1, 0)
	assertWeights(t, e.Sets, 75, 75, 75)
}

func TestPercentWithout1RMErrors(t *testing.T) {
	_, err := ResolveDay(evalSrc(t, squatDay+`ex("Squat", sets=[s(3, 5, pct(75))])`), newState(), 1, 1)
	if err == nil || !strings.Contains(err.Error(), "no 1RM for Squat") {
		t.Fatalf("got %v, want no 1RM error", err)
	}
}

func TestBodyweightLoad(t *testing.T) {
	st := newState()
	st.Bodyweight = kgW(81.3)
	e := resolveEx(t, squatDay+`ex("Pull Up", sets=[s(3, 5, bw(10))])`, st, 1, 1, 0)
	assertWeights(t, e.Sets, 91.3, 91.3, 91.3) // not rounded, and no default warmup
}

func TestAskSetWithRPE(t *testing.T) {
	e := resolveEx(t, squatDay+`ex("Squat", sets=[s(2, 5, rpe=8)])`, newState(), 1, 1, 0)
	assertWeights(t, e.Sets, 0, 0)
	if s := e.Sets[0]; !s.Ask || s.RPE != 8 {
		t.Fatalf("got %+v, want an ask set at RPE 8", s)
	}
}

func TestRoundingByEquipment(t *testing.T) {
	st := newState()
	st.Rounding = map[string]model.Rounding{"Dumbbell": {Kind: model.RoundStack, Increment: 2}}
	e := resolveEx(t, squatDay+`ex("Bench Press, Dumbbell", eq="Dumbbell", warmup=[], sets=[s(1, 8, kg(12.4))])`, st, 1, 1, 0)
	assertWeights(t, e.Sets, 12)
	st.Rounding["Bench Press, Dumbbell"] = model.Rounding{Kind: model.RoundStack, Increment: 5}
	e = resolveEx(t, squatDay+`ex("Bench Press, Dumbbell", eq="Dumbbell", warmup=[], sets=[s(1, 8, kg(12.4))])`, st, 1, 1, 0)
	assertWeights(t, e.Sets, 10) // the exercise name wins over equipment
}

func TestWarmupPercentOfFirstWorkingSet(t *testing.T) {
	st := newState()
	st.RM1["Squat"] = kgW(100)
	e := resolveEx(t, squatDay+`ex("Squat", warmup=[w(1, 8, pct(50))], sets=[s(3, 5, pct(75))])`, st, 1, 1, 0)
	assertWeights(t, e.Sets, 37.5, 75, 75, 75)
	if !e.Sets[0].Warmup || e.Sets[0].Reps != 8 || e.Sets[1].Warmup {
		t.Fatalf("sets %+v, want one warmup of 8 first", e.Sets)
	}
}

func TestDefaultWarmupInjected(t *testing.T) {
	e := resolveEx(t, squatDay+`ex("Squat", sets=[s(1, 5, kg(77.5))])`, newState(), 1, 1, 0)
	assertWeights(t, e.Sets, 37.5, 55, 65, 77.5) // 38.75, 54.25 and 65.875 rounded to plates
	for i, reps := range []int{8, 5, 3} {
		if s := e.Sets[i]; !s.Warmup || s.Reps != reps {
			t.Fatalf("set %d = %+v, want warmup x%d", i, s, reps)
		}
	}
	if got := DefaultWarmup(model.ResolvedSet{Weight: kgW(20)}); len(got) != 3 {
		t.Fatalf("DefaultWarmup gave %d sets, want 3 (skipping is the resolver's call)", len(got))
	}
	e = resolveEx(t, squatDay+`ex("Squat", sets=[s(1, 5, kg(20))])`, newState(), 1, 1, 0)
	assertWeights(t, e.Sets, 20) // at bar weight: no warmup
}

func TestOverlayWeightWins(t *testing.T) {
	st := newState()
	key := model.ProgKey{Exercise: "Squat"}
	st.Overlay.Weights = map[model.SetAddr]model.WeightOverride{{Key: key}: absW(kgW(110))}
	p := evalSrc(t, squatDay+`ex("Squat", warmup=[], sets=[s(2, 5, kg(100))])`)
	days, err := Resolve(p, st)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2", len(days))
	}
	for _, d := range days {
		assertWeights(t, d.Exercises[0].Sets, 110, 110)
	}
}

func TestOverlaySpecificAddressWins(t *testing.T) {
	st := newState()
	key := model.ProgKey{Exercise: "Squat", Label: "heavy"}
	st.Overlay.Weights = map[model.SetAddr]model.WeightOverride{
		{Key: key}:                          absW(kgW(110)),
		{Key: key, Week: 2}:                 absW(kgW(120)),
		{Key: key, Week: 2, Day: 1, Set: 2}: absW(kgW(130)),
	}
	st.Overlay.Reps = map[model.SetAddr]model.IntOverride{{Key: key, Set: 1}: absN(3)}
	src := squatDay + `ex("Squat", label="heavy", warmup=[], sets=[s(2, 5, kg(100))])`
	assertWeights(t, resolveEx(t, src, st, 1, 1, 0).Sets, 110, 110)
	e := resolveEx(t, src, st, 2, 1, 0)
	assertWeights(t, e.Sets, 120, 130)
	if e.Sets[0].Reps != 3 || e.Sets[0].MinReps != 3 || e.Sets[1].Reps != 5 {
		t.Fatalf("sets %+v, want reps override on set 1 only", e.Sets)
	}
}

func absW(w model.Weight) model.WeightOverride { return model.WeightOverride{Abs: &w} }
func absN(n int) model.IntOverride             { return model.IntOverride{Abs: &n} }

func TestOverlayRelativeOverride(t *testing.T) {
	st := newState()
	key := model.ProgKey{Exercise: "Squat"}
	st.Overlay.Weights = map[model.SetAddr]model.WeightOverride{{Key: key}: {Delta: kgW(5), Scale: 1.1}}
	st.Overlay.Reps = map[model.SetAddr]model.IntOverride{{Key: key}: {Delta: 2}}
	src := squatDay + `ex("Squat", warmup=[], sets=[s(2, 5, kg(100))])
day(2, 1, "Lower")
ex("Squat", warmup=[], sets=[s(2, 5, kg(150))])
`
	assertWeights(t, resolveEx(t, src, st, 1, 1, 0).Sets, 115, 115) // 100 x 1.1 + 5
	e := resolveEx(t, src, st, 2, 1, 0)
	assertWeights(t, e.Sets, 170, 170)
	if e.Sets[0].Reps != 7 {
		t.Fatalf("reps %d, want 5 + 2", e.Sets[0].Reps)
	}
}

func TestNumSetsTruncatesAndExtends(t *testing.T) {
	st := newState()
	key := model.ProgKey{Exercise: "Squat"}
	st.Overlay.NumSets = map[model.DayAddr]model.IntOverride{{Key: key}: absN(4), {Key: key, Week: 2}: absN(1)}
	src := squatDay + `ex("Squat", warmup=[], sets=[s(1, 5, kg(100)), s(1, 8, kg(80))])`
	e := resolveEx(t, src, st, 1, 1, 0)
	assertWeights(t, e.Sets, 100, 80, 80, 80)
	if e.Sets[3].Reps != 8 {
		t.Fatalf("extended set %+v, want a copy of the last set", e.Sets[3])
	}
	assertWeights(t, resolveEx(t, src, st, 2, 1, 0).Sets, 100)
	for _, n := range []int{maxListLen + 1, 1_000_000_000} {
		st.Overlay.NumSets[model.DayAddr{Key: key, Week: 2}] = absN(n)
		if got := len(resolveEx(t, src, st, 2, 1, 0).Sets); got != maxListLen {
			t.Fatalf("%d sets: got %d, want the cap %d", n, got, maxListLen)
		}
	}
}

func TestConvertsToUserUnits(t *testing.T) {
	st := newState()
	st.Units = model.UnitLb
	st.DefaultRounding = model.Rounding{Kind: model.RoundPlates, Bar: 45, Plates: []float64{45, 25, 10, 5, 2.5}}
	e := resolveEx(t, squatDay+`ex("Squat", warmup=[], sets=[s(1, 5, kg(100))])`, st, 1, 1, 0)
	if got := e.Sets[0].Weight; got != (model.Weight{Value: 220, Unit: model.UnitLb}) {
		t.Fatalf("got %v, want 220 lb", got) // 100 kg = 220.46 lb
	}
}

func TestAlternateCurrentFromOverlay(t *testing.T) {
	st := newState()
	st.Overlay.ExerciseVariation = map[model.ProgKey]int{{Exercise: "Bench Press"}: 1}
	d, err := ResolveDay(loadProgram(t, "alts.star"), st, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	e := d.Exercises[0]
	if e.Current != 1 || e.Name != "Bench Press" || len(e.Alternates) != 2 {
		t.Fatalf("got %+v, want current 1 of 2 alternates", e)
	}
	if e := d.Exercises[1]; e.Current != 1 || e.Name != "Pull Up" {
		t.Fatalf("got %+v, want script current", e)
	}
}

func TestDescriptionInherited(t *testing.T) {
	p := loadProgram(t, "desc.star")
	st := newState()
	for week, want := range map[int]string{1: "Brace hard", 2: "Brace hard", 3: "", 4: ""} {
		d, err := ResolveDay(p, st, week, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := d.Exercises[0].Desc; got != want {
			t.Fatalf("week %d desc %q, want %q", week, got, want)
		}
	}
	st.Overlay.DescIndex = map[model.ProgKey]int{{Exercise: "Bench Press"}: 2}
	d, err := ResolveDay(p, st, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Exercises[0].Desc; got != "Touch and go" {
		t.Fatalf("desc %q, want the second description", got)
	}
}

func TestRestDayWhenUndeclared(t *testing.T) {
	d, err := ResolveDay(evalSrc(t, squatDay+`ex("Squat", sets=[s(3, 5, kg(100))])`), newState(), 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "rest" || len(d.Exercises) != 0 || d.Week != 1 || d.Day != 3 {
		t.Fatalf("got %+v, want an empty rest day", d)
	}
}

func TestResolveDayOutOfRange(t *testing.T) {
	p := evalSrc(t, squatDay+`ex("Squat", sets=[s(3, 5, kg(100))])`)
	for _, wd := range [][2]int{{0, 1}, {3, 1}, {1, 0}, {1, 8}} {
		if _, err := ResolveDay(p, newState(), wd[0], wd[1]); err == nil || !strings.Contains(err.Error(), "outside the program") {
			t.Errorf("week %d day %d: got %v, want an out of range error", wd[0], wd[1], err)
		}
	}
	d, err := ResolveDay(loadProgram(t, "day_above7.star"), newState(), 1, 9)
	if err != nil || d.Name != "Ninth" {
		t.Fatalf("got %+v, %v; want day 9 of a floating program", d, err)
	}
}

func TestUnilateralFollowsAlternate(t *testing.T) {
	st := newState()
	st.Unilateral = map[string]bool{"Bench Press, Dumbbell": true}
	p := loadProgram(t, "alts.star")
	d, err := ResolveDay(p, st, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Exercises[0].Unilateral || d.Exercises[1].Unilateral {
		t.Fatalf("got %+v, want the dumbbell alternate unilateral", d.Exercises)
	}
	st.Overlay.ExerciseVariation = map[model.ProgKey]int{{Exercise: "Bench Press"}: 1}
	if d, err = ResolveDay(p, st, 1, 1); err != nil || d.Exercises[0].Unilateral {
		t.Fatalf("barbell alternate %+v, %v, want not unilateral", d.Exercises[0], err)
	}
}

// Without exercise, equipment or default rounding, weights round to a 2.5 kg
// or 5 lb stack rather than staying unrounded.
func TestDefaultRoundingWithoutEquipment(t *testing.T) {
	st := newState()
	st.DefaultRounding = model.Rounding{}
	st.RM1["Squat"] = kgW(80)
	e := resolveEx(t, squatDay+`ex("Squat", warmup=[], sets=[s(1, 5, pct(62))])`, st, 1, 1, 0)
	assertWeights(t, e.Sets, 50)

	st.Units = model.UnitLb
	e = resolveEx(t, squatDay+`ex("Squat", warmup=[], sets=[s(1, 5, pct(62))])`, st, 1, 1, 0)
	if w := e.Sets[0].Weight; w.Unit != model.UnitLb || w.Value != 110 {
		t.Fatalf("lb weight = %v, want 110 lb", w)
	}
}
