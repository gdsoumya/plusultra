package catalog_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/catalog"
)

func TestMuscleTaxonomy(t *testing.T) {
	for _, g := range strings.Split("abdominals abductors adductors biceps calves chest forearms glutes hamstrings lats lower_back middle_back neck quadriceps shoulders traps triceps obliques hip_flexors rotator_cuff serratus", " ") {
		if g = strings.ReplaceAll(g, "_", " "); !slices.Contains(catalog.Groups, g) {
			t.Errorf("group %q missing", g)
		}
	}
	for r, g := range catalog.Regions {
		if !slices.Contains(catalog.Groups, g) {
			t.Errorf("region %q maps to unknown group %q", r, g)
		}
		if slices.Contains(catalog.Groups, r) {
			t.Errorf("%q is both a group and a region", r)
		}
	}
	for in, want := range map[string]string{
		"Chest": "chest", " Front  Delts ": "front delts", "anterior deltoid": "front delts",
		"quads": "quadriceps", "abs": "abdominals", "upper pecs": "upper chest", "tibialis": "tibialis",
	} {
		if got, ok := catalog.Normalize(in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := catalog.Normalize("wings"); ok {
		t.Error("Normalize accepted an unknown name")
	}
	if names := catalog.MuscleNames(); len(names) != len(catalog.Groups)+len(catalog.Regions) || !slices.IsSorted(names) {
		t.Errorf("MuscleNames = %v", names)
	}
}

func TestCustomExerciseMusclesNormalized(t *testing.T) {
	e := setup(t)
	c, err := catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: "Cable Fly, High",
		PrimaryMuscles: []string{"Upper Pecs", "chest", "upper chest"}, SecondaryMuscles: []string{"Anterior Deltoid"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(c.PrimaryMuscles, []string{"upper chest", "chest"}) || !slices.Equal(c.SecondaryMuscles, []string{"front delts"}) {
		t.Errorf("muscles = %v / %v", c.PrimaryMuscles, c.SecondaryMuscles)
	}
	_, err = catalog.UpdateCustomExercise(e.ctx, e.q, e.user, c.Slug, catalog.Custom{Name: "Cable Fly, High", PrimaryMuscles: []string{"wings"}})
	if err == nil || !strings.Contains(err.Error(), `unknown muscle "wings"`) || !strings.Contains(err.Error(), "upper chest") {
		t.Errorf("update err = %v", err)
	}
	_, err = catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: "Wing Flap", SecondaryMuscles: []string{"wings"}})
	if err == nil {
		t.Error("create accepted an unknown secondary muscle")
	}
}
