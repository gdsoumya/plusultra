package logs_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func TestCalendarAnchoredShowsFuturePlan(t *testing.T) {
	e := setup(t)
	e.activate(base, &monday)
	e.activate(mealScript, nil)
	days := e.calendar("2026-11-09", "2026-11-29")
	if len(days) != 21 {
		t.Fatalf("%d days, want 21", len(days))
	}
	type want struct {
		date, kind         string
		week, day          int
		name, meal, status string
	}
	for i, w := range map[int]want{
		0:  {"2026-11-09", "training", 1, 1, "Lower", "training", logs.StatusSkipped},
		1:  {"2026-11-10", "rest", 1, 2, "", "rest", ""},
		2:  {"2026-11-11", "training", 1, 3, "Upper", "training", ""},
		4:  {"2026-11-13", "conditioning", 1, 5, "Cardio", "training", ""},
		9:  {"2026-11-18", "training", 2, 3, "Upper", "training", ""},
		20: {"2026-11-29", "rest", 3, 7, "", "rest", ""},
	} {
		d := days[i]
		if d.Date != w.date || d.Kind != w.kind || d.WorkoutWeek != w.week || d.WorkoutDay != w.day ||
			d.WorkoutName != w.name || d.MealKind != w.meal || d.LogStatus != w.status {
			t.Errorf("day %d = %+v, want %+v", i, d, w)
		}
	}
	// An explicit rest log on a planned day is rest, not skipped.
	if _, err := e.s.UpdateDay(e.ctx, e.user, logs.DayLogInput{Date: "2026-11-09", Status: logs.StatusRest}); err != nil {
		t.Fatal(err)
	}
	if d := e.calendar("2026-11-09", "2026-11-09")[0]; d.LogStatus != logs.StatusRest || d.Kind != "training" {
		t.Fatalf("after rest log: %+v", d)
	}
	v := e.day("2026-11-13")
	if v.Plan == nil || v.Plan.Kind != "conditioning" || v.MealPlan == nil || v.MealPlan.Meal.Name != "Train" {
		t.Fatalf("day view %+v", v)
	}
}

