package api

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/insights"
	"github.com/gdsoumya/plusultra/internal/model"
)

type insightServer struct {
	insights *insights.Service
	q        *sqlc.Queries
}

// user is the caller, their local today as the database stores dates (midnight UTC), and their weight unit.
func (s *insightServer) user(ctx context.Context) (uuid.UUID, time.Time, model.Unit, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return userID, time.Time{}, "", err
	}
	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return userID, time.Time{}, "", err
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return userID, today, model.Unit(u.Units), nil
}

// rangeOf parses an optional date range; zero times leave it open.
func rangeOf(r *pb.DateRange) (from, to time.Time, err error) {
	if r == nil {
		return from, to, nil
	}
	if from, err = parseDate(r.GetFrom()); err != nil {
		return from, to, err
	}
	to, err = parseDate(r.GetTo())
	return from, to, err
}

func weightIn(w model.Weight, u model.Unit) *pb.Weight { return weightPB(fromKg(w.Kg(), u)) }

func optWeightIn(w *model.Weight, u model.Unit) *pb.Weight {
	if w == nil {
		return nil
	}
	return weightIn(*w, u)
}

func countsPB(c insights.Counts) *pb.AdherenceCounts {
	return &pb.AdherenceCounts{Planned: i32(c.Planned), Done: i32(c.Done), Partial: i32(c.Partial), Skipped: i32(c.Skipped)}
}

// exerciseName is the display name for slug, empty when the catalog does not know it.
func (s *insightServer) exerciseName(ctx context.Context, userID uuid.UUID, slug string) (string, error) {
	ex, err := s.q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: slug, UserID: &userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return ex.Name, err
}

func (s *insightServer) WeeklySummary(ctx context.Context, req *connect.Request[pb.WeeklySummaryRequest]) (*connect.Response[pb.WeeklySummaryResponse], error) {
	userID, today, unit, err := s.user(ctx)
	if err != nil {
		return nil, err
	}
	start, err := parseDate(req.Msg.GetWeekStart())
	if err != nil {
		return nil, err
	}
	sum, err := s.insights.WeeklySummary(ctx, userID, start, today)
	if err != nil {
		return nil, err
	}
	res := &pb.WeeklySummaryResponse{
		WeekStart: req.Msg.GetWeekStart(), Adherence: countsPB(sum.Adherence), SetsPerMuscle: make(map[string]int32, len(sum.SetsPerMuscle)), SetsPerRegion: make(map[string]int32, len(sum.SetsPerRegion)),
		Macros: &pb.MacroSummary{Target: macrosPB(sum.Macros.TargetAvg), Actual: macrosPB(sum.Macros.ActualAvg)},
		Weight: &pb.WeightSummary{Start: optWeightIn(sum.Weight.Start, unit), End: optWeightIn(sum.Weight.End, unit), Average: optWeightIn(sum.Weight.Avg7, unit)},
	}
	for m, n := range sum.SetsPerMuscle {
		res.SetsPerMuscle[m] = i32(n)
	}
	for m, n := range sum.SetsPerRegion {
		res.SetsPerRegion[m] = i32(n)
	}
	for _, slug := range slices.Sorted(maps.Keys(sum.E1RM)) {
		name, err := s.exerciseName(ctx, userID, slug)
		if err != nil {
			return nil, err
		}
		res.E1Rm = append(res.E1Rm, &pb.LiftEstimate{Slug: slug, Name: name, E1Rm: weightIn(sum.E1RM[slug], unit)})
	}
	return connect.NewResponse(res), nil
}

func (s *insightServer) LiftHistory(ctx context.Context, req *connect.Request[pb.LiftHistoryRequest]) (*connect.Response[pb.LiftHistoryResponse], error) {
	userID, _, unit, err := s.user(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	from, to, err := rangeOf(m.GetDates())
	if err != nil {
		return nil, err
	}
	pts, err := s.insights.LiftHistory(ctx, userID, m.GetSlug(), from, to)
	if err != nil {
		return nil, err
	}
	name, err := s.exerciseName(ctx, userID, m.GetSlug())
	if err != nil {
		return nil, err
	}
	res := &pb.LiftHistoryResponse{Slug: m.GetSlug(), Name: name}
	for _, p := range pts {
		res.Points = append(res.Points, &pb.LiftPoint{Date: dateString(&p.Date), E1Rm: weightIn(p.E1RM, unit),
			BestWeight: weightIn(p.BestSet.Weight, unit), BestReps: i32(p.BestSet.Reps)})
	}
	return connect.NewResponse(res), nil
}

func (s *insightServer) WeightTrend(ctx context.Context, req *connect.Request[pb.WeightTrendRequest]) (*connect.Response[pb.WeightTrendResponse], error) {
	userID, _, unit, err := s.user(ctx)
	if err != nil {
		return nil, err
	}
	from, to, err := rangeOf(req.Msg.GetDates())
	if err != nil {
		return nil, err
	}
	tr, err := s.insights.WeightTrend(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	res := &pb.WeightTrendResponse{WeeklyRate: weightIn(tr.WeeklyRate, unit)}
	for _, p := range tr.Points {
		res.Points = append(res.Points, &pb.WeightPoint{Date: dateString(&p.Date), Weight: weightIn(p.Weight, unit), MovingAverage: weightIn(p.MovingAvg7, unit)})
	}
	return connect.NewResponse(res), nil
}

func (s *insightServer) Adherence(ctx context.Context, req *connect.Request[pb.AdherenceRequest]) (*connect.Response[pb.AdherenceResponse], error) {
	userID, today, _, err := s.user(ctx)
	if err != nil {
		return nil, err
	}
	from, to, err := rangeOf(req.Msg.GetDates())
	if err != nil {
		return nil, err
	}
	rep, err := s.insights.Adherence(ctx, userID, from, to, today)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.AdherenceResponse{Counts: countsPB(rep.Workouts)}), nil
}
