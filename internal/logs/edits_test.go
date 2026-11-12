package logs_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

// edited returns a user-edited pending set planned at weight kg.
func edited(weight float64) model.RecordSet {
	return model.RecordSet{Planned: model.RecordPlanned{Reps: 5, Weight: model.RecordWeight{Value: weight, Unit: model.UnitKg}}, Edited: true}
}

func TestEditedPendingSetSurvivesHook(t *testing.T) {
	e := setup(t)
	e.activate(hooked, nil)
	r := e.create(logs.NewRecord{Date: today})
	if r.Modified {
		t.Fatal("a fresh record is modified")
	}
	sets := r.Entries[0].Sets
	sets[0].Completed, sets[0].Reps, sets[0].Weight = true, 5, sets[0].Planned.Weight
	sets[2] = edited(95)
	r = e.update(r.ID, r.Entries)
	sets = r.Entries[0].Sets
	if sets[1].Planned.Weight.Value != 90 || sets[2].Planned.Weight.Value != 95 || !sets[2].Edited || !r.Modified {
		t.Fatalf("after set 1: %+v modified %v, want the hook to skip the edited set", sets, r.Modified)
	}
}

func TestAddedAndRemovedSetsFinalize(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	w1 := e.create(logs.NewRecord{Date: "2026-11-11"})
	w1.Entries[0].Sets = append(w1.Entries[0].Sets, edited(80))
	r := e.update(w1.ID, complete(w1, 5))
	if len(r.Entries[0].Sets) != 4 || !r.Entries[0].Sets[3].Edited || !r.Modified {
		t.Fatalf("added set: %+v modified %v", r.Entries[0].Sets, r.Modified)
	}
	e.finalize(w1.ID)
	if w := e.resolvedWeight(p.ID, 2, 3); w != 82.5 {
		t.Fatalf("week 2 bench %v after four good sets, want 82.5", w)
	}
	w2 := e.create(logs.NewRecord{Date: "2026-11-18"})
	w2.Entries[0].Sets = w2.Entries[0].Sets[:2]
	r = e.update(w2.ID, complete(w2, 5))
	if len(r.Entries[0].Sets) != 2 || !r.Modified {
		t.Fatalf("removed set: %+v modified %v", r.Entries[0].Sets, r.Modified)
	}
	e.finalize(w2.ID)
	if w := e.resolvedWeight(p.ID, 3, 3); w != 85 {
		t.Fatalf("week 3 bench %v after two good sets, want 85", w)
	}
}

func TestAdHocExerciseGetsNoProgression(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	r := e.create(logs.NewRecord{Date: "2026-11-11"})
	entries := complete(r, 5)
	done := edited(100)
	done.Completed, done.Reps, done.Weight = true, 5, done.Planned.Weight
	entries = append(entries,
		model.RecordEntry{Name: "Squat", Slug: "squat", Sets: []model.RecordSet{done}}, // on another day of the program
		model.RecordEntry{Name: "Bench Press", Sets: []model.RecordSet{done}},          // a duplicate of the plan's exercise
		model.RecordEntry{Name: "Curl", Sets: []model.RecordSet{done, done}})           // not in the program
	r = e.update(r.ID, entries)
	if got := logs.PlanEntries(&r.Day, r.Entries); !slices.Equal(got, []bool{true, false, false, false}) || !r.Modified {
		t.Fatalf("plan entries %v modified %v", got, r.Modified)
	}
	e.finalize(r.ID)
	if w := e.resolvedWeight(p.ID, 2, 3); w != 82.5 {
		t.Fatalf("week 2 bench %v, want one progression", w)
	}
	if w := e.resolvedWeight(p.ID, 2, 1); w != 100 {
		t.Fatalf("week 2 squat %v, want the ad hoc squat to leave it alone", w)
	}
}

func TestRemovedPlanExerciseGetsNoProgression(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	r := e.create(logs.NewRecord{Date: "2026-11-11"})
	if r = e.update(r.ID, nil); !r.Modified {
		t.Fatal("a record without its plan exercise is not modified")
	}
	e.finalize(r.ID)
	if w := e.resolvedWeight(p.ID, 2, 3); w != 80 {
		t.Fatalf("week 2 bench %v, want no progression", w)
	}
}

