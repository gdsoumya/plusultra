package programs_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// eightWeeks has days 1 and 2 in every week and nothing on day 3.
const eightWeeks = `program("Base", weeks=8)
day(1, 1, "Lower", weeks="1-8")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 2, "Upper", weeks="1-8")
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(60))])
`

const twoWeeks = `program("Short", weeks=2)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))])
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(60))], label="heavy", tags=[1])
day(1, 2, "B", weeks="1-2")
ex("Deadlift", warmup=[], sets=[s(1, 5, kg(140))])
`

func TestCreateInvalidScriptReturnsLineErrors(t *testing.T) {
	e := setup(t)
	_, err := e.s.Create(t.Context(), e.user, programs.KindWorkout, "program(\"x\", weeks=1)\nday(1, 1, \"A\")\nex(\"Squat\", sets=[])\n")
	assertCode(t, err, connect.CodeInvalidArgument)
	if ee, ok := errors.AsType[*engine.EvalError](err); !ok || ee.Line != 3 {
		t.Fatalf("err = %v, want an eval error on line 3", err)
	}

	v, err := e.s.Validate(t.Context(), e.user, programs.KindWorkout, "program(\"x\", weeks=1)\nday(1, 1, \"A\")\nex(\"Sqat\", sets=[s(1, 5, kg(1))])\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Errors) != 0 || len(v.NameErrors) != 1 || v.NameErrors[0].Line != 3 || !slices.Contains(v.NameErrors[0].Suggestions, "Squat") {
		t.Fatalf("validation %+v", v)
	}
	if v.Structure == nil || v.Structure.Weeks != 1 || len(v.Structure.Days) != 1 {
		t.Fatalf("structure %+v", v.Structure)
	}
	_, err = e.s.Create(t.Context(), e.user, programs.KindWorkout, "program(\"x\", weeks=1)\nday(1, 1, \"A\")\nex(\"Sqat\", sets=[s(1, 5, kg(1))])\n")
	assertCode(t, err, connect.CodeInvalidArgument)

	v, _ = e.s.Validate(t.Context(), e.user, programs.KindWorkout, "program(\"x\",\n")
	if len(v.Errors) != 1 || v.Errors[0].Line == 0 || v.Structure != nil {
		t.Fatalf("syntax error validation %+v", v)
	}

	p := e.create(programs.KindWorkout, eightWeeks)
	if p.Status != programs.StatusDraft || p.Revision != 1 || p.Name != "Base" || p.Kind != programs.KindWorkout {
		t.Fatalf("program %+v", p)
	}
	revs, err := e.s.ListRevisions(t.Context(), e.user, p.ID, 0, "")
	if err != nil || len(revs.Items) != 1 || revs.Items[0].Script != eightWeeks || revs.Items[0].CreatedBy != "user" {
		t.Fatalf("revisions %+v, %v", revs, err)
	}
}

