package logs

import (
	"slices"

	"github.com/gdsoumya/plusultra/internal/model"
)

// PlanEntries reports, per entry, whether it does one of the snapshot day's
// exercises: the first entry with an exercise's progression key is its plan
// entry, any other is ad hoc. Only plan entries run hooks and progressions. A
// day without planned exercises, as an ad hoc or imported record has, is no
// template, so every entry is a plan entry.
func PlanEntries(day *model.ResolvedDay, entries []model.RecordEntry) []bool {
	out := make([]bool, len(entries))
	if len(day.Exercises) == 0 {
		for i := range out {
			out[i] = true
		}
		return out
	}
	left := make(map[model.ProgKey]bool, len(day.Exercises))
	for _, e := range day.Exercises {
		left[progKey(e.Alternates, e.Name, e.Label)] = true
	}
	for i, en := range entries {
		k := progKey(en.Alternates, en.Name, en.Label)
		out[i] = left[k]
		delete(left, k)
	}
	return out
}

// progKey is the progression key of an exercise: its first alternate, or
// its name, with its label.
func progKey(alternates []string, name, label string) model.ProgKey {
	if len(alternates) > 0 {
		name = alternates[0]
	}
	return model.ProgKey{Exercise: name, Label: label}
}

// modified reports whether entries deviate from the planned day: an edited
// set, a plan exercise with another number of working sets (an import keeps
// one set for all warmups), an exercise added or one left out. A day without
// planned exercises deviates only by edited sets.
func modified(day *model.ResolvedDay, entries []model.RecordEntry) bool {
	edited := slices.ContainsFunc(entries, func(en model.RecordEntry) bool {
		return slices.ContainsFunc(en.Sets, func(s model.RecordSet) bool { return s.Edited })
	})
	if edited || len(day.Exercises) == 0 {
		return edited
	}
	working := make(map[model.ProgKey]int, len(day.Exercises))
	for _, e := range day.Exercises {
		working[progKey(e.Alternates, e.Name, e.Label)] = len(e.Sets) - countFunc(e.Sets, func(s model.ResolvedSet) bool { return s.Warmup })
	}
	for _, en := range entries {
		k := progKey(en.Alternates, en.Name, en.Label)
		n, ok := working[k]
		if !ok || n != len(en.Sets)-countFunc(en.Sets, func(s model.RecordSet) bool { return s.Warmup || s.Planned.Warmup }) {
			return true
		}
		delete(working, k)
	}
	return len(working) > 0
}

func countFunc[T any](xs []T, f func(T) bool) int {
	n := 0
	for _, x := range xs {
		if f(x) {
			n++
		}
	}
	return n
}
