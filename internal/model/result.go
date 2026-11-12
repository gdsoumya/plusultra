package model

import "go.starlark.net/starlark"

// SetResult is one working set of a workout: as done when Completed, else as
// the workout currently plans it. MinReps and Rest are only read for a set
// not completed. WorkTime and Rest are in seconds.
type SetResult struct {
	Reps, MinReps int
	Weight        Weight
	RPE           float64
	Rest          int
	Completed     bool
	WorkTime      int
	RepsLeft      int
}

// ExerciseResult is an exercise as done on week, day. Variation is the 1-based
// set variation done, 0 for the current one. Sets lists the working sets,
// warmups excluded. PromptValues answers the progression's prompt() state.
type ExerciseResult struct {
	Key                  ProgKey
	Week, Day, Variation int
	Sets                 []SetResult
	PromptValues         map[string]starlark.Value
}
