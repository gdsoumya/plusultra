package export_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/export"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func TestZipLayout(t *testing.T) {
	e := setup(t)
	e.seedGolden()
	// A second session on the 11th: in days.json and the CSV, not in the text.
	e.record("2026-11-11", 18, e.program("Conditioning"), 1, 1, "Bike", 20, []model.RecordEntry{{Name: "Air Bike", Sets: sets(1, planned(1, kg(0), ""))}})
	e.create(programs.KindWorkout, workoutScript)
	e.create(programs.KindWorkout, workoutScript) // a second "Base" needs its own file name
	_, err := catalog.CreateCustomFood(e.ctx, e.q, e.user, catalog.CustomFood{Name: "Paneer", Per100g: model.Macros{Kcal: 265, Protein: 18}})
	e.fatal(err)

	raw, err := e.s.ExportAll(e.ctx, e.user)
	e.fatal(err)
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	e.fatal(err)
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		e.fatal(err)
		b, err := io.ReadAll(rc)
		e.fatal(err)
		_ = rc.Close()
		files[f.Name] = string(b)
	}
	names := slices.Sorted(func(yield func(string) bool) {
		for n := range files {
			if !yield(n) {
				return
			}
		}
	})
	want := []string{"checkins.csv", "custom_exercises.json", "custom_foods.json", "days.json", "days/2026/11/11.txt", "days/2026/11/12.txt",
		"programs/16-week-strength.star", "programs/base-2.star", "programs/base.star", "programs/conditioning.star", "programs/upper-volume.star"}
	if !slices.Equal(names, want) {
		t.Fatalf("files %v", names)
	}
	var zipNames []string
	for _, f := range zr.File {
		zipNames = append(zipNames, f.Name)
	}
	if !slices.IsSorted(zipNames[len(zipNames)-4:]) {
		t.Fatalf("top-level files not in order: %v", zipNames)
	}
	first, second, _ := strings.Cut(golden(t), "\n\n")
	if files["days/2026/11/11.txt"] != first+"\n" || files["days/2026/11/12.txt"] != second {
		t.Fatalf("day files:\n%s\n%s", files["days/2026/11/11.txt"], files["days/2026/11/12.txt"])
	}
	if files["programs/base.star"] != workoutScript || !strings.HasPrefix(files["checkins.csv"], "date,weight_kg,body_fat,measurements\n2026-11-11,82.4,,{}\n") {
		t.Fatalf("base.star %q\ncheckins.csv %q", files["programs/base.star"], files["checkins.csv"])
	}
	var days []export.Day
	if err := json.Unmarshal([]byte(files["days.json"]), &days); err != nil || len(days) != 2 || days[0].Date != "2026-11-11" ||
		days[0].Workout.Program != "16-Week Strength" || len(days[0].Workouts) != 1 || days[0].Workouts[0].Program != "Conditioning" {
		t.Fatalf("days.json %v, %v", days, err)
	}
	csvOut, err := e.s.ExportDays(e.ctx, e.user, date("2026-11-11"), date("2026-11-11"), export.FormatCSV)
	e.fatal(err)
	if !strings.Contains(string(csvOut), "Air Bike") {
		t.Fatalf("csv lacks the second record:\n%s", csvOut)
	}
	var foods, exercises []map[string]any
	e.fatal(json.Unmarshal([]byte(files["custom_foods.json"]), &foods))
	e.fatal(json.Unmarshal([]byte(files["custom_exercises.json"]), &exercises))
	if len(foods) != 1 || foods[0]["name"] != "Paneer" || foods[0]["per100g"].(map[string]any)["Kcal"] != 265.0 || len(exercises) != 3 {
		t.Fatalf("custom foods %v, exercises %v", foods, exercises)
	}
}
