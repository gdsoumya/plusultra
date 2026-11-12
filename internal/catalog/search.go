package catalog

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const (
	defaultLimit = 20
	maxLimit     = 100
	maxSuggest   = 3
)

// SearchExercises finds catalog and userID's custom exercises by name or alias.
// An empty query, equipment or muscle applies no such filter.
func SearchExercises(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, query, equipment, muscle string, limit int) ([]sqlc.Exercise, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	return q.SearchCatalog(ctx, sqlc.SearchCatalogParams{
		UserID: &userID, Q: strings.TrimSpace(query), Equipment: normalizeToken(equipment),
		Muscle: normalizeToken(muscle), MaxRows: int32(min(limit, maxLimit)), //nolint:gosec // capped above
	})
}

// UnknownNameError is returned for a name that is not in the catalog. Kind is "food" or,
// when empty, "exercise".
type UnknownNameError struct {
	Kind        string
	Name        string
	Suggestions []string
}

func (e *UnknownNameError) Error() string {
	msg := fmt.Sprintf("unknown %s %q", cmp.Or(e.Kind, "exercise"), e.Name)
	if len(e.Suggestions) > 0 {
		msg += fmt.Sprintf(" (did you mean %q?)", strings.Join(e.Suggestions, `", "`))
	}
	return msg
}

// ResolveName maps a program exercise name to a slug. "Bench Press" is the
// default variant, "Bench Press, Dumbbell" a specific one. Matching ignores
// case and spacing and covers userID's custom exercises.
func ResolveName(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, name string) (string, error) {
	ex, err := q.ResolveExerciseName(ctx, sqlc.ResolveExerciseNameParams{UserID: &userID, Name: normalizeName(name)})
	if err == nil {
		return ex.Slug, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	similar, err := SearchExercises(ctx, q, userID, name, "", "", maxSuggest)
	if err != nil {
		return "", err
	}
	un := &UnknownNameError{Name: name}
	for _, s := range similar {
		un.Suggestions = append(un.Suggestions, s.Name)
	}
	return "", un
}
