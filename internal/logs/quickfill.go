package logs

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// QuickFill says what QuickFill fills for a date, and whether it finalizes the workout.
type QuickFill struct {
	Workout, Meals, Finalize bool
}

// QuickFill records a date as done exactly as planned: a workout record with
// every planned set completed and a meal log with every planned item. Parts
// already logged, or with nothing planned, are left as they are.
func (s *Service) QuickFill(ctx context.Context, userID uuid.UUID, date string, fill QuickFill) error {
	v, err := s.GetDay(ctx, userID, date)
	if err != nil {
		return err
	}
	if fill.Workout {
		rec := v.Workout
		if rec == nil && v.Plan != nil {
			switch rec, err = s.fillWorkout(ctx, userID, v); {
			case errors.Is(err, ErrConflict): // a concurrent fill got there first
				if v, err = s.GetDay(ctx, userID, date); err != nil {
					return err
				}
				rec = v.Workout
			case err != nil:
				return err
			}
		}
		if fill.Finalize && rec != nil && !rec.Finalized {
			if _, err := s.FinalizeWorkoutRecord(ctx, userID, rec.ID); err != nil {
				return err
			}
		}
	}
	if fill.Meals && v.Meal == nil && v.MealPlan != nil {
		_, err := s.meals.FillFromPlan(ctx, userID, date, v.MealPlan.Meal, v.MealProgram)
		if err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return nil
}

func (s *Service) fillWorkout(ctx context.Context, userID uuid.UUID, v DayView) (*WorkoutRecord, error) {
	r, err := s.CreateWorkoutRecord(ctx, userID, NewRecord{Date: v.Date, ProgramID: v.PlanProgram, Week: v.Plan.Week, Day: v.Plan.Day, OnlyOne: true})
	if err != nil {
		return nil, err
	}
	for i := range r.Entries {
		for j := range r.Entries[i].Sets {
			set := &r.Entries[i].Sets[j]
			set.Completed, set.Reps, set.Weight, set.RPE = true, set.Planned.Reps, set.Planned.Weight, set.Planned.RPE
		}
	}
	now := s.Now()
	r, err = s.UpdateWorkoutRecord(ctx, userID, r.ID, RecordInput{Entries: r.Entries, StartedAt: r.StartedAt, FinishedAt: &now})
	return &r, err
}
