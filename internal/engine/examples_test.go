package engine

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

func TestExamplesEvaluate(t *testing.T) {
	files, err := fs.Glob(testdata, "examples/*.star")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, f := range files {
		t.Run(path.Base(f), func(t *testing.T) {
			p := evalFile(t, f)
			st := parityState()
			if _, err := Resolve(p, st); err != nil {
				t.Fatal(err)
			}
			progressWeek(t, p, st, 1)
			if _, err := Resolve(p, st); err != nil {
				t.Fatalf("after progress: %v", err)
			}
		})
	}
}

// exampleDay resolves week, day of an example for the parity user.
func exampleDay(t *testing.T, p *model.Program, st *model.UserState, week, day int) *model.ResolvedDay {
	t.Helper()
	d, err := ResolveDay(p, st, week, day)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func exerciseNamed(t *testing.T, d *model.ResolvedDay, name, label string) model.ResolvedExercise {
	t.Helper()
	for _, e := range d.Exercises {
		if e.Name == name && e.Label == label {
			return e
		}
	}
	t.Fatalf("week %d day %d has no %q labelled %q", d.Week, d.Day, name, label)
	return model.ResolvedExercise{}
}

// summary lists e's working sets, runs of equal sets grouped: "4x5 77.5kg Work, 2x5 65kg Backoff".
func summary(e model.ResolvedExercise) string {
	work := slices.DeleteFunc(slices.Clone(e.Sets), func(s model.ResolvedSet) bool { return s.Warmup })
	var parts []string
	n := 0
	for i, s := range work {
		n++
		if i+1 < len(work) && work[i+1] == s {
			continue
		}
		part := fmt.Sprintf("%dx%d %gkg", n, s.Reps, s.Weight.Value)
		if s.Amrap {
			part += "+"
		}
		parts = append(parts, strings.TrimSpace(part+" "+s.Label))
		n = 0
	}
	return strings.Join(parts, ", ")
}

func assertSummary(t *testing.T, e model.ResolvedExercise, want string) {
	t.Helper()
	if got := summary(e); got != want {
		t.Errorf("%s: got %s, want %s", e.Name, got, want)
	}
}

func TestStrength16KnownValues(t *testing.T) {
	p := evalFile(t, "examples/strength16.star")
	st := parityState()
	if p.Weeks != 8 || p.Loop || p.WeekDesc[8] != "Taper and test" {
		t.Errorf("header %d weeks, loop %v, week 8 %q", p.Weeks, p.Loop, p.WeekDesc[8])
	}

	assertSummary(t, exerciseNamed(t, exampleDay(t, p, st, 2, 2), "Bench Press", ""), "4x5 77.5kg Work, 2x5 65kg Backoff")
	assertSummary(t, exerciseNamed(t, exampleDay(t, p, st, 4, 6), "Squat", ""), "3x3 82.5kg Work")

	d := exampleDay(t, p, st, 1, 1)
	rope := exerciseNamed(t, d, "Jump Rope", "")
	if s := rope.Sets[0]; d.Kind != "conditioning" || s.Work != 60 || s.Rest != 30 || !s.Auto || s.Weight.Unit != model.UnitKg {
		t.Errorf("jump rope on a %s day: %+v, want work 60 rest 30 auto in kg", d.Kind, s)
	}
	assertSummary(t, exerciseNamed(t, exampleDay(t, p, st, 1, 2), "Lateral Raise", ""), "3x12 12.5kg")

	deadlift := exerciseNamed(t, exampleDay(t, p, st, 7, 3), "Deadlift", "")
	assertSummary(t, deadlift, "2x2 145kg Work")
	if s := deadlift.Sets[len(deadlift.Sets)-1]; s.Rest != 240 {
		t.Errorf("week 7 deadlift rest %d, want 240", s.Rest)
	}

	d = exampleDay(t, p, st, 8, 2)
	var names []string
	for _, e := range d.Exercises {
		names = append(names, e.Name)
	}
	if !slices.Equal(names, []string{"Squat", "Bench Press", "Deadlift"}) {
		t.Fatalf("week 8 day 2 has %v, want the three singles", names)
	}
	for i, want := range []string{"2x1 100kg Work", "2x1 82.5kg Work", "2x1 130kg Work"} {
		assertSummary(t, d.Exercises[i], want)
	}
	if d := exampleDay(t, p, st, 8, 1); d.Kind != "note" || len(d.Exercises) != 0 {
		t.Errorf("week 8 day 1 is %s with %d exercises, want an empty note day", d.Kind, len(d.Exercises))
	}
	if d := exampleDay(t, p, st, 8, 4); d.Kind != "rest" {
		t.Errorf("week 8 day 4 is %s, want rest: conditioning stops after week 7", d.Kind)
	}
}

func TestGZCLPKnownValues(t *testing.T) {
	p := evalFile(t, "examples/gzclp.star")
	st := parityState()

	a1 := exampleDay(t, p, st, 1, 1)
	squat := exerciseNamed(t, a1, "Squat", "t1")
	assertSummary(t, squat, "4x3 105kg, 1x3 105kg+") // 75% of 140
	assertSummary(t, exerciseNamed(t, a1, "Bench Press", "t2"), "3x10 62.5kg")
	lat := exerciseNamed(t, a1, "Lat Pulldown", "t3")
	assertSummary(t, lat, "2x15 40kg, 1x15 40kg+")
	if s := lat.Sets[len(lat.Sets)-1]; s.Rest != 90 {
		t.Errorf("t3 rest %d, want 90", s.Rest)
	}
	assertSummary(t, exerciseNamed(t, exampleDay(t, p, st, 1, 4), "Deadlift", "t1"), "4x3 135kg, 1x3 135kg+")

	progressWeek(t, p, st, 1)
	squat = exerciseNamed(t, exampleDay(t, p, st, 1, 1), "Squat", "t1")
	assertSummary(t, squat, "4x3 110kg, 1x3 110kg+") // last set's weight plus 5 kg
	if squat.Desc != "T1." {
		t.Errorf("t1 description %q after a session, want the second one", squat.Desc)
	}
}
