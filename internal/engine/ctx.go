package engine

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// ctxValue is the ctx of a progression or update script.
//
// Read: week, day, day_in_week (both the day number), rm1, bodyweight,
// exercise (the active alternate), program_exercise ("label: Name" of the
// program line, or "Name"), label, set_index (update only), number_of_sets,
// program_number_of_sets (as the program plans them for this week, day and
// set variation, before this run's writes), completed_number_of_sets, set_variation,
// exercise_variation, description_index, state, shared(tag), and the vectors
// in setFields and resultFields plus original_weights (the planned weights
// before rounding). Planned vectors are the working sets as resolved for the
// workout, one per set done. Weights are in program units.
//
// shared(tag) is one store per tag. Any exercise reaches any tag, whether it
// carries the tag or not; a progression may write it.
//
// A progression's writes persist through delta, as overrides of the script:
// relative where the write is (see setVector and add_sets), else absolute. It
// writes every planned vector (for all weeks and days, or through
// at(week=, day=, variation=) views), number_of_sets and add_sets(n) (per set
// variation through a view), rm1, the variation and description indexes, and
// state. An update hook writes every planned vector, number_of_sets, add_sets
// and sets(...), for this workout's sets after set_index.
type ctxValue struct {
	opaque
	p         *model.Program
	st        *model.UserState
	e         *model.Exercise
	key       model.ProgKey
	res       model.ExerciseResult
	planned   model.ResolvedExercise
	warmups   []model.ResolvedSet
	orig      []model.ResolvedSet // working sets as planned
	raw       []model.Weight      // their weights before rounding
	sets      []model.ResolvedSet // working sets as the script changed them
	units     model.Unit
	rounding  model.Rounding
	rm1       model.Weight
	variation int            // the set variation of this workout
	index     map[string]int // set_variation, exercise_variation, description_index
	update    bool
	setIndex  int
	state     *stateValue
	shared    map[int]*stateValue
	delta     model.Overlay
}

func newCtx(p *model.Program, st *model.UserState, res model.ExerciseResult, update bool, setIndex int) (*ctxValue, error) {
	if err := checkResults(res.Sets); err != nil {
		return nil, err
	}
	e, err := findExercise(p, res)
	if err != nil {
		return nil, err
	}
	planned, raw, err := resolveExercise(e, res.Week, res.Day, st)
	if err != nil {
		return nil, err
	}
	nw := slices.IndexFunc(planned.Sets, func(s model.ResolvedSet) bool { return !s.Warmup })
	if nw < 0 {
		nw = len(planned.Sets)
	}
	work := planned.Sets[nw:]
	n := cmp.Or(len(res.Sets), len(work))
	sets := resize(slices.Clone(work), n) // stays empty when nothing is planned
	if setIndex < 0 || setIndex > len(sets) {
		return nil, fmt.Errorf("set index %d out of range 0-%d", setIndex, len(sets))
	}
	ov := &st.Overlay
	var decl map[string]model.StateVar
	if e.Progress != nil {
		decl = e.Progress.State
	}
	vars, err := loadState(decl, ov.State[res.Key], res.PromptValues)
	if err != nil {
		return nil, err
	}
	orig := slices.Clone(sets)
	if update {
		seed(sets, res.Sets)
	}
	variation := cmp.Or(res.Variation, setVariation(e, ov, res.Key))
	return &ctxValue{
		p: p, st: st, e: e, key: res.Key, res: res, planned: planned,
		warmups:   slices.Clone(planned.Sets[:nw]),
		orig:      orig,
		raw:       resize(raw, n),
		sets:      sets,
		units:     p.Units,
		rounding:  roundingFor(st, planned.Name, planned.Equipment),
		rm1:       cmp.Or(ov.RM1[planned.Name], st.RM1[planned.Name]),
		variation: variation,
		index: map[string]int{
			"set_variation":      variation,
			"exercise_variation": planned.Current,
			"description_index":  nth(cmp.Or(ov.DescIndex[res.Key], 1), max(len(e.Desc), 1)) + 1,
		},
		update: update, setIndex: setIndex,
		state:  &stateValue{vars: vars, readonly: update},
		shared: map[int]*stateValue{},
		delta: model.Overlay{Weights: map[model.SetAddr]model.WeightOverride{}, Reps: map[model.SetAddr]model.IntOverride{},
			MinReps: map[model.SetAddr]model.IntOverride{}, Rest: map[model.SetAddr]model.IntOverride{},
			Work: map[model.SetAddr]model.IntOverride{}, RPE: map[model.SetAddr]model.FloatOverride{},
			Amrap: map[model.SetAddr]model.BoolOverride{}, Ask: map[model.SetAddr]model.BoolOverride{},
			LogRPE: map[model.SetAddr]model.BoolOverride{}, NumSets: map[model.DayAddr]model.IntOverride{},
			SetVariation: map[model.ProgKey]int{}, ExerciseVariation: map[model.ProgKey]int{},
			DescIndex: map[model.ProgKey]int{}, State: map[model.ProgKey]map[string]starlark.Value{},
			RM1: map[string]model.Weight{}},
	}, nil
}

