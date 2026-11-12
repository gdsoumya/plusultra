package engine

import (
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

const nameOnly = `meal_program("N", weeks=1)
oats = food("Oats")
rice = food("Rice", ref="usda_sr:1")
banana = food("Banana", ref="usda_sr:2", per100g={"kcal": 89})
day(1, 1)
meal("M", oats(50), rice(100), banana(100))
`

func TestNameOnlyFoodIsUnresolved(t *testing.T) {
	p, err := EvaluateMeal(nameOnly, "n.star")
	if err != nil {
		t.Fatal(err)
	}
	items := p.Days[0].Meals[0].Items
	if f := items[0].Food; !f.Unresolved || f.Line != 2 || f.Per100g != (model.Macros{}) {
		t.Errorf("oats = %+v, want an unresolved food from line 2", f)
	}
	if f := items[1].Food; !f.Unresolved || f.Ref != "usda_sr:1" || f.Per100g != (model.Macros{}) {
		t.Errorf("rice = %+v, want an unresolved ref food", f)
	}
	if f := items[2].Food; f.Unresolved {
		t.Errorf("banana = %+v, want its own macros", f)
	}
	if p.Days[0].Target.Kcal != 89 {
		t.Errorf("target = %+v", p.Days[0].Target)
	}
}

func TestCatalogFoodsFillNameAndRefFoods(t *testing.T) {
	foods := map[string]model.Food{
		"Oats":      {Ref: "usda_sr:9", Per100g: model.Macros{Kcal: 380, Protein: 13}},
		"usda_sr:1": {Ref: "usda_sr:1", Per100g: model.Macros{Kcal: 130}},
		"Banana":    {Ref: "usda_sr:2", Per100g: model.Macros{Kcal: 1}},
	}
	p, err := EvaluateMealWith(nameOnly, "n.star", foods)
	if err != nil {
		t.Fatal(err)
	}
	items := p.Days[0].Meals[0].Items
	if f := items[0].Food; f.Unresolved || f.Ref != "usda_sr:9" || items[0].Macros != (model.Macros{Kcal: 190, Protein: 6.5}) {
		t.Errorf("oats = %+v %+v, want catalog macros and its ref", f, items[0].Macros)
	}
	if items[1].Macros.Kcal != 130 || items[2].Macros.Kcal != 89 { // explicit macros win
		t.Errorf("rice %v, banana %v kcal", items[1].Macros.Kcal, items[2].Macros.Kcal)
	}
	if p.Days[0].Target.Kcal != 190+130+89 {
		t.Errorf("target %+v, want the filled sum", p.Days[0].Target)
	}

	// A snapshot in the script, by name or ref, wins: an exported script evaluates as saved.
	p, err = EvaluateMealWith(nameOnly+`catalog_snapshot({"Oats": {"kcal": 400}, "usda_sr:1": {"kcal": 100}})`, "n.star", foods)
	if err != nil {
		t.Fatal(err)
	}
	if items = p.Days[0].Meals[0].Items; items[0].Macros.Kcal != 200 || items[1].Macros.Kcal != 100 || items[0].Food.Unresolved {
		t.Errorf("with snapshot: oats %+v, rice %+v", items[0], items[1])
	}
}