func TestCalendarFloatingNoFuture(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	days := e.calendar("2026-11-10", "2026-11-13")
	if d := days[1]; d.Kind != "training" || d.WorkoutWeek != 1 || d.WorkoutDay != 1 || d.WorkoutName != "A" {
		t.Fatalf("today = %+v, want the cursor's day", d)
	}
	for _, i := range []int{0, 2, 3} {
		if d := days[i]; d.Kind != "rest" || d.WorkoutWeek != 0 || d.LogStatus != "" {
			t.Fatalf("day %d = %+v, want nothing planned", i, d)
		}
	}
	if v := e.day("2026-11-12"); v.Plan != nil {
		t.Fatalf("tomorrow has a plan %+v, want none for a floating program", v.Plan)
	}
	r := e.create(logs.NewRecord{Date: today})
	if r.Day.Week != 1 || r.Day.Day != 1 || r.ProgramID == nil {
		t.Fatalf("record %+v, want the cursor's day", r.Day)
	}
	e.finalize(r.ID)
	days = e.calendar("2026-11-10", "2026-11-13")
	if d := days[1]; d.LogStatus != logs.StatusDone || d.WorkoutDay != 1 || d.WorkoutName != "A" {
		t.Fatalf("today after finalize = %+v", d)
	}
	if d := days[2]; d.Kind != "rest" || d.WorkoutWeek != 0 {
		t.Fatalf("tomorrow = %+v, want nothing planned", d)
	}
	if v := e.day(today); v.Plan == nil || v.Plan.Day != 2 || v.Workout == nil || !v.Workout.Finalized {
		t.Fatalf("today view %+v", v)
	}
	// A script edit that drops the cursor's day leaves nothing planned rather than an error.
	if err := e.q.DeleteResolvedDays(e.ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if v := e.day(today); v.Plan != nil || v.Workout == nil {
		t.Fatalf("today without the cursor's day %+v", v)
	}
}

func TestQuickFillMarksAllPlannedCompleted(t *testing.T) {
	e := setup(t)
	e.activate(base, &monday)
	e.activate(mealScript, nil)
	for range 2 { // the second fill finds everything filled and changes nothing
		if err := e.s.QuickFill(e.ctx, e.user, today, logs.QuickFill{Workout: true, Meals: true}); err != nil {
			t.Fatal(err)
		}
	}
	v := e.day(today)
	if v.Workout == nil || v.Workout.Finalized || len(v.Workout.Entries) != 1 || v.Workout.Entries[0].Name != "Bench Press" {
		t.Fatalf("workout %+v", v.Workout)
	}
	for i, s := range v.Workout.Entries[0].Sets {
		if !s.Completed || s.Reps != 5 || s.Reps != s.Planned.Reps || s.Weight.Value != 80 || s.Weight != s.Planned.Weight {
			t.Fatalf("set %d = %+v, want done as planned", i, s)
		}
	}
	if v.Meal == nil || len(v.Meal.Meals) != 1 || !v.Meal.Meals[0].Items[0].Planned || v.Meal.Meals[0].Items[0].Qty != 100 {
		t.Fatalf("meal %+v", v.Meal)
	}
	page, err := e.s.ListWorkoutRecords(e.ctx, e.user, today, today, uuid.Nil, "", 0)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("records %+v, %v", page, err)
	}
	if d := e.calendar(today, today)[0]; d.LogStatus != logs.StatusPartial {
		t.Fatalf("status %q, want partial until finalized", d.LogStatus)
	}
	if _, err := e.s.CreateWorkoutRecord(e.ctx, e.user, logs.NewRecord{Date: today, OnlyOne: true}); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("second record with OnlyOne: %v, want ErrConflict", err)
	}
	// A planned day without exercises is still done as planned.
	if err := e.s.QuickFill(e.ctx, e.user, "2026-11-13", logs.QuickFill{Workout: true, Finalize: true}); err != nil {
		t.Fatal(err)
	}
	if v := e.day("2026-11-13"); v.Log == nil || v.Log.Status != logs.StatusDone || v.Workout == nil || !v.Workout.Finalized {
		t.Fatalf("cardio day %+v, want done", v)
	}
}

func TestFinalizeAppliesProgressionToLaterDaysOnly(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(w1.ID, complete(w1, 4)) // short of the 5 planned reps: lp fails
	e.finalize(w1.ID)
	if w := e.resolvedWeight(p.ID, 2, 3); w != 80 {
		t.Fatalf("week 2 bench %v after a failed week 1, want 80", w)
	}
	if err := e.s.QuickFill(e.ctx, e.user, "2026-11-18", logs.QuickFill{Workout: true, Finalize: true}); err != nil {
		t.Fatal(err)
	}
	if w := e.resolvedWeight(p.ID, 3, 3); w != 82.5 {
		t.Fatalf("week 3 bench %v, want 82.5", w)
	}
	if w := e.resolvedWeight(p.ID, 2, 3); w != 80 {
		t.Fatalf("week 2 bench %v, want its day left alone", w)
	}
	if w := e.record(w1.ID).Entries[0].Sets[0].Planned.Weight.Value; w != 80 {
		t.Fatalf("week 1 record plans %v, want its snapshot kept", w)
	}
	v := e.day("2026-11-18")
	if v.Log == nil || v.Log.Status != logs.StatusDone || v.Workout == nil || !v.Workout.Finalized {
		t.Fatalf("week 2 day %+v", v)
	}
}

