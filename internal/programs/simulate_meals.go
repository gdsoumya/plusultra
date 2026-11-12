package programs

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/model"
)

// MealMapping is the meal day planned on one date of a simulated run; Missing
// when the meal program has none for it. Kind is the workout day's kind.
type MealMapping struct {
	Date           time.Time
	Name, Kind     string
	Totals, Target model.Macros
	Missing        bool
}

// mealMapping maps in's meal program onto every date from the first to the
// last of days. A saved program keeps its own start date; a script, or a
// program without one, is anchored at anchor like the workout.
func (s *Service) mealMapping(ctx context.Context, userID uuid.UUID, in *SimulateInput, anchor *time.Time, days []Day) ([]MealMapping, error) {
	if len(days) == 0 || days[0].Date == nil {
		return nil, invalidArgument(errors.New("meals need dated sessions: give anchor_start_date or an anchored program"))
	}
	var m *model.MealProgram
	start := *anchor
	if in.MealProgramID != uuid.Nil {
		p, err := get(ctx, s.Q, userID, in.MealProgramID)
		if err != nil {
			return nil, err
		}
		if p.Kind != KindMeal {
			return nil, invalidArgument(errors.New("meal_program_id is not a meal program"))
		}
		sc, err := load(p)
		if err != nil {
			return nil, err
		}
		m = sc.meal
		if p.StartDate != nil {
			start = monday(*p.StartDate)
		}
	} else {
		sc, err := checked(ctx, s.Q, userID, KindMeal, in.MealScript, nil)
		if err != nil {
			return nil, err
		}
		m = sc.meal
	}
	kinds := map[string]string{}
	for _, d := range days {
		kinds[d.Date.Format(time.DateOnly)] = d.Kind
	}
	var out []MealMapping
	last := *days[len(days)-1].Date
	for date := *days[0].Date; !date.After(last); date = date.AddDate(0, 0, 1) {
		mm := MealMapping{Date: date, Kind: cmp.Or(kinds[date.Format(time.DateOnly)], "rest"), Missing: true}
		if off := int(date.Sub(start).Hours() / 24); off >= 0 && off < m.Weeks*7 {
			if md := m.DayFor(off/7+1, off%7+1, mm.Kind); md != nil {
				mm.Name, mm.Totals, mm.Missing = md.Name, md.Totals(), false
				mm.Target = mm.Totals
				if md.Target != nil {
					mm.Target = *md.Target
				}
			}
		}
		out = append(out, mm)
	}
	return out, nil
}
