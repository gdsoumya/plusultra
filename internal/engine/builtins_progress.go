package engine

import (
	"errors"
	"fmt"
	"maps"
	"math"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

type progressionValue struct {
	opaque
	p model.Progression
}

func (*progressionValue) Type() string     { return "progression" }
func (v *progressionValue) String() string { return fmt.Sprintf("progression(%s)", v.p.Kind) }

type promptValue struct {
	opaque
	v starlark.Value
}

func (v *promptValue) Freeze()        { v.v.Freeze() }
func (*promptValue) Type() string     { return "prompt" }
func (v *promptValue) String() string { return fmt.Sprintf("prompt(%s)", v.v) }

// lp adds inc after `after` successes in a row. After fail_after failures in a
// row (1 unless given), it subtracts fail_dec or scales by fail_reset.
func lp(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	p := model.Progression{Kind: model.ProgressLP, After: 1, FailAfter: math.MinInt} // no int32 argument gives MinInt
	var (
		inc   stepArg
		dec   incArg
		reset number
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "inc", &inc, "after?", &p.After, "fail_after??", &p.FailAfter,
		"fail_dec??", &dec, "fail_reset??", &reset); err != nil {
		return nil, err
	}
	p.Inc, p.IncPct, p.FailDec, p.FailReset = inc.w, inc.pct, model.Weight(dec), float64(reset)
	switch {
	case p.After < 1:
		return nil, fmt.Errorf("%s: after must be at least 1", b.Name())
	case !(p.FailReset >= 0 && p.FailReset < 1):
		return nil, fmt.Errorf("%s: fail_reset must be a fraction below 1, like 0.9", b.Name())
	case p.FailDec.Value > 0 && p.FailReset > 0:
		return nil, fmt.Errorf("%s: give fail_dec or fail_reset, not both", b.Name())
	case p.FailAfter == math.MinInt:
		p.FailAfter = 1
	case p.FailAfter < 1:
		return nil, fmt.Errorf("%s: fail_after must be at least 1", b.Name())
	case p.FailDec.Value == 0 && p.FailReset == 0:
		return nil, fmt.Errorf("%s: fail_after needs fail_dec or fail_reset", b.Name())
	}
	return &progressionValue{p: p}, nil
}

func dp(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	p := model.Progression{Kind: model.ProgressDP}
	var inc incArg
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "inc", &inc, "lo", &p.Lo, "hi", &p.Hi); err != nil {
		return nil, err
	}
	p.Inc = model.Weight(inc)
	if p.Lo < 1 || p.Lo > p.Hi {
		return nil, fmt.Errorf("%s: want 1 <= lo <= hi", b.Name())
	}
	return &progressionValue{p: p}, nil
}

func sumReps(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	p := model.Progression{Kind: model.ProgressSumReps}
	var inc incArg
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "inc", &inc, "target", &p.Target); err != nil {
		return nil, err
	}
	p.Inc = model.Weight(inc)
	if p.Target < 1 {
		return nil, fmt.Errorf("%s: target must be at least 1", b.Name())
	}
	return &progressionValue{p: p}, nil
}

func custom(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	p := model.Progression{Kind: model.ProgressCustom, State: make(map[string]model.StateVar, len(kwargs))}
	if err := starlark.UnpackPositionalArgs(b.Name(), args, nil, 1, &p.Fn); err != nil {
		return nil, err
	}
	for _, kv := range kwargs {
		p.State[string(kv[0].(starlark.String))] = stateVar(kv[1])
	}
	return &progressionValue{p: p}, nil
}

func prompt(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "default", &v); err != nil {
		return nil, err
	}
	return &promptValue{v: v}, nil
}

func stateVar(v starlark.Value) model.StateVar {
	v.Freeze() // the script must not change initial state after declaring it
	if p, ok := v.(*promptValue); ok {
		return model.StateVar{Value: p.v, Prompt: true}
	}
	return model.StateVar{Value: v}
}

// attachProgress sets e's progression, update hook and extra state. State
// without a progression is kept on an empty Kind for inheritProgress.
func attachProgress(e *model.Exercise, progress, update progressArg, state *starlark.Dict) error {
	e.Progress = progress.p
	switch u := update.p; {
	case u == nil:
	case u.Kind == progressNone:
		e.Update = &model.Hook{}
	case u.Kind != model.ProgressCustom || len(u.State) > 0:
		return errors.New(`update takes custom(fn) without state, or "none"; declare state on progress`)
	default:
		e.Update = &model.Hook{Fn: u.Fn}
	}
	if state == nil {
		return nil
	}
	switch {
	case e.Progress == nil:
		e.Progress = &model.Progression{}
	case e.Progress.Kind == progressNone:
		return errors.New("state needs progress")
	}
	if e.Progress.State == nil {
		e.Progress.State = map[string]model.StateVar{}
	}
	for _, kv := range state.Items() {
		k, ok := starlark.AsString(kv[0])
		if !ok {
			return fmt.Errorf("state keys must be strings, got %s", kv[0].Type())
		}
		e.Progress.State[k] = stateVar(kv[1])
	}
	return nil
}

