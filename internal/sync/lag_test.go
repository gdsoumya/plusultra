package sync_test

import (
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// TestFinalizeFitsPullLag checks the assumption behind Pull's 5 s lag: the longest transaction
// that stamps updated_at, a finalize re-resolving a long program, commits well within it.
func TestFinalizeFitsPullLag(t *testing.T) {
	e := setup(t)
	script, err := os.ReadFile("../engine/testdata/examples/strength16.star")
	if err != nil {
		t.Fatal(err)
	}
	// The script must name known exercises: each ex() name and its alts.
	names := map[string]bool{}
	for _, call := range regexp.MustCompile(`ex\("[^"]+"|alts=\[[^\]]*\]`).FindAllString(string(script), -1) {
		for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(call, -1) {
			if !names[m[1]] {
				names[m[1]] = true
				e.exercises(m[1])
			}
		}
	}
	monday := day("2026-11-09")
	e.activate(programs.KindWorkout, string(script), &monday)
	rec, err := e.logs.CreateWorkoutRecord(e.ctx, e.user, logs.NewRecord{Date: "2026-11-10"}) // week 1 Tuesday, upper body
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range rec.Entries {
		for i := range en.Sets {
			s := &en.Sets[i]
			s.Completed, s.Reps, s.Weight = true, s.Planned.Reps, s.Planned.Weight
		}
	}
	if _, err := e.logs.UpdateWorkoutRecord(e.ctx, e.user, rec.ID, logs.RecordInput{Entries: rec.Entries}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := e.logs.Finalize(e.ctx, e.user, rec.ID); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	t.Logf("finalize of strength16 week 1 day 2 (%d exercises) took %v", len(rec.Entries), took)
	if took > 2*time.Second {
		t.Fatalf("finalize took %v; the pull lag (5 s) must exceed it with room to spare", took)
	}
}
