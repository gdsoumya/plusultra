package logs_test

import (
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// base is anchored on weekdays: Lower on Monday, Upper on Wednesday, Cardio on Friday.
const base = `program("Base", weeks=3)
day(1, 1, "Lower", weeks="1-3")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 3, "Upper", weeks="1-3")
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))], progress=lp(kg(2.5)))
day(1, 5, "Cardio", kind="conditioning", weeks="1-3")
`

const floating = `program("Short", weeks=2)
day(1, 1, "A", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 2, "B", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))])
`

const noLoop = `program("Once", weeks=1, loop=False)
day(1, 1, "A")
ex("Squat", warmup=[], sets=[s(1, 5, kg(100))])
`

const hooked = `program("Hook", weeks=1)
def hook(ctx):
    if ctx.set_index == 1:
        ctx.weights = kg(90)
day(1, 1, "A")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], update=custom(hook))
`

const shrinking = `program("Shrink", weeks=1)
def hook(ctx):
    if ctx.set_index == 1:
        ctx.number_of_sets = 2
day(1, 1, "A")
ex("Squat", warmup=[], sets=[s(5, 5, kg(100))], update=custom(hook))
`

const mealScript = `meal_program("Eat", weeks=1)
oats = food("Oats", per100g={"kcal": 380})
day(1, kind="training", name="Train")
meal("Breakfast", oats(100))
day(1, kind="rest", name="Rest")
meal("Breakfast", oats(50))
`

func (e env) activate(script string, start *time.Time) sqlc.Program {
	kind := programs.KindWorkout
	if len(script) > 5 && script[:5] == "meal_" {
		kind = programs.KindMeal
	}
	p, err := e.programs.Create(e.ctx, e.user, kind, script)
	if err != nil {
		panic(err)
	}
	if p, err = e.programs.Activate(e.ctx, e.user, p.ID, start); err != nil {
		panic(err)
	}
	return p
}

func (e env) calendar(from, to string) []logs.CalendarDay {
	days, err := e.s.GetCalendar(e.ctx, e.user, from, to)
	if err != nil {
		panic(err)
	}
	return days
}

func (e env) day(date string) logs.DayView {
	v, err := e.s.GetDay(e.ctx, e.user, date)
	if err != nil {
		panic(err)
	}
	return v
}

func (e env) create(in logs.NewRecord) logs.WorkoutRecord {
	r, err := e.s.CreateWorkoutRecord(e.ctx, e.user, in)
	if err != nil {
		panic(err)
	}
	return r
}

func (e env) record(id uuid.UUID) logs.WorkoutRecord {
	r, err := e.s.GetWorkoutRecord(e.ctx, e.user, id)
	if err != nil {
		panic(err)
	}
	return r
}

// complete returns r's entries with every working set done at the planned weight for reps reps.
func complete(r logs.WorkoutRecord, reps int) []model.RecordEntry {
	for _, en := range r.Entries {
		for i := range en.Sets {
			s := &en.Sets[i]
			s.Completed, s.Reps, s.Weight = true, reps, s.Planned.Weight
		}
	}
	return r.Entries
}

func (e env) update(id uuid.UUID, entries []model.RecordEntry) logs.WorkoutRecord {
	r, err := e.s.UpdateWorkoutRecord(e.ctx, e.user, id, logs.RecordInput{Entries: entries})
	if err != nil {
		panic(err)
	}
	return r
}

func (e env) finalize(id uuid.UUID) logs.FinalizeResult {
	res, err := e.s.FinalizeWorkoutRecord(e.ctx, e.user, id)
	if err != nil {
		panic(err)
	}
	return res
}

// resolvedWeight is the first working-set weight of day (week, day) of program id.
func (e env) resolvedWeight(id uuid.UUID, week, day int) float64 {
	days, err := e.programs.ResolvedDays(e.ctx, e.user, id, programs.Range{FromWeek: week, ToWeek: week})
	if err != nil {
		panic(err)
	}
	for _, d := range days {
		if d.Day == day {
			return d.Workout.Exercises[0].Sets[0].Weight.Value
		}
	}
	panic("no such day")
}

// clearSnapshot makes id look finalized before migration 0007: no state before.
func (e env) clearSnapshot(id uuid.UUID) {
	if err := e.q.SetWorkoutRecordStateBefore(e.ctx, sqlc.SetWorkoutRecordStateBeforeParams{ID: id, UserID: e.user}); err != nil {
		panic(err)
	}
}

func (e env) cursor(id uuid.UUID) (int, int) {
	c, err := e.q.GetCursor(e.ctx, id)
	if err != nil {
		panic(err)
	}
	return int(c.NextWeek), int(c.NextDay)
}

const unilateral = `program("Uni", weeks=3)
def f(ctx):
    if ctx.completed_reps_left >= ctx.reps:
        ctx.weights += kg(2.5)
day(1, 1, "A", weeks="1-3")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(f))
`
