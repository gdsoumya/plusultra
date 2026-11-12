package programs_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func (e env) simulate(in programs.SimulateInput) programs.SimulateOutput {
	e.t.Helper()
	out, err := e.s.Simulate(e.t.Context(), e.user, in)
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

func TestSimulateUsesUserData(t *testing.T) {
	e := setup(t)
	slug, err := catalog.ResolveName(t.Context(), e.s.Q, e.user, "Squat")
	if err != nil {
		t.Fatal(err)
	}
	rm1, step := 150.0, model.Rounding{Kind: model.RoundStack, Increment: 10}
	if err := catalog.SetExerciseData(t.Context(), e.s.Q, e.user, catalog.ExerciseData{Slug: slug, RM1: &rm1, Rounding: &step, Unilateral: true}); err != nil {
		t.Fatal(err)
	}
	src := `program("P", weeks=2)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(1, 5, pct(50))], progress=lp(kg(10)))
`
	days := e.simulate(programs.SimulateInput{Script: src, Weeks: 2}).Days
	if w := days[0].Workout.Exercises[0].Sets[0].Weight.Value; w != 80 { // 75 rounded to the 10 kg stack
		t.Fatalf("week 1 squat %v, want 80", w)
	}
	if !days[0].Workout.Exercises[0].Unilateral {
		t.Fatalf("squat %+v, want unilateral from the user's data", days[0].Workout.Exercises[0])
	}
	if w := days[1].Workout.Exercises[0].Sets[0].Weight.Value; w != 90 {
		t.Fatalf("week 2 squat %v, want 90", w)
	}
	// Assumed reps are keyed by catalog slug: three of five reps fail lp.
	days = e.simulate(programs.SimulateInput{Script: src, Weeks: 2, Reps: map[string]int{slug: 3}}).Days
	if w := days[1].Workout.Exercises[0].Sets[0].Weight.Value; w != 80 {
		t.Fatalf("week 2 squat after missed reps %v, want 80", w)
	}
	out, err := e.s.Simulate(t.Context(), e.user, programs.SimulateInput{Script: `program("P", weeks=1)` + "\nday(1, 1, \"A\")\nex(\"Bench Press\", sets=[s(1, 5, pct(50))])\n", Weeks: 1})
	assertCode(t, err, connect.CodeInvalidArgument) // no 1RM for Bench Press
	if len(out.Warnings) != 1 || out.Warnings[0].Code != "pct_without_rm1" {
		t.Fatalf("warnings with the error %+v", out.Warnings)
	}
	// Overrides stand in for the missing data without saving anything.
	days = e.simulate(programs.SimulateInput{Script: src, Weeks: 1, Overrides: programs.Overrides{
		RM1: map[string]model.Weight{"Squat": {Value: 200}}, Rounding: map[string]model.Rounding{"Squat": {Kind: model.RoundStack, Increment: 2.5}}}}).Days
	if w := days[0].Workout.Exercises[0].Sets[0].Weight.Value; w != 100 {
		t.Fatalf("squat with overrides %v, want 100", w)
	}
}

func TestStoredWeightsAreKg(t *testing.T) {
	e := setup(t)
	u, err := e.s.Q.CreateUser(t.Context(), sqlc.CreateUserParams{Username: "lee", PasswordHash: "x", Units: "lb", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	ex, err := catalog.CreateCustomExercise(t.Context(), e.s.Q, u.ID, catalog.Custom{Name: "Squat"})
	if err != nil {
		t.Fatal(err)
	}
	rm1 := 100.0
	if err := catalog.SetExerciseData(t.Context(), e.s.Q, u.ID, catalog.ExerciseData{Slug: ex.Slug, RM1: &rm1}); err != nil {
		t.Fatal(err)
	}
	var bw pgtype.Numeric
	if err := bw.Scan("80"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Q.UpsertCheckin(t.Context(), sqlc.UpsertCheckinParams{ID: uuid.New(), UserID: u.ID, Date: today,
		Weight: bw, Measurements: []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	src := `program("P", weeks=1)
day(1, 1, "A")
ex("Squat", warmup=[], sets=[s(1, 5, pct(50)), s(1, 5, bw())])
`
	out, err := e.s.Simulate(t.Context(), u.ID, programs.SimulateInput{Script: src, Weeks: 1})
	if err != nil {
		t.Fatal(err)
	}
	// The engine works in the user's unit; the stored values are kilograms
	// either way. 50 kg is 110.2 lb, rounded to the 5 lb default stack.
	sets := out.Days[0].Workout.Exercises[0].Sets
	if w := sets[0].Weight; w.Unit != model.UnitLb || w.Value != 110 {
		t.Errorf("50%% of a stored 100 kg 1RM = %+v, want 110 lb", w)
	}
	if w := sets[1].Weight; w.Unit != model.UnitLb || math.Abs(w.Kg()-80) > 1e-9 {
		t.Errorf("bodyweight from a stored 80 kg check-in = %+v, want 80 kg in lb", w)
	}
}

const anchored = `program("P", weeks=2)
day(1, 1, "Lower", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100)), s(2, 5, kg(90))], progress=lp(kg(2.5)))
day(1, 3, "Upper", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))])
`

func TestSimulateCompactDatesAndFilter(t *testing.T) {
	e := setup(t)
	monday := date(2026, 11, 9)
	out := e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2, AnchorStart: &monday})
	want := []string{
		"W1D1 Mon 2026-11-09 Lower | Squat 100x5 x3, 90x5 x2",
		"W1D3 Wed 2026-11-11 Upper | Bench Press 80x5 x3",
		"W2D1 Mon 2026-11-16 Lower | Squat 102.5x5 x3, 92.5x5 x2",
		"W2D3 Wed 2026-11-18 Upper | Bench Press 80x5 x3",
	}
	if !slices.Equal(out.Compact, want) {
		t.Fatalf("compact %q, want %q", out.Compact, want)
	}
	if d := out.Days[2].Date; d == nil || !d.Equal(date(2026, 11, 16)) {
		t.Fatalf("week 2 date %v", d)
	}

	wed := date(2026, 11, 11)
	out = e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2, AnchorStart: &monday, FromDate: &wed, Trace: true})
	want[2] = "W2D1 Mon 2026-11-16 Lower | Squat 100x5 x3, 90x5 x2" // week 1's squat never ran
	if !slices.Equal(out.Compact, want[1:]) || len(out.Trace) != 1 || out.Trace[0].Week != 2 || out.Trace[0].Function != "lp" {
		t.Fatalf("from wednesday: %q, trace %+v", out.Compact, out.Trace)
	}

	out = e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2, AnchorStart: &monday,
		Filter: programs.Filter{Exercises: []string{"bench press"}, Weeks: []int{2}}})
	if !slices.Equal(out.Compact, want[3:]) || len(out.Days) != 1 {
		t.Fatalf("filtered: %q, %d days", out.Compact, len(out.Days))
	}
	// A looping program keeps dating sessions past its last week.
	out = e.simulate(programs.SimulateInput{Script: anchored, Weeks: 3, AnchorStart: &monday, From: programs.Position{Week: 2, Day: 3}})
	if len(out.Compact) != 5 || !strings.HasPrefix(out.Compact[0], "W2D3 Wed 2026-11-18") || !strings.HasPrefix(out.Compact[1], "W1D1 Mon 2026-11-23") ||
		!strings.HasPrefix(out.Compact[4], "W2D3 Wed 2026-12-02") {
		t.Fatalf("looped: %q", out.Compact)
	}
}