// checkResults rejects results no workout produces; they reach vectors and
// arithmetic unclamped.
func checkResults(sets []model.SetResult) error {
	if len(sets) > maxListLen {
		return fmt.Errorf("more than %d sets", maxListLen)
	}
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	for i, r := range sets {
		switch {
		case !finite(r.Weight.Value) || !finite(r.RPE):
			return fmt.Errorf("set %d: weight and rpe must be finite", i+1)
		case r.Reps < 0 || r.MinReps < 0 || r.Rest < 0 || r.WorkTime < 0 || r.RepsLeft < 0:
			return fmt.Errorf("set %d: reps and times must not be negative", i+1)
		}
	}
	return nil
}

// seed takes an update's pending sets from the workout as the caller carries
// it, so earlier hook runs stick. A completed set keeps its plan, which hooks
// compare with what was done.
func seed(sets []model.ResolvedSet, results []model.SetResult) {
	for i, r := range results[:min(len(results), len(sets))] {
		s := &sets[i]
		if r.Completed {
			continue
		}
		if r.Weight.Unit != "" {
			s.Weight = applyWeight(model.WeightOverride{Abs: &r.Weight}, s.Weight)
		}
		if r.Reps > 0 {
			setReps(s, min(r.Reps, maxListLen))
		}
		if r.MinReps > 0 {
			s.MinReps = min(r.MinReps, maxListLen)
		}
		if r.RPE > 0 {
			s.RPE = min(r.RPE, 10)
		}
		if r.Rest > 0 {
			s.Rest = min(r.Rest, maxSeconds)
		}
	}
}

// findExercise is the exercise res reports, as placed on its week and day.
func findExercise(p *model.Program, res model.ExerciseResult) (*model.Exercise, error) {
	if res.Week >= 1 && res.Week <= p.Weeks && res.Day >= 1 {
		if d := p.DayAt(res.Week, res.Day); d != nil {
			for i := range d.Exercises {
				if e := &d.Exercises[i]; (model.ProgKey{Exercise: e.Names[0], Label: e.Label}) == res.Key {
					return e, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("week %d day %d has no exercise %q with label %q", res.Week, res.Day, res.Key.Exercise, res.Key.Label)
}

func (*ctxValue) Type() string     { return "ctx" }
func (c *ctxValue) String() string { return fmt.Sprintf("ctx(%s)", c.planned.Name) }

func (c *ctxValue) AttrNames() []string {
	names := slices.Concat(ctxNames, slices.Collect(maps.Keys(setFields)), slices.Collect(maps.Keys(resultFields)))
	slices.Sort(names)
	return names
}

func (c *ctxValue) Attr(name string) (starlark.Value, error) {
	if _, ok := setFields[name]; ok {
		v := c.vector(name, c.sets)
		v.ctx, v.scope = c, model.SetAddr{Key: c.key}
		return v, nil
	}
	if f, ok := resultFields[name]; ok {
		elems := make([]starlark.Value, len(c.res.Sets))
		for i, r := range c.res.Sets {
			if !r.Completed {
				r = model.SetResult{}
			}
			elems[i] = f(r, c.units)
		}
		return &vec{elems: elems}, nil
	}
	switch name {
	case "week":
		return starlark.MakeInt(c.res.Week), nil
	case "day", "day_in_week":
		return starlark.MakeInt(c.res.Day), nil
	case "rm1":
		return weightOf(convert(c.rm1, c.units)), nil
	case "bodyweight":
		return weightOf(convert(c.st.Bodyweight, c.units)), nil
	case "exercise":
		return starlark.String(c.planned.Name), nil
	case "program_exercise":
		return starlark.String(programName(c.key)), nil
	case "label":
		return starlark.String(c.key.Label), nil
	case "number_of_sets":
		return starlark.MakeInt(len(c.sets)), nil
	case "program_number_of_sets":
		return starlark.MakeInt(plannedCount(c.e, &c.st.Overlay, c.res.Week, c.res.Day, c.variation)), nil
	case "completed_number_of_sets":
		n := 0
		for _, r := range c.res.Sets {
			if r.Completed {
				n++
			}
		}
		return starlark.MakeInt(n), nil
	case "set_variation", "exercise_variation", "description_index":
		return starlark.MakeInt(c.index[name]), nil
	case "original_weights":
		elems := make([]starlark.Value, len(c.raw))
		for i, w := range c.raw {
			elems[i] = weightOf(convert(w, c.units))
		}
		return &vec{elems: elems}, nil
	case "state":
		return c.state, nil
	case "shared":
		return starlark.NewBuiltin("shared", c.sharedState), nil
	case "at":
		return starlark.NewBuiltin("at", c.at), nil
	case "add_sets":
		return c.addSets(model.DayAddr{Key: c.key}), nil
	case "set_index", "sets":
		if !c.update {
			return nil, fmt.Errorf("ctx.%s is only available in update", name)
		}
		if name == "sets" {
			return starlark.NewBuiltin("sets", c.setRange), nil
		}
		return starlark.MakeInt(c.setIndex), nil
	}
	return nil, nil
}

// vector is a read-only vector of field name, one element per set.
func (c *ctxValue) vector(name string, sets []model.ResolvedSet) *vec {
	v := &vec{elems: make([]starlark.Value, len(sets)), name: name}
	for i := range sets {
		v.elems[i] = setFields[name].value(&sets[i], c.units)
	}
	return v
}