func TestFinalizeIdempotent(t *testing.T) {
	e := setup(t)
	e.activate(base, &monday)
	r := e.create(logs.NewRecord{Date: today})
	e.finalize(r.ID)
	if _, err := e.s.FinalizeWorkoutRecord(e.ctx, e.user, r.ID); !errors.Is(err, logs.ErrFinalized) {
		t.Fatalf("second finalize: %v, want ErrFinalized", err)
	}
	if err := e.s.Finalize(e.ctx, e.user, r.ID); !errors.Is(err, logs.ErrFinalized) {
		t.Fatalf("Finalize: %v, want ErrFinalized", err)
	}
	got := e.record(r.ID)
	if !got.Finalized || got.FinishedAt == nil || got.Entries[0].Sets[0].Completed {
		t.Fatalf("record %+v", got)
	}
}

// Editing a finished workout replays its progression from the state before it (R79): the
// corrected result decides the program's next sessions. Finalized, finished time and cursor stay.
func TestEditFinalizedReplaysProgression(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(w1.ID, complete(w1, 5))
	done := e.finalize(w1.ID).Record
	if w := e.resolvedWeight(p.ID, 2, 3); w != 82.5 {
		t.Fatalf("week 2 bench %v, want 82.5", w)
	}
	got, err := e.s.UpdateWorkoutRecord(e.ctx, e.user, w1.ID, logs.RecordInput{Entries: complete(w1, 4), FinishedAt: nil})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Finalized || got.FinishedAt == nil || !got.FinishedAt.Equal(*done.FinishedAt) || got.Entries[0].Sets[0].Reps != 4 || *got.ProgramID != p.ID || got.Day.Week != 1 {
		t.Fatalf("edited record %+v, want finalized with its finish time and day kept", got)
	}
	if w := e.resolvedWeight(p.ID, 2, 3); w != 80 {
		t.Fatalf("week 2 bench %v after the edit, want 80", w)
	}
	// An ad hoc finished workout just takes the edit.
	adhoc := e.create(logs.NewRecord{Date: "2026-11-12", AdHoc: &logs.AdHoc{Name: "Gym", Entries: w1.Entries}})
	e.finalize(adhoc.ID)
	if got, err := e.s.UpdateWorkoutRecord(e.ctx, e.user, adhoc.ID, logs.RecordInput{Entries: complete(adhoc, 3)}); err != nil || !got.Finalized || got.Entries[0].Sets[0].Reps != 3 {
		t.Fatalf("ad hoc edit %+v, %v", got, err)
	}
}

// A later finished workout is a fact: the replay re-runs its progression from its own stored
// planned values and results and rewrites nothing on it but its snapshot.
func TestEditFinalizedKeepsLaterRecordsAsFacts(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(w1.ID, complete(w1, 5))
	e.finalize(w1.ID) // bench 80 -> 82.5
	w2 := e.create(logs.NewRecord{Date: "2026-11-18"})
	e.update(w2.ID, complete(w2, 5))
	e.finalize(w2.ID) // 82.5 -> 85
	if w := e.resolvedWeight(p.ID, 3, 3); w != 85 {
		t.Fatalf("week 3 bench %v, want 85", w)
	}
	before, err := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w2.ID, UserID: e.user})
	if err != nil {
		t.Fatal(err)
	}
	e.update(w1.ID, complete(w1, 4)) // week 1 fails: 80 stays; week 2 passes at its stored 82.5 x 5: 82.5
	if w := e.resolvedWeight(p.ID, 3, 3); w != 82.5 {
		t.Fatalf("week 3 bench %v after the edit, want 82.5", w)
	}
	after, err := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w2.ID, UserID: e.user})
	if err != nil {
		t.Fatal(err)
	}
	if string(after.StateBefore) == string(before.StateBefore) {
		t.Fatalf("week 2 snapshot unchanged: %s", after.StateBefore)
	}
	before.StateBefore, after.StateBefore = nil, nil
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("week 2 record changed by the replay:\n%+v\n%+v", before, after)
	}
}

