package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// ErrUpstream means an external service (Open Food Facts) failed or timed out.
var ErrUpstream = errors.New("catalog: upstream unavailable")

// Food is a row of the food catalog. Per100g holds a model.Macros and Units a []Unit, both as JSON.
type Food = sqlc.Food

// Unit is a household measure of a food, such as one egg.
type Unit struct {
	Name  string  `json:"name"`
	Grams float64 `json:"grams"`
}

// CustomFood is the user-editable part of a custom food.
type CustomFood struct {
	Name    string
	Aliases []string
	Barcode string
	Per100g model.Macros
	Units   []Unit
}

var barcodeRE = regexp.MustCompile(`^\d{8,14}$`)

// foodRow is a food as catalog importers and the barcode cache write it.
type foodRow struct {
	Source, SourceID, Name, Barcode string
	Aliases                         []string
	Per100g                         model.Macros
	Units                           []Unit
}

// encode returns the JSON columns and the nullable barcode of r.
func (r foodRow) encode() (per100g, units []byte, barcode *string, err error) {
	if per100g, err = json.Marshal(r.Per100g); err != nil {
		return
	}
	if units, err = json.Marshal(append([]Unit{}, r.Units...)); err != nil {
		return
	}
	if r.Barcode != "" {
		barcode = &r.Barcode
	}
	return
}

// save upserts r. It fails with pgx.ErrNoRows if a row with another owner holds (source, source id).
func (r foodRow) save(ctx context.Context, q *sqlc.Queries, userID *uuid.UUID) (Food, error) {
	per100g, units, barcode, err := r.encode()
	if err != nil {
		return Food{}, err
	}
	return q.UpsertFood(ctx, sqlc.UpsertFoodParams{
		Source: r.Source, SourceID: r.SourceID, Name: r.Name, Aliases: r.Aliases, Barcode: barcode,
		Per100g: per100g, Units: units, UserID: userID,
	})
}

func (c CustomFood) validate() (CustomFood, error) {
	c.Name = strings.Join(strings.Fields(c.Name), " ")
	if c.Name == "" {
		return c, invalid("food name is empty")
	}
	if c.Barcode != "" && !barcodeRE.MatchString(c.Barcode) {
		return c, invalid("barcode must be 8 to 14 digits")
	}
	m := c.Per100g
	for _, v := range []float64{m.Kcal, m.Protein, m.Carbs, m.Fat, m.Fiber, m.WaterML} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return c, invalid("macros must be finite and not negative")
		}
	}
	for _, u := range c.Units {
		if strings.TrimSpace(u.Name) == "" || !(u.Grams > 0) || math.IsInf(u.Grams, 0) {
			return c, invalid("unit %q needs a name and positive grams", u.Name)
		}
	}
	return c, nil
}

// CreateCustomFood adds a food only userID can see. Its source id is random, so it never collides with a catalog row.
func CreateCustomFood(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, c CustomFood) (Food, error) {
	c, err := c.validate()
	if err != nil {
		return Food{}, err
	}
	return foodRow{
		Source: "custom", SourceID: uuid.NewString(), Name: c.Name, Aliases: c.Aliases,
		Barcode: c.Barcode, Per100g: c.Per100g, Units: c.Units,
	}.save(ctx, q, &userID)
}

// UpdateCustomFood edits one of userID's custom foods.
func UpdateCustomFood(ctx context.Context, q *sqlc.Queries, userID, id uuid.UUID, c CustomFood) (Food, error) {
	c, err := c.validate()
	if err != nil {
		return Food{}, err
	}
	per100g, units, barcode, err := foodRow{Barcode: c.Barcode, Per100g: c.Per100g, Units: c.Units}.encode()
	if err != nil {
		return Food{}, err
	}
	f, err := q.UpdateCustomFood(ctx, sqlc.UpdateCustomFoodParams{
		ID: id, UserID: &userID, Name: c.Name, Aliases: c.Aliases, Barcode: barcode, Per100g: per100g, Units: units,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Food{}, ErrNotFound
	}
	return f, err
}

// DeleteCustomFood removes one of userID's custom foods.
func DeleteCustomFood(ctx context.Context, q *sqlc.Queries, userID, id uuid.UUID) error {
	n, err := q.DeleteCustomFood(ctx, sqlc.DeleteCustomFoodParams{ID: id, UserID: &userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// NoUnitError says a food has no household unit of the given name; Units are the ones it has.
type NoUnitError struct {
	Food, Unit string
	Units      []string
}

func (e *NoUnitError) Error() string {
	has := "it is measured in grams"
	if len(e.Units) > 0 {
		has = "it has " + strings.Join(e.Units, ", ")
	}
	return fmt.Sprintf("%s has no unit %q; %s", e.Food, e.Unit, has)
}

// FrozenFood is f with its macros decoded, to freeze into a meal program or
// log, and with per-unit macros when unit is a household unit of f rather
// than grams.
func FrozenFood(f Food, unit string) (model.Food, error) {
	out := model.Food{Name: f.Name, Ref: f.Source + ":" + f.SourceID}
	if err := json.Unmarshal(f.Per100g, &out.Per100g); err != nil || unit == "" || unit == "g" {
		return out, err
	}
	var units []Unit
	if err := json.Unmarshal(f.Units, &units); err != nil {
		return out, err
	}
	i := slices.IndexFunc(units, func(u Unit) bool { return u.Name == unit })
	if i < 0 {
		names := make([]string, len(units))
		for i, u := range units {
			names[i] = u.Name
		}
		return out, &NoUnitError{Food: f.Name, Unit: unit, Units: names}
	}
	pu := out.Per100g.Scale(units[i].Grams / 100)
	out.PerUnit, out.Unit = &pu, unit
	return out, nil
}

// GetFood returns a catalog food or one of userID's custom foods.
func GetFood(ctx context.Context, q *sqlc.Queries, userID, id uuid.UUID) (Food, error) {
	f, err := q.GetFood(ctx, sqlc.GetFoodParams{ID: id, UserID: &userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Food{}, ErrNotFound
	}
	return f, err
}
