package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// Every date a log can have.
var (
	minDate = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	maxDate = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
)

// ExportAll returns a zip of everything userID has: programs/<name>.star,
// days/YYYY/MM/DD.txt, days.json, checkins.csv, custom_exercises.json and
// custom_foods.json.
func (s *Service) ExportAll(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, b []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	ps, err := s.Q.ListProgramsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	used := map[string]int{}
	for _, p := range ps {
		text, err := exportProgram(ctx, s.Q, p)
		if err != nil {
			return nil, err
		}
		name := slug(p.Name)
		if used[name]++; used[name] > 1 {
			name = fmt.Sprintf("%s-%d", name, used[name])
		}
		if err := add("programs/"+name+".star", []byte(text)); err != nil {
			return nil, err
		}
	}
	days, units, err := s.loadDays(ctx, userID, minDate, maxDate)
	if err != nil {
		return nil, err
	}
	for _, d := range days {
		if err := add("days/"+strings.ReplaceAll(d.Date, "-", "/")+".txt", []byte(formatDays([]Day{d}, units))); err != nil {
			return nil, err
		}
	}
	daysJSON, err := json.MarshalIndent(days, "", "  ")
	if err != nil {
		return nil, err
	}
	exs, err := s.Q.ListCustomExercises(ctx, &userID)
	if err != nil {
		return nil, err
	}
	foods, err := s.Q.ListCustomFoods(ctx, &userID)
	if err != nil {
		return nil, err
	}
	exJSON, err := json.MarshalIndent(customExercises(exs), "", "  ")
	if err != nil {
		return nil, err
	}
	foodJSON, err := json.MarshalIndent(customFoods(foods), "", "  ")
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"days.json": daysJSON, "checkins.csv": checkinsCSV(days), "custom_exercises.json": exJSON, "custom_foods.json": foodJSON}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if err := add(name, files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func checkinsCSV(days []Day) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"date", "weight_kg", "body_fat", "measurements"})
	for _, d := range days {
		if d.Checkin == nil {
			continue
		}
		m := []byte("{}")
		if len(d.Checkin.Measurements) > 0 {
			m, _ = json.Marshal(d.Checkin.Measurements)
		}
		_ = w.Write([]string{d.Date, optNum(d.Checkin.WeightKg), optNum(d.Checkin.BodyFat), string(m)})
	}
	w.Flush()
	return buf.Bytes()
}

func optNum(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// The JSON columns of custom rows are objects, not the base64 that []byte would give.
type customExercise struct {
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	Equipment        string          `json:"equipment"`
	Aliases          []string        `json:"aliases"`
	PrimaryMuscles   []string        `json:"primary_muscles"`
	SecondaryMuscles []string        `json:"secondary_muscles"`
	Category         string          `json:"category"`
	DefaultWarmup    json.RawMessage `json:"default_warmup,omitempty"`
}

type customFood struct {
	ID      uuid.UUID       `json:"id"`
	Name    string          `json:"name"`
	Aliases []string        `json:"aliases"`
	Barcode *string         `json:"barcode,omitempty"`
	Per100g json.RawMessage `json:"per100g"`
	Units   json.RawMessage `json:"units"`
}

func customExercises(rows []sqlc.Exercise) []customExercise {
	out := make([]customExercise, len(rows))
	for i, r := range rows {
		out[i] = customExercise{Slug: r.Slug, Name: r.Name, Equipment: r.Equipment, Aliases: r.Aliases, PrimaryMuscles: r.PrimaryMuscles,
			SecondaryMuscles: r.SecondaryMuscles, Category: r.Category, DefaultWarmup: r.DefaultWarmup}
	}
	return out
}

func customFoods(rows []sqlc.Food) []customFood {
	out := make([]customFood, len(rows))
	for i, r := range rows {
		out[i] = customFood{ID: r.ID, Name: r.Name, Aliases: r.Aliases, Barcode: r.Barcode, Per100g: r.Per100g, Units: r.Units}
	}
	return out
}
