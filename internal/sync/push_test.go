package sync_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/sync"
)

func TestPushLWW(t *testing.T) {
	e := setup(t)
	meals := logs.NewMealService(e.q, e.pool)
	ml, dl := mealLog("2026-11-10", 100), dayLog("2026-11-10", "partial")
	e.push(sync.Rows{MealLogs: []logs.MealLog{ml}, DayLogs: []logs.DayLog{dl}})

	// The server changes both rows after the device last saw version 1.
	srv, err := meals.Update(e.ctx, e.user, ml.ID, logs.MealLogInput{Date: ml.Date, DeviceID: "web"})
	if err != nil || srv.Version != 2 {
		t.Fatalf("server update: %+v, %v", srv, err)
	}
	if _, err := e.q.UpsertDayLog(e.ctx, sqlc.UpsertDayLogParams{UserID: e.user, Date: day(dl.Date), Status: "skipped", DeviceID: "web"}); err != nil {
		t.Fatal(err)
	}

	// The device's edit is older in version but newer in time: it wins.
	ml.Version, ml.UpdatedAt, ml.Meals[0].Items[0].Qty = 1, srv.UpdatedAt.Add(time.Minute), 300
	dl.Version, dl.UpdatedAt, dl.Status = 1, srv.UpdatedAt.Add(time.Minute), "done"
	if res := e.push(sync.Rows{MealLogs: []logs.MealLog{ml}, DayLogs: []logs.DayLog{dl}}); !slices.Contains(res.Applied, ml.ID) || !slices.Contains(res.Applied, dl.ID) {
		t.Fatalf("push = %+v", res)
	}
	got, err := meals.Get(e.ctx, e.user, ml.ID)
	if err != nil || got.Version != 3 || got.DeviceID != "phone" || got.Totals.Kcal != 390 {
		t.Fatalf("meal log after LWW = %+v, %v", got, err)
	}
	d, err := e.q.GetDay(e.ctx, sqlc.GetDayParams{UserID: e.user, Date: day(dl.Date)})
	if err != nil || d.Status != "done" || d.Version != 3 || d.DeviceID != "phone" || d.ID != dl.ID {
		t.Fatalf("day log after LWW = %+v, %v", d, err)
	}

	// A device that saw the latest version applies even with an older clock.
	ml.Version, ml.UpdatedAt = 3, time.Time{}
	if res := e.push(sync.Rows{MealLogs: []logs.MealLog{ml}}); !slices.Contains(res.Applied, ml.ID) {
		t.Fatalf("push at current version = %+v", res)
	}
}

func TestPushStaleVersionConflict(t *testing.T) {
	e := setup(t)
	checkins := logs.NewCheckinService(e.q, e.pool)
	ck, rec := checkin("2026-11-10", 80), record("2026-11-10")
	e.push(sync.Rows{Checkins: []logs.Checkin{ck}, WorkoutRecords: []logs.WorkoutRecord{rec}})

	kg := 81.0
	srv, err := checkins.Update(e.ctx, e.user, ck.ID, logs.CheckinInput{Date: ck.Date, WeightKg: &kg, DeviceID: "web"})
	if err != nil {
		t.Fatal(err)
	}
	srvRec, err := e.q.UpsertWorkoutRecord(e.ctx, sqlc.UpsertWorkoutRecordParams{
		ID: rec.ID, UserID: e.user, Date: day(rec.Date), ResolvedDay: []byte(`{}`), Entries: []byte(`[{"name":"Squat"}]`), DeviceID: "web",
	})
	if err != nil {
		t.Fatal(err)
	}

	old, stale := 79.0, srv.UpdatedAt.Add(-time.Hour)
	ck.Version, ck.UpdatedAt, ck.WeightKg = 1, stale, &old
	rec.Version, rec.UpdatedAt, rec.Entries = 1, stale, []model.RecordEntry{{Name: "Bench"}}
	res := e.push(sync.Rows{Checkins: []logs.Checkin{ck}, WorkoutRecords: []logs.WorkoutRecord{rec}})
	if len(res.Applied) != 0 || len(res.Conflicts.Checkins) != 1 || len(res.Conflicts.WorkoutRecords) != 1 ||
		res.Reasons[ck.ID] != "outdated" || res.Reasons[rec.ID] != "outdated" {
		t.Fatalf("push = %+v", res)
	}
	if c := res.Conflicts.Checkins[0]; c.ID != ck.ID || c.Version != 2 || *c.WeightKg != 81 {
		t.Fatalf("conflict check-in = %+v", c)
	}
	if r := res.Conflicts.WorkoutRecords[0]; r.ID != rec.ID || r.Version != srvRec.Version || len(r.Entries) != 1 || r.Entries[0].Name != "Squat" {
		t.Fatalf("conflict record = %+v", r)
	}
	got, err := checkins.Get(e.ctx, e.user, ck.ID, false)
	if err != nil || *got.WeightKg != 81 || got.Version != 2 {
		t.Fatalf("server check-in changed: %+v, %v", got, err)
	}
}

