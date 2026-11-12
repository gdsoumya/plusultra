package catalog

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Equipment is a named piece of gear and how weights are rounded on it.
type Equipment struct {
	Name   string
	Config model.Rounding
}

// GetEquipment lists userID's equipment by name.
func GetEquipment(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) ([]Equipment, error) {
	rows, err := q.ListEquipment(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Equipment, len(rows))
	for i, r := range rows {
		out[i].Name = r.Name
		if err := json.Unmarshal(r.Config, &out[i].Config); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SetEquipment creates or replaces one piece of equipment.
func SetEquipment(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, name string, cfg model.Rounding) error {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return invalid("equipment name is empty")
	}
	if err := validateRounding(cfg); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = q.UpsertEquipment(ctx, sqlc.UpsertEquipmentParams{UserID: userID, Name: name, Config: raw})
	return err
}

// DeleteEquipment removes one piece of equipment; a missing name is not an error.
func DeleteEquipment(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, name string) error {
	return q.DeleteEquipment(ctx, sqlc.DeleteEquipmentParams{UserID: userID, Name: name})
}

// SeedDefaultEquipment adds the starter equipment the user does not have yet.
func SeedDefaultEquipment(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) error {
	for _, e := range defaultEquipment() {
		raw, err := json.Marshal(e.Config)
		if err != nil {
			return err
		}
		if err := q.SeedEquipment(ctx, sqlc.SeedEquipmentParams{UserID: userID, Name: e.Name, Config: raw}); err != nil {
			return err
		}
	}
	return nil
}

func defaultEquipment() []Equipment {
	// Whole kilos up to 50 plus the .5 steps that adjustable dumbbells add.
	var dumbbells []float64
	for w := 1.0; w <= 50; w++ {
		dumbbells = append(dumbbells, w)
	}
	for w := 2.5; w < 50; w += 5 {
		dumbbells = append(dumbbells, w)
	}
	slices.Sort(dumbbells)
	return []Equipment{
		{"Barbell", model.Rounding{Kind: model.RoundPlates, Bar: 20, Plates: []float64{25, 20, 15, 10, 5, 2.5, 1.25}}},
		{"Dumbbell", model.Rounding{Kind: model.RoundFixed, Plates: dumbbells}},
		{"Stack", model.Rounding{Kind: model.RoundStack, Increment: 5}},
		{"Bodyweight", model.Rounding{Kind: model.RoundBodyweight}},
	}
}
