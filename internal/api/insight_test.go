package api

import (
	"encoding/json"
	"math"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

const lbPerKg = 1 / 0.45359237

func date(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

// lb reports whether w is wantKg expressed in pounds.
func lb(w *pb.Weight, wantKg float64) bool {
	return w.GetUnit() == pb.Unit_UNIT_LB && math.Abs(w.GetValue()-wantKg*lbPerKg) < 1e-3
}

func TestInsightsOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	e.seedBench(t)
	reg := e.register(t, "alice", "")
	tok := reg.GetToken()
	e.setUnits(t, tok, pb.Unit_UNIT_LB)
	userID := uuid.MustParse(reg.GetUser().GetId())
	q := sqlc.New(e.pool)
	c := plusultrav1connect.NewInsightServiceClient(http.DefaultClient, e.url)

	// Tuesday 2026-11-10: a finalized bench session of 5 x 100 kg, logged as done, 80 then 81 kg on the scale, lunch eaten.
	entries, err := json.Marshal([]model.RecordEntry{{Name: "Bench Press, Barbell", Slug: "bench-press-barbell",
		Sets: []model.RecordSet{{Completed: true, Reps: 5, Weight: model.RecordWeight{Value: 100, Unit: model.UnitKg}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertWorkoutRecord(ctx, sqlc.UpsertWorkoutRecordParams{
		ID: uuid.New(), UserID: userID, Date: date("2026-11-10"), ResolvedDay: []byte(`{}`), Entries: entries, Finalized: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertDayLog(ctx, sqlc.UpsertDayLogParams{UserID: userID, Date: date("2026-11-10"), Status: "done"}); err != nil {
		t.Fatal(err)
	}
	checkins := plusultrav1connect.NewCheckinServiceClient(http.DefaultClient, e.url)
	for d, kg := range map[string]float64{"2026-11-09": 80, "2026-11-10": 81} {
		call(checkins.CreateCheckin(ctx, as(tok, &pb.CreateCheckinRequest{Checkin: &pb.Checkin{Id: uuid.NewString(), Date: d, Weight: &pb.Weight{Value: kg, Unit: pb.Unit_UNIT_KG}}}))).ok(t)
	}
	meals := plusultrav1connect.NewMealServiceClient(http.DefaultClient, e.url)
	call(meals.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-10", &pb.Meal{Name: "Lunch", Items: []*pb.MealItem{oats(100)}})}))).ok(t)

	const e1rm = 100 * (1 + 5.0/30)
	week := call(c.WeeklySummary(ctx, as(tok, &pb.WeeklySummaryRequest{WeekStart: "2026-11-09"}))).ok(t)
	if week.GetWeekStart() != "2026-11-09" || week.GetAdherence().GetPlanned() != 1 || week.GetAdherence().GetDone() != 1 || week.GetSetsPerMuscle()["chest"] != 1 ||
		len(week.GetE1Rm()) != 1 || week.GetE1Rm()[0].GetSlug() != "bench-press-barbell" || week.GetE1Rm()[0].GetName() != "Bench Press, Barbell" || !lb(week.GetE1Rm()[0].GetE1Rm(), e1rm) ||
		week.GetMacros().GetTarget().GetKcal() != 2000 || week.GetMacros().GetActual().GetKcal() != 380 ||
		!lb(week.GetWeight().GetStart(), 80) || !lb(week.GetWeight().GetEnd(), 81) || !lb(week.GetWeight().GetAverage(), 80.5) {
		t.Fatalf("weekly summary = %v", week)
	}
	_, err = c.WeeklySummary(ctx, as(tok, &pb.WeeklySummaryRequest{WeekStart: "2026-11-10"}))
	assertCode(t, err, connect.CodeInvalidArgument)

	hist := call(c.LiftHistory(ctx, as(tok, &pb.LiftHistoryRequest{Slug: "bench-press-barbell"}))).ok(t)
	if hist.GetSlug() != "bench-press-barbell" || hist.GetName() != "Bench Press, Barbell" || len(hist.GetPoints()) != 1 {
		t.Fatalf("lift history = %v", hist)
	}
	if p := hist.GetPoints()[0]; p.GetDate() != "2026-11-10" || p.GetBestReps() != 5 || !lb(p.GetBestWeight(), 100) || !lb(p.GetE1Rm(), e1rm) {
		t.Fatalf("lift point = %v", p)
	}
	before := call(c.LiftHistory(ctx, as(tok, &pb.LiftHistoryRequest{Slug: "bench-press-barbell", Dates: &pb.DateRange{From: "2026-11-01", To: "2026-11-09"}}))).ok(t)
	if len(before.GetPoints()) != 0 || before.GetName() != "Bench Press, Barbell" {
		t.Fatalf("lift history before = %v", before)
	}

	trend := call(c.WeightTrend(ctx, as(tok, &pb.WeightTrendRequest{Dates: &pb.DateRange{From: "2026-11-01", To: "2026-11-30"}}))).ok(t)
	if len(trend.GetPoints()) != 2 || trend.GetPoints()[0].GetDate() != "2026-11-09" || !lb(trend.GetPoints()[1].GetWeight(), 81) ||
		!lb(trend.GetPoints()[1].GetMovingAverage(), 80.5) || !lb(trend.GetWeeklyRate(), 7) {
		t.Fatalf("weight trend = %v", trend)
	}
	_, err = c.WeightTrend(ctx, as(tok, &pb.WeightTrendRequest{}))
	assertCode(t, err, connect.CodeInvalidArgument)

	adh := call(c.Adherence(ctx, as(tok, &pb.AdherenceRequest{Dates: &pb.DateRange{From: "2026-11-09", To: "2026-11-15"}}))).ok(t)
	if adh.GetCounts().GetPlanned() != 1 || adh.GetCounts().GetDone() != 1 || adh.GetCounts().GetSkipped() != 0 {
		t.Fatalf("adherence = %v", adh)
	}

	bob := e.invitee(t, tok, "bob").GetToken()
	other := call(c.WeeklySummary(ctx, as(bob, &pb.WeeklySummaryRequest{WeekStart: "2026-11-09"}))).ok(t)
	if other.GetAdherence().GetPlanned() != 0 || len(other.GetE1Rm()) != 0 || other.GetWeight().GetStart() != nil || other.GetMacros().GetActual().GetKcal() != 0 {
		t.Fatalf("bob sees alice's week: %v", other)
	}
	if hist := call(c.LiftHistory(ctx, as(bob, &pb.LiftHistoryRequest{Slug: "bench-press-barbell"}))).ok(t); len(hist.GetPoints()) != 0 {
		t.Fatalf("bob sees alice's lifts: %v", hist)
	}
}