func TestPushFinalizeRequested(t *testing.T) {
	e := setup(t)
	rec := record("2026-11-10")
	rec.Finalized = true
	if res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{rec}}); !slices.Contains(res.Applied, rec.ID) {
		t.Fatalf("push = %+v", res)
	}
	if !slices.Equal(e.fin.records, []uuid.UUID{rec.ID}) || !slices.Equal(e.fin.users, []uuid.UUID{e.user}) {
		t.Fatalf("finalize calls = %v %v", e.fin.users, e.fin.records)
	}
	got, err := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: rec.ID, UserID: e.user})
	if err != nil || got.Finalized {
		t.Fatalf("only the finalizer may set finalized: %+v, %v", got, err)
	}

	// A finalized record takes edits (R79) without another finalize; finalized stays.
	e.exec(`UPDATE workout_records SET finalized = true, version = version + 1 WHERE id = $1`, rec.ID)
	now := time.Now()
	for _, push := range []func(*logs.WorkoutRecord){
		func(r *logs.WorkoutRecord) { r.Finalized = true },
		func(r *logs.WorkoutRecord) { r.Finalized, r.Entries = false, []model.RecordEntry{{Name: "Bench"}} },
	} {
		r := rec
		r.Version, r.UpdatedAt = got.Version+1, now.Add(time.Hour)
		push(&r)
		if res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{r}}); !slices.Contains(res.Applied, rec.ID) || len(res.Reasons) != 0 {
			t.Fatalf("push to a finalized record = %+v", res)
		}
	}
	row, err := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: rec.ID, UserID: e.user})
	if err != nil {
		t.Fatal(err)
	}
	after, err := logs.DecodeRecord(row)
	if err != nil || !after.Finalized || after.Version != got.Version+3 || len(after.Entries) != 1 || after.Entries[0].Name != "Bench" {
		t.Fatalf("finalized record after edits: %+v, %v", after, err)
	}
	if len(e.fin.records) != 1 {
		t.Fatalf("finalize called %d times", len(e.fin.records))
	}
	// A tombstone deletes it (R75): the app's "Reset day" removes a finished workout.
	tomb := rec
	tomb.Version, tomb.UpdatedAt, tomb.DeletedAt = got.Version+2, now.Add(2*time.Hour), &now
	if res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{tomb}}); !slices.Contains(res.Applied, rec.ID) {
		t.Fatalf("tombstone push = %+v", res)
	}
	if after, err := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: rec.ID, UserID: e.user}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("after tombstone: %+v, %v, want deleted", after, err)
	}
}

func TestPushFinalizeErrorIsConflict(t *testing.T) {
	e := setup(t)
	e.fin.err = errors.New("progression failed")
	rec := record("2026-11-10")
	rec.Finalized = true
	res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{rec}})
	if slices.Contains(res.Applied, rec.ID) || len(res.Conflicts.WorkoutRecords) != 1 {
		t.Fatalf("push = %+v", res)
	}
	if r := res.Conflicts.WorkoutRecords[0]; r.ID != rec.ID || r.Finalized || r.Version != 1 {
		t.Fatalf("conflict record = %+v", r)
	}
	if res.Reasons[rec.ID] != "finalize_failed" {
		t.Fatalf("reasons = %v", res.Reasons)
	}
}

