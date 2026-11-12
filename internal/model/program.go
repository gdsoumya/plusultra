// Package model holds the plain data types produced by the engine.
package model

import "go.starlark.net/starlark"

// Unit is a weight unit.
type Unit string

// Weight units.
const (
	UnitKg Unit = "kg"
	UnitLb Unit = "lb"
)

// Weight is an absolute weight.
type Weight struct {
	Value float64
	Unit  Unit
}

// LoadKind says how a set's load is given.
type LoadKind int

// Load kinds.
const (
	LoadAbsolute LoadKind = iota
	LoadPercent
	LoadBodyweight
	LoadNone
)

// Load is a set's load: an absolute weight, a percent of 1RM, or bodyweight plus Delta.
type Load struct {
	Kind   LoadKind
	Weight Weight
	Pct    float64
	Delta  Weight
}

// Valid reports whether u is a known unit.
func (u Unit) Valid() bool { return u == UnitKg || u == UnitLb }

// SetGroup is Count sets of MinReps to MaxReps reps (equal for fixed reps).
type SetGroup struct {
	Count, MinReps, MaxReps int
	Load                    Load
	RPE                     float64
	Rest, Work              int // seconds
	Auto, CountUp           bool
	Amrap, Ask, LogRPE      bool
	Label                   string
}

// Exercise is one ex() declaration. Names[Current-1] is the active alternate.
type Exercise struct {
	Names      []string
	Current    int
	Equipment  string
	Label      string
	Weeks      []int // nil: the weeks of its day
	Sets       []SetGroup
	Variations [][]SetGroup // set when the exercise has set variations; Sets is then empty
	// SetVariation is the 1-based set variation to start with; 0 is the first.
	SetVariation int
	Warmup       []SetGroup
	WarmupSet    bool     // false: use the default warmup
	Desc         []string // nil: inherit; empty: no description
	Superset     string
	Tags         []int
	Progress     *Progression // its own or the one it inherits; nil for none
	Update       *Hook        // its own or the one it inherits; nil for none
	Line         int
}

// Progression kinds.
const (
	ProgressLP      = "lp"
	ProgressDP      = "dp"
	ProgressSumReps = "sum_reps"
	ProgressCustom  = "custom"
)

// Progression is how an exercise progresses after a workout. Fields beyond
// Kind are used by the kinds that need them. lp adds Inc, or IncPct percent
// when that is set, after After successes in a row, and after FailAfter
// failures in a row subtracts FailDec or scales by FailReset.
type Progression struct {
	Kind      string
	Inc       Weight
	IncPct    float64
	After     int
	FailAfter int
	FailDec   Weight
	FailReset float64
	Lo, Hi    int
	Target    int
	Fn        *starlark.Function
	State     map[string]StateVar
}

// StateVar is a custom progression's initial state value. Prompt values are
// asked from the user after the exercise.
type StateVar struct {
	Value  starlark.Value
	Prompt bool
}

// Hook is a script run on each completed set.
type Hook struct {
	Fn *starlark.Function
}

// Day is one day() declaration with the exercises declared after it.
type Day struct {
	Week, Num int
	Name      string
	Desc      string
	DescSet   bool // false: inherit Desc
	Kind      string
	Weeks     []int // nil: only Week
	Exercises []Exercise
	Line      int
}

// Program is the structure built by evaluating a workout script. Days holds
// the raw declarations; DayAt gives the effective day for a week.
type Program struct {
	Name     string
	Weeks    int
	Units    Unit
	Loop     bool
	Notes    string
	WeekDesc map[int]string
	Days     []Day
}
