package export_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/export"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

const workoutScript = `program("Base", weeks=2)
day(1, 1, "Lower", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 3, "Upper", weeks="1-2")
ex("Bench Press", label="heavy", warmup=[], sets=[s(3, 5, kg(60))])
`

type env struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	q    *sqlc.Queries
	s    *export.Service
	user uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	pool := dbtest.New(t)
	q := sqlc.New(pool)
	ctx := t.Context()
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Squat", "Bench Press", "Deadlift"} {
		if _, err := catalog.CreateCustomExercise(ctx, q, u.ID, catalog.Custom{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	progs := programs.NewService(pool)
	return env{t: t, ctx: ctx, pool: pool, q: q, s: export.NewService(pool, progs, logs.NewService(pool, progs)), user: u.ID}
}

func (e env) fatal(err error) {
	e.t.Helper()
	if err != nil {
		e.t.Fatal(err)
	}
}

func date(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func kg(v float64) model.RecordWeight { return model.RecordWeight{Value: v, Unit: model.UnitKg} }

func (e env) create(kind, script string) sqlc.Program {
	e.t.Helper()
	p, err := e.s.Programs.Create(e.ctx, e.user, kind, script)
	e.fatal(err)
	return p
}

// program inserts a bare finished program row, enough for a workout record to name it.
func (e env) program(name string) uuid.UUID { return e.programWith(name, programs.StatusFinished) }

func (e env) programWith(name, status string) uuid.UUID { //nolint:unparam // status varies per test
	e.t.Helper()
	p, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{UserID: e.user, Kind: programs.KindWorkout, Name: name,
		Structure: []byte("{}"), Status: status})
	e.fatal(err)
	return p.ID
}

func (e env) dayLog(day, status, notes string) {
	e.t.Helper()
	_, err := e.q.UpsertDayLog(e.ctx, sqlc.UpsertDayLogParams{UserID: e.user, Date: date(day), Status: status, Notes: notes})
	e.fatal(err)
}

func (e env) checkin(day string, weightKg float64) {
	e.t.Helper()
	_, err := e.s.Checkins.Create(e.ctx, e.user, logsCheckin(day, weightKg))
	e.fatal(err)
}

// record stores a workout record of program on day, started at hour o'clock and lasting minutes.
func (e env) record(day string, hour int, program uuid.UUID, week, dayNum int, dayName string, minutes int, entries []model.RecordEntry) {
	e.t.Helper()
	rd, err := json.Marshal(model.ResolvedDay{Week: week, Day: dayNum, Name: dayName, Kind: "training"})
	e.fatal(err)
	es, err := json.Marshal(entries)
	e.fatal(err)
	start := date(day).Add(time.Duration(hour) * time.Hour)
	end := start.Add(time.Duration(minutes) * time.Minute)
	_, err = e.q.UpsertWorkoutRecord(e.ctx, sqlc.UpsertWorkoutRecordParams{ID: uuid.New(), UserID: e.user, Date: date(day), ProgramID: &program,
		ResolvedDay: rd, Entries: es, StartedAt: &start, FinishedAt: &end, Finalized: true})
	e.fatal(err)
}

// sets builds n working sets planned as p, completed with the given reps (0 is not done) and the planned weight.
func sets(n int, p model.RecordPlanned, reps ...int) []model.RecordSet {
	out := make([]model.RecordSet, n)
	for i := range out {
		out[i] = model.RecordSet{Planned: p, Completed: true, Reps: p.Reps, Weight: p.Weight}
		if i < len(reps) {
			out[i].Reps, out[i].Completed = reps[i], reps[i] > 0
		}
	}
	return out
}

func warmups(n int) []model.RecordSet {
	out := make([]model.RecordSet, n)
	for i := range out {
		out[i] = model.RecordSet{Planned: model.RecordPlanned{Reps: 5, Weight: kg(40), Warmup: true}, Completed: true, Reps: 5, Weight: kg(40), Warmup: true}
	}
	return out
}

func planned(reps int, w model.RecordWeight, label string) model.RecordPlanned {
	return model.RecordPlanned{Reps: reps, MinReps: reps, Weight: w, Label: label}
}

// Foods of the golden log, with macros that sum to its totals.
var (
	chicken = model.Food{Name: "Chicken breast raw", Per100g: model.Macros{Kcal: 1061, Protein: 84.4, Carbs: 99.5, Fat: 32.5}}
	rice    = model.Food{Name: "Basmati rice raw", Per100g: model.Macros{Kcal: 350, Protein: 7.5, Carbs: 77.5, Fat: 0.5}}
	banana  = model.Food{Name: "Banana", Per100g: model.Macros{Kcal: 90, Protein: 1, Carbs: 22.5, Fat: 0.5}}
	oats    = model.Food{Name: "Oats", Per100g: model.Macros{Kcal: 2100, Protein: 165, Carbs: 250, Fat: 50}}
	eggUnit = model.Macros{Kcal: 70, Protein: 6, Fat: 5}
	egg     = model.Food{Name: "Egg, whole", Per100g: model.Macros{Kcal: 140, Protein: 12, Fat: 10}, PerUnit: &eggUnit, Unit: "egg"}
)

func item(f model.Food, qty float64, planned bool) model.Item {
	unit := "g"
	if f.PerUnit != nil {
		unit = f.Unit
	}
	return model.Item{Food: f, Qty: qty, Unit: unit, Planned: planned}
}

// seedGolden stores the data of testdata/days.txt.
func (e env) seedGolden() {
	e.t.Helper()
	e.dayLog("2026-11-11", "done", "slept badly")
	e.checkin("2026-11-11", 82.4)
	strength := e.program("16-Week Strength")
	bench := append(warmups(3), sets(4, planned(5, kg(75), "Work"))...)
	bench = append(bench, sets(2, planned(5, kg(62.5), "Backoff"))...)
	e.record("2026-11-11", 10, strength, 1, 2, "Tue Upper A bench", 58, []model.RecordEntry{
		{Name: "Bench Press", Sets: bench},
		{Name: "Lat Pulldown", Label: "heavy", Sets: sets(4, planned(6, kg(70), ""), 6, 6, 6, 5)},
	})
	e.mealLog("2026-11-11", model.Macros{Kcal: 2600, Protein: 180, Carbs: 300, Fat: 70},
		model.Meal{Name: "Lunch", Items: []model.Item{item(chicken, 200, true), item(rice, 80, true)}},
		model.Meal{Name: "Snack", Items: []model.Item{item(banana, 120, false)}})

	e.dayLog("2026-11-12", "partial", "")
	e.checkin("2026-11-12", 82)
	volume := e.program("Upper volume")
	row := sets(3, model.RecordPlanned{Reps: 10, MinReps: 10, Ask: true}, 10, 10, 9)
	for i := range row {
		row[i].Weight = kg(30)
	}
	e.record("2026-11-12", 10, volume, 2, 1, "Pull and press", 52, []model.RecordEntry{
		{Name: "Pull Up", Sets: sets(3, planned(8, kg(92), ""), 8, 7, 0)},
		{Name: "Bench Press", Sets: append(warmups(2), sets(3, model.RecordPlanned{Reps: 12, MinReps: 8, Weight: kg(60)}, 11, 10, 9)...)},
		{Name: "Overhead Press", Sets: sets(3, model.RecordPlanned{Reps: 5, MinReps: 5, Weight: kg(47.5), RPE: 8})},
		{Name: "Bent Over Row, Dumbbell", Sets: row},
	})
	e.mealLog("2026-11-12", model.Macros{Kcal: 2400, Protein: 180, Carbs: 250, Fat: 70},
		model.Meal{Name: "Breakfast", Items: []model.Item{item(oats, 80, true), item(egg, 3, true), item(banana, 0, true)}})
}
