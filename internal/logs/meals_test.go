package logs_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

func TestQuickFillMealsFromPlan(t *testing.T) {
	e := setup(t)
	target := model.Macros{Kcal: 2600, Protein: 180}
	day := &model.MealDay{
		Target: &target,
		Meals:  []model.Meal{meal("Lunch", chicken(200)), meal("Snack", egg(3))},
	}
	for i := range day.Meals {
		for j := range day.Meals[i].Items {
			day.Meals[i].Items[j].Macros = day.Meals[i].Items[j].Food.Per100g
		}
	}
	p, err := e.q.CreateProgram(e.ctx, programParams(e.user))
	if err != nil {
		t.Fatal(err)
	}
	prog := p.ID
	got, err := e.meals.FillFromPlan(e.ctx, e.user, "2026-03-02", day, prog)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProgramID == nil || *got.ProgramID != prog || got.Target != target {
		t.Fatalf("program %v target %+v", got.ProgramID, got.Target)
	}
	for _, m := range got.Meals {
		for _, it := range m.Items {
			if !it.Planned {
				t.Fatalf("%s not planned", it.Food.Name)
			}
		}
	}
	near(t, "kcal", got.Totals.Kcal, 240+210)
	if _, err := e.meals.FillFromPlan(e.ctx, e.user, "2026-03-02", day, prog); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("second fill: %v", err)
	}
}

func TestEditQuantityRescales(t *testing.T) {
	e := setup(t)
	it := chicken(200)
	it.Macros.Kcal = 999 // client-supplied macros are ignored
	l, err := e.meals.Create(e.ctx, e.user, input("2026-03-02", meal("Lunch", it)))
	if err != nil {
		t.Fatal(err)
	}
	near(t, "kcal", l.Totals.Kcal, 240)

	in := input("2026-03-02", meal("Lunch", chicken(300)))
	in.DeviceID = "tablet"
	u, err := e.meals.Update(e.ctx, e.user, l.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "kcal", u.Totals.Kcal, 360)
	near(t, "item kcal", u.Meals[0].Items[0].Macros.Kcal, 360)
	if u.Version != 2 || u.DeviceID != "tablet" || !u.UpdatedAt.After(l.UpdatedAt) {
		t.Fatalf("version %d device %q", u.Version, u.DeviceID)
	}

	if _, err := e.meals.Update(e.ctx, e.user, uuid.New(), in); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err := e.meals.Delete(e.ctx, e.user, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.meals.Get(e.ctx, e.user, l.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	if err := e.meals.Delete(e.ctx, e.user, l.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	// A deleted date can be logged again.
	if _, err := e.meals.Create(e.ctx, e.user, input("2026-03-02")); err != nil {
		t.Fatal(err)
	}
}

func TestAdHocItem(t *testing.T) {
	e := setup(t)
	oats, err := catalog.CreateCustomFood(e.ctx, e.q, e.user, catalog.CustomFood{
		Name: "Oats", Per100g: model.Macros{Kcal: 380, Protein: 13},
		Units: []catalog.Unit{{Name: "cup", Grams: 80}},
	})
	if err != nil {
		t.Fatal(err)
	}
	l, err := e.meals.Create(e.ctx, e.user, input("2026-03-02", meal("Breakfast", egg(2))))
	if err != nil {
		t.Fatal(err)
	}

	g, err := e.meals.AddAdHocItem(e.ctx, e.user, l.ID, "Breakfast", logs.AdHocItem{FoodID: oats.ID, Qty: 50, Unit: "g"})
	if err != nil {
		t.Fatal(err)
	}
	it := g.Meals[0].Items[1]
	if it.Planned || it.Food.Ref != oats.ID.String() || it.Food.Name != "Oats" {
		t.Fatalf("item %+v", it)
	}
	near(t, "oats kcal", it.Macros.Kcal, 190)

	g, err = e.meals.AddAdHocItem(e.ctx, e.user, l.ID, "Breakfast", logs.AdHocItem{FoodID: oats.ID, Qty: 1.5, Unit: "cup"})
	if err != nil {
		t.Fatal(err)
	}
	near(t, "cup kcal", g.Meals[0].Items[2].Macros.Kcal, 380*0.8*1.5)

	// Custom food into a meal that does not exist yet.
	custom := model.Food{Name: "Protein bar", Per100g: model.Macros{Kcal: 400, Protein: 30}}
	g, err = e.meals.AddAdHocItem(e.ctx, e.user, l.ID, "Snack", logs.AdHocItem{Food: &custom, Qty: 60, Unit: "g"})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Meals) != 2 || g.Meals[1].Name != "Snack" {
		t.Fatalf("meals %+v", g.Meals)
	}
	near(t, "total kcal", g.Totals.Kcal, 140+190+380*0.8*1.5+240)

	for name, in := range map[string]logs.AdHocItem{
		"unknown food": {FoodID: uuid.New(), Qty: 1, Unit: "g"},
		"unknown unit": {FoodID: oats.ID, Qty: 1, Unit: "bowl"},
		"no food":      {Qty: 1, Unit: "g"},
		"negative qty": {FoodID: oats.ID, Qty: -1, Unit: "g"},
	} {
		_, err := e.meals.AddAdHocItem(e.ctx, e.user, l.ID, "Breakfast", in)
		want := logs.ErrInvalid
		if name == "unknown food" {
			want = logs.ErrNotFound
		}
		if !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", name, err, want)
		}
	}
	if _, err := e.meals.AddAdHocItem(e.ctx, e.user, uuid.New(), "Breakfast", logs.AdHocItem{FoodID: oats.ID, Qty: 1, Unit: "g"}); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("missing log: %v", err)
	}
}

