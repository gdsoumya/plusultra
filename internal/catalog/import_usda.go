package catalog

import (
	"archive/zip"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// usdaSources maps the FoodData Central data types we import to a food source.
var usdaSources = map[string]string{"foundation_food": "usda_foundation", "sr_legacy_food": "usda_sr"}

// Atwater energy (nutrient 957 and 958) is the only kcal Foundation foods carry; 208 wins when present.
const (
	nbrKcal     = "208"
	nbrAtwaterA = "957"
	nbrAtwaterB = "958"
)

type usdaFood struct {
	row      foodRow
	atwater  float64
	hasPlain bool
}

// ImportUSDA loads Foundation and SR Legacy foods from a FoodData Central CSV
// release zip. Nutrients are per 100 g; portions become units. Re-running
// updates rows by fdc_id.
func ImportUSDA(ctx context.Context, q *sqlc.Queries, zipPath string) (int, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, fmt.Errorf("catalog: open dataset: %w", err)
	}
	defer zr.Close() //nolint:errcheck // read-only
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[path.Base(f.Name)] = f // releases nest the CSVs in a folder
	}
	each := func(name string, fn func(row) error) error {
		f, ok := files[name]
		if !ok {
			return fmt.Errorf("catalog: dataset has no %s", name)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close() //nolint:errcheck // read-only
		return eachRow(rc, fn)
	}

	foods := map[string]*usdaFood{}
	if err := each("food.csv", func(r row) error {
		if src, ok := usdaSources[r.get("data_type")]; ok {
			foods[r.get("fdc_id")] = &usdaFood{row: foodRow{Source: src, SourceID: r.get("fdc_id"), Name: r.get("description")}}
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("catalog: read food.csv: %w", err)
	}

	nutrients := map[string]string{} // nutrient id -> nutrient_nbr
	if err := each("nutrient.csv", func(r row) error {
		nutrients[r.get("id")] = r.get("nutrient_nbr")
		return nil
	}); err != nil {
		return 0, fmt.Errorf("catalog: read nutrient.csv: %w", err)
	}
	if err := each("food_nutrient.csv", func(r row) error {
		if f := foods[r.get("fdc_id")]; f != nil {
			f.set(nutrients[r.get("nutrient_id")], num(r.get("amount")))
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("catalog: read food_nutrient.csv: %w", err)
	}

	measures := map[string]string{}
	if err := each("measure_unit.csv", func(r row) error {
		measures[r.get("id")] = r.get("name")
		return nil
	}); err != nil {
		return 0, fmt.Errorf("catalog: read measure_unit.csv: %w", err)
	}
	if err := each("food_portion.csv", func(r row) error {
		if f := foods[r.get("fdc_id")]; f != nil {
			f.addPortion(r, measures)
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("catalog: read food_portion.csv: %w", err)
	}

	for _, id := range slices.Sorted(maps.Keys(foods)) {
		f := foods[id]
		if !f.hasPlain {
			f.row.Per100g.Kcal = f.atwater
		}
		if _, err := f.row.save(ctx, q, nil); err != nil {
			return 0, fmt.Errorf("catalog: upsert fdc %s: %w", id, err)
		}
	}
	return len(foods), nil
}

func (f *usdaFood) set(nbr string, v float64) {
	m := &f.row.Per100g
	switch nbr {
	case nbrKcal:
		m.Kcal, f.hasPlain = v, true
	case nbrAtwaterA, nbrAtwaterB:
		f.atwater = max(f.atwater, v)
	case "203":
		m.Protein = v
	case "205":
		m.Carbs = v
	case "204":
		m.Fat = v
	case "291":
		m.Fiber = v
	case "255":
		m.WaterML = v // g of water is ml
	}
}

// addPortion records one household measure as grams per single unit. SR Legacy
// names a portion by measure unit and modifier ("egg", "large"), Foundation by
// description ("1 medium").
func (f *usdaFood) addPortion(r row, measures map[string]string) {
	amount, grams := num(r.get("amount")), num(r.get("gram_weight"))
	if amount == 0 || grams == 0 {
		return
	}
	name := r.get("portion_description")
	if name == "" {
		measure := measures[r.get("measure_unit_id")]
		if measure == "undetermined" {
			measure = ""
		}
		name = strings.Trim(strings.Join([]string{measure, r.get("modifier")}, ", "), ", ")
	}
	// Food.units keys allow 100 characters; the longest USDA measure is about 80.
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
	}
	if name == "" || slices.ContainsFunc(f.row.Units, func(u Unit) bool { return u.Name == name }) {
		return
	}
	f.row.Units = append(f.row.Units, Unit{Name: name, Grams: grams / amount})
}
