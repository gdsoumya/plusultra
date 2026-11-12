package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

type macrosJSON struct {
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein"`
	Carbs   float64 `json:"carbs"`
	Fat     float64 `json:"fat"`
	Fiber   float64 `json:"fiber"`
	WaterML float64 `json:"water_ml"`
}

type mealJSON struct {
	Items []struct {
		Macros  macrosJSON `json:"macros"`
		Planned bool       `json:"planned"`
	} `json:"items"`
}

// mealDay is one date's meal logs: the target, what was eaten, and whether the
// day had a plan (a target or planned items) and any eaten item.
type mealDay struct {
	date           time.Time
	target, actual model.Macros
	planned, eaten bool
}

// mealDays merges the meal logs of each date, oldest first. When a date has
// several logs the last non-empty target wins and eaten macros add up.
func (s *Service) mealDays(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]mealDay, error) {
	logs, err := s.q.ListMealLogsByDate(ctx, sqlc.ListMealLogsByDateParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return nil, err
	}
	var days []mealDay
	for _, l := range logs {
		var target macrosJSON
		var meals []mealJSON
		if err := json.Unmarshal(l.Target, &target); err != nil {
			return nil, fmt.Errorf("meal log %s target: %w", l.ID, err)
		}
		if err := json.Unmarshal(l.Meals, &meals); err != nil {
			return nil, fmt.Errorf("meal log %s meals: %w", l.ID, err)
		}
		if n := len(days); n == 0 || !days[n-1].date.Equal(l.Date) {
			days = append(days, mealDay{date: l.Date})
		}
		d := &days[len(days)-1]
		if t := model.Macros(target); t != (model.Macros{}) {
			d.target = t
		}
		d.planned = d.planned || target.Kcal > 0
		for _, m := range meals {
			for _, it := range m.Items {
				if it.Planned {
					d.planned = true
					continue
				}
				d.eaten = true
				d.actual = d.actual.Add(model.Macros(it.Macros))
			}
		}
	}
	return days, nil
}
