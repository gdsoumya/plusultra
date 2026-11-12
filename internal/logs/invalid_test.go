package logs_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

func setupUser(t *testing.T, e env, name string) uuid.UUID {
	t.Helper()
	u, err := e.q.CreateUser(e.ctx, sqlc.CreateUserParams{Username: name, PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func programParams(user uuid.UUID) sqlc.CreateProgramParams {
	return sqlc.CreateProgramParams{UserID: user, Kind: "meal", Name: "cut", Script: "", Structure: []byte(`{}`), Status: "active"}
}

func ptr[T any](v T) *T { return &v }

func TestInvalidInputs(t *testing.T) {
	e := setup(t)
	manyItems := make([]model.Item, 201)
	for i := range manyItems {
		manyItems[i] = chicken(1)
	}
	manyMeals := make([]model.Meal, 51)
	for i := range manyMeals {
		manyMeals[i] = meal("m")
	}
	badUnit := egg(1)
	badUnit.Unit = "g"
	badUnit.Food.Unit = ""
	badUnit.Food.PerUnit = nil
	cases := map[string]logs.MealLogInput{
		"not a date":      input("2026-13-01"),
		"feb 30":          input("2026-02-30"),
		"wrong format":    input("3/2/2026"),
		"negative qty":    input("2026-03-02", meal("a", chicken(-1))),
		"nan qty":         input("2026-03-02", meal("a", chicken(math.NaN()))),
		"huge qty":        input("2026-03-02", meal("a", chicken(100001))),
		"inf macros":      input("2026-03-02", meal("a", withKcal(chicken(1), math.Inf(1)))),
		"negative macros": input("2026-03-02", meal("a", withKcal(chicken(1), -5))),
		"long name":       input("2026-03-02", meal("a", named(chicken(1), strings.Repeat("x", 201)))),
		"empty name":      input("2026-03-02", meal("a", named(chicken(1), ""))),
		"empty meal name": input("2026-03-02", meal("")),
		"too many items":  input("2026-03-02", meal("a", manyItems...)),
		"too many meals":  input("2026-03-02", manyMeals...),
		"unit mismatch":   input("2026-03-02", meal("a", withUnit(egg(1), "cup"))),
		"gram food unit":  input("2026-03-02", meal("a", withUnit(badUnit, "egg"))),
		"bad target": func() logs.MealLogInput {
			in := input("2026-03-02")
			in.Target = &model.Macros{Kcal: -1}
			return in
		}(),
	}
	for name, in := range cases {
		if _, err := e.meals.Create(e.ctx, e.user, in); !errors.Is(err, logs.ErrInvalid) {
			t.Errorf("meal %s: %v", name, err)
		}
	}

	ci := map[string]logs.CheckinInput{
		"bad date":       {Date: "2026-00-10"},
		"weight low":     {Date: "2026-03-02", WeightKg: ptr(19.9)},
		"weight high":    {Date: "2026-03-02", WeightKg: ptr(500.1)},
		"weight nan":     {Date: "2026-03-02", WeightKg: ptr(math.NaN())},
		"fat low":        {Date: "2026-03-02", BodyFat: ptr(1.9)},
		"fat high":       {Date: "2026-03-02", BodyFat: ptr(70.1)},
		"bad key":        {Date: "2026-03-02", Measurements: map[string]float64{"Waist": 80}},
		"empty key":      {Date: "2026-03-02", Measurements: map[string]float64{"": 80}},
		"long key":       {Date: "2026-03-02", Measurements: map[string]float64{strings.Repeat("a", 33): 80}},
		"negative value": {Date: "2026-03-02", Measurements: map[string]float64{"waist": -1}},
		"huge value":     {Date: "2026-03-02", Measurements: map[string]float64{"waist": 501}},
		"not an image":   {Date: "2026-03-02", Photo: []byte("GIF89a....")},
	}
	for name, in := range ci {
		if _, err := e.checkins.Create(e.ctx, e.user, in); !errors.Is(err, logs.ErrInvalid) {
			t.Errorf("checkin %s: %v", name, err)
		}
	}
	// Boundaries are accepted.
	if _, err := e.checkins.Create(e.ctx, e.user, logs.CheckinInput{Date: "2026-03-02", WeightKg: ptr(20.0), BodyFat: ptr(70.0), Measurements: map[string]float64{"waist": 500, "left_arm": 0}}); err != nil {
		t.Fatal(err)
	}
}

func withKcal(it model.Item, k float64) model.Item { it.Food.Per100g.Kcal = k; return it }
func named(it model.Item, n string) model.Item     { it.Food.Name = n; return it }
func withUnit(it model.Item, u string) model.Item  { it.Unit = u; return it }

func sqlcPhotosParams(user, id uuid.UUID) sqlc.ListPhotosByIDParams {
	return sqlc.ListPhotosByIDParams{UserID: user, Ids: []uuid.UUID{id}}
}

func TestMealLogProgramMustBeTheUsers(t *testing.T) {
	e := setup(t)
	theirs, err := e.q.CreateProgram(e.ctx, programParams(setupUser(t, e, "bob")))
	if err != nil {
		t.Fatal(err)
	}
	mine, err := e.q.CreateProgram(e.ctx, programParams(e.user))
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]uuid.UUID{"another user's": theirs.ID, "a missing": uuid.New()} {
		in := input("2026-03-02")
		in.ProgramID = &id
		if _, err := e.meals.Create(e.ctx, e.user, in); !errors.Is(err, logs.ErrInvalid) {
			t.Fatalf("create with %s program: err = %v", name, err)
		}
	}
	in := input("2026-03-02")
	in.ProgramID = &mine.ID
	l, err := e.meals.Create(e.ctx, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	in.ProgramID = &theirs.ID
	if _, err := e.meals.Update(e.ctx, e.user, l.ID, in); !errors.Is(err, logs.ErrInvalid) {
		t.Fatalf("update to another user's program: err = %v", err)
	}

	// A log keeps its program after the program is deleted.
	if _, err := e.q.DeleteProgram(e.ctx, sqlc.DeleteProgramParams{ID: mine.ID, UserID: e.user}); err != nil {
		t.Fatal(err)
	}
	in.ProgramID = &mine.ID
	if _, err := e.meals.Update(e.ctx, e.user, l.ID, in); err != nil {
		t.Fatalf("update keeping a deleted program: %v", err)
	}
	if _, err := e.meals.AddAdHocItem(e.ctx, e.user, l.ID, "Snack", logs.AdHocItem{Food: &model.Food{Name: "Apple"}, Qty: 100}); err != nil {
		t.Fatalf("ad hoc item on a log of a deleted program: %v", err)
	}
}