func TestActivateAnchoredTuesdayStart(t *testing.T) {
	e := setup(t)
	p := e.create(programs.KindWorkout, eightWeeks)
	start := date(2026, 11, 10)
	if start.Weekday() != time.Tuesday {
		t.Fatal("fixture date is not a Tuesday")
	}
	p = e.activate(p.ID, &start)
	if p.Status != programs.StatusActive || !p.StartDate.Equal(start) || !p.EndDate.Equal(date(2027, 1, 3)) {
		t.Fatalf("program %+v start %v end %v", p, p.StartDate, p.EndDate)
	}
	days := e.days(p.ID, programs.Range{})
	if len(days) != 8*7-1 {
		t.Fatalf("%d days, want every date from the start to the end", len(days))
	}
	first, second := days[0], days[1]
	if first.Week != 1 || first.Day != 2 || !first.Date.Equal(start) || first.Kind != "training" ||
		first.Workout.Exercises[0].Name != "Bench Press" {
		t.Fatalf("first day %+v", first)
	}
	if second.Day != 3 || !second.Date.Equal(today) || second.Kind != "rest" || len(second.Workout.Exercises) != 0 {
		t.Fatalf("day 3 %+v, want a rest day on %v", second, today)
	}
	if last := days[len(days)-1]; last.Week != 8 || last.Day != 7 || !last.Date.Equal(*p.EndDate) {
		t.Fatalf("last day %+v", last)
	}

	// A new revision re-resolves from today on; yesterday keeps its plan.
	if _, err := e.s.Update(t.Context(), e.user, p.ID, strings.Replace(eightWeeks, "kg(60)", "kg(70)", 1)); err != nil {
		t.Fatal(err)
	}
	days = e.days(p.ID, programs.Range{FromWeek: 1, ToWeek: 2})
	if firstWeight(t, days[0]) != 60 || firstWeight(t, days[7]) != 70 || days[7].Day != 2 || days[7].Week != 2 {
		t.Fatalf("after update: %v on %v, %v on %v", firstWeight(t, days[0]), days[0].Date, firstWeight(t, days[7]), days[7].Date)
	}

	// A shorter revision ends on the Sunday of its own last week.
	p, err := e.s.Update(t.Context(), e.user, p.ID, strings.ReplaceAll(eightWeeks, "8", "4"))
	if err != nil {
		t.Fatal(err)
	}
	days = e.days(p.ID, programs.Range{})
	if !p.EndDate.Equal(date(2026, 12, 6)) || !days[len(days)-1].Date.Equal(*p.EndDate) {
		t.Fatalf("end %v, last day %v, want the Sunday of week 4", p.EndDate, days[len(days)-1].Date)
	}
}

func TestActivateAnchoredDay8Fails(t *testing.T) {
	e := setup(t)
	p := e.create(programs.KindWorkout, `program("Long week", weeks=1)
day(1, 8, "Eighth")
ex("Squat", sets=[s(1, 5, kg(100))])
`)
	_, err := e.s.Activate(t.Context(), e.user, p.ID, &today)
	assertCode(t, err, connect.CodeInvalidArgument)
	e.activate(p.ID, nil) // floating weeks may have any number of days
}

func TestActivateFloatingCursor(t *testing.T) {
	e := setup(t)
	p := e.activate(e.create(programs.KindWorkout, twoWeeks).ID, nil)
	if p.Status != programs.StatusActive || p.StartDate != nil || p.EndDate != nil {
		t.Fatalf("program %+v", p)
	}
	c, err := e.s.Q.GetCursor(t.Context(), p.ID)
	if err != nil || c.NextWeek != 1 || c.NextDay != 1 {
		t.Fatalf("cursor %+v, %v", c, err)
	}
	days := e.days(p.ID, programs.Range{})
	if len(days) != 4 || days[0].Date != nil || days[3].Week != 2 || days[3].Day != 2 {
		t.Fatalf("days %+v", days)
	}
}

func TestTwoActiveWorkoutProgramsRejected(t *testing.T) {
	e := setup(t)
	a := e.activate(e.create(programs.KindWorkout, twoWeeks).ID, nil)
	b := e.create(programs.KindWorkout, eightWeeks)
	_, err := e.s.Activate(t.Context(), e.user, b.ID, nil)
	assertCode(t, err, connect.CodeFailedPrecondition)
	e.activate(e.create(programs.KindMeal, mealScript).ID, nil) // one meal program may run beside it

	if _, err := e.s.Finish(t.Context(), e.user, a.ID); err != nil {
		t.Fatal(err)
	}
	e.activate(b.ID, nil)
	_, err = e.s.Finish(t.Context(), e.user, a.ID)
	assertCode(t, err, connect.CodeFailedPrecondition)
}

