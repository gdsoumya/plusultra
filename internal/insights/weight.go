package insights

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// avgDays is the moving average window, so a window needs avgDays-1 days of lookback.
const avgDays = 7

// WeightPoint is a body weight check-in with the 7-day moving average ending on it.
type WeightPoint struct {
	Date               time.Time
	Weight, MovingAvg7 model.Weight
}

// Trend is body weight over a range. WeeklyRate is the least-squares slope of
// the raw check-in weights in kg per week.
type Trend struct {
	Points     []WeightPoint
	WeeklyRate model.Weight
}

type weighIn struct {
	date time.Time
	kg   float64
}

// WeightTrend returns the check-in weights from..to with moving averages and
// the weekly rate. Averages also use the six days before from, so the first
// point is not skewed by the window edge.
func (s *Service) WeightTrend(ctx context.Context, userID uuid.UUID, from, to time.Time) (Trend, error) {
	ws, avg, first, err := s.weighIns(ctx, userID, from, to)
	if err != nil {
		return Trend{}, err
	}
	ws, avg = ws[first:], avg[first:]
	t := Trend{Points: make([]WeightPoint, len(ws)), WeeklyRate: inKg(weeklySlope(ws))}
	for i, w := range ws {
		t.Points[i] = WeightPoint{w.date, inKg(w.kg), inKg(avg[i])}
	}
	return t, nil
}

// weighIns returns the check-in weights from lookback days before from through
// to, their moving averages, and the index of the first one on or after from.
func (s *Service) weighIns(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]weighIn, []float64, int, error) {
	lo := from
	if !from.IsZero() {
		lo = from.AddDate(0, 0, 1-avgDays)
	}
	lo, to = span(lo, to)
	rows, err := s.q.ListCheckinsByDate(ctx, sqlc.ListCheckinsByDateParams{UserID: userID, FromDate: lo, ToDate: to})
	if err != nil {
		return nil, nil, 0, err
	}
	var ws []weighIn
	for _, c := range rows {
		v, err := c.Weight.Float64Value()
		if err != nil {
			return nil, nil, 0, err
		}
		if v.Valid {
			ws = append(ws, weighIn{c.Date, v.Float64})
		}
	}
	first := 0
	for first < len(ws) && ws[first].date.Before(from) {
		first++
	}
	return ws, movingAvg(ws), first, nil
}

// movingAvg averages the check-ins in the 7 days ending on each one, inclusive.
// ws must be ordered by date.
func movingAvg(ws []weighIn) []float64 {
	out := make([]float64, len(ws))
	lo, sum := 0, 0.0
	for i, w := range ws {
		sum += w.kg
		for !ws[lo].date.After(w.date.AddDate(0, 0, -avgDays)) {
			sum -= ws[lo].kg
			lo++
		}
		out[i] = sum / float64(i-lo+1)
	}
	return out
}

// weeklySlope fits weight against the day of each check-in and returns the slope per week.
func weeklySlope(ws []weighIn) float64 {
	n := float64(len(ws))
	if n < 2 {
		return 0
	}
	var sx, sy, sxy, sxx float64
	for _, w := range ws {
		x := w.date.Sub(ws[0].date).Hours() / 24
		sx, sy, sxy, sxx = sx+x, sy+w.kg, sxy+x*w.kg, sxx+x*x
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return 0
	}
	return 7 * (n*sxy - sx*sy) / den
}
