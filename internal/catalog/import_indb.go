package catalog

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

const sourceINDB = "indb"

// ImportINDB loads a table of the Indian Nutrient Databank repository from a CSV
// export of its xlsx sheet. INDB.xlsx holds 1014 recipes; UK_fct.xlsx and
// US_fct.xlsx hold the raw ingredients behind them (no IFCT table is published
// there). Per-100 g values come from the food_code (or food_code_org, which wins
// when present), food_name, energy_kcal, protein_g, carb_g, fat_g and fibre_g
// columns. servings_unit with unit_serving_energy_kcal becomes a unit, scaled by
// energy. Rows without a numeric energy are skipped. Re-running updates rows by code.
func ImportINDB(ctx context.Context, q *sqlc.Queries, path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // operator-supplied path
	if err != nil {
		return 0, fmt.Errorf("catalog: open dataset: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only
	n := 0
	err = eachRow(f, func(r row) error {
		code, name := r.get("food_code_org"), r.get("food_name")
		if code == "" {
			code = r.get("food_code")
		}
		kcal, ok := parseNum(r.get("energy_kcal"))
		if code == "" || name == "" || !ok {
			return nil
		}
		fr := foodRow{Source: sourceINDB, SourceID: code, Name: name, Per100g: model.Macros{
			Kcal: kcal, Protein: num(r.get("protein_g")), Carbs: num(r.get("carb_g")),
			Fat: num(r.get("fat_g")), Fiber: num(r.get("fibre_g")),
		}}
		if unit, serving := r.get("servings_unit"), num(r.get("unit_serving_energy_kcal")); unit != "" && serving > 0 && kcal > 0 {
			fr.Units = []Unit{{Name: unit, Grams: math.Round(serving/kcal*1000) / 10}}
		}
		if _, err := fr.save(ctx, q, nil); err != nil {
			return fmt.Errorf("catalog: upsert %s: %w", code, err)
		}
		n++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("catalog: read dataset: %w", err)
	}
	return n, nil
}

// parseNum parses a CSV number; blanks, "Tr", "N", "NA" and negatives are not numbers.
func parseNum(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil && v >= 0 && !math.IsInf(v, 0)
}

// num is parseNum with zero for what is not a number.
func num(s string) float64 {
	v, _ := parseNum(s)
	return v
}
