package logs

import (
	"math"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

const (
	maxQuantity = 100000
	maxMeals    = 50
	maxItems    = 200
	gram        = "g"
)

func checkNumber(what string, v, hi float64) error {
	if math.IsNaN(v) || v < 0 || v > hi {
		return invalid("%s must be between 0 and %g", what, hi)
	}
	return nil
}

func checkMacros(what string, m model.Macros) error {
	for _, v := range []float64{m.Kcal, m.Protein, m.Carbs, m.Fat, m.Fiber, m.WaterML} {
		if err := checkNumber(what, v, maxQuantity); err != nil {
			return err
		}
	}
	return nil
}

func checkTarget(t model.Macros) error { return checkMacros("target", t) }

// normalizeMeals validates meals and returns a copy whose item macros are rescaled to
// their quantities. It never trusts macros sent by the client.
func normalizeMeals(meals []model.Meal) ([]model.Meal, error) {
	if len(meals) > maxMeals {
		return nil, invalid("at most %d meals", maxMeals)
	}
	out := make([]model.Meal, len(meals))
	total := 0
	for i, m := range meals {
		if m.Name == "" || len(m.Name) > 100 || len(m.Time) > 16 || len(m.Desc) > 2000 {
			return nil, invalid("meal %d: name must be 1-100 characters, time 16 and desc 2000", i+1)
		}
		total += len(m.Items)
		if total > maxItems {
			return nil, invalid("at most %d items per log", maxItems)
		}
		out[i] = m
		out[i].Items = make([]model.Item, len(m.Items))
		for j, it := range m.Items {
			if err := checkItem(&it); err != nil {
				return nil, invalid("meal %q item %d: %v", m.Name, j+1, err)
			}
			out[i].Items[j] = it
		}
	}
	resolve(out)
	return out, nil
}

// resolve sets each item's macros from its frozen food and quantity.
func resolve(meals []model.Meal) {
	d := model.MealDay{Meals: meals}
	engine.ResolveMealTotals(&d)
}

func checkItem(it *model.Item) error {
	f := &it.Food
	if f.Name == "" || len(f.Name) > 200 || len(f.Ref) > 64 || len(f.Unit) > 32 || len(it.Note) > 500 {
		return invalid("name must be 1-200 characters, ref 64, unit 32 and note 500")
	}
	if err := checkNumber("quantity", it.Qty, maxQuantity); err != nil {
		return err
	}
	if err := checkMacros("per-100g macros", f.Per100g); err != nil {
		return err
	}
	if f.PerUnit != nil {
		if err := checkMacros("per-unit macros", *f.PerUnit); err != nil {
			return err
		}
	}
	if it.Unit == "" {
		it.Unit = gram
	}
	if it.Unit != gram && (f.PerUnit == nil || it.Unit != f.Unit) {
		return invalid("unit %q is not g or the food's unit", it.Unit)
	}
	return nil
}
