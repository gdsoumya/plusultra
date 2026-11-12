package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// finalizeStub stands in for the workout service's finalize.
type finalizeStub struct {
	mu    sync.Mutex
	calls []uuid.UUID // user id, record id, ...
	err   error
}

func (f *finalizeStub) Finalize(_ context.Context, userID, recordID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, userID, recordID)
	return f.err
}

type syncEnv struct {
	*testEnv
	sync plusultrav1connect.SyncServiceClient
	fin  *finalizeStub
}

func newSyncEnv(t *testing.T) *syncEnv {
	t.Helper()
	fin := &finalizeStub{}
	e := newEnv(t, httpsURL, func(d *Deps) { d.Finalizer = fin })
	return &syncEnv{e, plusultrav1connect.NewSyncServiceClient(http.DefaultClient, e.url), fin}
}

func (e *syncEnv) push(t *testing.T, token string, rows *pb.SyncRows) *pb.PushResponse {
	t.Helper()
	res, err := e.sync.Push(t.Context(), as(token, &pb.PushRequest{DeviceId: "phone", Rows: rows}))
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	return res.Msg
}

func (e *syncEnv) pull(t *testing.T, token, cursor string, limit int32) *pb.PullResponse {
	t.Helper()
	res, err := e.sync.Pull(t.Context(), as(token, &pb.PullRequest{Cursor: cursor, Limit: limit}))
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	return res.Msg
}

// age moves every synced row a minute into the past, beyond the pull's lag.
func (e *syncEnv) age(t *testing.T) {
	t.Helper()
	for _, table := range []string{"day_logs", "workout_records", "meal_logs", "checkins"} {
		if _, err := e.pool.Exec(t.Context(), `UPDATE `+table+` SET updated_at = updated_at - interval '1 minute'`); err != nil {
			t.Fatal(err)
		}
	}
}

func kgPB(v float64) *pb.Weight { return &pb.Weight{Value: v, Unit: pb.Unit_UNIT_KG} }

func dayLogRow(date string) *pb.DayLog {
	return &pb.DayLog{Id: uuid.NewString(), Date: date, Status: pb.DayStatus_DAY_STATUS_DONE, Notes: "felt strong", Readiness: &pb.Readiness{Sleep: proto.Int32(4)}}
}

func recordRow(date string) *pb.WorkoutRecord {
	return &pb.WorkoutRecord{
		Id: uuid.NewString(), Date: date, Week: 1, Day: 2, DayName: "Upper A", Kind: "training", StartedAt: timestamppb.Now(),
		Entries: []*pb.ExerciseEntry{{
			Name: "Bench Press", Alternates: []string{"Bench Press"}, Current: 1, PromptValues: map[string]float64{"rir": 2},
			Sets: []*pb.SetEntry{{Planned: &pb.ResolvedSet{Reps: 5, Weight: kgPB(80), Amrap: true}, Completed: true, Reps: 6, Weight: kgPB(80), Rpe: 8}},
		}},
	}
}

func mealLogRow(date string, grams float64) *pb.MealLog {
	rice := &pb.FrozenFood{Name: "Rice", Per100G: &pb.Macros{Kcal: 130, Carbs: 28}}
	return &pb.MealLog{Id: uuid.NewString(), Date: date, Target: &pb.Macros{Kcal: 2000},
		Meals: []*pb.Meal{{Name: "Lunch", Items: []*pb.MealItem{{Food: rice, Qty: grams, Unit: "g"}}}}}
}

func checkinRow(date string) *pb.Checkin {
	return &pb.Checkin{Id: uuid.NewString(), Date: date, Weight: &pb.Weight{Value: 176.37, Unit: pb.Unit_UNIT_LB}, Measurements: map[string]float64{"waist": 82}}
}

