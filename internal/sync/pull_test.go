package sync_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
	"github.com/gdsoumya/plusultra/internal/sync"
)

func TestPullSinceCursor(t *testing.T) {
	e := setup(t)
	ml := mealLog("2026-11-10", 200)
	if res := e.push(sync.Rows{
		DayLogs:        []logs.DayLog{dayLog("2026-11-10", "done")},
		WorkoutRecords: []logs.WorkoutRecord{record("2026-11-10")},
		MealLogs:       []logs.MealLog{ml},
		Checkins:       []logs.Checkin{checkin("2026-11-10", 80)},
	}); len(res.Applied) != 4 {
		t.Fatalf("applied = %v, reasons = %v", res.Applied, res.Reasons)
	}
	e.age()

	c := e.pull("", 0)
	if len(c.DayLogs) != 1 || len(c.WorkoutRecords) != 1 || len(c.MealLogs) != 1 || len(c.Checkins) != 1 || c.HasMore {
		t.Fatalf("first pull = %+v", c)
	}
	if got := c.MealLogs[0].Totals.Kcal; got != 260 {
		t.Fatalf("meal log totals kcal = %v, want 260", got)
	}
	if again := e.pull(c.Cursor, 0); len(again.DayLogs)+len(again.WorkoutRecords)+len(again.MealLogs)+len(again.Checkins) != 0 || again.Cursor != c.Cursor {
		t.Fatalf("pull at cursor = %+v", again)
	}

	if _, err := e.q.DeleteMealLog(e.ctx, sqlc.DeleteMealLogParams{ID: ml.ID, UserID: e.user}); err != nil {
		t.Fatal(err)
	}
	e.age()
	next := e.pull(c.Cursor, 0)
	if len(next.MealLogs) != 1 || next.MealLogs[0].ID != ml.ID || next.MealLogs[0].DeletedAt == nil || len(next.DayLogs)+len(next.Checkins)+len(next.WorkoutRecords) != 0 {
		t.Fatalf("pull after delete = %+v", next)
	}
	if next.Cursor == c.Cursor {
		t.Fatal("cursor did not advance")
	}

	if _, err := e.svc.Pull(e.ctx, e.user, "not a cursor", 0, noon); !errors.Is(err, sync.ErrInvalid) {
		t.Fatalf("bad cursor: err = %v", err)
	}
	if _, err := e.svc.Pull(e.ctx, e.user, "", 501, noon); !errors.Is(err, sync.ErrInvalid) {
		t.Fatalf("limit 501: err = %v", err)
	}
}

func TestPullLimitAndCursorDoNotSkipRows(t *testing.T) {
	e := setup(t)
	rows := sync.Rows{
		DayLogs:        []logs.DayLog{dayLog("2026-11-01", "done"), dayLog("2026-11-02", "rest")},
		WorkoutRecords: []logs.WorkoutRecord{record("2026-11-01")},
		MealLogs:       []logs.MealLog{mealLog("2026-11-01", 100), mealLog("2026-11-02", 100)},
		Checkins:       []logs.Checkin{checkin("2026-11-01", 80), checkin("2026-11-02", 79)},
	}
	if res := e.push(rows); len(res.Applied) != 7 {
		t.Fatalf("applied = %v, reasons = %v", res.Applied, res.Reasons)
	}
	// Interleave the tables in time, with a tie across tables that only the id breaks.
	base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	at := func(table string, id uuid.UUID, i int) {
		e.exec(`UPDATE `+table+` SET updated_at = $1 WHERE id = $2`, base.Add(time.Duration(i)*time.Microsecond), id)
	}
	at("day_logs", rows.DayLogs[0].ID, 0)
	at("meal_logs", rows.MealLogs[0].ID, 1)
	at("checkins", rows.Checkins[0].ID, 1)
	at("workout_records", rows.WorkoutRecords[0].ID, 2)
	at("day_logs", rows.DayLogs[1].ID, 3)
	at("checkins", rows.Checkins[1].ID, 3)
	at("meal_logs", rows.MealLogs[1].ID, 4)

	seen := map[uuid.UUID]int{}
	cursor, pulls := "", 0
	for {
		c := e.pull(cursor, 3)
		pulls++
		n := 0
		for _, r := range c.DayLogs {
			seen[r.ID]++
			n++
		}
		for _, r := range c.WorkoutRecords {
			seen[r.ID]++
			n++
		}
		for _, r := range c.MealLogs {
			seen[r.ID]++
			n++
		}
		for _, r := range c.Checkins {
			seen[r.ID]++
			n++
		}
		if n > 3 {
			t.Fatalf("pull %d returned %d rows, limit 3", pulls, n)
		}
		cursor = c.Cursor
		if !c.HasMore {
			break
		}
		if pulls > 10 {
			t.Fatal("pull never finished")
		}
	}
	if len(seen) != 7 || pulls != 3 {
		t.Fatalf("seen %d rows in %d pulls, want 7 in 3", len(seen), pulls)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("row %s seen %d times", id, n)
		}
	}

	// More rows left in one table than the limit: the page that fills up must say so.
	e.push(sync.Rows{DayLogs: []logs.DayLog{
		dayLog("2026-11-03", "done"), dayLog("2026-11-04", "done"), dayLog("2026-11-05", "done"), dayLog("2026-11-06", "done"),
	}})
	e.age()
	c := e.pull(cursor, 3)
	if len(c.DayLogs) != 3 || !c.HasMore {
		t.Fatalf("page of 3 of 4 = %d rows, has_more %v", len(c.DayLogs), c.HasMore)
	}
	if c = e.pull(c.Cursor, 3); len(c.DayLogs) != 1 || c.HasMore {
		t.Fatalf("last page = %d rows, has_more %v", len(c.DayLogs), c.HasMore)
	}
}

