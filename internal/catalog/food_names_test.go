package catalog_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

func TestResolveFoodNamePrecedence(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	ann, bob := newUser(ctx, t, q, "ann"), newUser(ctx, t, q, "bob")
	if _, err := q.UpsertFood(ctx, sqlc.UpsertFoodParams{Source: "t", SourceID: "1", Name: "Basmati", Per100g: []byte(`{}`), Units: []byte(`[]`)}); err != nil {
		t.Fatal(err)
	}
	mine, err := catalog.CreateCustomFood(ctx, q, ann, catalog.CustomFood{Name: "Basmati", Per100g: model.Macros{Kcal: 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		who         string
		name        string
		source, id  string
		customOwner bool
	}{
		{"ann", " basmati ", "custom", mine.SourceID, true}, // custom beats concept and catalog
		{"bob", "BASMATI", "ifct", "A015", false},           // concept alias beats the catalog row of that name
		{"bob", "Rice,white,  raw", "ifct", "A015", false},  // concept name, spacing ignored
		{"bob", "chawal", "ifct", "A015", false},
		{"bob", "Oats", "usda_sr", "100007", false},       // catalog name
		{"bob", "rice, parboiled", "ifct", "A014", false}, // unique prefix
	} {
		u := bob
		if c.who == "ann" {
			u = ann
		}
		f, err := catalog.ResolveFoodName(ctx, q, u, c.name)
		if err != nil || f.Source != c.source || f.SourceID != c.id {
			t.Errorf("%s resolves %q to %+v, %v; want %s:%s", c.who, c.name, f, err, c.source, c.id)
		}
	}
}

func TestResolveFoodNameUnknown(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	bob := newUser(ctx, t, q, "bob")
	var un *catalog.UnknownNameError
	_, err := catalog.ResolveFoodName(ctx, q, bob, "chiken breast")
	if !errors.As(err, &un) || len(un.Suggestions) == 0 || len(un.Suggestions) > 3 || !strings.Contains(strings.Join(un.Suggestions, "|"), "Chicken breast, raw") {
		t.Fatalf("err = %v, want suggestions with the chicken concept", err)
	}
	if !strings.Contains(err.Error(), "unknown food") {
		t.Errorf("message = %q", err)
	}
	if _, err := catalog.ResolveFoodName(ctx, q, bob, "zzzz qqqq"); !errors.As(err, &un) || len(un.Suggestions) != 0 {
		t.Errorf("gibberish = %v", err)
	}
	_, err = catalog.ResolveFoodName(ctx, q, bob, "chicken")
	if !errors.As(err, &un) || len(un.Suggestions) != 3 || !strings.HasPrefix(un.Suggestions[0], "Chicken") {
		t.Errorf("ambiguous prefix = %v, want the candidates as suggestions", err)
	}
}

func TestValidateMealNames(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	bob, ann := newUser(ctx, t, q, "bob"), newUser(ctx, t, q, "ann")
	mine, err := catalog.CreateCustomFood(ctx, q, bob, catalog.CustomFood{Name: "Mine", Per100g: model.Macros{Kcal: 1}})
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := catalog.CreateCustomFood(ctx, q, ann, catalog.CustomFood{Name: "Theirs", Per100g: model.Macros{Kcal: 1}})
	if err != nil {
		t.Fatal(err)
	}
	item := func(f model.Food) model.Item { return model.Item{Food: f, Qty: 100, Unit: "g"} }
	m := &model.MealProgram{Days: []model.MealDay{{Meals: []model.Meal{{Items: []model.Item{
		item(model.Food{Name: "basmati", Line: 3, Unresolved: true}),
		item(model.Food{Name: "Unobtainium", Line: 5, Unresolved: true}),
		item(model.Food{Name: "Unobtainium", Line: 5, Unresolved: true}),
		item(model.Food{Name: "Water", Line: 10}), // explicit zero macros
		item(model.Food{Name: "mine", Ref: "custom:" + mine.SourceID, Line: 11}),
		item(model.Food{Name: "theirs", Ref: "custom:" + theirs.SourceID, Line: 12}),
		item(model.Food{Name: "x", Ref: "ifct:A015", Line: 6}),
		item(model.Food{Name: "x", Ref: "ifct:ZZZ", Line: 7}),
		item(model.Food{Name: "x", Ref: "nocolon", Line: 8}),
		item(model.Food{Name: "Unobtainium", Per100g: model.Macros{Kcal: 1}, Line: 9}),
	}}}}}}
	got, err := catalog.ValidateMealNames(ctx, q, bob, m)
	if err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, e := range got {
		lines = append(lines, e.Line)
	}
	if len(got) != 4 || lines[0] != 5 || lines[1] != 12 || lines[2] != 7 || lines[3] != 8 || got[2].Name != "ifct:ZZZ" {
		t.Errorf("errors = %+v", got)
	}
}
