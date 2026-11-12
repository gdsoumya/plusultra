package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// importFile names imported scripts in error positions.
const importFile = "import.star"

// Result is what Import did: the draft program, or the dates written, and
// what was skipped.
type Result struct {
	Program  *sqlc.Program
	Dates    []string
	Warnings []string
}

// WorkoutImporter records the workouts of imported day logs; the workout
// record service satisfies it.
type WorkoutImporter interface {
	// ImportWorkout creates the workout record of date from the program named
	// w.Program at w.Week and w.Day with w.Entries as done, and finalizes it
	// when finalize is set.
	ImportWorkout(ctx context.Context, userID uuid.UUID, date string, w Workout, finalize bool) error
}

// Import reads text of kind: a program script, saved as a draft with the
// state its export carried, or day logs in the text export format.
func (s *Service) Import(ctx context.Context, userID uuid.UUID, kind, text string) (Result, error) {
	switch kind {
	case KindProgram:
		return s.importProgram(ctx, userID, text)
	case KindDayLog:
		return s.importDays(ctx, userID, text)
	}
	return Result{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("kind must be %s or %s", KindProgram, KindDayLog))
}

func (s *Service) importProgram(ctx context.Context, userID uuid.UUID, text string) (Result, error) {
	script, meal := engine.SplitExport(text)
	var o *model.Overlay
	var foods map[string]model.Food
	kind := programs.KindWorkout
	if meal {
		kind = programs.KindMeal
		_, snap, err := engine.EvaluateSnapshot(text, importFile)
		if err != nil {
			return Result{}, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if len(snap) > 0 {
			foods = snap
		}
	} else {
		_, st, err := engine.EvaluateState(text, importFile)
		if err != nil {
			return Result{}, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if st != nil {
			o = &st.Overlay
		}
	}
	p, warnings, err := s.Programs.Import(ctx, userID, kind, script, o, foods)
	if err != nil {
		return Result{}, err
	}
	return Result{Program: &p, Warnings: warnings}, nil
}

// importDays resolves every food first, so bad text fails before anything
// is written; the dates are then written one at a time, and an error after
// the first write names the dates already written.
func (s *Service) importDays(ctx context.Context, userID uuid.UUID, text string) (Result, error) {
	blocks, err := parseDays(text)
	if err != nil {
		return Result{}, err
	}
	existing := make([]logs.MealLog, len(blocks))
	for i, b := range blocks {
		if b.Meals == nil {
			continue
		}
		l, ok, err := s.existingMealLog(ctx, userID, b.Date)
		if err != nil {
			return Result{}, err
		}
		if ok {
			existing[i] = l
		}
		for j := range b.Meals.Meals {
			if err := s.resolveMeal(ctx, userID, &b.Meals.Meals[j], l.Meals, b.mealLines[j]); err != nil {
				return Result{}, err
			}
		}
	}
	var res Result
	for i, b := range blocks {
		if err := s.writeDay(ctx, userID, b, existing[i], &res.Warnings); err != nil {
			return res, afterWriting(res.Dates, err)
		}
		res.Dates = append(res.Dates, b.Date)
	}
	return res, nil
}

func afterWriting(dates []string, err error) error {
	ce, ok := errors.AsType[*connect.Error](err)
	if len(dates) == 0 || !ok {
		return err
	}
	return connect.NewError(ce.Code(), fmt.Errorf("after writing %s: %s", strings.Join(dates, ", "), ce.Message()))
}

// existingMealLog is the log of date, or a zero log and false.
func (s *Service) existingMealLog(ctx context.Context, userID uuid.UUID, date string) (logs.MealLog, bool, error) {
	l, err := s.Meals.ByDate(ctx, userID, date)
	if errors.Is(err, logs.ErrNotFound) {
		return logs.MealLog{}, false, nil
	}
	return l, err == nil, err
}

// resolveMeal gives each item of m its frozen food: the food of the same
// item in prior, the meal's items in the log the date already has, else the
// catalog food of its name.
func (s *Service) resolveMeal(ctx context.Context, userID uuid.UUID, m *model.Meal, prior []model.Meal, line int) error {
	var known []model.Item
	if i := slices.IndexFunc(prior, func(p model.Meal) bool { return p.Name == m.Name }); i >= 0 {
		known = prior[i].Items
	}
	for i := range m.Items {
		it := &m.Items[i]
		if j := slices.IndexFunc(known, func(k model.Item) bool { return k.Food.Name == it.Food.Name && (it.Unit == "g" || it.Unit == k.Unit) }); j >= 0 {
			it.Food = known[j].Food
			continue
		}
		row, err := catalog.ResolveFoodName(ctx, s.Q, userID, it.Food.Name)
		if un, ok := errors.AsType[*catalog.UnknownNameError](err); ok {
			msg := fmt.Sprintf("unknown food %q", it.Food.Name)
			if len(un.Suggestions) > 0 {
				msg += "; did you mean " + strings.Join(un.Suggestions, ", ") + "?"
			}
			return invalid(line, "%s", msg)
		}
		if err != nil {
			return err
		}
		if it.Food, err = catalog.FrozenFood(row, it.Unit); err != nil {
			if nu, ok := errors.AsType[*catalog.NoUnitError](err); ok {
				return invalid(line, "%v", nu)
			}
			return err
		}
	}
	return nil
}

// writeDay stores one parsed day. Values the text leaves out (readiness,
// body fat, measurements, the target's fiber and water, the meal program)
// are kept from what the date already has.
func (s *Service) writeDay(ctx context.Context, userID uuid.UUID, b *block, existing logs.MealLog, warnings *[]string) error {
	if b.Status != "" {
		if err := s.writeDayLog(ctx, userID, b); err != nil {
			return logsError(b.line, err)
		}
	}
	if b.Checkin != nil {
		if err := s.writeCheckin(ctx, userID, b); err != nil {
			return logsError(b.line, err)
		}
	}
	if b.Meals != nil {
		target := b.Meals.Target
		in := logs.MealLogInput{Date: b.Date, Target: &target, Meals: b.Meals.Meals, DeviceID: "import"}
		var err error
		if existing.ID == uuid.Nil {
			_, err = s.Meals.Create(ctx, userID, in)
		} else {
			target.Fiber, target.WaterML = existing.Target.Fiber, existing.Target.WaterML
			in.ProgramID = existing.ProgramID
			_, err = s.Meals.Update(ctx, userID, existing.ID, in)
		}
		if err != nil {
			return logsError(b.mealLines[0], err)
		}
	}
	if b.Workout != nil {
		if s.Workouts == nil {
			*warnings = append(*warnings, fmt.Sprintf("line %d: workout not imported; workout import is not available", b.workoutLine))
			return nil
		}
		err := s.Workouts.ImportWorkout(ctx, userID, b.Date, *b.Workout, b.Status == logs.StatusDone)
		if errors.Is(err, logs.ErrFinalized) { // finalized records are immutable; the text's copy is not applied
			*warnings = append(*warnings, fmt.Sprintf("line %d: the workout of %s is finalized and was not changed", b.workoutLine, b.Date))
			return nil
		}
		return logsError(b.workoutLine, err)
	}
	return nil
}

// writeDayLog sets the date's status and notes, keeping its readiness.
func (s *Service) writeDayLog(ctx context.Context, userID uuid.UUID, b *block) error {
	in := logs.DayLogInput{Date: b.Date, Status: b.Status, Notes: b.Notes, DeviceID: "import"}
	date, _ := time.Parse(time.DateOnly, b.Date)
	old, err := s.Q.GetDay(ctx, sqlc.GetDayParams{UserID: userID, Date: date})
	switch {
	case err == nil && len(old.Readiness) > 0:
		in.Readiness = new(logs.Readiness)
		if err := json.Unmarshal(old.Readiness, in.Readiness); err != nil {
			return err
		}
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	_, err = s.Logs.UpdateDay(ctx, userID, in)
	return err
}

func (s *Service) writeCheckin(ctx context.Context, userID uuid.UUID, b *block) error {
	in := logs.CheckinInput{Date: b.Date, WeightKg: b.Checkin.WeightKg, DeviceID: "import"}
	page, err := s.Checkins.List(ctx, userID, b.Date, b.Date, "", 1, false)
	if err != nil {
		return err
	}
	if len(page.Items) == 0 {
		_, err = s.Checkins.Create(ctx, userID, in)
		return err
	}
	old := page.Items[0]
	in.BodyFat, in.Measurements = old.BodyFat, old.Measurements
	_, err = s.Checkins.Update(ctx, userID, old.ID, in)
	return err
}
