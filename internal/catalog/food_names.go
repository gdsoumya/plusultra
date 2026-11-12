package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// ResolveFoodName maps a food name in a meal script to a catalog food. Matching ignores case and
// spacing. userID's custom foods win, then concept names and aliases (giving the concept's
// preferred member, else its best member present), then catalog names and aliases, then a
// name that is the only one starting with name. Anything else is an *UnknownNameError; for a
// prefix shared by several foods its suggestions are those foods.
func ResolveFoodName(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, name string) (Food, error) {
	n := normalizeName(name)
	f, err := q.ResolveCustomFood(ctx, sqlc.ResolveCustomFoodParams{UserID: &userID, Name: n})
	if !errors.Is(err, pgx.ErrNoRows) {
		return f, err
	}
	if f, ok, err := resolveConcept(ctx, q, n); ok || err != nil {
		return f, err
	}
	rows, err := q.ResolveCatalogFood(ctx, n)
	if err != nil {
		return Food{}, err
	}
	if len(rows) > 0 && (isExactFood(rows[0], n) || len(rows) == 1) {
		return rows[0], nil
	}
	if len(rows) > 1 { // ambiguous prefix: offer the candidates
		un := &UnknownNameError{Kind: "food", Name: name}
		for _, r := range rows[:min(len(rows), maxSuggest)] {
			un.Suggestions = append(un.Suggestions, r.Name)
		}
		return Food{}, un
	}
	return Food{}, unknownFood(ctx, q, userID, name)
}

func resolveConcept(ctx context.Context, q *sqlc.Queries, n string) (Food, bool, error) {
	c, err := q.ResolveFoodConcept(ctx, n)
	if errors.Is(err, pgx.ErrNoRows) {
		return Food{}, false, nil
	}
	if err != nil {
		return Food{}, false, err
	}
	members, err := ListConceptMembers(ctx, q, c.ID) // the preferred member comes first
	if err != nil || len(members) == 0 {
		return Food{}, false, err
	}
	return members[0], true, nil
}

func isExactFood(f Food, n string) bool {
	return normalizeName(f.Name) == n || slices.ContainsFunc(f.Aliases, func(a string) bool { return normalizeName(a) == n })
}

func unknownFood(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, name string) error {
	sug, err := q.SuggestFoodNames(ctx, sqlc.SuggestFoodNamesParams{Name: normalizeName(name), UserID: &userID})
	if err != nil {
		return err
	}
	return &UnknownNameError{Kind: "food", Name: name, Suggestions: sug}
}

// ValidateMealNames resolves the food names of m that carry no macros and no ref, and checks that
// every food ref exists. A ref is "source:source_id" with source one of usda_sr, usda_foundation,
// ifct, indb, reference or off for catalog foods, or custom with the source id of one of userID's own
// custom foods ("custom:<uuid>"); another user's custom food does not exist for userID.
// A food is name-only when Unresolved is set, not when its macros are zero. Each problem is reported once per food() call line.
// The error is for lookup failures, not for unknown names.
func ValidateMealNames(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, m *model.MealProgram) ([]NameError, error) {
	var out []NameError
	type key struct {
		line int
		name string
	}
	seen := map[key]bool{}
	report := func(e NameError) {
		k := key{e.Line, e.Name}
		if !seen[k] {
			seen[k] = true
			out = append(out, e)
		}
	}
	for _, d := range m.Days {
		for _, meal := range d.Meals {
			for _, it := range meal.Items {
				f := it.Food
				switch {
				case f.Ref != "":
					ok, err := foodRefExists(ctx, q, userID, f.Ref)
					if err != nil {
						return nil, err
					}
					if !ok {
						report(NameError{Line: f.Line, Name: f.Ref})
					}
				case f.Unresolved:
					_, err := ResolveFoodName(ctx, q, userID, f.Name)
					var un *UnknownNameError
					switch {
					case errors.As(err, &un):
						report(NameError{Line: f.Line, Name: f.Name, Suggestions: un.Suggestions})
					case err != nil:
						return nil, err
					}
				}
			}
		}
	}
	return out, nil
}

func foodRefExists(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, ref string) (bool, error) {
	source, id, ok := strings.Cut(ref, ":")
	if !ok || source == "" || id == "" {
		return false, nil
	}
	_, err := q.GetFoodBySource(ctx, sqlc.GetFoodBySourceParams{Source: source, SourceID: id, UserID: &userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
