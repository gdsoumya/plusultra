package programcatalog

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

func kg(x float64) model.Weight { return model.Weight{Value: x, Unit: model.UnitKg} }

// typicalUser knows maxes for the four main lifts only, so a catalogue program
// may use pct() on nothing else.
func typicalUser() *model.UserState {
	return &model.UserState{
		Units:      model.UnitKg,
		Bodyweight: kg(80),
		RM1: map[string]model.Weight{
			"Squat": kg(140), "Bench Press": kg(100), "Deadlift": kg(180), "Overhead Press": kg(60),
		},
		Rounding: map[string]model.Rounding{
			"dumbbell":   {Kind: model.RoundFixed, Plates: []float64{5, 7.5, 10, 12.5, 15, 17.5, 20, 22.5, 25, 27.5, 30}},
			"cable":      {Kind: model.RoundStack, Increment: 2.5},
			"bodyweight": {Kind: model.RoundBodyweight},
		},
		DefaultRounding: model.Rounding{Kind: model.RoundPlates, Bar: 20, Plates: []float64{1.25, 2.5, 5, 10, 15, 20, 25}},
	}
}

func TestEveryScriptHasAnEntry(t *testing.T) {
	files, err := fs.Glob(scripts, "programs/*.star")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(entries) {
		t.Fatalf("%d scripts, %d entries", len(files), len(entries))
	}
	if _, ok := Get("no-such-program"); ok {
		t.Fatal("Get found a program that does not exist")
	}
}

func TestCatalogProgramsRun(t *testing.T) {
	seen := map[string]bool{}
	level := -1
	for _, e := range All() {
		t.Run(e.Slug, func(t *testing.T) {
			if seen[e.Slug] {
				t.Fatal("duplicate slug")
			}
			seen[e.Slug] = true
			l := slices.Index(Levels, e.Level)
			if l < 0 || l < level {
				t.Fatalf("level %q out of order", e.Level)
			}
			level = l
			if e.Author == "" || e.Summary == "" || e.Days < 1 || e.Days > 7 || e.Weeks < 1 {
				t.Fatalf("incomplete entry %+v", e)
			}
			p, err := engine.Evaluate(e.Script, e.Slug+".star")
			if err != nil {
				t.Fatal(err)
			}
			st := typicalUser()
			if _, err := engine.Resolve(p, st); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			// Two cycles, so loops and progressions run past the declared weeks.
			res, err := engine.Simulate(t.Context(), p, st, engine.SimOptions{Weeks: min(2*p.Weeks, 24)})
			if err != nil {
				t.Fatalf("simulate: %v", err)
			}
			for _, w := range res.Warnings {
				if !slices.Contains(informational, w.Code) {
					t.Errorf("week %d day %d %s: %s", w.Week, w.Day, w.Exercise, w.Message)
				}
			}
		})
	}
}

// informational warnings describe rounding and the same lift progressing twice in a week, both intended here.
var informational = []string{"rounding_source", "rounding_moved", "double_progression_in_week", "truncated"}

func TestMaxes(t *testing.T) {
	e, _ := Get("gzclp")
	want := []string{"Squat", "Bench Press", "Overhead Press", "Deadlift"}
	if !slices.Equal(e.Maxes, want) {
		t.Fatalf("gzclp maxes = %v", e.Maxes)
	}
	if e, _ := Get("beginner-full-body"); len(e.Maxes) != 0 {
		t.Fatalf("beginner-full-body maxes = %v", e.Maxes)
	}
}
