package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

const (
	maxPlates      = 20
	maxFixedWeight = 200
)

// ExerciseData is what a user records about one exercise.
type ExerciseData struct {
	Slug     string
	RM1      *float64
	Rounding *model.Rounding
	Notes    string
	// Unilateral exercises are done one side at a time and logged per side.
	Unilateral bool
}

// GetExerciseData returns ErrNotFound when userID has recorded nothing for slug.
func GetExerciseData(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, slug string) (ExerciseData, error) {
	row, err := q.GetUserExerciseData(ctx, sqlc.GetUserExerciseDataParams{UserID: userID, ExerciseSlug: slug})
	if errors.Is(err, pgx.ErrNoRows) {
		return ExerciseData{}, ErrNotFound
	}
	if err != nil {
		return ExerciseData{}, err
	}
	d := ExerciseData{Slug: slug, Notes: row.Notes, Unilateral: row.Unilateral}
	if f, err := row.Rm1.Float64Value(); err == nil && f.Valid {
		d.RM1 = &f.Float64
	}
	if row.Rounding != nil {
		d.Rounding = new(model.Rounding)
		if err := json.Unmarshal(row.Rounding, d.Rounding); err != nil {
			return ExerciseData{}, err
		}
	}
	return d, nil
}

// SetExerciseData replaces what userID recorded for d.Slug.
func SetExerciseData(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, d ExerciseData) error {
	p := sqlc.UpsertUserExerciseDataParams{UserID: userID, ExerciseSlug: d.Slug, Notes: d.Notes, Unilateral: d.Unilateral}
	if d.RM1 != nil {
		if !(*d.RM1 > 0) || math.IsInf(*d.RM1, 0) {
			return invalid("1RM must be positive")
		}
		if err := p.Rm1.Scan(strconv.FormatFloat(*d.RM1, 'f', -1, 64)); err != nil {
			return err
		}
	}
	if d.Rounding != nil {
		if err := validateRounding(*d.Rounding); err != nil {
			return err
		}
		var err error
		if p.Rounding, err = json.Marshal(d.Rounding); err != nil {
			return err
		}
	}
	if _, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: d.Slug, UserID: &userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	_, err := q.UpsertUserExerciseData(ctx, p)
	return err
}

func validateRounding(r model.Rounding) error {
	switch r.Kind {
	case model.RoundPlates:
		if len(r.Plates) > maxPlates {
			return invalid("at most %d plates", maxPlates)
		}
		if !(r.Bar >= 0) || math.IsInf(r.Bar, 0) {
			return invalid("bar weight must not be negative")
		}
		return positive(r.Plates, "plate")
	case model.RoundFixed:
		if len(r.Plates) == 0 || len(r.Plates) > maxFixedWeight {
			return invalid("fixed rounding needs 1 to %d weights", maxFixedWeight)
		}
		return positive(r.Plates, "weight")
	case model.RoundStack:
		if !(r.Increment > 0) || math.IsInf(r.Increment, 0) {
			return invalid("increment must be positive")
		}
	case model.RoundBodyweight:
	default:
		return invalid("unknown rounding kind %q", r.Kind)
	}
	return nil
}

func positive(vs []float64, what string) error {
	for _, v := range vs {
		if !(v > 0) || math.IsInf(v, 0) {
			return invalid("%s weights must be positive", what)
		}
	}
	return nil
}
