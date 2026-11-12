package logs_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// today is the services' clock date, a Wednesday; monday starts its week.
var (
	today  = "2026-11-11"
	monday = time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
)

type env struct {
	ctx      context.Context
	q        *sqlc.Queries
	meals    *logs.MealService
	checkins *logs.CheckinService
	programs *programs.Service
	s        *logs.Service
	user     uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	pool := dbtest.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Squat", "Bench Press"} {
		if _, err := catalog.CreateCustomExercise(ctx, q, u.ID, catalog.Custom{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	now := func() time.Time { return monday.AddDate(0, 0, 2).Add(12 * time.Hour) }
	ps := programs.NewService(pool)
	ps.Now = now
	s := logs.NewService(pool, ps)
	s.Now = now
	return env{ctx, q, logs.NewMealService(q, pool), logs.NewCheckinService(q, pool), ps, s, u.ID}
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

// chicken is a gram-weighed item with per-100g macros.
func chicken(qty float64) model.Item {
	return model.Item{
		Food: model.Food{Name: "Chicken breast, raw", Per100g: model.Macros{Kcal: 120, Protein: 22.5, Fat: 2.6}},
		Qty:  qty, Unit: "g",
	}
}

func egg(qty float64) model.Item {
	pu := model.Macros{Kcal: 70, Protein: 6, Fat: 5}
	return model.Item{Food: model.Food{Name: "Egg", Per100g: model.Macros{Kcal: 140}, PerUnit: &pu, Unit: "egg"}, Qty: qty, Unit: "egg"}
}

func input(date string, meals ...model.Meal) logs.MealLogInput {
	return logs.MealLogInput{Date: date, Meals: meals, DeviceID: "phone"}
}

func meal(name string, items ...model.Item) model.Meal { return model.Meal{Name: name, Items: items} }
