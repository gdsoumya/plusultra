package logs_test

import (
	"testing"

	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// A simulation from a program starts at its cursor with the state its
// finalized workouts left behind.
func TestSimulateFromProgramState(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	r := e.create(logs.NewRecord{Date: today, ProgramID: p.ID, Week: 1, Day: 1})
	e.update(r.ID, complete(r, 5))
	e.finalize(r.ID)

	out, err := e.programs.Simulate(e.ctx, e.user, programs.SimulateInput{ProgramID: p.ID, Weeks: 3})
	if err != nil {
		t.Fatal(err)
	}
	d := out.Days
	if len(d) != 5 || d[0].Week != 1 || d[0].Day != 2 || d[1].Week != 2 || d[1].Workout.Exercises[0].Sets[0].Weight.Value != 102.5 ||
		d[3].Week != 1 || d[3].Workout.Exercises[0].Sets[0].Weight.Value != 105 { // the loop back to week 1 keeps progressing
		t.Fatalf("days %+v", d)
	}
	if w := e.resolvedWeight(p.ID, 2, 1); w != 102.5 {
		t.Fatalf("simulate changed the stored state: %v", w)
	}
	// A script override keeps the program's state.
	out, err = e.programs.Simulate(e.ctx, e.user, programs.SimulateInput{ProgramID: p.ID, Weeks: 2,
		Script: floating + `ex("Deadlift", warmup=[], sets=[s(1, 5, kg(120))])` + "\n"})
	if err != nil || len(out.Compact) != 3 || out.Compact[1] != "W2D1 A | Squat 102.5x5 x3" || out.Compact[0] != "W1D2 B | Bench Press 80x5 x3 | Deadlift 120x5" {
		t.Fatalf("override: %q, %v", out.Compact, err)
	}
}
