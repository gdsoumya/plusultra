package programs_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// today is the service clock's date: a Wednesday.
var today = date(2026, 11, 11)

type env struct {
	t    *testing.T
	s    *programs.Service
	user uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	s := programs.NewService(dbtest.New(t))
	s.Now = func() time.Time { return today.Add(12 * time.Hour) }
	u, err := s.Q.CreateUser(t.Context(), sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Squat", "Bench Press", "Deadlift"} {
		if _, err := catalog.CreateCustomExercise(t.Context(), s.Q, u.ID, catalog.Custom{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	return env{t: t, s: s, user: u.ID}
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func (e env) create(kind, script string) sqlc.Program {
	e.t.Helper()
	p, err := e.s.Create(e.t.Context(), e.user, kind, script)
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e env) activate(id uuid.UUID, start *time.Time) sqlc.Program {
	e.t.Helper()
	p, err := e.s.Activate(e.t.Context(), e.user, id, start)
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e env) days(id uuid.UUID, r programs.Range) []programs.Day {
	e.t.Helper()
	days, err := e.s.ResolvedDays(e.t.Context(), e.user, id, r)
	if err != nil {
		e.t.Fatal(err)
	}
	return days
}

func assertCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != want {
		t.Fatalf("err = %v, want code %v", err, want)
	}
}

// firstWeight is the first working set weight of the day's first exercise.
func firstWeight(t *testing.T, d programs.Day) float64 {
	t.Helper()
	if d.Workout == nil || len(d.Workout.Exercises) == 0 || len(d.Workout.Exercises[0].Sets) == 0 {
		t.Fatalf("no sets on %+v", d)
	}
	for _, s := range d.Workout.Exercises[0].Sets {
		if !s.Warmup {
			return s.Weight.Value
		}
	}
	t.Fatal("no working sets")
	return 0
}

func kg(v float64) model.Weight { return model.Weight{Value: v, Unit: model.UnitKg} }