// Editing an earlier finished workout replays every later one of the program too, each from the
// state the replay reaches, so a later record's snapshot is refreshed.
func TestEditFinalizedReplaysLaterRecords(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(w1.ID, complete(w1, 5))
	e.finalize(w1.ID)                            // squat 100 -> 102.5
	e.create(logs.NewRecord{Date: "2026-11-12"}) // day B, no progression
	w2 := e.create(logs.NewRecord{Date: "2026-11-13", Week: 2, Day: 1})
	e.update(w2.ID, complete(w2, 5))
	e.finalize(w2.ID) // 102.5 -> 105
	if w := e.resolvedWeight(p.ID, 1, 1); w != 105 {
		t.Fatalf("squat %v, want 105", w)
	}
	before, _ := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w2.ID, UserID: e.user})
	e.update(w1.ID, complete(w1, 4)) // week 1 fails: 100 stays, then w2 lifts it to 102.5
	if w := e.resolvedWeight(p.ID, 1, 1); w != 102.5 {
		t.Fatalf("squat %v after editing week 1, want 102.5", w)
	}
	after, _ := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w2.ID, UserID: e.user})
	if string(after.StateBefore) == string(before.StateBefore) || after.Version != before.Version {
		t.Fatalf("later record snapshot %s -> %s, version %d -> %d, want a new snapshot without a version bump", before.StateBefore, after.StateBefore, before.Version, after.Version)
	}
	if w, d := e.cursor(p.ID); w != 2 || d != 2 {
		t.Fatalf("cursor = %d/%d, want 2/2 kept", w, d)
	}
}

// A record finalized before snapshots existed (migration 0007) has no state before; editing it
// replays the whole program from its activation baseline, so every record gets a snapshot (R81).
func TestEditFinalizedWithoutSnapshotReplaysFromStart(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(w1.ID, complete(w1, 5))
	e.finalize(w1.ID) // squat 100 -> 102.5
	w2 := e.create(logs.NewRecord{Date: "2026-11-13", Week: 2, Day: 1})
	e.update(w2.ID, complete(w2, 5))
	e.finalize(w2.ID) // 102.5 -> 105
	for _, id := range []uuid.UUID{w1.ID, w2.ID} {
		e.clearSnapshot(id)
	}
	e.update(w1.ID, complete(w1, 4)) // week 1 fails: 100 stays, then w2 lifts it to 102.5
	if w := e.resolvedWeight(p.ID, 1, 1); w != 102.5 {
		t.Fatalf("squat %v after editing week 1, want 102.5", w)
	}
	r1, _ := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w1.ID, UserID: e.user})
	r2, _ := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w2.ID, UserID: e.user})
	if r1.StateBefore == nil || r2.StateBefore == nil || string(r1.StateBefore) == string(r2.StateBefore) {
		t.Fatalf("snapshots %s and %s, want both filled from the replay", r1.StateBefore, r2.StateBefore)
	}
	if w, d := e.cursor(p.ID); w != 2 || d != 2 {
		t.Fatalf("cursor = %d/%d, want 2/2 kept", w, d)
	}
}

// Editing a record that holds its snapshot replays from that snapshot only: an earlier record
// without one is left alone.
func TestEditFinalizedWithSnapshotKeepsEarlierRecords(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(w1.ID, complete(w1, 5))
	e.finalize(w1.ID) // squat 100 -> 102.5
	e.clearSnapshot(w1.ID)
	w2 := e.create(logs.NewRecord{Date: "2026-11-13", Week: 2, Day: 1})
	e.update(w2.ID, complete(w2, 5))
	e.finalize(w2.ID)                // 102.5 -> 105
	e.update(w2.ID, complete(w2, 4)) // week 2 fails: 102.5 stays
	if w := e.resolvedWeight(p.ID, 1, 1); w != 102.5 {
		t.Fatalf("squat %v after editing week 2, want 102.5", w)
	}
	if r1, _ := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: w1.ID, UserID: e.user}); r1.StateBefore != nil {
		t.Fatalf("earlier record got snapshot %s, want it left without one", r1.StateBefore)
	}
}

