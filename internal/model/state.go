package model

import (
	"maps"

	"go.starlark.net/starlark"
)

// Rounding kinds.
const (
	RoundPlates     = "plates"
	RoundFixed      = "fixed"
	RoundStack      = "stack"
	RoundBodyweight = "bodyweight"
)

// Rounding is how weights round to the user's equipment, in the user's units.
// Plates holds the per-side plates for plates rounding and the available
// weights for fixed rounding.
type Rounding struct {
	Kind      string
	Increment float64
	Bar       float64
	Plates    []float64
}

// ProgKey identifies an exercise in a program by its first name and label.
type ProgKey struct{ Exercise, Label string }

// SetAddr addresses overlay set values. A zero Week, Day, Variation or Set
// matches all; the most specific matching address wins.
type SetAddr struct {
	Key                       ProgKey
	Week, Day, Variation, Set int
}

// DayAddr addresses an exercise's set count. A zero Week, Day or Variation
// matches all.
type DayAddr struct {
	Key                  ProgKey
	Week, Day, Variation int
}

// WeightOverride changes a set's script weight. Abs replaces it; otherwise it
// becomes script x Scale + Delta, where a zero Scale means 1.
type WeightOverride struct {
	Abs   *Weight
	Delta Weight
	Scale float64
}

// Override changes a script number: Abs replaces it, otherwise Delta is added.
type Override[T int | float64] struct {
	Abs   *T
	Delta T
}

// IntOverride is the override of reps, min_reps, rest and set counts.
type IntOverride = Override[int]

// FloatOverride is the override of rpe.
type FloatOverride = Override[float64]

// BoolOverride replaces a set flag.
type BoolOverride struct {
	Abs *bool
}

// Overlay is the mutable progression state applied over a program's script
// during resolution: the most specific override at a set's address changes
// that set's script value. Variation and description indexes are 1-based.
type Overlay struct {
	Weights                                    map[SetAddr]WeightOverride
	Reps, MinReps, Rest, Work                  map[SetAddr]IntOverride
	RPE                                        map[SetAddr]FloatOverride
	Amrap, Ask, LogRPE                         map[SetAddr]BoolOverride
	NumSets                                    map[DayAddr]IntOverride
	SetVariation, ExerciseVariation, DescIndex map[ProgKey]int
	State                                      map[ProgKey]map[string]starlark.Value
	RM1                                        map[string]Weight
}

// UserState is what resolution needs about the user. RM1, Unilateral and
// Equipment (the catalog's equipment of the exercise) are keyed by exercise
// name. Rounding is keyed by exercise name or equipment name, in any case;
// resolution tries the exercise name, then the script's equipment, then the
// catalog's, then DefaultRounding.
type UserState struct {
	Units           Unit
	Bodyweight      Weight
	RM1             map[string]Weight
	Unilateral      map[string]bool
	Equipment       map[string]string
	Rounding        map[string]Rounding
	DefaultRounding Rounding
	Overlay         Overlay
}

// Merge copies d's entries into o, replacing existing ones. State is replaced
// per key, since a progression delta holds a key's whole state.
func (o *Overlay) Merge(d Overlay) {
	o.Weights = merge(o.Weights, d.Weights)
	o.Reps = merge(o.Reps, d.Reps)
	o.MinReps = merge(o.MinReps, d.MinReps)
	o.Rest = merge(o.Rest, d.Rest)
	o.Work = merge(o.Work, d.Work)
	o.RPE = merge(o.RPE, d.RPE)
	o.Amrap = merge(o.Amrap, d.Amrap)
	o.Ask = merge(o.Ask, d.Ask)
	o.LogRPE = merge(o.LogRPE, d.LogRPE)
	o.NumSets = merge(o.NumSets, d.NumSets)
	o.SetVariation = merge(o.SetVariation, d.SetVariation)
	o.ExerciseVariation = merge(o.ExerciseVariation, d.ExerciseVariation)
	o.DescIndex = merge(o.DescIndex, d.DescIndex)
	o.State = merge(o.State, d.State)
	o.RM1 = merge(o.RM1, d.RM1)
}

func merge[K comparable, V any](dst, src map[K]V) map[K]V {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[K]V, len(src))
	}
	maps.Copy(dst, src)
	return dst
}
