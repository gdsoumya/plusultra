package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// daily plans squats every weekday, so the test works whatever today is.
const daily = `program("Daily", weeks=2)
for n in range(1, 8):
    day(1, n, "D%d" % n, weeks="1-2")
    ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
`

const dailyMeals = `meal_program("Eat", weeks=1)
oats = food("Oats", per100g={"kcal": 380})
day(1, kind="training", name="Train")
meal("Breakfast", oats(100))
day(1, kind="rest", name="Rest")
meal("Breakfast", oats(50))
`

type calendarEnv struct {
	*testEnv
	tok  string
	user uuid.UUID
	cal  plusultrav1connect.CalendarServiceClient
	wo   plusultrav1connect.WorkoutServiceClient
}

// calendarUser registers a UTC user who knows Squat and runs daily from this week's Monday.
func (e *testEnv) calendarUser(t *testing.T, name, invite string) calendarEnv {
	t.Helper()
	res, err := e.auth.Register(t.Context(), connect.NewRequest(&pb.RegisterRequest{
		Username: name, Password: password, InviteCode: invite, Timezone: "UTC",
	}))
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(res.Msg.GetUser().GetId())
	if _, err := catalog.CreateCustomExercise(t.Context(), sqlc.New(e.pool), id, catalog.Custom{Name: "Squat"}); err != nil {
		t.Fatal(err)
	}
	return calendarEnv{
		testEnv: e, tok: res.Msg.GetToken(), user: id,
		cal: plusultrav1connect.NewCalendarServiceClient(http.DefaultClient, e.url),
		wo:  plusultrav1connect.NewWorkoutServiceClient(http.DefaultClient, e.url),
	}
}

func (c calendarEnv) activate(t *testing.T, kind, script string, start *time.Time) sqlc.Program {
	t.Helper()
	p, err := c.programs.Create(t.Context(), c.user, kind, script)
	if err != nil {
		t.Fatal(err)
	}
	if p, err = c.programs.Activate(t.Context(), c.user, p.ID, start); err != nil {
		t.Fatal(err)
	}
	return p
}

func (c calendarEnv) day(t *testing.T, date string) *pb.DayView {
	t.Helper()
	res, err := c.cal.GetDay(t.Context(), as(c.tok, &pb.GetDayRequest{Date: date}))
	if err != nil {
		t.Fatalf("get day %s: %v", date, err)
	}
	return res.Msg.GetDay()
}

func squatWeight(t *testing.T, d *pb.ResolvedDay) float64 {
	t.Helper()
	if len(d.GetExercises()) == 0 || len(d.GetExercises()[0].GetSets()) == 0 {
		t.Fatalf("no sets in %v", d)
	}
	return d.GetExercises()[0].GetSets()[0].GetWeight().GetValue()
}

func TestCalendarAndWorkoutsOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	c := e.calendarUser(t, "alice", "")
	ctx := t.Context()
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monday := today.AddDate(0, 0, -(int(today.Weekday())+6)%7)
	day := func(d time.Time) string { return d.Format(time.DateOnly) }
	p := c.activate(t, programs.KindWorkout, daily, &monday)
	c.activate(t, programs.KindMeal, dailyMeals, nil)

	calRes, err := c.cal.GetCalendar(ctx, as(c.tok, &pb.GetCalendarRequest{Dates: &pb.DateRange{From: day(monday), To: day(monday.AddDate(0, 0, 13))}}))
	if err != nil {
		t.Fatal(err)
	}
	days := calRes.Msg.GetDays()
	if len(days) != 14 || days[13].GetWorkoutWeek() != 2 || days[13].GetWorkoutDay() != 7 || days[13].GetWorkoutName() != "D7" || days[13].GetKind() != "training" {
		t.Fatalf("calendar %v", days)
	}
	for i, d := range days {
		want := pb.DayStatus_DAY_STATUS_UNSPECIFIED
		if monday.AddDate(0, 0, i).Before(today) {
			want = pb.DayStatus_DAY_STATUS_SKIPPED
		}
		if d.GetLogStatus() != want || d.GetMealKind() != "training" {
			t.Fatalf("day %d = %v, want status %v and training meals", i, d, want)
		}
	}

	view := c.day(t, day(today))
	if plan := view.GetPlan(); plan.GetProgramId() != p.ID.String() || plan.GetDate() != day(today) || plan.GetWeek() != 1 ||
		squatWeight(t, plan) != 100 || plan.GetExercises()[0].GetSets()[0].GetWeight().GetUnit() != pb.Unit_UNIT_KG {
		t.Fatalf("plan %v", plan)
	}
	if mp := view.GetMealPlan(); mp.GetName() != "Train" || mp.GetKind() != "training" || mp.GetTotals().GetKcal() != 380 || len(mp.GetMeals()) != 1 {
		t.Fatalf("meal plan %v", mp)
	}
	if view.GetLog() != nil || view.GetWorkout() != nil || view.GetMeal() != nil || view.GetCheckin() != nil {
		t.Fatalf("fresh day has logs: %v", view)
	}

	tomorrow := day(today.AddDate(0, 0, 1))
	upd, err := c.cal.UpdateDay(ctx, as(c.tok, &pb.UpdateDayRequest{Date: tomorrow, Status: pb.DayStatus_DAY_STATUS_REST.Enum(),
		Notes: "travel", Readiness: &pb.Readiness{Sleep: proto.Int32(4)}}))
	if err != nil {
		t.Fatal(err)
	}
	if l := upd.Msg.GetLog(); l.GetStatus() != pb.DayStatus_DAY_STATUS_REST || l.GetNotes() != "travel" || l.GetReadiness().GetSleep() != 4 || l.GetVersion() != 1 {
		t.Fatalf("log %v", l)
	}
	if _, err := c.cal.UpdateDay(ctx, as(c.tok, &pb.UpdateDayRequest{Date: tomorrow, Notes: "still travel"})); err != nil {
		t.Fatal(err)
	}
	if l := c.day(t, tomorrow).GetLog(); l.GetStatus() != pb.DayStatus_DAY_STATUS_REST || l.GetNotes() != "still travel" || l.GetVersion() != 2 {
		t.Fatalf("log after notes-only update %v", l)
	}

	id := uuid.NewString()
	created, err := c.wo.CreateWorkoutRecord(ctx, as(c.tok, &pb.CreateWorkoutRecordRequest{Id: id, Date: day(today)}))
	if err != nil {
		t.Fatal(err)
	}
	rec := created.Msg.GetRecord()
	if rec.GetId() != id || rec.GetProgramId() != p.ID.String() || rec.GetWeek() != 1 || rec.GetDayName() != "D"+string(rune('0'+rec.GetDay())) ||
		len(rec.GetEntries()) != 1 || len(rec.GetEntries()[0].GetSets()) != 3 || !strings.HasSuffix(rec.GetEntries()[0].GetSlug(), "squat") ||
		rec.GetEntries()[0].GetSets()[0].GetCompleted() || rec.GetEntries()[0].GetSets()[0].GetPlanned().GetWeight().GetValue() != 100 || rec.GetStartedAt() == nil ||
		rec.GetModified() || !rec.GetEntries()[0].GetFromProgram() {
		t.Fatalf("record %v", rec)
	}
	for _, s := range rec.GetEntries()[0].GetSets() {
		s.Completed, s.Reps, s.Weight = true, 5, &pb.Weight{Value: 100} // unit unspecified: the user's
	}
	rec.GetEntries()[0].GetSets()[2].Edited = true
	updated, err := c.wo.UpdateWorkoutRecord(ctx, as(c.tok, &pb.UpdateWorkoutRecordRequest{Record: rec}))
	if err != nil {
		t.Fatal(err)
	}
	rec = updated.Msg.GetRecord()
	if s := rec.GetEntries()[0].GetSets()[2]; !s.GetCompleted() || s.GetWeight().GetUnit() != pb.Unit_UNIT_KG || rec.GetVersion() != 2 || !s.GetEdited() || !rec.GetModified() {
		t.Fatalf("updated record %v", rec)
	}

	fin, err := c.wo.FinalizeWorkoutRecord(ctx, as(c.tok, &pb.FinalizeWorkoutRecordRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if !fin.Msg.GetRecord().GetFinalized() || fin.Msg.GetRecord().GetFinishedAt() == nil || len(fin.Msg.GetOutput()) != 0 {
		t.Fatalf("finalized %v", fin.Msg)
	}
	if view := c.day(t, day(today)); view.GetLog().GetStatus() != pb.DayStatus_DAY_STATUS_DONE || !view.GetWorkout().GetFinalized() {
		t.Fatalf("today after finalize %v", view)
	}
	if w := squatWeight(t, c.day(t, tomorrow).GetPlan()); w != 102.5 {
		t.Fatalf("tomorrow's squat %v, want 102.5", w)
	}
	_, err = c.wo.FinalizeWorkoutRecord(ctx, as(c.tok, &pb.FinalizeWorkoutRecordRequest{Id: id}))
	assertCode(t, err, connect.CodeFailedPrecondition)
	// Editing the finished workout replays its progression (R79): a failed last set undoes the step.
	rec.GetEntries()[0].GetSets()[2].Reps = 4
	edited, err := c.wo.UpdateWorkoutRecord(ctx, as(c.tok, &pb.UpdateWorkoutRecordRequest{Record: rec}))
	if err != nil || !edited.Msg.GetRecord().GetFinalized() || edited.Msg.GetRecord().GetEntries()[0].GetSets()[2].GetReps() != 4 {
		t.Fatalf("edit finalized: %v, %v", edited, err)
	}
	if w := squatWeight(t, c.day(t, tomorrow).GetPlan()); w != 100 {
		t.Fatalf("tomorrow's squat %v after the edit, want 100", w)
	}

	filled, err := c.cal.QuickFillDay(ctx, as(c.tok, &pb.QuickFillDayRequest{Date: tomorrow, Workout: true, Meals: true, Finalize: true}))
	if err != nil {
		t.Fatal(err)
	}
	v := filled.Msg.GetDay()
	if w := v.GetWorkout(); !w.GetFinalized() || !w.GetEntries()[0].GetSets()[2].GetCompleted() || w.GetEntries()[0].GetSets()[2].GetReps() != 5 {
		t.Fatalf("quick filled workout %v", w)
	}
	if m := v.GetMeal(); m == nil || !m.GetMeals()[0].GetItems()[0].GetPlanned() || m.GetTotals().GetKcal() != 380 || v.GetLog().GetStatus() != pb.DayStatus_DAY_STATUS_DONE {
		t.Fatalf("quick filled day %v", v)
	}
	if w := squatWeight(t, c.day(t, day(today.AddDate(0, 0, 2))).GetPlan()); w != 102.5 {
		t.Fatalf("squat in two days %v, want 102.5", w)
	}

	list, err := c.wo.ListWorkoutRecords(ctx, as(c.tok, &pb.ListWorkoutRecordsRequest{
		Dates: &pb.DateRange{From: day(monday), To: day(monday.AddDate(0, 0, 13))}, ProgramId: p.ID.String(), Limit: 1}))
	if err != nil || len(list.Msg.GetRecords()) != 1 || !list.Msg.GetHasMore() || list.Msg.GetRecords()[0].GetId() != id {
		t.Fatalf("page 1 %v, %v", list.Msg, err)
	}
	list, err = c.wo.ListWorkoutRecords(ctx, as(c.tok, &pb.ListWorkoutRecordsRequest{
		Dates: &pb.DateRange{From: day(monday), To: day(monday.AddDate(0, 0, 13))}, Cursor: list.Msg.GetNextCursor()}))
	if err != nil || len(list.Msg.GetRecords()) != 1 || list.Msg.GetHasMore() || list.Msg.GetRecords()[0].GetDate() != tomorrow {
		t.Fatalf("page 2 %v, %v", list.Msg, err)
	}

	adHoc, err := c.wo.CreateWorkoutRecord(ctx, as(c.tok, &pb.CreateWorkoutRecordRequest{Id: uuid.NewString(), Date: day(today.AddDate(0, 0, 2)),
		Source: &pb.CreateWorkoutRecordRequest_AdHoc{AdHoc: &pb.AdHoc{DayName: "Hotel gym", Entries: []*pb.ExerciseEntry{{
			Name: "Push Up", Sets: []*pb.SetEntry{{Planned: &pb.ResolvedSet{Reps: 20}, Completed: true}}}}}}}))
	if err != nil {
		t.Fatal(err)
	}
	if r := adHoc.Msg.GetRecord(); r.GetProgramId() != "" || r.GetDayName() != "Hotel gym" || r.GetEntries()[0].GetSets()[0].GetCompleted() {
		t.Fatalf("ad hoc record %v", r)
	}
	if _, err := c.wo.FinalizeWorkoutRecord(ctx, as(c.tok, &pb.FinalizeWorkoutRecordRequest{Id: adHoc.Msg.GetRecord().GetId()})); err != nil {
		t.Fatal(err)
	}

	_, err = c.cal.GetDay(ctx, as(c.tok, &pb.GetDayRequest{Date: "2026-13-01"}))
	assertCode(t, err, connect.CodeInvalidArgument)
	_, err = c.wo.GetWorkoutRecord(ctx, as(c.tok, &pb.GetWorkoutRecordRequest{Id: uuid.NewString()}))
	assertCode(t, err, connect.CodeNotFound)
	readOnly := e.key(t, c.tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = c.wo.CreateWorkoutRecord(ctx, as(readOnly, &pb.CreateWorkoutRecordRequest{Id: uuid.NewString(), Date: day(today)}))
	assertCode(t, err, connect.CodePermissionDenied)
	_, err = c.cal.QuickFillDay(ctx, as(readOnly, &pb.QuickFillDayRequest{Date: day(today), Workout: true}))
	assertCode(t, err, connect.CodePermissionDenied)
	if _, err := c.wo.GetWorkoutRecord(ctx, as(readOnly, &pb.GetWorkoutRecordRequest{Id: id})); err != nil {
		t.Fatalf("read key cannot read: %v", err)
	}

	bob := e.calendarUser(t, "bob", e.inviteFor(t, c.tok))
	_, err = bob.wo.GetWorkoutRecord(ctx, as(bob.tok, &pb.GetWorkoutRecordRequest{Id: id}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = bob.wo.CreateWorkoutRecord(ctx, as(bob.tok, &pb.CreateWorkoutRecordRequest{Id: uuid.NewString(), Date: day(today),
		Source: &pb.CreateWorkoutRecordRequest_ProgramDay{ProgramDay: &pb.ProgramDay{ProgramId: p.ID.String(), Week: 1, Day: 1}}}))
	assertCode(t, err, connect.CodeNotFound)

	// Deleting the finished workout keeps the progression it applied (R75).
	if _, err = c.wo.DeleteWorkoutRecord(ctx, as(c.tok, &pb.DeleteWorkoutRecordRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	if view := c.day(t, day(today)); view.GetLog().GetStatus() == pb.DayStatus_DAY_STATUS_DONE || view.GetWorkout() != nil {
		t.Fatalf("today after delete %v", view)
	}
	if w := squatWeight(t, c.day(t, tomorrow).GetPlan()); w != 100 {
		t.Fatalf("tomorrow's squat after delete %v, want 100 as the edit left it", w)
	}
}

func (e *testEnv) inviteFor(t *testing.T, adminTok string) string {
	t.Helper()
	res, err := e.auth.CreateInvite(t.Context(), as(adminTok, &pb.CreateInviteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.GetInvite().GetCode()
}
