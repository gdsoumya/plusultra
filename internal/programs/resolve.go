package programs

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Day is a resolved day of a program. Date is set when the program is
// anchored. Workout is set for a workout program and Meal for a meal program;
// Day is 0 for a floating meal day picked by Kind.
type Day struct {
	Week, Day int
	Date      *time.Time
	Kind      string
	Workout   *model.ResolvedDay
	Meal      *model.MealDay
}

// Range selects resolved days, bounds inclusive and zero meaning open: by
// date for days that have one, and by week for every day.
type Range struct {
	From, To         time.Time
	FromWeek, ToWeek int
}

func (r Range) has(d Day) bool {
	if d.Date != nil && (!r.From.IsZero() && d.Date.Before(r.From) || !r.To.IsZero() && d.Date.After(r.To)) {
		return false
	}
	return (r.FromWeek == 0 || d.Week >= r.FromWeek) && (r.ToWeek == 0 || d.Week <= r.ToWeek)
}

// ResolvedDays returns program id's resolved days in r, in week then day order.
func (s *Service) ResolvedDays(ctx context.Context, userID, id uuid.UUID, r Range) ([]Day, error) {
	p, err := get(ctx, s.Q, userID, id)
	if err != nil {
		return nil, err
	}
	var days []Day
	if p.Kind == KindMeal && p.StartDate == nil {
		sc, err := load(p)
		if err != nil {
			return nil, err
		}
		days = mealDays(sc.meal)
	} else {
		rows, err := s.Q.ListResolvedDays(ctx, id)
		if err != nil {
			return nil, err
		}
		days = make([]Day, len(rows))
		for i, row := range rows {
			if days[i], err = DayFrom(p.Kind, row); err != nil {
				return nil, err
			}
		}
	}
	return slices.DeleteFunc(days, func(d Day) bool { return !r.has(d) }), nil
}

// Resolve recomputes an active program's days from today on, from its script
// and progression state.
func (s *Service) Resolve(ctx context.Context, userID, id uuid.UUID) error {
	return s.resolve(ctx, userID, id, false)
}

// ResolveAll recomputes every stored day of an active program, past weeks
// included, so a change to the user's exercise data leaves no stale day behind.
// Per-set overrides are keyed by week and day, so past weeks keep their history.
func (s *Service) ResolveAll(ctx context.Context, userID, id uuid.UUID) error {
	return s.resolve(ctx, userID, id, true)
}

func (s *Service) resolve(ctx context.Context, userID, id uuid.UUID, all bool) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		p, err := lock(ctx, q, userID, id)
		if err != nil {
			return err
		}
		if p.Status != StatusActive {
			return failedPrecondition(errors.New("program is not active"))
		}
		sc, err := load(p)
		if err != nil {
			return err
		}
		var from time.Time
		if !all {
			if from, err = s.today(ctx, q, userID); err != nil {
				return err
			}
		}
		_, err = s.resolveFrom(ctx, q, p, sc, from)
		return err
	})
}

// resolveFrom resolves p, evaluated as sc, and stores its days dated from
// from on, and its floating days. A workout program's calendar decides the
// active meal program's days, so those follow from today or from, whichever
// is later; past meal days keep their plan. It returns every day of p.
func (s *Service) resolveFrom(ctx context.Context, q *sqlc.Queries, p sqlc.Program, sc script, from time.Time) ([]Day, error) {
	if err := q.DeleteResolvedDaysFrom(ctx, sqlc.DeleteResolvedDaysFromParams{ProgramID: p.ID, FromDate: &from}); err != nil {
		return nil, err
	}
	var days []Day
	var err error
	switch {
	case sc.workout != nil:
		days, err = workoutDays(ctx, q, p, sc.workout)
	case p.StartDate != nil:
		days, err = anchoredMealDays(ctx, q, p.UserID, sc.meal, *p.StartDate)
	default:
		return mealDays(sc.meal), nil // nothing about floating meal days is per user, so they are not stored
	}
	if err != nil {
		return nil, err
	}
	for _, d := range days {
		if d.Date != nil && d.Date.Before(from) {
			continue
		}
		row, err := d.row(p.ID)
		if err != nil {
			return nil, err
		}
		if err := q.UpsertResolvedDay(ctx, row); err != nil {
			return nil, err
		}
	}
	if sc.workout == nil {
		return days, nil
	}
	today, err := s.today(ctx, q, p.UserID)
	if err != nil {
		return nil, err
	}
	if from.After(today) {
		today = from
	}
	return days, s.resolveMeals(ctx, q, p.UserID, today)
}

// workoutDays resolves workout program p for its user: every date of an
// anchored program, undeclared ones as rest days, or every declared day of a
// floating one.
func workoutDays(ctx context.Context, q *sqlc.Queries, p sqlc.Program, prog *model.Program) ([]Day, error) {
	st, sl, err := userState(ctx, q, p.UserID, prog)
	if err != nil {
		return nil, err
	}
	if st.Overlay, err = LoadOverlay(ctx, q, p.ID); err != nil {
		return nil, err
	}
	if p.StartDate == nil {
		rds, err := engine.Resolve(prog, st)
		if err != nil {
			return nil, failedPrecondition(err)
		}
		days := make([]Day, len(rds))
		for i := range rds {
			sl.set(&rds[i])
			days[i] = Day{Week: rds[i].Week, Day: rds[i].Day, Kind: rds[i].Kind, Workout: &rds[i]}
		}
		return days, nil
	}
	days := schedule(*p.StartDate, prog.Weeks)
	for i := range days {
		rd, err := engine.ResolveDay(prog, st, days[i].Week, days[i].Day)
		if err != nil {
			return nil, failedPrecondition(err)
		}
		sl.set(rd)
		days[i].Kind, days[i].Workout = rd.Kind, rd
	}
	return days, nil
}

func (d Day) row(programID uuid.UUID) (sqlc.UpsertResolvedDayParams, error) {
	var v any = d.Meal
	if d.Workout != nil {
		v = d.Workout
	}
	payload, err := json.Marshal(v)
	return sqlc.UpsertResolvedDayParams{ProgramID: programID, Week: int32(d.Week), Day: int32(d.Day), //nolint:gosec // weeks and days are small
		Date: d.Date, Kind: d.Kind, Payload: payload}, err
}

// DayFrom decodes a stored resolved day of a program of kind.
func DayFrom(kind string, row sqlc.ResolvedDay) (Day, error) {
	d := Day{Week: int(row.Week), Day: int(row.Day), Date: row.Date, Kind: row.Kind}
	if kind == KindWorkout {
		d.Workout = new(model.ResolvedDay)
		return d, json.Unmarshal(row.Payload, d.Workout)
	}
	d.Meal = new(model.MealDay)
	return d, json.Unmarshal(row.Payload, d.Meal)
}
