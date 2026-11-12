package engine

import (
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

func TestProgressionSummary(t *testing.T) {
	custom := evalSrc(t, squatDay+"def three_strikes(ctx):\n    pass\nex(\"Squat\", sets=[s(3, 5, kg(100))], progress=custom(three_strikes))\n").Days[0].Exercises[0].Progress
	cases := []struct {
		p    *model.Progression
		want string
	}{
		{nil, ""},
		{&model.Progression{Kind: model.ProgressLP, Inc: kgW(2.5), After: 1}, "Linear: +2.5 kg after a successful workout"},
		{&model.Progression{Kind: model.ProgressLP, Inc: kgW(2.5), After: 2, FailAfter: 3, FailReset: 0.9}, "Linear: +2.5 kg after 2 successes, -10% after 3 failures"},
		{&model.Progression{Kind: model.ProgressLP, Inc: model.Weight{Value: 5, Unit: model.UnitLb}, After: 1, FailAfter: 1, FailDec: model.Weight{Value: 10, Unit: model.UnitLb}}, "Linear: +5 lb after a successful workout, -10 lb after a failure"},
		{&model.Progression{Kind: model.ProgressLP, IncPct: 5, After: 1}, "Linear: +5% after a successful workout"},
		{&model.Progression{Kind: model.ProgressDP, Inc: kgW(2.5), Lo: 8, Hi: 12}, "Double progression 8-12 reps, then +2.5 kg"},
		{&model.Progression{Kind: model.ProgressSumReps, Inc: kgW(2.5), Target: 30}, "Sum of reps >= 30: +2.5 kg"},
		{custom, "Custom (three_strikes)"},
	}
	for _, c := range cases {
		if got := progressionSummary(c.p); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.p, got, c.want)
		}
	}
}

func TestResolvedExerciseCarriesProgression(t *testing.T) {
	e := resolveEx(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))`, newState(), 1, 1, 0)
	if e.Progression != "Linear: +2.5 kg after a successful workout" {
		t.Fatalf("progression %q", e.Progression)
	}
	if e := resolveEx(t, squatDay+`ex("Squat", warmup=[], sets=[s(3, 5, kg(100))])`, newState(), 1, 1, 0); e.Progression != "" {
		t.Fatalf("progression %q, want none", e.Progression)
	}
}