func TestSimulateAssertionsAndDiff(t *testing.T) {
	e := setup(t)
	out := e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2,
		Expect: []programs.Expectation{
			{Week: 2, Day: 1, Exercise: "Squat", Weight: &model.Weight{Value: 102.5}, Reps: 5},
			{Week: 2, Day: 1, Exercise: "squat", Set: 4, Weight: &model.Weight{Value: 95}},
			{Week: 2, Day: 2, Exercise: "Squat", Reps: 5},
			{Week: 1, Day: 1, Exercise: "Squat", Set: 6},
		},
		CompareScript: strings.Replace(anchored, "kg(80)", "kg(85)", 1),
	})
	a := out.Assertions
	if len(a) != 4 || !a[0].OK || a[0].Actual != "102.5 kg x5" || a[1].OK || a[1].Message != "weight 92.5 kg, want 95 kg" ||
		a[2].OK || a[2].Actual != "" || !strings.Contains(a[2].Message, "no session") || a[3].OK || !strings.Contains(a[3].Message, "has 5 sets") {
		t.Fatalf("assertions %+v", a)
	}
	want := []programs.DiffCell{
		{Week: 1, Day: 3, Exercise: "Bench Press", Set: 1, Field: "weight", A: "80 kg", B: "85 kg"},
		{Week: 1, Day: 3, Exercise: "Bench Press", Set: 2, Field: "weight", A: "80 kg", B: "85 kg"},
		{Week: 1, Day: 3, Exercise: "Bench Press", Set: 3, Field: "weight", A: "80 kg", B: "85 kg"},
	}
	if len(out.Diff) != 6 || !slices.Equal(out.Diff[:3], want) || out.Diff[3].Week != 2 {
		t.Fatalf("diff %+v", out.Diff)
	}

	out = e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2,
		CompareScenario: []engine.Logged{{Week: 1, Day: 1, Exercise: "Squat", MissLastSetBy: 1}}})
	if len(out.Diff) != 5 || out.Diff[0] != (programs.DiffCell{Week: 2, Day: 1, Exercise: "Squat", Set: 1, Field: "weight", A: "102.5 kg", B: "100 kg"}) {
		t.Fatalf("scenario diff %+v", out.Diff)
	}
	if out = e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2, CompareScript: anchored}); len(out.Diff) != 0 {
		t.Fatalf("diff of the same script %+v", out.Diff)
	}
}