func TestTotalsVsTarget(t *testing.T) {
	e := setup(t)
	target := model.Macros{Kcal: 2000, Protein: 150, Carbs: 200, Fat: 60}
	in := input("2026-03-02", meal("Lunch", chicken(250)), meal("Dinner", chicken(100), egg(2)))
	in.Target = &target
	l, err := e.meals.Create(e.ctx, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.meals.ByDate(e.ctx, e.user, "2026-03-02")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != l.ID || got.Target != target {
		t.Fatalf("target %+v", got.Target)
	}
	near(t, "kcal", got.Totals.Kcal, 300+120+140)
	near(t, "protein", got.Totals.Protein, 22.5*3.5+12)
	if _, err := e.meals.ByDate(e.ctx, e.user, "2026-03-03"); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("by date: %v", err)
	}
	if _, err := e.meals.Create(e.ctx, e.user, input("2026-03-02")); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("duplicate date: %v", err)
	}
	other := setupUser(t, e, "bob")
	if _, err := e.meals.Get(e.ctx, other, l.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("other user get: %v", err)
	}
}

func TestMealListPagination(t *testing.T) {
	e := setup(t)
	for d := 1; d <= 5; d++ {
		if _, err := e.meals.Create(e.ctx, e.user, input(fmt.Sprintf("2026-03-%02d", d))); err != nil {
			t.Fatal(err)
		}
	}
	var dates []string
	cursor := ""
	for pages := 0; ; pages++ {
		p, err := e.meals.List(e.ctx, e.user, "2026-03-01", "2026-03-31", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range p.Items {
			dates = append(dates, l.Date)
		}
		if p.HasMore != (pages < 2) {
			t.Fatalf("page %d has_more=%v", pages, p.HasMore)
		}
		if !p.HasMore {
			if p.NextCursor != "" {
				t.Fatalf("cursor on last page: %q", p.NextCursor)
			}
			break
		}
		cursor = p.NextCursor
	}
	if got := strings.Join(dates, ","); got != "2026-03-01,2026-03-02,2026-03-03,2026-03-04,2026-03-05" {
		t.Fatalf("dates %s", got)
	}
	// An exact multiple of the limit ends without a phantom next page.
	p, err := e.meals.List(e.ctx, e.user, "2026-03-01", "2026-03-04", "", 4)
	if err != nil || p.HasMore || len(p.Items) != 4 {
		t.Fatalf("exact page: %+v %v", p, err)
	}
	if _, err := e.meals.List(e.ctx, e.user, "2026-03-01", "2026-03-31", "garbage", 2); !errors.Is(err, logs.ErrInvalid) {
		t.Fatalf("bad cursor: %v", err)
	}
}