func TestPullResolvedDaysAndActivePrograms(t *testing.T) {
	e := setup(t)
	start := day("2026-11-09")
	active, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{
		UserID: e.user, Kind: "workout", Name: "PPL", Script: "x", Structure: []byte(`{}`), Status: "active", StartDate: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{
		UserID: e.user, Kind: "workout", Name: "Draft", Script: "x", Structure: []byte(`{}`), Status: "draft",
	}); err != nil {
		t.Fatal(err)
	}
	for i := range 35 { // 2026-11-09 .. 2026-12-13
		d := start.AddDate(0, 0, i)
		if err := e.q.UpsertResolvedDay(e.ctx, sqlc.UpsertResolvedDayParams{
			ProgramID: active.ID, Week: int32(i/7 + 1), Day: int32(i%7 + 1), Date: &d, Payload: []byte(`{}`), //nolint:gosec // small
		}); err != nil {
			t.Fatal(err)
		}
	}

	c := e.pull("", 0)
	if len(c.Programs) != 1 || c.Programs[0].ID != active.ID {
		t.Fatalf("programs = %+v", c.Programs)
	}
	if len(c.ResolvedDays) != 28 || len(c.ResolvedMealDays) != 0 || len(c.Cursors) != 0 {
		t.Fatalf("resolved days = %d, meal days = %d, cursors = %v", len(c.ResolvedDays), len(c.ResolvedMealDays), c.Cursors)
	}
	if first, last := c.ResolvedDays[0].Day.Date.Format(time.DateOnly), c.ResolvedDays[27].Day.Date.Format(time.DateOnly); first != "2026-11-11" || last != "2026-12-08" {
		t.Fatalf("resolved days %s..%s, want 2026-11-11..2026-12-08", first, last)
	}
}

func TestPullLagsRecentRows(t *testing.T) {
	e := setup(t)
	dl := dayLog("2026-11-10", "done")
	e.push(sync.Rows{DayLogs: []logs.DayLog{dl}})
	e.exec(`UPDATE day_logs SET updated_at = now() - interval '1 second'`)
	if c := e.pull("", 0); len(c.DayLogs) != 0 || c.Cursor != "" {
		t.Fatalf("a row stamped 1 s ago was pulled: %+v", c)
	}
	e.exec(`UPDATE day_logs SET updated_at = now() - interval '10 seconds'`)
	if c := e.pull("", 0); len(c.DayLogs) != 1 || c.DayLogs[0].ID != dl.ID {
		t.Fatalf("a row stamped 10 s ago was not pulled: %+v", c)
	}
}

