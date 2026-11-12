package logs

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

const maxCalendarDays = 366

// CalendarDay is one date of the calendar: the workout planned or done, the
// meal day kind and the day's status. Kind is rest when nothing is planned.
type CalendarDay struct {
	Date                    string
	Kind                    string
	WorkoutWeek, WorkoutDay int
	WorkoutName             string
	MealKind                string
	LogStatus               string
}

// DayView is everything about one date. Plan is nil on a rest day; MealPlan
// carries the meal program's week, day and kind for the date.
type DayView struct {
	Date        string
	Log         *DayLog
	Plan        *model.ResolvedDay
	PlanProgram uuid.UUID
	MealPlan    *programs.Day
	MealProgram uuid.UUID
	Workout     *WorkoutRecord
	Meal        *MealLog
	Checkin     *Checkin
}

// GetCalendar returns one entry per date from from to to, at most 366 days.
// Anchored programs show their future days; a floating program shows only
// its next day, on today.
func (s *Service) GetCalendar(ctx context.Context, userID uuid.UUID, from, to string) ([]CalendarDay, error) {
	f, err := parseDate(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDate(to)
	if err != nil {
		return nil, err
	}
	if t.Before(f) || t.Sub(f) >= maxCalendarDays*24*time.Hour {
		return nil, invalid("the range must be 1-%d days", maxCalendarDays)
	}
	p, err := s.plans(ctx, userID, f, t)
	if err != nil {
		return nil, err
	}
	logRows, err := s.q.ListDays(ctx, sqlc.ListDaysParams{UserID: userID, FromDate: f, ToDate: t})
	if err != nil {
		return nil, err
	}
	logs := map[string]string{}
	for _, l := range logRows {
		logs[l.Date.Format(time.DateOnly)] = l.Status
	}
	recRows, err := s.q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: f, ToDate: t})
	if err != nil {
		return nil, err
	}
	recs := map[string]WorkoutRecord{}
	for _, r := range recRows {
		rec, err := DecodeRecord(r)
		if err != nil {
			return nil, err
		}
		if _, ok := recs[rec.Date]; !ok {
			recs[rec.Date] = rec
		}
	}
	var out []CalendarDay
	for d := f; !d.After(t); d = d.AddDate(0, 0, 1) {
		date := d.Format(time.DateOnly)
		cd := CalendarDay{Date: date, Kind: "rest"}
		plan, planned := p.days[date]
		if planned {
			cd.Kind, cd.WorkoutWeek, cd.WorkoutDay, cd.WorkoutName = cmp.Or(plan.Kind, "rest"), plan.Week, plan.Day, plan.Workout.Name
		}
		rec, done := recs[date]
		if done {
			cd.Kind, cd.WorkoutWeek, cd.WorkoutDay, cd.WorkoutName = rec.Day.Kind, rec.Day.Week, rec.Day.Day, rec.Day.Name
			cd.LogStatus = StatusPartial
			if rec.Finalized {
				cd.LogStatus = StatusDone
			}
		}
		if status, ok := logs[date]; ok {
			cd.LogStatus = status
		} else if !done && planned && isWorkout(plan.Kind) && d.Before(p.today) {
			cd.LogStatus = StatusSkipped
		}
		if md, ok := p.mealDay(date, cd.WorkoutWeek, cd.WorkoutDay, cd.Kind); ok {
			cd.MealKind = md.Kind
		}
		out = append(out, cd)
	}
	return out, nil
}

// GetDay returns everything about date.
func (s *Service) GetDay(ctx context.Context, userID uuid.UUID, date string) (DayView, error) {
	d, err := parseDate(date)
	if err != nil {
		return DayView{}, err
	}
	p, err := s.plans(ctx, userID, d, d)
	if err != nil {
		return DayView{}, err
	}
	v := DayView{Date: date}
	var week, day int
	kind := "rest"
	if plan, ok := p.days[date]; ok {
		week, day, kind = plan.Week, plan.Day, plan.Kind
		if kind != "rest" {
			v.Plan, v.PlanProgram = plan.Workout, p.workout.ID
		}
	}
	if l, err := s.q.GetDay(ctx, sqlc.GetDayParams{UserID: userID, Date: d}); err == nil {
		log, err := DecodeDayLog(l)
		if err != nil {
			return v, err
		}
		v.Log = &log
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	recs, err := s.q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: d, ToDate: d})
	if err != nil {
		return v, err
	}
	if len(recs) > 0 {
		rec, err := DecodeRecord(recs[0])
		if err != nil {
			return v, err
		}
		v.Workout = &rec
		week, day, kind = rec.Day.Week, rec.Day.Day, rec.Day.Kind
	}
	if md, ok := p.mealDay(date, week, day, kind); ok {
		v.MealPlan, v.MealProgram = &md, p.meal.ID
	}
	meal, err := s.meals.ByDate(ctx, userID, date)
	switch {
	case err == nil:
		v.Meal = &meal
	case !errors.Is(err, ErrNotFound):
		return v, err
	}
	if c, err := s.q.GetCheckinByDate(ctx, sqlc.GetCheckinByDateParams{UserID: userID, Date: d}); err == nil {
		ci, err := DecodeCheckin(c)
		if err != nil {
			return v, err
		}
		v.Checkin = &ci
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	return v, nil
}
