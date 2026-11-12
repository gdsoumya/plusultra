package programs

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// freeze fills the unresolved foods of meal script sc, evaluated from src,
// from userID's catalog, by ref or else by name, adds them to the foods sc
// was evaluated with and evaluates src again with them all. The returned
// script keeps those foods so the stored structure fixes their macros as of
// this save. A food an item counts in a unit is frozen with that unit; one
// unit per food, so two different units on one catalog food fail at the fill.
func freeze(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, src string, sc script) (script, error) {
	pending, units := map[string]model.Food{}, map[string]string{}
	for _, d := range sc.meal.Days {
		for _, meal := range d.Meals {
			for _, it := range meal.Items {
				if !it.Food.Unresolved {
					continue
				}
				key := cmp.Or(it.Food.Ref, it.Food.Name)
				pending[key] = it.Food
				if it.Unit != "" && it.Unit != "g" {
					units[key] = it.Unit
				}
			}
		}
	}
	if len(pending) == 0 && len(sc.foods) == 0 {
		return sc, nil
	}
	foods := map[string]model.Food{}
	maps.Copy(foods, sc.foods)
	for key, f := range pending {
		var err error
		if foods[key], err = catalogFood(ctx, q, userID, f, units[key]); err != nil {
			return script{}, err
		}
	}
	filled, err := engine.EvaluateMealWith(src, scriptFile, foods)
	return script{meal: filled, foods: foods}, err
}

// catalogFood is the catalog food f names by ref or name, which validation
// has found, with per-unit macros for unit when that is not grams.
func catalogFood(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, f model.Food, unit string) (model.Food, error) {
	var row catalog.Food
	var err error
	if source, id, ok := strings.Cut(f.Ref, ":"); ok {
		row, err = q.GetFoodBySource(ctx, sqlc.GetFoodBySourceParams{Source: source, SourceID: id, UserID: &userID})
	} else {
		row, err = catalog.ResolveFoodName(ctx, q, userID, f.Name)
	}
	if err != nil {
		return model.Food{}, err
	}
	out, err := catalog.FrozenFood(row, unit)
	if nu, ok := errors.AsType[*catalog.NoUnitError](err); ok {
		return out, &engine.EvalError{File: scriptFile, Line: f.Line, Msg: nu.Error()}
	}
	return out, err
}

// LoadMeal evaluates saved meal program p with the catalog foods frozen when
// it was saved. Every evaluation of a saved meal script must go through it:
// evaluated alone, foods given by name or ref have no macros.
func LoadMeal(p sqlc.Program) (*model.MealProgram, error) {
	if p.Kind != KindMeal {
		return nil, fmt.Errorf("program %s is a %s program, not a meal program", p.ID, p.Kind)
	}
	sc, err := load(p)
	return sc.meal, err
}

// load evaluates stored program p with the catalog foods frozen when it was saved.
func load(p sqlc.Program) (script, error) {
	if p.Kind != KindMeal {
		return evaluate(p.Kind, p.Script, nil)
	}
	var st Structure
	if err := json.Unmarshal(p.Structure, &st); err != nil {
		return script{}, err
	}
	m, err := engine.EvaluateMealWith(p.Script, scriptFile, st.Foods)
	return script{meal: m, foods: st.Foods}, err
}
