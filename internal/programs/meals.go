package programs

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// resolveMeals re-resolves userID's active anchored meal program from from on.
func (s *Service) resolveMeals(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, from time.Time) error {
	p, err := activeProgram(ctx, q, userID, KindMeal)
	if err != nil || p == nil || p.StartDate == nil {
		return err
	}
	sc, err := load(*p)
	if err != nil {
		return err
	}
	_, err = s.resolveFrom(ctx, q, *p, sc, from)
	return err
}

// anchoredMealDays picks m's day for each date from start: the numbered day
// of that position, else the day of the kind the workout calendar has on that
// date, rest when it has nothing. Dates with no matching day are left out.
func anchoredMealDays(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, m *model.MealProgram, start time.Time) ([]Day, error) {
	kinds, err := workoutCalendar(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	days := schedule(start, m.Weeks)
	out := days[:0]
	for _, d := range days {
		d.Kind = cmp.Or(kinds[d.Date.Format(time.DateOnly)], "rest")
		if d.Meal = m.DayFor(d.Week, d.Day, d.Kind); d.Meal != nil {
			out = append(out, d)
		}
	}
	return out, nil
}

// workoutCalendar maps the dates of userID's active anchored workout program to their day kinds.
func workoutCalendar(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (map[string]string, error) {
	w, err := activeProgram(ctx, q, userID, KindWorkout)
	if err != nil || w == nil || w.StartDate == nil {
		return nil, err
	}
	rows, err := q.ListResolvedDays(ctx, w.ID)
	kinds := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Date != nil {
			kinds[r.Date.Format(time.DateOnly)] = r.Kind
		}
	}
	return kinds, err
}

// mealDays lists each week of floating meal program m: its numbered days,
// then a day for each kind it declares, which may come from an earlier week.
func mealDays(m *model.MealProgram) []Day {
	var nums []int
	var kinds []string
	for _, d := range m.Days {
		switch {
		case d.Num > 0:
			nums = append(nums, d.Num)
		case !slices.Contains(kinds, d.Kind):
			kinds = append(kinds, d.Kind)
		}
	}
	slices.Sort(nums)
	nums = slices.Compact(nums)
	var days []Day
	for w := 1; w <= m.Weeks; w++ {
		for _, n := range nums {
			if d := m.DayFor(w, n, ""); d != nil {
				days = append(days, Day{Week: w, Day: n, Kind: d.Kind, Meal: d})
			}
		}
		for _, k := range kinds {
			if d := m.DayFor(w, 0, k); d != nil {
				days = append(days, Day{Week: w, Kind: k, Meal: d})
			}
		}
	}
	return days
}