func TestSimulateMealMapping(t *testing.T) {
	e := setup(t)
	monday := date(2026, 11, 9)
	meals := `meal_program("Eat", weeks=1)
oats = food("Oats", per100g={"kcal": 380})
day(1, kind="training", name="Train", target=target(kcal=2500))
meal("Breakfast", oats(100))
day(1, kind="rest", name="Rest")
meal("Breakfast", oats(50))
`
	out := e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2, AnchorStart: &monday, MealScript: meals})
	if len(out.Meals) != 10 || !out.Meals[0].Date.Equal(monday) || out.Meals[0].Name != "Train" || out.Meals[0].Kind != "training" ||
		out.Meals[0].Target.Kcal != 2500 || out.Meals[0].Totals.Kcal != 380 || out.Meals[1].Name != "Rest" || out.Meals[1].Kind != "rest" ||
		out.Meals[1].Target.Kcal != 190 || out.Meals[6].Missing || out.Meals[7].Name != "" || !out.Meals[7].Missing || out.Meals[7].Kind != "training" || !out.Meals[9].Missing {
		t.Fatalf("meals %+v", out.Meals)
	}
	_, err := e.s.Simulate(t.Context(), e.user, programs.SimulateInput{Script: anchored, Weeks: 1, MealScript: meals})
	assertCode(t, err, connect.CodeInvalidArgument) // no dates to map

	p := e.create(programs.KindMeal, meals)
	nov16 := date(2026, 11, 16)
	e.activate(p.ID, &nov16)
	out = e.simulate(programs.SimulateInput{Script: anchored, Weeks: 2, AnchorStart: &monday, MealProgramID: p.ID})
	if len(out.Meals) != 10 || !out.Meals[0].Missing || out.Meals[7].Missing || out.Meals[7].Name != "Train" {
		t.Fatalf("meals of a program starting a week later %+v", out.Meals)
	}
}