// Deleting a finished workout (R75, the app's "Reset day") keeps the progression it applied
// and the cursor it moved; only the record and the day's done status go.
func TestDeleteFinalizedKeepsProgression(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	r := e.create(logs.NewRecord{Date: today})
	e.update(r.ID, complete(r, 5))
	e.finalize(r.ID)
	if err := e.s.DeleteWorkoutRecord(e.ctx, e.user, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.GetWorkoutRecord(e.ctx, e.user, r.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("get deleted: %v, want ErrNotFound", err)
	}
	if w, d := e.cursor(p.ID); w != 1 || d != 2 {
		t.Fatalf("cursor = %d/%d, want 1/2 kept", w, d)
	}
	if w := e.resolvedWeight(p.ID, 2, 1); w != 102.5 {
		t.Fatalf("week 2 squat %v, want 102.5 kept", w)
	}
	if v := e.day(today); v.Workout != nil || v.Log == nil || v.Log.Status != logs.StatusPlanned {
		t.Fatalf("day %+v, want no workout and the done status reset", v)
	}
}

func TestPutWorkoutRecordForSync(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	pushed := e.create(logs.NewRecord{Date: "2026-11-18"}) // a device's own snapshot of week 2 day 3
	if err := e.s.DeleteWorkoutRecord(e.ctx, e.user, pushed.ID); err != nil {
		t.Fatal(err)
	}
	pushed.Entries = complete(pushed, 5)
	pushed.Finalized, pushed.DeviceID = true, "phone"
	got, err := e.s.PutWorkoutRecord(e.ctx, e.user, pushed)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != pushed.ID || got.Finalized || *got.ProgramID != p.ID || got.Day.Week != 2 || !got.Entries[0].Sets[0].Completed || got.DeviceID != "phone" {
		t.Fatalf("put record %+v, want the device's row with finalized left to the server", got)
	}
	other, err := e.q.CreateUser(e.ctx, sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	foreign := pushed
	foreign.ID = uuid.New()
	if _, err := e.s.PutWorkoutRecord(e.ctx, other.ID, foreign); !errors.Is(err, logs.ErrInvalid) {
		t.Fatalf("foreign program: %v, want ErrInvalid", err)
	}
	pushed.Entries[0].Sets[0].Reps = 4
	if got, err = e.s.PutWorkoutRecord(e.ctx, e.user, pushed); err != nil || got.Entries[0].Sets[0].Reps != 4 || *got.ProgramID != p.ID {
		t.Fatalf("second put %+v, %v", got, err)
	}
	e.finalize(pushed.ID) // 4 reps: bench stays 80 in week 3
	pushed.Entries, pushed.Date = complete(pushed, 5), "2026-11-19"
	got, err = e.s.PutWorkoutRecord(e.ctx, e.user, pushed)
	if err != nil || !got.Finalized || got.Date != "2026-11-18" || got.Entries[0].Sets[0].Reps != 5 {
		t.Fatalf("put over finalized %+v, %v, want the entries taken and the date kept", got, err)
	}
	if w := e.resolvedWeight(p.ID, 3, 3); w != 82.5 {
		t.Fatalf("week 3 bench %v after the pushed edit, want 82.5", w)
	}
}

func TestUpdateDayKeepsClientID(t *testing.T) {
	e := setup(t)
	id := uuid.New()
	log, err := e.s.UpdateDay(e.ctx, e.user, logs.DayLogInput{ID: id, Date: today, Status: logs.StatusRest})
	if err != nil || log.ID != id {
		t.Fatalf("log %+v, %v, want the client's id kept", log, err)
	}
	log, err = e.s.UpdateDay(e.ctx, e.user, logs.DayLogInput{ID: uuid.New(), Date: today, Notes: "later"})
	if err != nil || log.ID != id || log.Status != logs.StatusRest || log.Notes != "later" {
		t.Fatalf("second log %+v, %v, want the date's first id", log, err)
	}
	if _, err := e.s.UpdateDay(e.ctx, e.user, logs.DayLogInput{ID: id, Date: "2026-11-12", Status: logs.StatusRest}); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("taken id on another date: %v, want ErrConflict", err)
	}
}

func TestFinalizeAdvancesCursorFloating(t *testing.T) {
	e := setup(t)
	p := e.activate(floating, nil)
	for i, want := range [][2]int{{1, 2}, {2, 1}, {2, 2}, {1, 1}} {
		r := e.create(logs.NewRecord{Date: monday.AddDate(0, 0, 2+i).Format("2006-01-02")})
		e.finalize(r.ID)
		if w, d := e.cursor(p.ID); w != want[0] || d != want[1] {
			t.Fatalf("after %d finalizes cursor = %d/%d, want %v", i+1, w, d, want)
		}
	}
	if _, err := e.programs.Finish(e.ctx, e.user, p.ID); err != nil {
		t.Fatal(err)
	}
	once := e.activate(noLoop, nil)
	e.finalize(e.create(logs.NewRecord{Date: today}).ID)
	got, err := e.q.GetProgram(e.ctx, sqlc.GetProgramParams{ID: once.ID, UserID: e.user})
	if err != nil || got.Status != programs.StatusFinished {
		t.Fatalf("status %q, %v, want finished after the last day without loop", got.Status, err)
	}
}

func TestSkippedDayNoProgression(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	if d := e.calendar("2026-11-09", "2026-11-09")[0]; d.LogStatus != logs.StatusSkipped {
		t.Fatalf("passed planned day %+v, want skipped", d)
	}
	if w := e.resolvedWeight(p.ID, 2, 1); w != 100 {
		t.Fatalf("week 2 squat %v, want 100 untouched", w)
	}
	ov, err := programs.LoadOverlay(e.ctx, e.q, p.ID)
	if err != nil || len(ov.Weights) != 0 || len(ov.State) != 0 {
		t.Fatalf("overlay %+v, %v, want empty", ov, err)
	}
	log, err := e.s.UpdateDay(e.ctx, e.user, logs.DayLogInput{Date: "2026-11-09", Status: logs.StatusRest, Notes: "sick"})
	if err != nil || log.Status != logs.StatusRest || log.Notes != "sick" {
		t.Fatalf("log %+v, %v", log, err)
	}
	v := e.day("2026-11-09")
	if v.Log == nil || v.Log.Status != logs.StatusRest || v.Plan == nil || v.Plan.Name != "Lower" || v.Workout != nil {
		t.Fatalf("rest day view %+v", v)
	}
}

func TestStartedRecordKeepsSnapshot(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	later := e.create(logs.NewRecord{Date: "2026-11-18"})
	r := e.create(logs.NewRecord{Date: "2026-11-11"})
	e.update(r.ID, complete(r, 5))
	e.finalize(r.ID)
	if w2, w3 := e.resolvedWeight(p.ID, 2, 3), e.resolvedWeight(p.ID, 3, 3); w2 != 82.5 || w3 != 82.5 {
		t.Fatalf("later bench %v, %v, want 82.5", w2, w3)
	}
	got := e.record(later.ID)
	if w := got.Entries[0].Sets[0].Planned.Weight.Value; w != 80 || got.Day.Exercises[0].Sets[0].Weight.Value != 80 {
		t.Fatalf("started record plans %v, want its snapshot of 80", w)
	}
}

func TestUpdateRunsHooksOnPendingSets(t *testing.T) {
	e := setup(t)
	e.activate(hooked, nil)
	r := e.create(logs.NewRecord{Date: today})
	sets := r.Entries[0].Sets
	sets[0].Completed, sets[0].Reps, sets[0].Weight = true, 5, sets[0].Planned.Weight
	r = e.update(r.ID, r.Entries)
	sets = r.Entries[0].Sets
	if sets[0].Planned.Weight.Value != 100 || sets[1].Planned.Weight.Value != 90 || sets[2].Planned.Weight.Value != 90 {
		t.Fatalf("after set 1: %+v, want the hook to change only the pending sets", sets)
	}
	sets[1].Completed, sets[1].Reps, sets[1].Weight = true, 5, sets[1].Planned.Weight
	r = e.update(r.ID, r.Entries)
	if sets = r.Entries[0].Sets; sets[2].Planned.Weight.Value != 90 || !sets[1].Completed || sets[1].Weight.Value != 90 {
		t.Fatalf("after set 2: %+v, want the carried plan kept", sets)
	}
}

func TestUpdateHookDropsPendingSets(t *testing.T) {
	e := setup(t)
	e.activate(shrinking, nil)
	r := e.create(logs.NewRecord{Date: today})
	sets := r.Entries[0].Sets
	sets[0].Completed, sets[0].Reps, sets[0].Weight = true, 5, sets[0].Planned.Weight
	r = e.update(r.ID, r.Entries)
	if sets = r.Entries[0].Sets; len(sets) != 2 || !sets[0].Completed || sets[1].Completed {
		t.Fatalf("after number_of_sets = 2: %+v, want the done set and one pending", sets)
	}
}

func TestRecordsAreOwned(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	other, err := e.q.CreateUser(e.ctx, sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.s.CreateWorkoutRecord(e.ctx, other.ID, logs.NewRecord{Date: today, ProgramID: p.ID, Week: 1, Day: 3})
	if !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("another user's program: %v, want ErrNotFound", err)
	}
	r := e.create(logs.NewRecord{Date: today})
	if _, err := e.s.GetWorkoutRecord(e.ctx, other.ID, r.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("another user's record: %v, want ErrNotFound", err)
	}
	if _, err := e.s.CreateWorkoutRecord(e.ctx, other.ID, logs.NewRecord{ID: r.ID, Date: today, AdHoc: &logs.AdHoc{Name: "Gym"}}); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("taken id: %v, want ErrConflict", err)
	}
	if err := e.s.DeleteWorkoutRecord(e.ctx, e.user, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.GetWorkoutRecord(e.ctx, e.user, r.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("deleted record: %v, want ErrNotFound", err)
	}
}

// Left-side reps logged on a record reach the progression, and a record
// snapshots the exercise's unilateral flag from the plan.
func TestUnilateralRecord(t *testing.T) {
	e := setup(t)
	slug, err := catalog.ResolveName(e.ctx, e.q, e.user, "Squat")
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.SetExerciseData(e.ctx, e.q, e.user, catalog.ExerciseData{Slug: slug, Unilateral: true}); err != nil {
		t.Fatal(err)
	}
	p := e.activate(unilateral, &monday)
	leftReps := func(r logs.WorkoutRecord, n int) []model.RecordEntry {
		entries := complete(r, 5)
		for i := range entries[0].Sets {
			entries[0].Sets[i].RepsLeft = n
		}
		return entries
	}
	w1 := e.create(logs.NewRecord{Date: "2026-11-09"})
	if !w1.Entries[0].Unilateral {
		t.Fatalf("entry %+v, want unilateral from the user's data", w1.Entries[0])
	}
	e.update(w1.ID, leftReps(w1, 4))
	e.finalize(w1.ID)
	if w := e.resolvedWeight(p.ID, 2, 1); w != 100 {
		t.Fatalf("week 2 squat %v after 4 left reps, want 100", w)
	}
	w2 := e.create(logs.NewRecord{Date: "2026-11-16"})
	e.update(w2.ID, leftReps(w2, 5))
	e.finalize(w2.ID)
	if w := e.resolvedWeight(p.ID, 3, 1); w != 102.5 {
		t.Fatalf("week 3 squat %v after 5 left reps, want 102.5", w)
	}
}
