package catalog

import (
	"context"
	_ "embed" // the default concepts
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

//go:embed data/food-concepts.csv
var foodConceptsCSV string

// FoodConcept groups the rows of different sources that are the same food, such as raw white
// rice from IFCT and USDA. Members and Preferred are "source:source_id" keys of foods.
type FoodConcept struct {
	ID, Name, Category string
	Aliases            []string
	Preferred          string
	Members            []string
}

// FoodConcepts parses the embedded food-concepts.csv.
func FoodConcepts() ([]FoodConcept, error) {
	return parseFoodConcepts(strings.NewReader(foodConceptsCSV))
}

func parseFoodConcepts(src io.Reader) ([]FoodConcept, error) {
	var out []FoodConcept
	seen := map[string]bool{}
	err := eachRow(src, func(r row) error {
		c := FoodConcept{
			ID: r.get("concept_id"), Name: r.get("name"), Category: r.get("category"), Aliases: splitList(r.get("aliases")),
			Preferred: r.get("preferred_source") + ":" + r.get("preferred_source_id"), Members: splitList(r.get("member_keys")),
		}
		switch {
		case c.ID == "" || c.Name == "":
			return invalid("food concept %q needs an id and a name", c.ID)
		case seen[c.ID]:
			return invalid("food concept %q is listed twice", c.ID)
		case !slices.Contains(c.Members, c.Preferred):
			return invalid("food concept %q: preferred %s is not among its members", c.ID, c.Preferred)
		}
		seen[c.ID] = true
		out = append(out, c)
		return nil
	})
	return out, err
}

// TxBeginner starts transactions; a *pgxpool.Pool is one.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ImportFoodConcepts loads the concept layer from the CSV at path, or the embedded default when
// path is empty. It sets concept_id on every listed catalog food that exists, clears assignments
// the file no longer lists and drops concepts it no longer names, all in one transaction. It
// returns the foods assigned.
// Members that are not imported yet are skipped, so it is safe to run after any food import.
func ImportFoodConcepts(ctx context.Context, db TxBeginner, path string) (int, error) {
	var src io.Reader = strings.NewReader(foodConceptsCSV)
	if path != "" {
		f, err := os.Open(path) //nolint:gosec // operator-supplied path
		if err != nil {
			return 0, fmt.Errorf("catalog: open food concepts: %w", err)
		}
		defer f.Close() //nolint:errcheck // read-only
		src = f
	}
	concepts, err := parseFoodConcepts(src)
	if err != nil {
		return 0, fmt.Errorf("catalog: read food concepts: %w", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("catalog: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := sqlc.New(tx)

	assigned, missing := 0, 0
	ids := make([]string, 0, len(concepts))
	for _, c := range concepts {
		ids = append(ids, c.ID)
		ps, pid, _ := strings.Cut(c.Preferred, ":")
		if err := q.UpsertFoodConcept(ctx, sqlc.UpsertFoodConceptParams{
			ID: c.ID, Name: c.Name, Aliases: c.Aliases, Category: c.Category, PreferredSource: ps, PreferredSourceID: pid,
		}); err != nil {
			return 0, fmt.Errorf("catalog: upsert concept %s: %w", c.ID, err)
		}
		got, err := q.AssignConceptMembers(ctx, sqlc.AssignConceptMembersParams{ConceptID: &c.ID, Keys: c.Members})
		if err != nil {
			return 0, fmt.Errorf("catalog: assign concept %s: %w", c.ID, err)
		}
		assigned += len(got)
		for _, m := range c.Members {
			if !slices.ContainsFunc(got, func(g sqlc.AssignConceptMembersRow) bool { return g.Source+":"+g.SourceID == m }) {
				missing++
				slog.Debug("concept member not imported", "concept", c.ID, "member", m)
			}
		}
		if err := q.ClearStaleConceptMembers(ctx, sqlc.ClearStaleConceptMembersParams{ConceptID: &c.ID, Keys: c.Members}); err != nil {
			return 0, fmt.Errorf("catalog: clear concept %s: %w", c.ID, err)
		}
	}
	if err := q.PruneFoodConcepts(ctx, ids); err != nil {
		return 0, fmt.Errorf("catalog: prune concepts: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("catalog: commit food concepts: %w", err)
	}
	if missing > 0 {
		slog.Info("food concept members not imported, skipped", "missing", missing)
	}
	return assigned, nil
}

// ListConceptMembers returns the foods of a concept, the preferred one first.
func ListConceptMembers(ctx context.Context, q *sqlc.Queries, conceptID string) ([]Food, error) {
	return q.ListConceptMembers(ctx, &conceptID)
}
