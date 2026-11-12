package programs_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func TestReresolveAllIfUpgraded(t *testing.T) {
	e := setup(t)
	workout := e.activate(e.create(programs.KindWorkout, eightWeeks).ID, &today)
	meal := e.activate(e.create(programs.KindMeal, mealScript).ID, &today)
	draft := e.create(programs.KindWorkout, twoWeeks)

	bob, err := e.s.Q.CreateUser(t.Context(), sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Squat", "Bench Press", "Deadlift"} {
		if _, err := catalog.CreateCustomExercise(t.Context(), e.s.Q, bob.ID, catalog.Custom{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	b := env{t: t, s: e.s, user: bob.ID}
	bobWorkout := b.activate(b.create(programs.KindWorkout, twoWeeks).ID, nil)

	// Stale rows stand in for an engine whose output changed.
	e.stale(workout.ID, 2, 1)
	b.stale(bobWorkout.ID, 1, 1)
	if err := e.s.Q.DeleteResolvedDays(t.Context(), meal.ID); err != nil {
		t.Fatal(err)
	}

	if err := e.s.ReresolveAllIfUpgraded(t.Context(), "v2"); err != nil {
		t.Fatal(err)
	}
	if n := len(e.days(workout.ID, programs.Range{FromWeek: 2, ToWeek: 2})[0].Workout.Exercises); n != 1 {
		t.Fatalf("ann's week 2 day 1 has %d exercises after upgrade, want 1", n)
	}
	if n := len(b.days(bobWorkout.ID, programs.Range{})[0].Workout.Exercises); n != 2 {
		t.Fatalf("bob's day 1 has %d exercises after upgrade, want 2", n)
	}
	if len(e.days(meal.ID, programs.Range{})) == 0 {
		t.Fatal("meal days not re-resolved")
	}
	if rows, err := e.s.Q.ListResolvedDays(t.Context(), draft.ID); err != nil || len(rows) != 0 {
		t.Fatalf("draft has %d resolved days, err %v", len(rows), err)
	}

	e.stale(workout.ID, 2, 1)
	if err := e.s.ReresolveAllIfUpgraded(t.Context(), "v2"); err != nil {
		t.Fatal(err)
	}
	if n := len(e.days(workout.ID, programs.Range{FromWeek: 2, ToWeek: 2})[0].Workout.Exercises); n != 0 {
		t.Fatal("same version re-resolved again")
	}
}

// stale empties the payload of program id's resolved day.
func (e env) stale(id uuid.UUID, week, day int32) {
	e.t.Helper()
	r, err := e.s.Q.GetResolvedDay(e.t.Context(), sqlc.GetResolvedDayParams{ProgramID: id, Week: week, Day: day})
	if err != nil {
		e.t.Fatal(err)
	}
	err = e.s.Q.UpsertResolvedDay(e.t.Context(), sqlc.UpsertResolvedDayParams{ProgramID: id, Week: week, Day: day, Date: r.Date, Kind: r.Kind, Payload: []byte(`{}`)})
	if err != nil {
		e.t.Fatal(err)
	}
}
