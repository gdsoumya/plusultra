package insights

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// Report is adherence over a date range, workouts and meals apart.
type Report struct{ Workouts, Meals Counts }

// Adherence counts planned, done, partial, skipped and pending days from..to
// inclusive. Dates are the user's local calendar dates and today is the user's
// local today: planned days before it with no result are skipped, later ones pending.
func (s *Service) Adherence(ctx context.Context, userID uuid.UUID, from, to, today time.Time) (Report, error) {
	w, err := s.workoutCounts(ctx, userID, from, to, today)
	if err != nil {
		return Report{}, err
	}
	m, err := s.mealCounts(ctx, userID, from, to, today)
	return Report{Workouts: w, Meals: m}, err
}

// workoutCounts treats a day as planned when a program resolved a workout onto
// it, the user logged it with a workout status, or a workout record exists, so
// unplanned sessions count too. The log is the user's word on a day; without
// one a record counts as done once finalized, else as partial, like the calendar.
func (s *Service) workoutCounts(ctx context.Context, userID uuid.UUID, from, to, today time.Time) (Counts, error) {
	planned, err := s.q.ListPlannedWorkoutDates(ctx, sqlc.ListPlannedWorkoutDatesParams{UserID: userID, FromDate: &from, ToDate: &to})
	if err != nil {
		return Counts{}, err
	}
	logs, err := s.q.ListDays(ctx, sqlc.ListDaysParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return Counts{}, err
	}
	days := map[string]time.Time{}
	results := map[string]outcome{}
	for _, d := range planned {
		if d != nil {
			days[d.Format(time.DateOnly)] = *d
		}
	}
	for _, l := range logs {
		k := l.Date.Format(time.DateOnly)
		switch l.Status {
		case "done":
			results[k] = outDone
		case "partial":
			results[k] = outPartial
		case "skipped":
			results[k] = outSkipped
		case "planned":
		default:
			continue
		}
		days[k] = l.Date
	}
	records, err := s.q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return Counts{}, err
	}
	for _, r := range records {
		k := r.Date.Format(time.DateOnly)
		if _, logged := results[k]; logged {
			continue
		}
		if r.Finalized {
			results[k] = outDone
		} else {
			results[k] = outPartial
		}
		days[k] = r.Date
	}
	var c Counts
	for k, d := range days {
		o, ok := results[k]
		if !ok {
			o = settle(d, today)
		}
		c.add(o)
	}
	c.finish()
	return c, nil
}

func (s *Service) mealCounts(ctx context.Context, userID uuid.UUID, from, to, today time.Time) (Counts, error) {
	days, err := s.mealDays(ctx, userID, from, to)
	if err != nil {
		return Counts{}, err
	}
	var c Counts
	for _, d := range days {
		switch {
		case !d.planned:
		case d.eaten:
			c.add(outDone)
		default:
			c.add(settle(d.date, today))
		}
	}
	c.finish()
	return c, nil
}
