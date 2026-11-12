package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
	"github.com/gdsoumya/plusultra/internal/sync"
)

type fakeFinalizer struct {
	users, records []uuid.UUID
	err            error
}

func (f *fakeFinalizer) Finalize(_ context.Context, userID, recordID uuid.UUID) error {
	f.users, f.records = append(f.users, userID), append(f.records, recordID)
	return f.err
}

type env struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	q     *sqlc.Queries
	progs *programs.Service
	logs  *logs.Service
	fin   *fakeFinalizer
	svc   *sync.Service
	user  uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	pool := dbtest.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	progs := programs.NewService(pool)
	ls := logs.NewService(pool, progs)
	fin := &fakeFinalizer{}
	return env{t, ctx, pool, q, progs, ls, fin, sync.New(q, pool, ls, progs, fin), u.ID}
}

// push pushes rows from "phone" and fails the test on error.
func (e env) push(rows sync.Rows) sync.PushResult {
	e.t.Helper()
	res, err := e.svc.Push(e.ctx, e.user, "phone", rows)
	if err != nil {
		e.t.Fatal(err)
	}
	return res
}

// noon is the wall clock of pulls: 2026-11-11 for a user in UTC.
var noon = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func (e env) pull(cursor string, limit int) sync.Changes {
	e.t.Helper()
	c, err := e.svc.Pull(e.ctx, e.user, cursor, limit, noon)
	if err != nil {
		e.t.Fatal(err)
	}
	return c
}

// age moves every synced row a minute into the past, beyond the pull's lag.
func (e env) age() {
	e.t.Helper()
	for _, table := range []string{"day_logs", "workout_records", "meal_logs", "checkins"} {
		e.exec(`UPDATE ` + table + ` SET updated_at = updated_at - interval '1 minute'`)
	}
}

func (e env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func dayLog(date, status string) logs.DayLog {
	return logs.DayLog{ID: uuid.New(), Date: date, Status: status}
}

func record(date string) logs.WorkoutRecord {
	return logs.WorkoutRecord{ID: uuid.New(), Date: date}
}

// exercises adds names to the user's custom exercises, as program scripts must name known ones.
func (e env) exercises(names ...string) {
	e.t.Helper()
	for _, name := range names {
		if _, err := catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: name}); err != nil {
			e.t.Fatal(err)
		}
	}
}

// activate creates script as userID's program of kind and activates it, anchored on start or floating.
func (e env) activate(kind, script string, start *time.Time) sqlc.Program {
	e.t.Helper()
	p, err := e.progs.Create(e.ctx, e.user, kind, script)
	if err == nil {
		p, err = e.progs.Activate(e.ctx, e.user, p.ID, start)
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func mealLog(date string, grams float64) logs.MealLog {
	item := model.Item{Food: model.Food{Name: "Rice", Per100g: model.Macros{Kcal: 130}}, Qty: grams, Unit: "g"}
	return logs.MealLog{ID: uuid.New(), Date: date, Meals: []model.Meal{{Name: "Lunch", Items: []model.Item{item}}}}
}

func checkin(date string, kg float64) logs.Checkin {
	return logs.Checkin{ID: uuid.New(), Date: date, WeightKg: &kg}
}