func TestSyncPullPages(t *testing.T) {
	e := newSyncEnv(t)
	tok := e.register(t, "alice", "").GetToken()
	rows := &pb.SyncRows{
		DayLogs: []*pb.DayLog{dayLogRow("2026-11-10")}, WorkoutRecords: []*pb.WorkoutRecord{recordRow("2026-11-10")},
		MealLogs: []*pb.MealLog{mealLogRow("2026-11-10", 200)}, Checkins: []*pb.Checkin{checkinRow("2026-11-10")},
	}
	if res := e.push(t, tok, rows); len(res.GetAppliedIds()) != 4 {
		t.Fatalf("push = %v", res)
	}
	e.age(t)

	first := e.pull(t, tok, "", 3)
	second := e.pull(t, tok, first.GetCursor(), 3)
	if !first.GetHasMore() || second.GetHasMore() || second.GetCursor() == first.GetCursor() {
		t.Fatalf("pages: has_more %v then %v", first.GetHasMore(), second.GetHasMore())
	}
	got := &pb.SyncRows{}
	proto.Merge(got, first.GetRows())
	proto.Merge(got, second.GetRows())
	if len(got.GetDayLogs()) != 1 || len(got.GetWorkoutRecords()) != 1 || len(got.GetMealLogs()) != 1 || len(got.GetCheckins()) != 1 {
		t.Fatalf("rows over two pages = %v", got)
	}

	d := got.GetDayLogs()[0]
	if d.GetId() != rows.DayLogs[0].GetId() || d.GetStatus() != pb.DayStatus_DAY_STATUS_DONE || d.GetNotes() != "felt strong" ||
		d.GetReadiness().GetSleep() != 4 || d.GetReadiness().Stress != nil || d.GetVersion() != 1 || d.GetDeviceId() != "phone" || d.GetUpdatedAt() == nil {
		t.Fatalf("day log = %v", d)
	}
	r := got.GetWorkoutRecords()[0]
	set := r.GetEntries()[0].GetSets()[0]
	if r.GetWeek() != 1 || r.GetDay() != 2 || r.GetDayName() != "Upper A" || r.GetKind() != "training" || r.GetStartedAt() == nil ||
		r.GetEntries()[0].GetPromptValues()["rir"] != 2 || !set.GetPlanned().GetAmrap() || set.GetReps() != 6 || set.GetWeight().GetValue() != 80 || r.GetFinalized() {
		t.Fatalf("record = %v", r)
	}
	m := got.GetMealLogs()[0]
	if m.GetTotals().GetKcal() != 260 || m.GetTarget().GetKcal() != 2000 || m.GetMeals()[0].GetItems()[0].GetMacros().GetCarbs() != 56 {
		t.Fatalf("meal log = %v", m)
	}
	c := got.GetCheckins()[0]
	if math.Abs(c.GetWeight().GetValue()-80) > 0.01 || c.GetWeight().GetUnit() != pb.Unit_UNIT_KG || c.GetMeasurements()["waist"] != 82 {
		t.Fatalf("check-in = %v", c)
	}
}

func TestSyncPushConflicts(t *testing.T) {
	e := newSyncEnv(t)
	tok := e.register(t, "alice", "").GetToken()
	ml := mealLogRow("2026-11-10", 100)
	e.push(t, tok, &pb.SyncRows{MealLogs: []*pb.MealLog{ml}})

	stale := proto.CloneOf(ml)
	stale.UpdatedAt = timestamppb.New(time.Now().Add(-time.Hour))
	taken := mealLogRow("2026-11-10", 300)
	res := e.push(t, tok, &pb.SyncRows{MealLogs: []*pb.MealLog{stale, taken}})
	if len(res.GetAppliedIds()) != 0 || len(res.GetConflicts().GetMealLogs()) != 2 {
		t.Fatalf("push = %v", res)
	}
	for _, c := range res.GetConflicts().GetMealLogs() {
		if c.GetId() != ml.GetId() || c.GetVersion() != 1 || c.GetTotals().GetKcal() != 130 {
			t.Fatalf("conflict = %v, want the server's log", c)
		}
	}
	if r := res.GetReasons(); len(r) != 2 || r[ml.GetId()] != "outdated" || r[taken.GetId()] != "date_taken" {
		t.Fatalf("reasons = %v", r)
	}

	ml.Version, ml.Meals[0].Items[0].Qty = 1, 300
	if res := e.push(t, tok, &pb.SyncRows{MealLogs: []*pb.MealLog{ml}}); len(res.GetAppliedIds()) != 1 || res.GetAppliedIds()[0] != ml.GetId() {
		t.Fatalf("push at the current version = %v", res)
	}
}