func TestPutKeepsEditedSetsThroughHooks(t *testing.T) {
	e := setup(t)
	e.activate(hooked, nil)
	pushed := e.create(logs.NewRecord{Date: today})
	if err := e.s.DeleteWorkoutRecord(e.ctx, e.user, pushed.ID); err != nil {
		t.Fatal(err)
	}
	pushed.Entries[0].Sets[2] = edited(95)
	r, err := e.s.PutWorkoutRecord(e.ctx, e.user, pushed)
	if err != nil || !r.Entries[0].Sets[2].Edited || !r.Modified {
		t.Fatalf("put %+v, %v", r.Entries[0].Sets, err)
	}
	sets := r.Entries[0].Sets
	sets[0].Completed, sets[0].Reps, sets[0].Weight = true, 5, sets[0].Planned.Weight
	sets = e.update(r.ID, r.Entries).Entries[0].Sets
	if sets[1].Planned.Weight.Value != 90 || sets[2].Planned.Weight.Value != 95 {
		t.Fatalf("after the push: %+v, want the hook to skip the edited set", sets)
	}
}

func TestShrinkingHookKeepsEditedSet(t *testing.T) {
	e := setup(t)
	e.activate(shrinking, nil)
	r := e.create(logs.NewRecord{Date: today})
	sets := r.Entries[0].Sets
	sets[0].Completed, sets[0].Reps, sets[0].Weight = true, 5, sets[0].Planned.Weight
	sets[2] = edited(95)
	if sets = e.update(r.ID, r.Entries).Entries[0].Sets; len(sets) != 3 || !sets[2].Edited {
		t.Fatalf("after the shrink: %+v, want the edited set kept among 3", sets)
	}
}

// TestPutFillsSnapshotFromProgram: a device or import sends the day's header
// only; the server snapshots the plan so ad hoc entries are told apart.
func TestPutFillsSnapshotFromProgram(t *testing.T) {
	e := setup(t)
	p := e.activate(base, &monday)
	bench := func(n int) model.RecordEntry {
		en := model.RecordEntry{Name: "Bench Press", Alternates: []string{"Bench Press"}}
		for range n {
			s := edited(80)
			s.Edited, s.Completed, s.Reps, s.Weight = false, true, 5, s.Planned.Weight
			en.Sets = append(en.Sets, s)
		}
		return en
	}
	curl := bench(2)
	curl.Name, curl.Alternates = "Curl", nil
	r, err := e.s.PutWorkoutRecord(e.ctx, e.user, logs.WorkoutRecord{ID: uuid.New(), Date: "2026-11-11", ProgramID: &p.ID,
		Day: model.ResolvedDay{Week: 1, Day: 3}, Entries: []model.RecordEntry{bench(3), bench(1), curl}})
	if err != nil {
		t.Fatal(err)
	}
	if plan := logs.PlanEntries(&r.Day, r.Entries); len(r.Day.Exercises) != 1 || r.Day.Name != "Upper" || !slices.Equal(plan, []bool{true, false, false}) || !r.Modified {
		t.Fatalf("pushed record day %+v plan %v modified %v", r.Day, plan, r.Modified)
	}
	e.finalize(r.ID)
	if w := e.resolvedWeight(p.ID, 2, 3); w != 82.5 {
		t.Fatalf("week 2 bench %v, want one progression", w)
	}
}

// Clients send an empty slug after switching an alternate or adding an exercise by
// name; the server resolves it on every write so insights never see "" (R80).
func TestWritesFillMissingSlugs(t *testing.T) {
	e := setup(t)
	e.activate(base, &monday)
	want, err := catalog.ResolveName(e.ctx, e.q, e.user, "Bench Press")
	if err != nil {
		t.Fatal(err)
	}
	bench := model.RecordEntry{Name: "bench press", Sets: []model.RecordSet{edited(60)}}

	r := e.create(logs.NewRecord{Date: "2026-11-11"})
	r = e.update(r.ID, append(r.Entries, bench))
	if got := r.Entries[1].Slug; got != want {
		t.Fatalf("update slug = %q, want %q", got, want)
	}
	e.finalize(r.ID)
	r.Entries[1].Slug = ""
	if r = e.update(r.ID, r.Entries); r.Entries[1].Slug != want {
		t.Fatalf("finalized update slug = %q", r.Entries[1].Slug)
	}

	adhoc := e.create(logs.NewRecord{Date: "2026-11-12", AdHoc: &logs.AdHoc{Name: "Extra", Entries: []model.RecordEntry{bench}}})
	if adhoc.Entries[0].Slug != want {
		t.Fatalf("ad hoc slug = %q", adhoc.Entries[0].Slug)
	}

	pushed := logs.WorkoutRecord{ID: uuid.New(), Date: "2026-11-13", Entries: []model.RecordEntry{bench}}
	got, err := e.s.PutWorkoutRecord(e.ctx, e.user, pushed)
	if err != nil || got.Entries[0].Slug != want {
		t.Fatalf("put slug = %q, %v", got.Entries[0].Slug, err)
	}
}
