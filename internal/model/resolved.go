package model

// ResolvedSet is one concrete set with its weight rounded to the user's equipment.
type ResolvedSet struct {
	Reps, MinReps                     int
	Weight                            Weight
	RPE                               float64
	Rest, Work                        int // seconds
	Amrap, Ask, LogRPE, Auto, CountUp bool
	Warmup                            bool
	Label                             string
}

// ResolvedExercise is an exercise on a resolved day. Sets lists warmups first.
// Name is Alternates[Current-1]. Slug is Name's catalog slug, empty when unknown.
type ResolvedExercise struct {
	Name, Equipment, Label string
	Slug                   string `json:",omitempty"` // set by the programs service, not the engine
	Alternates             []string
	Current                int
	Desc                   string
	Superset               string
	Unilateral             bool   `json:",omitempty"` // from the user's data for Name
	Progression            string `json:",omitempty"` // human summary of the progression, empty for none
	Sets                   []ResolvedSet
}

// ResolvedDay is a program day with concrete sets for one user.
type ResolvedDay struct {
	Week, Day        int
	Name, Desc, Kind string
	Exercises        []ResolvedExercise
}
