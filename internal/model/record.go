package model

// RecordWeight is a weight as stored in a workout record's entries.
type RecordWeight struct {
	Value float64 `json:"value"`
	Unit  Unit    `json:"unit"`
}

// RecordPlanned is the set the program planned.
type RecordPlanned struct {
	Reps    int          `json:"reps"`
	Weight  RecordWeight `json:"weight"`
	RPE     float64      `json:"rpe"`
	Rest    int          `json:"rest"`
	Warmup  bool         `json:"warmup"`
	MinReps int          `json:"min_reps,omitempty"`
	Amrap   bool         `json:"amrap,omitempty"`
	Ask     bool         `json:"ask,omitempty"`
	Label   string       `json:"label,omitempty"`
	Work    int          `json:"work,omitempty"`
	LogRPE  bool         `json:"log_rpe,omitempty"`
	Auto    bool         `json:"auto,omitempty"`
	CountUp bool         `json:"count_up,omitempty"`
}

// PlanSet is s as a record stores it.
func PlanSet(s ResolvedSet) RecordPlanned {
	return RecordPlanned{Reps: s.Reps, Weight: RecordWeight(s.Weight), RPE: s.RPE, Rest: s.Rest, Warmup: s.Warmup,
		MinReps: s.MinReps, Amrap: s.Amrap, Ask: s.Ask, Label: s.Label, Work: s.Work, LogRPE: s.LogRPE, Auto: s.Auto, CountUp: s.CountUp}
}

// Resolved is the planned set in the engine's form.
func (p RecordPlanned) Resolved() ResolvedSet {
	return ResolvedSet{Reps: p.Reps, Weight: Weight(p.Weight), RPE: p.RPE, Rest: p.Rest, Warmup: p.Warmup,
		MinReps: p.MinReps, Amrap: p.Amrap, Ask: p.Ask, Label: p.Label, Work: p.Work, LogRPE: p.LogRPE, Auto: p.Auto, CountUp: p.CountUp}
}

// RecordSet is one set of a workout record: the plan and what was done.
type RecordSet struct {
	Planned   RecordPlanned `json:"planned"`
	Completed bool          `json:"completed"`
	Reps      int           `json:"reps"`
	Weight    RecordWeight  `json:"weight"`
	RPE       float64       `json:"rpe"`
	WorkTime  int           `json:"work_time"`
	RepsLeft  int           `json:"reps_left"`
	Warmup    bool          `json:"warmup"`
	Edited    bool          `json:"edited,omitempty"` // changed or added by the user; hooks leave it alone
}

// RecordEntry is one exercise of a workout record, the element type of the
// workout_records.entries JSON array. PromptValues answer the progression's
// prompt() state by name.
type RecordEntry struct {
	Name         string             `json:"name"`
	Slug         string             `json:"slug"`
	Equipment    string             `json:"equipment"`
	Label        string             `json:"label"`
	Alternates   []string           `json:"alternates"`
	Current      int                `json:"current"`
	Desc         string             `json:"desc,omitempty"`
	Superset     string             `json:"superset,omitempty"`
	Variation    int                `json:"variation,omitempty"`
	PromptValues map[string]float64 `json:"prompt_values,omitempty"`
	Unilateral   bool               `json:"unilateral,omitempty"` // reps is the right side, reps_left the left
	Sets         []RecordSet        `json:"sets"`
}

const kgPerLb = 0.45359237

// Kg returns w in kilograms; an unset unit counts as kg.
func (w Weight) Kg() float64 {
	if w.Unit == UnitLb {
		return w.Value * kgPerLb
	}
	return w.Value
}