func TestPullFloatingProgramDays(t *testing.T) {
	e := setup(t)
	p, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{
		UserID: e.user, Kind: "workout", Name: "GZCLP", Script: "x", Structure: []byte(`{"Weeks":6,"Loop":true}`), Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	for w := int32(1); w <= 6; w++ {
		for d := int32(1); d <= 5; d++ {
			if err := e.q.UpsertResolvedDay(e.ctx, sqlc.UpsertResolvedDayParams{ProgramID: p.ID, Week: w, Day: d, Payload: []byte(`{}`)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	at := func(days []sync.PlanDay, i int) [2]int { return [2]int{days[i].Day.Week, days[i].Day.Day} }

	// No cursor yet: the program starts at its first day.
	if c := e.pull("", 0); len(c.ResolvedDays) != 28 || at(c.ResolvedDays, 0) != [2]int{1, 1} || len(c.Cursors) != 0 {
		t.Fatalf("without a cursor: %d days from %v, cursors %v", len(c.ResolvedDays), at(c.ResolvedDays, 0), c.Cursors)
	}

	if _, err := e.q.UpsertCursor(e.ctx, sqlc.UpsertCursorParams{ProgramID: p.ID, NextWeek: 2, NextDay: 3}); err != nil {
		t.Fatal(err)
	}
	c := e.pull("", 0)
	if cur, ok := c.Cursors[p.ID]; !ok || cur.NextWeek != 2 || cur.NextDay != 3 {
		t.Fatalf("cursors = %v", c.Cursors)
	}
	// 23 days from week 2 day 3 to the end, then the program loops to week 1.
	if len(c.ResolvedDays) != 28 || at(c.ResolvedDays, 0) != [2]int{2, 3} || at(c.ResolvedDays, 22) != [2]int{6, 5} ||
		at(c.ResolvedDays, 23) != [2]int{1, 1} || at(c.ResolvedDays, 27) != [2]int{1, 5} {
		t.Fatalf("looping days = %v", c.ResolvedDays)
	}

	e.exec(`UPDATE programs SET structure = '{"Weeks":6,"Loop":false}' WHERE id = $1`, p.ID)
	if c := e.pull("", 0); len(c.ResolvedDays) != 23 || at(c.ResolvedDays, 22) != [2]int{6, 5} {
		t.Fatalf("non-looping days = %d", len(c.ResolvedDays))
	}
}

func TestPullMealDaysFromLocalToday(t *testing.T) {
	e := setup(t)
	e.exec(`UPDATE users SET timezone = 'Pacific/Kiritimati' WHERE id = $1`, e.user) // UTC+14: noon UTC is the next day
	start := day("2026-11-09")
	p, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{
		UserID: e.user, Kind: "meal", Name: "Cut", Script: "x", Structure: []byte(`{}`), Status: "active", StartDate: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 40 {
		d := start.AddDate(0, 0, i)
		if err := e.q.UpsertResolvedDay(e.ctx, sqlc.UpsertResolvedDayParams{
			ProgramID: p.ID, Week: int32(i/7 + 1), Day: int32(i%7 + 1), Date: &d, Kind: "rest", Payload: []byte(`{}`), //nolint:gosec // small
		}); err != nil {
			t.Fatal(err)
		}
	}
	c := e.pull("", 0)
	if len(c.ResolvedDays) != 0 || len(c.ResolvedMealDays) != 28 {
		t.Fatalf("workout days = %d, meal days = %d", len(c.ResolvedDays), len(c.ResolvedMealDays))
	}
	if first := c.ResolvedMealDays[0].Day.Date.Format(time.DateOnly); first != "2026-11-12" {
		t.Fatalf("first meal day %s, want the user's local today 2026-11-12", first)
	}
}

func TestPullFloatingMealDaysFromTheWorkoutWeek(t *testing.T) {
	e := setup(t)
	e.exercises("Squat")
	workout := e.activate(programs.KindWorkout, `program("Short", weeks=3)
day(1, 1, "A", weeks="1-3")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))])
`, nil)
	meals := e.activate(programs.KindMeal, `meal_program("Eat", weeks=3)
oats = food("Oats", per100g={"kcal": 380})
day(1, kind="training", name="Train")
meal("Breakfast", oats(100))
day(1, kind="rest", name="Rest")
meal("Breakfast", oats(50))
`, nil)
	if _, err := e.q.UpsertCursor(e.ctx, sqlc.UpsertCursorParams{ProgramID: workout.ID, NextWeek: 2, NextDay: 1}); err != nil {
		t.Fatal(err)
	}

	// Meal weeks follow the workout's: week 2 on, then the weeks before it, for when the workout loops.
	c := e.pull("", 0)
	var got []string
	for _, d := range c.ResolvedMealDays {
		if d.ProgramID != meals.ID || d.Day.Date != nil || d.Day.Meal == nil {
			t.Fatalf("meal day = %+v", d)
		}
		got = append(got, fmt.Sprintf("%d %s %s", d.Day.Week, d.Day.Kind, d.Day.Meal.Name))
	}
	want := []string{"2 training Train", "2 rest Rest", "3 training Train", "3 rest Rest", "1 training Train", "1 rest Rest"}
	if !slices.Equal(got, want) {
		t.Fatalf("meal days = %q, want %q", got, want)
	}
	if len(c.ResolvedDays) == 0 || c.ResolvedDays[0].Day.Week != 2 || c.ResolvedDays[0].Day.Workout == nil {
		t.Fatalf("workout days = %+v", c.ResolvedDays)
	}
}