// progressArg unpacks a progression, or "none" to turn an inherited one off.
type progressArg struct{ p *model.Progression }

func (a *progressArg) Unpack(v starlark.Value) error {
	switch v := v.(type) {
	case *progressionValue:
		p := v.p
		p.State = maps.Clone(p.State) // one progression value can feed many ex() calls via a template
		a.p = &p
		return nil
	case starlark.String:
		if v == "none" {
			a.p = &model.Progression{Kind: progressNone}
			return nil
		}
	}
	return fmt.Errorf(`got %s, want a progression or "none"`, v)
}

// incArg unpacks a progression increment, a kg or lb value.
type incArg model.Weight

func (i *incArg) Unpack(v starlark.Value) error {
	w, ok := v.(*weightValue)
	if !ok {
		return fmt.Errorf("got %s, want kg or lb", v.Type())
	}
	if !validWeight(w.v) {
		return errWeight
	}
	*i = incArg(w.Weight())
	return nil
}

// stepArg unpacks an lp increment: kg or lb, or pct(x) for x percent more.
type stepArg struct {
	w   model.Weight
	pct float64
}

func (s *stepArg) Unpack(v starlark.Value) error {
	p, ok := v.(*pctValue)
	if !ok {
		return (*incArg)(&s.w).Unpack(v)
	}
	if !validWeight(p.v) {
		return errWeight
	}
	s.pct = p.v
	return nil
}

var progressDocs = docGroup("Progressions",
	BuiltinDoc{Name: "lp", Signature: "lp(inc, after=1, fail_after=1, fail_dec=None, fail_reset=None)",
		Doc: "Linear progression. A workout succeeds when every set is completed with at least its target reps. " +
			"After `after` successes in a row, every set of the exercise in every week and day gains inc: kg() or lb(), or pct(x) for x percent more. " +
			"After fail_after failures in a row it subtracts fail_dec or scales by fail_reset, a fraction below 1 such as 0.9; " +
			"give one of them, and fail_after only with one. With neither, failures change nothing. " +
			"The streaks live in the exercise state as successes and failures.",
		Example: "# fragment\nex(\"Squat\", sets=[s(3, 5, kg(100))], progress=lp(kg(2.5), fail_after=3, fail_reset=0.9))\n" +
			"ex(\"Overhead Press\", sets=[s(3, 5, kg(40))], progress=lp(pct(5)))"},
	BuiltinDoc{Name: "dp", Signature: "dp(inc, lo, hi)",
		Doc: "Double progression. After a successful workout every set does one more rep, until the first set's reps reach hi; " +
			"the next success puts reps back to lo and adds inc to the weights. Write the sets at lo reps, not as a range.",
		Example: "# fragment\nex(\"Lateral Raise\", eq=\"Dumbbell\", warmup=[], sets=[s(3, 10, kg(10))], progress=dp(kg(2.5), 10, 15))"},
	BuiltinDoc{Name: "sum_reps", Signature: "sum_reps(inc, target)",
		Doc:     "Sum-of-reps progression: when the completed reps of all sets add up to target or more, the weights gain inc.",
		Example: "# fragment\nex(\"Bench Press\", sets=[s(3, 10, kg(60), amrap=True)], progress=sum_reps(kg(2.5), 36))"},
	BuiltinDoc{Name: "custom", Signature: "custom(fn, **state)",
		Doc: "A custom progression: fn(ctx) runs once after the workout is finished. Keyword arguments declare state variables " +
			"with their initial values, read and set as ctx.state.<name>; values may be None, bools, numbers, strings, weights, " +
			"lists, tuples and dicts. Define fn with def at the top level. As update=custom(fn), without state, " +
			"fn runs during the workout after each completed set instead.",
		Example: "# fragment\ndef three_strikes(ctx):\n    if ctx.completed_reps >= ctx.reps:\n        ctx.weights += kg(2.5)\n        ctx.state.misses = 0\n" +
			"    else:\n        ctx.state.misses += 1\n        if ctx.state.misses >= 3:\n            ctx.weights *= 0.9\n            ctx.state.misses = 0\n\n" +
			"ex(\"Squat\", sets=[s(3, 5, kg(100))], progress=custom(three_strikes, misses=0))"},
	BuiltinDoc{Name: "prompt", Signature: "prompt(default)",
		Doc:     "A state value the app asks the user for after the exercise, default when unanswered: custom(fn, rating=prompt(3)) or state={\"rating\": prompt(3)}.",
		Example: "# fragment\ndef by_feel(ctx):\n    if ctx.state.rating >= 4:\n        ctx.weights += kg(2.5)\n\nex(\"Bench Press\", sets=[s(3, 8, kg(60))], progress=custom(by_feel, rating=prompt(3)))"},
)
