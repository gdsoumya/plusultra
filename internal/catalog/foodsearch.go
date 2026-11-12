package catalog

import (
	"context"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// FoodHit is a search result. A concept hit is the concept's preferred (or best ranked) member
// shown under the concept's name, with ConceptID set and OtherSources counting the other
// sources that hold the same food. ConceptID is empty for rows outside any concept.
type FoodHit struct {
	Food
	ConceptID    string
	OtherSources int
}

// SearchFoods finds foods visible to userID (nil for the catalog alone) where
// every word of query starts a word of the name or aliases, so "oat" finds "Oats".
// A food in a food concept also matches the concept's name and aliases, and the
// members of a concept collapse into one hit. A query that matches no word may still find
// a concept by trigram similarity of its name (a typo such as "chiken"); those fuzzy hits
// come after every word or prefix match. The user's custom foods come first, then concept
// hits, then the rest, each by exact and prefix name match and then by text rank, nudged up for INDB, IFCT and Foundation entries and for "raw", down for
// "cooked" or "fried". source limits results to sources with that prefix ("usda" covers both).
func SearchFoods(ctx context.Context, q *sqlc.Queries, userID *uuid.UUID, query, source string, limit int) ([]FoodHit, error) {
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) == 0 || limit <= 0 {
		return nil, nil
	}
	rows, err := q.SearchFoods(ctx, sqlc.SearchFoodsParams{
		Tsq:     strings.Join(words, ":* & ") + ":*",
		Lq:      strings.Join(words, " "),
		UserID:  userID,
		Source:  source,
		MaxRows: int32(min(limit, 200)),
	})
	if err != nil {
		return nil, err
	}
	hits := make([]FoodHit, len(rows))
	for i, r := range rows {
		hits[i] = FoodHit{Food: r.Food, OtherSources: int(r.OtherSources)}
		if r.ConceptID != nil {
			hits[i].ConceptID, hits[i].Name = *r.ConceptID, r.Shown
		}
	}
	return hits, nil
}