func TestUpdateCreatesRevisionKeepsState(t *testing.T) {
	e := setup(t)
	p := e.activate(e.create(programs.KindWorkout, twoWeeks).ID, nil)
	squat := model.ProgKey{Exercise: "Squat"}
	ov := model.Overlay{Weights: map[model.SetAddr]model.WeightOverride{{Key: squat}: {Delta: kg(5), Scale: 1}}}
	if err := programs.SaveOverlay(t.Context(), e.s.Q, p.ID, ov); err != nil {
		t.Fatal(err)
	}
	if err := e.s.Resolve(t.Context(), e.user, p.ID); err != nil {
		t.Fatal(err)
	}
	if w := firstWeight(t, e.days(p.ID, programs.Range{})[0]); w != 105 {
		t.Fatalf("resolved squat %v, want 105 with the overlay", w)
	}

	script := `program("Short v2", weeks=2)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 3, kg(110))])
`
	p, err := e.s.Update(t.Context(), e.user, p.ID, script)
	if err != nil {
		t.Fatal(err)
	}
	if p.Revision != 2 || p.Name != "Short v2" || p.Script != script || p.Status != programs.StatusActive {
		t.Fatalf("program %+v", p)
	}
	revs, err := e.s.ListRevisions(t.Context(), e.user, p.ID, 0, "")
	if err != nil || len(revs.Items) != 2 || revs.Items[0].Revision != 2 || revs.Items[0].Script != script || revs.Items[1].Script != twoWeeks {
		t.Fatalf("revisions %+v, %v", revs, err)
	}
	days := e.days(p.ID, programs.Range{})
	if len(days) != 2 || firstWeight(t, days[0]) != 115 {
		t.Fatalf("days after update %+v", days)
	}
	_, err = e.s.Update(t.Context(), e.user, p.ID, `program("bad", weeks=1)`+"\nex()")
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestUpdateKeepsOverlayForSurvivingExercises(t *testing.T) {
	e := setup(t)
	p := e.create(programs.KindWorkout, twoWeeks)
	squat, bench := model.ProgKey{Exercise: "Squat"}, model.ProgKey{Exercise: "Bench Press", Label: "heavy"}
	tag := model.ProgKey{Exercise: "tag:1"}
	five := 5
	ov := model.Overlay{
		Weights: map[model.SetAddr]model.WeightOverride{{Key: squat}: {Delta: kg(5), Scale: 1}, {Key: bench, Week: 2}: {Delta: kg(2.5), Scale: 1}},
		NumSets: map[model.DayAddr]model.IntOverride{{Key: bench}: {Abs: &five}},
		State: map[model.ProgKey]map[string]starlark.Value{
			bench: {"n": starlark.MakeInt(1)}, squat: {"n": starlark.MakeInt(2)}, tag: {"n": starlark.MakeInt(3)},
		},
		ExerciseVariation: map[model.ProgKey]int{bench: 2},
		RM1:               map[string]model.Weight{"Bench Press": kg(100)},
	}
	if err := programs.SaveOverlay(t.Context(), e.s.Q, p.ID, ov); err != nil {
		t.Fatal(err)
	}
	// Bench Press keeps its name but loses its label, so it is a different exercise.
	if _, err := e.s.Update(t.Context(), e.user, p.ID, `program("Short", weeks=2)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))])
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(60))])
`); err != nil {
		t.Fatal(err)
	}
	got, err := programs.LoadOverlay(t.Context(), e.s.Q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Weights) != 1 || got.Weights[model.SetAddr{Key: squat}].Delta != kg(5) {
		t.Errorf("weights %+v, want only squat's", got.Weights)
	}
	if len(got.NumSets) != 0 || len(got.ExerciseVariation) != 0 {
		t.Errorf("bench overrides kept: %+v %+v", got.NumSets, got.ExerciseVariation)
	}
	if len(got.State) != 2 || got.State[squat]["n"].String() != "2" || got.State[tag]["n"].String() != "3" {
		t.Errorf("state %+v, want squat and shared tag state", got.State)
	}
	if got.RM1["Bench Press"] != kg(100) {
		t.Errorf("1RMs %+v, want them kept", got.RM1)
	}
}
