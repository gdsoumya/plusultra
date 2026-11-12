package logs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// plans is what the active programs plan for a range of dates: workout days by
// date (a floating program's next day on today only), anchored meal days by
// date, and a floating meal program to pick from by the workout's kind.
type plans struct {
	today         time.Time
	workout, meal *sqlc.Program
	days          map[string]programs.Day
	mealDays      map[string]programs.Day
	floatingMeal  *model.MealProgram
}

func (s *Service) plans(ctx context.Context, userID uuid.UUID, from, to time.Time) (*plans, error) {
	today, err := s.today(ctx, userID)
	if err != nil {
		return nil, err
	}
	p := &plans{today: today, days: map[string]programs.Day{}, mealDays: map[string]programs.Day{}}
	active, err := s.q.ListProgramsByUserAndStatus(ctx, sqlc.ListProgramsByUserAndStatusParams{UserID: userID, Status: programs.StatusActive})
	if err != nil {
		return nil, err
	}
	for i := range active {
		switch active[i].Kind {
		case programs.KindWorkout:
			p.workout = &active[i]
		case programs.KindMeal:
			p.meal = &active[i]
		}
	}
	if w := p.workout; w != nil {
		if w.StartDate != nil {
			err = datedDays(ctx, s.q, w, from, to, p.days)
		} else if !today.Before(from) && !today.After(to) {
			err = s.cursorDay(ctx, w, p)
		}
		if err != nil {
			return nil, err
		}
	}
	if m := p.meal; m != nil {
		if m.StartDate != nil {
			err = datedDays(ctx, s.q, m, from, to, p.mealDays)
		} else {
			p.floatingMeal, err = programs.LoadMeal(*m)
		}
	}
	return p, err
}

func datedDays(ctx context.Context, q *sqlc.Queries, p *sqlc.Program, from, to time.Time, into map[string]programs.Day) error {
	rows, err := q.ListResolvedDaysByDateRange(ctx, sqlc.ListResolvedDaysByDateRangeParams{ProgramID: p.ID, FromDate: &from, ToDate: &to})
	if err != nil {
		return err
	}
	for _, row := range rows {
		d, err := programs.DayFrom(p.Kind, row)
		if err != nil {
			return err
		}
		into[row.Date.Format(time.DateOnly)] = d
	}
	return nil
}

// cursorDay puts a floating program's next day on today.
func (s *Service) cursorDay(ctx context.Context, w *sqlc.Program, p *plans) error {
	c, err := s.q.GetCursor(ctx, w.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	row, err := s.q.GetResolvedDay(ctx, sqlc.GetResolvedDayParams{ProgramID: w.ID, Week: c.NextWeek, Day: c.NextDay})
	if errors.Is(err, pgx.ErrNoRows) { // a script edit removed the cursor's day; nothing is planned until it moves
		return nil
	}
	if err != nil {
		return err
	}
	d, err := programs.DayFrom(w.Kind, row)
	p.days[p.today.Format(time.DateOnly)] = d
	return err
}

// mealDay is the meal day for date, whose workout is (week, day) of kind.
func (p *plans) mealDay(date string, week, day int, kind string) (programs.Day, bool) {
	switch {
	case p.meal == nil:
		return programs.Day{}, false
	case p.meal.StartDate != nil:
		d, ok := p.mealDays[date]
		return d, ok
	}
	if !isWorkout(kind) {
		kind = "rest"
	}
	md := p.floatingMeal.DayFor(max(week, 1), day, kind)
	if md == nil {
		return programs.Day{}, false
	}
	return programs.Day{Week: max(week, 1), Day: md.Num, Kind: md.Kind, Meal: md}, true
}

func isWorkout(kind string) bool { return kind == "training" || kind == "conditioning" }