func TestPushTombstone(t *testing.T) {
	e := setup(t)
	rows := sync.Rows{
		DayLogs:        []logs.DayLog{dayLog("2026-11-10", "done")},
		WorkoutRecords: []logs.WorkoutRecord{record("2026-11-10")},
		MealLogs:       []logs.MealLog{mealLog("2026-11-10", 100)},
		Checkins:       []logs.Checkin{checkin("2026-11-10", 80)},
	}
	e.push(rows)
	e.age()
	cur := e.pull("", 0).Cursor

	now := time.Now()
	rows.DayLogs[0].Version, rows.DayLogs[0].DeletedAt = 1, &now
	rows.WorkoutRecords[0].Version, rows.WorkoutRecords[0].DeletedAt = 1, &now
	rows.MealLogs[0].Version, rows.MealLogs[0].DeletedAt = 1, &now
	rows.Checkins[0].Version, rows.Checkins[0].DeletedAt = 1, &now
	unknown := mealLog("2026-11-12", 100)
	unknown.DeletedAt = &now
	rows.MealLogs = append(rows.MealLogs, unknown)
	if res := e.push(rows); len(res.Applied) != 5 {
		t.Fatalf("push = %+v", res)
	}

	e.age()
	c := e.pull(cur, 0)
	if len(c.DayLogs) != 1 || len(c.WorkoutRecords) != 1 || len(c.MealLogs) != 1 || len(c.Checkins) != 1 {
		t.Fatalf("pull after tombstones = %+v", c)
	}
	if c.DayLogs[0].DeletedAt == nil || c.WorkoutRecords[0].DeletedAt == nil || c.MealLogs[0].DeletedAt == nil || c.Checkins[0].DeletedAt == nil {
		t.Fatalf("rows are not tombstones: %+v", c)
	}

	// A device that edited a deleted row after the delete brings it back.
	ml := rows.MealLogs[0]
	ml.Version, ml.DeletedAt, ml.UpdatedAt = 1, nil, time.Now().Add(time.Minute)
	if res := e.push(sync.Rows{MealLogs: []logs.MealLog{ml}}); !slices.Contains(res.Applied, ml.ID) {
		t.Fatalf("resurrect = %+v", res)
	}
	if _, err := e.q.GetMealLog(e.ctx, sqlc.GetMealLogParams{ID: ml.ID, UserID: e.user}); err != nil {
		t.Fatal(err)
	}
}

func TestPushUniquenessCollisionIsConflict(t *testing.T) {
	e := setup(t)
	first := sync.Rows{
		DayLogs:  []logs.DayLog{dayLog("2026-11-10", "done")},
		MealLogs: []logs.MealLog{mealLog("2026-11-10", 100)},
		Checkins: []logs.Checkin{checkin("2026-11-10", 80), checkin("2026-11-11", 80)},
	}
	e.push(first)

	second := sync.Rows{
		DayLogs:  []logs.DayLog{dayLog("2026-11-10", "skipped")},
		MealLogs: []logs.MealLog{mealLog("2026-11-10", 500)},
		Checkins: []logs.Checkin{checkin("2026-11-10", 90)},
	}
	// An existing check-in moved onto a taken date collides too.
	moved := first.Checkins[1]
	moved.Version, moved.Date = 1, "2026-11-10"
	second.Checkins = append(second.Checkins, moved)
	res := e.push(second)
	if len(res.Applied) != 0 {
		t.Fatalf("applied = %v", res.Applied)
	}
	if c := res.Conflicts; len(c.DayLogs) != 1 || c.DayLogs[0].ID != first.DayLogs[0].ID ||
		len(c.MealLogs) != 1 || c.MealLogs[0].ID != first.MealLogs[0].ID ||
		len(c.Checkins) != 2 || c.Checkins[0].ID != first.Checkins[0].ID || c.Checkins[1].ID != first.Checkins[0].ID {
		t.Fatalf("conflicts = %+v", c)
	}
	for _, id := range []uuid.UUID{second.DayLogs[0].ID, second.MealLogs[0].ID, second.Checkins[0].ID, moved.ID} {
		if res.Reasons[id] != "date_taken" {
			t.Fatalf("no reason for %s: %v", id, res.Reasons)
		}
	}
	if ml, err := e.q.GetMealLogByDate(e.ctx, sqlc.GetMealLogByDateParams{UserID: e.user, Date: day("2026-11-10")}); err != nil || ml.ID != first.MealLogs[0].ID {
		t.Fatalf("server meal log = %+v, %v", ml, err)
	}
}

