// Package catalog holds the exercise and food catalogs, per-user exercise data and equipment.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// Sentinel errors for the service functions; the API layer maps them to statuses.
var (
	ErrNotFound = errors.New("catalog: not found")
	ErrInvalid  = errors.New("catalog: invalid")
	ErrConflict = errors.New("catalog: already exists")
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Custom is the user-editable part of a custom exercise.
type Custom struct {
	Name             string
	Equipment        string
	Category         string
	PrimaryMuscles   []string
	SecondaryMuscles []string
	Instructions     []string
}

// CreateCustomExercise adds an exercise only userID can see or use.
func CreateCustomExercise(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, c Custom) (sqlc.Exercise, error) {
	name := strings.Join(strings.Fields(c.Name), " ")
	if name == "" {
		return sqlc.Exercise{}, invalid("exercise name is empty")
	}
	c.Equipment = normalizeToken(c.Equipment)
	if err := c.normalizeMuscles(); err != nil {
		return sqlc.Exercise{}, err
	}
	slug := "custom-" + userID.String()[:8] + "-" + slugify(name, c.Equipment)
	_, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: slug, UserID: &userID})
	if err == nil {
		return sqlc.Exercise{}, ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Exercise{}, err
	}
	return upsertCustom(ctx, q, userID, slug, name, c, sqlc.Exercise{})
}

// UpdateCustomExercise edits one of userID's custom exercises. The slug never changes.
func UpdateCustomExercise(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, slug string, c Custom) (sqlc.Exercise, error) {
	name := strings.Join(strings.Fields(c.Name), " ")
	if name == "" {
		return sqlc.Exercise{}, invalid("exercise name is empty")
	}
	cur, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: slug, UserID: &userID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.UserID == nil) {
		return sqlc.Exercise{}, ErrNotFound
	}
	if err != nil {
		return sqlc.Exercise{}, err
	}
	if err := c.normalizeMuscles(); err != nil {
		return sqlc.Exercise{}, err
	}
	return upsertCustom(ctx, q, userID, slug, name, c, cur)
}

func (c *Custom) normalizeMuscles() (err error) {
	if c.PrimaryMuscles, err = normalizeMuscles(c.PrimaryMuscles); err != nil {
		return err
	}
	c.SecondaryMuscles, err = normalizeMuscles(c.SecondaryMuscles)
	return err
}

func upsertCustom(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, slug, name string, c Custom, cur sqlc.Exercise) (sqlc.Exercise, error) {
	ex, err := q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{
		Slug: slug, Name: name, Equipment: normalizeToken(c.Equipment), Aliases: cur.Aliases,
		PrimaryMuscles: c.PrimaryMuscles, SecondaryMuscles: c.SecondaryMuscles, Category: c.Category,
		Instructions: c.Instructions, DefaultWarmup: cur.DefaultWarmup, Images: cur.Images, Source: "custom", UserID: &userID,
	})
	if errors.Is(err, pgx.ErrNoRows) { // the slug belongs to someone else's row
		return sqlc.Exercise{}, ErrConflict
	}
	return ex, err
}

// DeleteCustomExercise removes one of userID's custom exercises.
func DeleteCustomExercise(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, slug string) error {
	n, err := q.DeleteCustomExercise(ctx, sqlc.DeleteCustomExerciseParams{Slug: slug, UserID: &userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// slugify joins parts into a lowercase hyphenated slug without apostrophes.
func slugify(parts ...string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.Join(parts, " ")) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case r == '\'':
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func normalizeToken(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// normalizeName lowercases and collapses whitespace around words and commas,
// so "Squat,bodyweight" and "squat,  Bodyweight" both become "squat, bodyweight".
func normalizeName(s string) string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = normalizeToken(p)
	}
	return strings.Join(parts, ", ")
}

// equipmentTitle is how an equipment token reads in a name suffix.
func equipmentTitle(eq string) string {
	words := strings.Fields(eq)
	for i, w := range words {
		if w == "ez" {
			words[i] = "EZ"
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