func TestSyncFinalizeOnPush(t *testing.T) {
	e := newSyncEnv(t)
	reg := e.register(t, "alice", "")
	rec := recordRow("2026-11-10")
	rec.Finalized = true
	if res := e.push(t, reg.GetToken(), &pb.SyncRows{WorkoutRecords: []*pb.WorkoutRecord{rec}}); len(res.GetAppliedIds()) != 1 {
		t.Fatalf("push = %v", res)
	}
	if len(e.fin.calls) != 2 || e.fin.calls[0].String() != reg.GetUser().GetId() || e.fin.calls[1].String() != rec.GetId() {
		t.Fatalf("finalize calls = %v", e.fin.calls)
	}

	e.fin.err = errors.New("progression failed")
	failing := recordRow("2026-11-11")
	failing.Finalized = true
	res := e.push(t, reg.GetToken(), &pb.SyncRows{WorkoutRecords: []*pb.WorkoutRecord{failing}})
	if c := res.GetConflicts().GetWorkoutRecords(); len(res.GetAppliedIds()) != 0 || len(c) != 1 || c[0].GetId() != failing.GetId() || c[0].GetFinalized() {
		t.Fatalf("push with a failing finalize = %v", res)
	}
	if r := res.GetReasons()[failing.GetId()]; r != "finalize_failed" {
		t.Fatalf("reason = %q", r)
	}
	if logs := e.logs.String(); !strings.Contains(logs, "progression failed") {
		t.Fatalf("finalize error not logged:\n%s", logs)
	}
}

func TestSyncPushNeedsWrite(t *testing.T) {
	e := newSyncEnv(t)
	tok := e.register(t, "alice", "").GetToken()
	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	rows := &pb.SyncRows{DayLogs: []*pb.DayLog{dayLogRow("2026-11-10")}}
	_, err := e.sync.Push(t.Context(), as(ro, &pb.PushRequest{DeviceId: "script", Rows: rows}))
	assertCode(t, err, connect.CodePermissionDenied)
	if _, err := e.sync.Pull(t.Context(), as(ro, &pb.PullRequest{})); err != nil {
		t.Fatalf("pull with a read key: %v", err)
	}
	rw := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ, pb.ApiKeyScope_API_KEY_SCOPE_WRITE).GetKey()
	if res := e.push(t, rw, rows); len(res.GetAppliedIds()) != 1 {
		t.Fatalf("push with a write key = %v", res)
	}
}