func TestPushRejectsOtherUsersIDs(t *testing.T) {
	e := setup(t)
	other, err := e.q.CreateUser(e.ctx, sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	rows := sync.Rows{
		DayLogs:        []logs.DayLog{dayLog("2026-11-10", "done")},
		WorkoutRecords: []logs.WorkoutRecord{record("2026-11-10")},
		MealLogs:       []logs.MealLog{mealLog("2026-11-10", 100)},
		Checkins:       []logs.Checkin{checkin("2026-11-10", 80)},
	}
	if _, err := e.svc.Push(e.ctx, other.ID, "bobs-phone", rows); err != nil {
		t.Fatal(err)
	}

	rows.DayLogs[0].Date = "2026-11-11" // a free date for ann, but bob's id
	res := e.push(rows)
	if len(res.Applied) != 0 || len(res.Reasons) != 4 {
		t.Fatalf("push = %+v", res)
	}
	for id, reason := range res.Reasons {
		if reason != "invalid" {
			t.Fatalf("reason for %s = %q, want invalid", id, reason)
		}
	}
	if c := res.Conflicts; len(c.DayLogs)+len(c.WorkoutRecords)+len(c.MealLogs)+len(c.Checkins) != 0 {
		t.Fatalf("leaked rows: %+v", c)
	}
	if r, err := e.q.GetWorkoutRecord(e.ctx, sqlc.GetWorkoutRecordParams{ID: rows.WorkoutRecords[0].ID, UserID: other.ID}); err != nil || r.DeviceID != "bobs-phone" {
		t.Fatalf("bob's record = %+v, %v", r, err)
	}
}

func TestPhotoBytesRejected(t *testing.T) {
	e := setup(t)
	ck := checkin("2026-11-10", 80)
	ck.Photo = []byte("\x89PNG\r\n\x1a\n")
	ml := mealLog("2026-11-10", 100)
	if _, err := e.svc.Push(e.ctx, e.user, "phone", sync.Rows{MealLogs: []logs.MealLog{ml}, Checkins: []logs.Checkin{ck}}); !errors.Is(err, sync.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if c := e.pull("", 0); len(c.MealLogs)+len(c.Checkins) != 0 {
		t.Fatalf("rows applied despite the rejected push: %+v", c)
	}

	// A pushed photo id is not trusted either; photos change only through the check-in API.
	ck.Photo, ck.PhotoID = nil, new(uuid.New())
	if res := e.push(sync.Rows{Checkins: []logs.Checkin{ck}}); !slices.Contains(res.Applied, ck.ID) {
		t.Fatalf("push = %+v", res)
	}
	if got, err := e.q.GetCheckin(e.ctx, sqlc.GetCheckinParams{ID: ck.ID, UserID: e.user}); err != nil || got.PhotoID != nil {
		t.Fatalf("check-in = %+v, %v", got, err)
	}
}

func TestPushRejectsUnknownProgram(t *testing.T) {
	e := setup(t)
	bob, err := e.q.CreateUser(e.ctx, sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	program := func(user uuid.UUID) uuid.UUID {
		p, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{UserID: user, Kind: "workout", Name: "P", Script: "x", Structure: []byte(`{}`), Status: "active"})
		if err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	theirs := program(bob.ID)
	rec, missing, ml := record("2026-11-10"), record("2026-11-11"), mealLog("2026-11-10", 100)
	rec.ProgramID, missing.ProgramID, ml.ProgramID = &theirs, new(uuid.New()), &theirs
	res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{rec, missing}, MealLogs: []logs.MealLog{ml}})
	if c := res.Conflicts; len(res.Applied) != 0 || len(c.WorkoutRecords)+len(c.MealLogs) != 0 {
		t.Fatalf("push = %+v", res)
	}
	for _, id := range []uuid.UUID{rec.ID, missing.ID, ml.ID} {
		if res.Reasons[id] != "unknown_program" {
			t.Fatalf("reason for %s = %q", id, res.Reasons[id])
		}
	}

	// A record keeps its program after the program is deleted.
	mine := program(e.user)
	own := record("2026-11-12")
	own.ProgramID = &mine
	e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{own}})
	if _, err := e.q.DeleteProgram(e.ctx, sqlc.DeleteProgramParams{ID: mine, UserID: e.user}); err != nil {
		t.Fatal(err)
	}
	own.Version = 1
	if res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{own}}); !slices.Contains(res.Applied, own.ID) {
		t.Fatalf("push keeping a deleted program = %+v", res)
	}
}

