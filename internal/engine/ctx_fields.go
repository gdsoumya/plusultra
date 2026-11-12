package engine

import (
	"cmp"
	"errors"
	"fmt"
	"math"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// maxSeconds bounds rest and work timers.
const maxSeconds = 24 * 3600

type fieldKind int

const (
	kindWeight fieldKind = iota
	kindInt
	kindFloat
	kindBool
)

// persistFn records edit e at a in delta d, over the user's overlay base.
type persistFn func(d, base *model.Overlay, a model.SetAddr, e edit) error

// setField is a planned per-set vector. get and set see it as a float64; a
// weight's value is in its set's unit.
type setField struct {
	kind    fieldKind
	hi      float64
	get     func(*model.ResolvedSet) float64
	set     func(*model.ResolvedSet, float64)
	persist persistFn
}

var setFields = map[string]setField{
	"weights": {kind: kindWeight,
		get: func(s *model.ResolvedSet) float64 { return s.Weight.Value },
		set: func(s *model.ResolvedSet, v float64) { s.Weight.Value = v },
		persist: func(d, base *model.Overlay, a model.SetAddr, e edit) error {
			return assign(d.Weights, base.Weights, a, setAddr, joinSet, func(o model.WeightOverride) model.WeightOverride { return composeWeight(o, e) })
		}},
	"reps": {kind: kindInt, hi: maxListLen,
		get:     func(s *model.ResolvedSet) float64 { return float64(s.Reps) },
		set:     func(s *model.ResolvedSet, v float64) { setReps(s, int(v)) },
		persist: persistNum(func(o *model.Overlay) map[model.SetAddr]model.IntOverride { return o.Reps }, maxListLen)},
	"rpe": {kind: kindFloat, hi: 10,
		get:     func(s *model.ResolvedSet) float64 { return s.RPE },
		set:     func(s *model.ResolvedSet, v float64) { s.RPE = v },
		persist: persistNum(func(o *model.Overlay) map[model.SetAddr]model.FloatOverride { return o.RPE }, 10)},
	"min_reps": intSetField(func(s *model.ResolvedSet) *int { return &s.MinReps }, maxListLen,
		persistNum(func(o *model.Overlay) map[model.SetAddr]model.IntOverride { return o.MinReps }, maxListLen)),
	"rest": intSetField(func(s *model.ResolvedSet) *int { return &s.Rest }, maxSeconds,
		persistNum(func(o *model.Overlay) map[model.SetAddr]model.IntOverride { return o.Rest }, maxSeconds)),
	"work": intSetField(func(s *model.ResolvedSet) *int { return &s.Work }, maxSeconds,
		persistNum(func(o *model.Overlay) map[model.SetAddr]model.IntOverride { return o.Work }, maxSeconds)),
	"amrap": boolSetField(func(s *model.ResolvedSet) *bool { return &s.Amrap },
		func(o *model.Overlay) map[model.SetAddr]model.BoolOverride { return o.Amrap }),
	"ask": boolSetField(func(s *model.ResolvedSet) *bool { return &s.Ask },
		func(o *model.Overlay) map[model.SetAddr]model.BoolOverride { return o.Ask }),
	"log_rpe": boolSetField(func(s *model.ResolvedSet) *bool { return &s.LogRPE },
		func(o *model.Overlay) map[model.SetAddr]model.BoolOverride { return o.LogRPE }),
}

func persistNum[T int | float64](m func(*model.Overlay) map[model.SetAddr]model.Override[T], hi T) persistFn {
	return func(d, base *model.Overlay, a model.SetAddr, e edit) error {
		return assign(m(d), m(base), a, setAddr, joinSet, func(o model.Override[T]) model.Override[T] { return composeNum(o, e, hi) })
	}
}

func intSetField(f func(*model.ResolvedSet) *int, hi int, persist persistFn) setField {
	return setField{kind: kindInt, hi: float64(hi), persist: persist,
		get: func(s *model.ResolvedSet) float64 { return float64(*f(s)) },
		set: func(s *model.ResolvedSet, v float64) { *f(s) = int(v) }}
}

// boolSetField is a set flag. Flags have no relative writes, so every edit replaces.
func boolSetField(f func(*model.ResolvedSet) *bool, m func(*model.Overlay) map[model.SetAddr]model.BoolOverride) setField {
	return setField{kind: kindBool, hi: 1,
		get: func(s *model.ResolvedSet) float64 { return boolNum(*f(s)) },
		set: func(s *model.ResolvedSet, v float64) { *f(s) = v != 0 },
		persist: func(d, base *model.Overlay, a model.SetAddr, e edit) error {
			b := e.v != 0
			return assign(m(d), m(base), a, setAddr, joinSet, func(model.BoolOverride) model.BoolOverride { return model.BoolOverride{Abs: &b} })
		}}
}

func boolNum(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// value is s's field as a script value; weights are given in unit u.
func (f setField) value(s *model.ResolvedSet, u model.Unit) starlark.Value {
	switch f.kind {
	case kindWeight:
		return weightOf(convert(s.Weight, u))
	case kindInt:
		return starlark.MakeInt(int(f.get(s)))
	case kindFloat:
		return starlark.Float(f.get(s))
	}
	return starlark.Bool(f.get(s) != 0)
}

// parse converts a script value into an absolute edit within the field's range.
func (f setField) parse(v starlark.Value) (edit, error) {
	switch f.kind {
	case kindWeight:
		w, err := toWeight(v)
		return edit{abs: true, v: w.Value, unit: w.Unit}, err
	case kindInt:
		n, err := starlark.AsInt32(v)
		return edit{abs: true, v: clamp(float64(n), 0, f.hi)}, err
	case kindFloat:
		var x number
		if err := x.Unpack(v); err != nil || math.IsNaN(float64(x)) {
			return edit{}, cmp.Or(err, errors.New("want a number"))
		}
		return edit{abs: true, v: clamp(float64(x), 0, f.hi)}, nil
	}
	return edit{abs: true, v: boolNum(bool(v.Truth()))}, nil
}

// apply changes s's field by e, as the overlay would.
func (f setField) apply(s *model.ResolvedSet, e edit) {
	if f.kind == kindWeight {
		s.Weight = applyWeight(composeWeight(model.WeightOverride{}, e), s.Weight)
		return
	}
	f.set(s, applyNum(composeNum(model.FloatOverride{}, e, f.hi), f.get(s), f.hi))
}

// resultFields are the per-set vectors of the workout's results. A set not
// completed reads as zero.
var resultFields = map[string]func(model.SetResult, model.Unit) starlark.Value{
	"completed_weights":   func(r model.SetResult, u model.Unit) starlark.Value { return weightOf(convert(r.Weight, u)) },
	"completed_reps":      func(r model.SetResult, _ model.Unit) starlark.Value { return starlark.MakeInt(r.Reps) },
	"completed_reps_left": func(r model.SetResult, _ model.Unit) starlark.Value { return starlark.MakeInt(r.RepsLeft) },
	"completed_rpe":       func(r model.SetResult, _ model.Unit) starlark.Value { return starlark.Float(r.RPE) },
	"completed_work":      func(r model.SetResult, _ model.Unit) starlark.Value { return starlark.MakeInt(r.WorkTime) },
}

var ctxNames = []string{"add_sets", "at", "bodyweight", "completed_number_of_sets", "day", "day_in_week", "description_index",
	"exercise", "exercise_variation", "label", "number_of_sets", "original_weights", "program_exercise",
	"program_number_of_sets", "rm1", "set_index", "set_variation", "sets", "shared", "state", "week"}

// toWeight converts a script weight, clamped to 0 and the weight limit.
func toWeight(v starlark.Value) (model.Weight, error) {
	w, ok := v.(*weightValue)
	switch {
	case !ok:
		return model.Weight{}, fmt.Errorf("got %s, want kg or lb", v.Type())
	case math.IsNaN(w.v) || math.IsInf(w.v, 0):
		return model.Weight{}, errors.New("weight must be finite")
	}
	return model.Weight{Value: clamp(w.v, 0, maxWeight(w.unit)), Unit: w.unit}, nil
}

// toCount converts a script int, clamped to 0 and hi.
func toCount(v starlark.Value, hi int) (int, error) {
	n, err := starlark.AsInt32(v)
	return clamp(n, 0, hi), err
}