func TestSyncRejectsOtherUsersIDsPerRow(t *testing.T) {
	e := newSyncEnv(t)
	alice := e.register(t, "alice", "").GetToken()
	inv, err := e.auth.CreateInvite(t.Context(), as(alice, &pb.CreateInviteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	bob := e.register(t, "bob", inv.Msg.GetInvite().GetCode()).GetToken()
	rows := &pb.SyncRows{
		DayLogs: []*pb.DayLog{dayLogRow("2026-11-10")}, WorkoutRecords: []*pb.WorkoutRecord{recordRow("2026-11-10")},
		MealLogs: []*pb.MealLog{mealLogRow("2026-11-10", 100)}, Checkins: []*pb.Checkin{checkinRow("2026-11-10")},
	}
	e.push(t, alice, rows)

	theirs := proto.CloneOf(rows)
	theirs.DayLogs[0].Date = "2026-11-12" // a free date for bob, but alice's id
	ok := mealLogRow("2026-11-11", 100)
	theirs.MealLogs = append(theirs.MealLogs, ok)
	res := e.push(t, bob, theirs)
	if len(res.GetAppliedIds()) != 1 || res.GetAppliedIds()[0] != ok.GetId() || proto.Size(res.GetConflicts()) != 0 {
		t.Fatalf("bob's push = %v", res)
	}
	for _, id := range []string{rows.DayLogs[0].GetId(), rows.WorkoutRecords[0].GetId(), rows.MealLogs[0].GetId(), rows.Checkins[0].GetId()} {
		if r := res.GetReasons()[id]; r != "invalid" {
			t.Fatalf("reason for %s = %q, want invalid", id, r)
		}
	}
	e.age(t)
	for _, r := range e.pull(t, alice, "", 0).GetRows().GetWorkoutRecords() {
		if r.GetVersion() != 1 {
			t.Fatalf("alice's record changed: %v", r)
		}
	}
}

func TestSyncRejectsBadRequests(t *testing.T) {
	e := newSyncEnv(t)
	tok := e.register(t, "alice", "").GetToken()
	photo := checkinRow("2026-11-10")
	photo.Photo = []byte("\x89PNG\r\n\x1a\n")
	noID := dayLogRow("2026-11-10")
	noID.Id = ""
	for name, rows := range map[string]*pb.SyncRows{
		"photo bytes": {Checkins: []*pb.Checkin{photo}},
		"empty id":    {DayLogs: []*pb.DayLog{noID}},
	} {
		_, err := e.sync.Push(t.Context(), as(tok, &pb.PushRequest{DeviceId: "phone", Rows: rows}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want invalid argument", name, err)
		}
	}
	_, err := e.sync.Pull(t.Context(), as(tok, &pb.PullRequest{Cursor: "not a cursor"}))
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestSyncPullPlan(t *testing.T) {
	e := newSyncEnv(t)
	reg := e.register(t, "alice", "")
	user := uuid.MustParse(reg.GetUser().GetId())
	q := sqlc.New(e.pool)
	ctx := t.Context()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	payload := func(v any) []byte {
		b, err := json.Marshal(v)
		must(err)
		return b
	}

	floating, err := q.CreateProgram(ctx, sqlc.CreateProgramParams{UserID: user, Kind: "workout", Name: "GZCLP", Script: "x", Status: "active",
		Structure: []byte(`{"Weeks":1,"Loop":true,"Days":[{"Week":1,"Day":1,"Name":"A","Kind":"training"},{"Week":1,"Day":2,"Name":"B","Kind":"training"}]}`)})
	must(err)
	for d := int32(1); d <= 2; d++ {
		day := model.ResolvedDay{Week: 1, Day: int(d), Name: "AB"[d-1 : d], Kind: "training", Exercises: []model.ResolvedExercise{{
			Name: "Bench Press", Slug: "bench-press", Current: 1, Sets: []model.ResolvedSet{{Reps: 5, Weight: model.Weight{Value: 60, Unit: model.UnitKg}}},
		}}}
		must(q.UpsertResolvedDay(ctx, sqlc.UpsertResolvedDayParams{ProgramID: floating.ID, Week: 1, Day: d, Kind: "training", Payload: payload(day)}))
	}
	_, err = q.UpsertCursor(ctx, sqlc.UpsertCursorParams{ProgramID: floating.ID, NextWeek: 1, NextDay: 2})
	must(err)

	y, mo, dd := time.Now().UTC().Date()
	today := time.Date(y, mo, dd, 0, 0, 0, 0, time.UTC)
	meals, err := q.CreateProgram(ctx, sqlc.CreateProgramParams{UserID: user, Kind: "meal", Name: "Cut", Script: "x", Status: "active",
		Structure: []byte(`{}`), StartDate: &today})
	must(err)
	oats := model.Item{Food: model.Food{Name: "Oats"}, Qty: 100, Unit: "g", Macros: model.Macros{Kcal: 380, Protein: 13}, Planned: true}
	mealDay := model.MealDay{Name: "Rest day", Meals: []model.Meal{{Name: "Breakfast", Items: []model.Item{oats}}}}
	must(q.UpsertResolvedDay(ctx, sqlc.UpsertResolvedDayParams{ProgramID: meals.ID, Week: 1, Day: 3, Date: &today, Kind: "rest", Payload: payload(mealDay)}))

	res := e.pull(t, reg.GetToken(), "", 0)
	progs := map[string]*pb.Program{}
	for _, p := range res.GetPrograms() {
		progs[p.GetId()] = p
	}
	f, m := progs[floating.ID.String()], progs[meals.ID.String()]
	if len(progs) != 2 || f.GetMode() != pb.ProgramMode_PROGRAM_MODE_FLOATING || f.GetCursor().GetNextDay() != 2 || !f.GetStructure().GetLoop() ||
		len(f.GetStructure().GetDays()) != 2 || f.GetKind() != pb.ProgramKind_PROGRAM_KIND_WORKOUT || f.GetStatus() != pb.ProgramStatus_PROGRAM_STATUS_ACTIVE {
		t.Fatalf("floating program = %v", f)
	}
	if m.GetMode() != pb.ProgramMode_PROGRAM_MODE_ANCHORED || m.GetCursor() != nil || m.GetStartDate() != today.Format(time.DateOnly) || m.GetKind() != pb.ProgramKind_PROGRAM_KIND_MEAL {
		t.Fatalf("meal program = %v", m)
	}

	days := res.GetResolvedDays()
	if len(days) != 2 || days[0].GetDay() != 2 || days[0].GetName() != "B" || days[1].GetDay() != 1 || days[0].GetDate() != "" ||
		days[0].GetProgramId() != floating.ID.String() || days[0].GetExercises()[0].GetSlug() != "bench-press" ||
		days[0].GetExercises()[0].GetSets()[0].GetWeight().GetValue() != 60 {
		t.Fatalf("resolved days = %v", days)
	}
	md := res.GetResolvedMealDays()
	if len(md) != 1 || md[0].GetDate() != today.Format(time.DateOnly) || md[0].GetKind() != "rest" || md[0].GetDay() != 3 ||
		md[0].GetTotals().GetKcal() != 380 || md[0].GetTarget().GetKcal() != 380 || !md[0].GetMeals()[0].GetItems()[0].GetPlanned() {
		t.Fatalf("meal days = %v", md)
	}
}

func TestSyncUnspecifiedUnitIsTheUsers(t *testing.T) {
	e := newSyncEnv(t)
	tok := e.register(t, "alice", "").GetToken()
	if _, err := e.pool.Exec(t.Context(), `UPDATE users SET units = 'lb'`); err != nil {
		t.Fatal(err)
	}
	ck := checkinRow("2026-11-10")
	ck.Weight = &pb.Weight{Value: 176.37}
	rec := recordRow("2026-11-10")
	rec.Entries[0].Sets[0].Weight = &pb.Weight{Value: 135}
	e.push(t, tok, &pb.SyncRows{Checkins: []*pb.Checkin{ck}, WorkoutRecords: []*pb.WorkoutRecord{rec}})
	e.age(t)

	rows := e.pull(t, tok, "", 0).GetRows()
	if w := rows.GetCheckins()[0].GetWeight(); w.GetUnit() != pb.Unit_UNIT_LB || math.Abs(w.GetValue()-176.37) > 0.01 {
		t.Fatalf("check-in weight = %v, want 176.37 lb", w)
	}
	if w := rows.GetWorkoutRecords()[0].GetEntries()[0].GetSets()[0].GetWeight(); w.GetUnit() != pb.Unit_UNIT_LB || w.GetValue() != 135 {
		t.Fatalf("set weight = %v, want 135 lb", w)
	}
	var kg float64
	if err := e.pool.QueryRow(t.Context(), `SELECT weight FROM checkins`).Scan(&kg); err != nil || math.Abs(kg-80) > 0.01 {
		t.Fatalf("stored weight = %v kg, %v", kg, err)
	}
}

func TestSyncPullsTombstones(t *testing.T) {
	e := newSyncEnv(t)
	tok := e.register(t, "alice", "").GetToken()
	rows := &pb.SyncRows{
		DayLogs: []*pb.DayLog{dayLogRow("2026-11-10")}, WorkoutRecords: []*pb.WorkoutRecord{recordRow("2026-11-10")},
		MealLogs: []*pb.MealLog{mealLogRow("2026-11-10", 100)}, Checkins: []*pb.Checkin{checkinRow("2026-11-10")},
	}
	e.push(t, tok, rows)
	gone := timestamppb.Now()
	rows.DayLogs[0].Version, rows.DayLogs[0].DeletedAt = 1, gone
	rows.WorkoutRecords[0].Version, rows.WorkoutRecords[0].DeletedAt = 1, gone
	rows.MealLogs[0].Version, rows.MealLogs[0].DeletedAt = 1, gone
	rows.Checkins[0].Version, rows.Checkins[0].DeletedAt = 1, gone
	if res := e.push(t, tok, rows); len(res.GetAppliedIds()) != 4 {
		t.Fatalf("tombstones = %v", res)
	}
	e.age(t)
	got := e.pull(t, tok, "", 0).GetRows()
	if got.GetDayLogs()[0].GetDeletedAt() == nil || got.GetWorkoutRecords()[0].GetDeletedAt() == nil ||
		got.GetMealLogs()[0].GetDeletedAt() == nil || got.GetCheckins()[0].GetDeletedAt() == nil {
		t.Fatalf("pulled rows are not tombstones: %v", got)
	}
}