func TestPushKeepsRecordSnapshot(t *testing.T) {
	e := setup(t)
	rec := record("2026-11-10")
	rec.Day = model.ResolvedDay{Week: 1, Day: 2, Name: "Upper"}
	e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{rec}})
	snapshot := `{"Day": 2, "Name": "Upper", "Week": 1, "Exercises": [{"Name": "Bench Press"}]}`
	e.exec(`UPDATE workout_records SET resolved_day = $1 WHERE id = $2`, snapshot, rec.ID)

	// A device sends only the day's header; the planned day stays as the record was created.
	rec.Version, rec.Day, rec.Entries = 1, model.ResolvedDay{Week: 3, Day: 1, Name: "Lower"}, []model.RecordEntry{{Name: "Bench Press"}}
	if res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{rec}}); !slices.Contains(res.Applied, rec.ID) {
		t.Fatalf("push = %+v", res)
	}
	got, err := e.logs.GetWorkoutRecord(e.ctx, e.user, rec.ID)
	if err != nil || got.Day.Name != "Upper" || len(got.Day.Exercises) != 1 || len(got.Entries) != 1 || got.Entries[0].Name != "Bench Press" {
		t.Fatalf("record = %+v, %v", got, err)
	}
}

// Two devices that each started a program's planned day share one record (P2-R41); other days
// of the program and ad hoc workouts on the date stay records of their own.
func TestPushPlannedDayTaken(t *testing.T) {
	e := setup(t)
	p, err := e.q.CreateProgram(e.ctx, sqlc.CreateProgramParams{UserID: e.user, Kind: "workout", Name: "P", Script: "x", Structure: []byte(`{}`), Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	planned := func(week, d int) logs.WorkoutRecord {
		r := record("2026-11-10")
		r.ProgramID, r.Day = &p.ID, model.ResolvedDay{Week: week, Day: d, Name: "Upper"}
		return r
	}
	first := planned(1, 2)
	e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{first}})

	second, otherDay, adHoc := planned(1, 2), planned(1, 3), record("2026-11-10")
	res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{second, otherDay, adHoc}})
	if res.Reasons[second.ID] != "date_taken" || len(res.Conflicts.WorkoutRecords) != 1 || res.Conflicts.WorkoutRecords[0].ID != first.ID {
		t.Fatalf("push of the same planned day = %+v", res)
	}
	if !slices.Contains(res.Applied, otherDay.ID) || !slices.Contains(res.Applied, adHoc.ID) {
		t.Fatalf("other records not applied: %+v", res)
	}

	// Once the first is deleted, the planned day is free again.
	if err := e.logs.DeleteWorkoutRecord(e.ctx, e.user, first.ID); err != nil {
		t.Fatal(err)
	}
	if res := e.push(sync.Rows{WorkoutRecords: []logs.WorkoutRecord{second}}); !slices.Contains(res.Applied, second.ID) {
		t.Fatalf("push after delete = %+v", res)
	}
}

// An item with its macros overridden keeps them and its ref, marked edited (P2-R44), and a
// hand-typed food counted per unit needs no grams (P2-R43).
func TestPushKeepsItemMacros(t *testing.T) {
	e := setup(t)
	edited := model.Item{Food: model.Food{Name: "Oats, rolled", Ref: "usda:173904", Per100g: model.Macros{Kcal: 400, Protein: 15}}, Qty: 80, Unit: "g", Edited: true}
	plate := model.Macros{Kcal: 650, Protein: 20}
	thali := model.Item{Food: model.Food{Name: "Thali", PerUnit: &plate, Unit: "plate"}, Qty: 2, Unit: "plate"}
	ml := logs.MealLog{ID: uuid.New(), Date: "2026-11-10", Meals: []model.Meal{{Name: "Lunch", Items: []model.Item{edited, thali}}}}
	if res := e.push(sync.Rows{MealLogs: []logs.MealLog{ml}}); !slices.Contains(res.Applied, ml.ID) {
		t.Fatalf("push = %+v", res)
	}
	got, err := logs.NewMealService(e.q, e.pool).Get(e.ctx, e.user, ml.ID)
	if err != nil {
		t.Fatal(err)
	}
	items := got.Meals[0].Items
	if f := items[0].Food; f.Ref != edited.Food.Ref || f.Per100g != edited.Food.Per100g || items[0].Macros.Kcal != 320 || !items[0].Edited {
		t.Fatalf("edited item = %+v", items[0])
	}
	if items[1].Unit != "plate" || items[1].Macros.Kcal != 1300 || items[1].Macros.Protein != 40 {
		t.Fatalf("per-unit item = %+v", items[1])
	}
}
