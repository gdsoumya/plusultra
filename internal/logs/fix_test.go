package logs_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

func TestConcurrentAdHocAdds(t *testing.T) {
	e := setup(t)
	l, err := e.meals.Create(e.ctx, e.user, input("2026-03-02", meal("Lunch")))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := model.Food{Name: "Bar", Per100g: model.Macros{Kcal: 100}}
			_, errs[i] = e.meals.AddAdHocItem(e.ctx, e.user, l.ID, "Lunch", logs.AdHocItem{Food: &f, Qty: 100, Unit: "g"})
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := e.meals.Get(e.ctx, e.user, l.ID)
	if err != nil || len(got.Meals[0].Items) != len(errs) {
		t.Fatalf("items kept: %d, err %v", len(got.Meals[0].Items), err)
	}
}

func TestConcurrentSameDateCreate(t *testing.T) {
	e := setup(t)
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.meals.Create(e.ctx, e.user, input("2026-03-02"))
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, logs.ErrConflict):
			t.Fatal(err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d creates succeeded", ok)
	}
}

func TestCreateIDReuseAndUpdateDate(t *testing.T) {
	e := setup(t)
	in := input("2026-03-02")
	in.ID = uuid.New()
	l, err := e.meals.Create(e.ctx, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	moved := input("2026-03-05")
	moved.ID = l.ID
	if _, err := e.meals.Create(e.ctx, e.user, moved); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("id on another date: %v", err)
	}
	if got, _ := e.meals.Get(e.ctx, e.user, l.ID); got.Date != "2026-03-02" {
		t.Fatalf("log moved to %s", got.Date)
	}
	if _, err := e.meals.Update(e.ctx, e.user, l.ID, moved); !errors.Is(err, logs.ErrInvalid) {
		t.Fatalf("update date: %v", err)
	}
	// A tombstone with the same id and date can be resurrected.
	if err := e.meals.Delete(e.ctx, e.user, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.meals.Create(e.ctx, e.user, in); err != nil {
		t.Fatalf("resurrect: %v", err)
	}
}

func TestZeroQuantityHasNoMacros(t *testing.T) {
	e := setup(t)
	l, err := e.meals.Create(e.ctx, e.user, input("2026-03-02", meal("Lunch", chicken(0), egg(0), chicken(100))))
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range l.Meals[0].Items[:2] {
		if it.Macros != (model.Macros{}) {
			t.Fatalf("%s macros %+v", it.Food.Name, it.Macros)
		}
	}
	near(t, "kcal", l.Totals.Kcal, 120)
}
