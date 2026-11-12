package engine

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// ctxKey is the thread-local holding the *ctxValue of a progression or update run.
const ctxKey = "engine.ctx"

// scriptFuncs are the functions progression and update scripts call.
var scriptFuncs = map[string]builtinFn{
	"calc_1rm":            calc1RM,
	"training_max":        trainingMax,
	"rpe_multiplier":      rpeMultiplier,
	"round_weight":        withCtx(rounder(0)),
	"increment":           withCtx(rounder(1)),
	"decrement":           withCtx(rounder(-1)),
	"is_exercise":         withCtx(isExercise),
	"is_program_exercise": withCtx(isProgramExercise),
	"floor":               roundFn(math.Floor),
	"ceil":                roundFn(math.Ceil),
	"round":               roundFn(roundHalfUp),
	"sum":                 sum,
	"zero_or_gte":         zeroOrGte,
}

type ctxFn = func(*ctxValue, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)

// withCtx adapts f to a builtin that only works while a ctx runs.
func withCtx(f ctxFn) builtinFn {
	return func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		c, ok := t.Local(ctxKey).(*ctxValue)
		if !ok {
			return nil, fmt.Errorf("%s: only inside a progression or update", b.Name())
		}
		return f(c, b, args, kwargs)
	}
}

func calc1RM(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	w, reps, err := weightReps(b, args, kwargs)
	if err != nil {
		return nil, err
	}
	return weightOf(epley(w, reps)), nil
}

func trainingMax(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	w, reps, err := weightReps(b, args, kwargs)
	if err != nil {
		return nil, err
	}
	tm := epley(w, reps)
	tm.Value *= 0.9
	return weightOf(tm), nil
}

func weightReps(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (model.Weight, int, error) {
	var w *weightValue
	var reps int
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "weight", &w, "reps", &reps); err != nil {
		return model.Weight{}, 0, err
	}
	if reps < 0 {
		return model.Weight{}, 0, fmt.Errorf("%s: reps must not be negative", b.Name())
	}
	return w.Weight(), reps, nil
}

// epley estimates a 1RM from weight lifted for reps.
func epley(w model.Weight, reps int) model.Weight {
	w.Value *= 1 + float64(reps)/30
	return w
}

// rpeChart is the RTS chart's percent of 1RM in half-rep steps of reps done
// plus reps in reserve, minus one. Each row of the chart (reps 1-12 at RPE
// 6-10) is a window of this one sequence.
var rpeChart = [...]float64{100, 97.8, 95.5, 93.9, 92.2, 90.7, 89.2, 87.8, 86.3, 85.0, 83.7, 82.4, 81.1, 79.9,
	78.6, 77.4, 76.2, 75.1, 73.9, 72.3, 70.7, 69.4, 68.0, 66.7, 65.3, 64.0, 62.6, 61.3, 59.9, 58.6, 57.4}

func rpeMultiplier(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var reps, rpe number
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "reps", &reps, "rpe", &rpe); err != nil {
		return nil, err
	}
	if math.IsNaN(float64(reps)) || math.IsNaN(float64(rpe)) {
		return nil, fmt.Errorf("%s: reps and rpe must be numbers", b.Name())
	}
	return starlark.Float(rpeMult(float64(reps), float64(rpe))), nil
}

// rpeMult is the fraction of 1RM lifted for reps at rpe, interpolated
// between the chart's half steps. It is rounded to 4 decimals so scripts and
// docs do not see float noise like 0.8109999999999999.
func rpeMult(reps, rpe float64) float64 {
	k := 2 * (min(max(reps, 1), 12) - 1 + 10 - min(max(rpe, 6), 10))
	i := int(k)
	lo, hi := rpeChart[i], rpeChart[min(i+1, len(rpeChart)-1)]
	return math.Round((lo+(hi-lo)*(k-float64(i)))*100) / 10000
}

// rounder rounds a weight to the exercise's equipment (dir 0), or steps to the
// next loadable weight above (1) or below (-1) it.
func rounder(dir float64) ctxFn {
	return func(c *ctxValue, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var w *weightValue
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &w); err != nil {
			return nil, err
		}
		units := cmp.Or(c.st.Units, model.UnitKg)
		return weightOf(convert(nextWeight(convert(w.Weight(), units), c.rounding, dir), w.unit)), nil
	}
}

// nextWeight is w rounded to r for dir 0, else the nearest weight r loads
// above (dir 1) or below (dir -1) it, or w rounded when there is none.
func nextWeight(w model.Weight, r model.Rounding, dir float64) model.Weight {
	cur := RoundWeight(w, r)
	if dir == 0 {
		return cur
	}
	if r.Kind == model.RoundFixed && len(r.Plates) > 0 {
		next := slices.DeleteFunc(slices.Clone(r.Plates), func(p float64) bool { return (p-cur.Value)*dir <= 0 })
		if len(next) > 0 {
			cur.Value = slices.Min(next)
			if dir < 0 {
				cur.Value = slices.Max(next)
			}
		}
		return cur
	}
	d := loadStep(r, w.Unit)
	for i := 1; i <= 100; i++ { // a sparse plate set can round the first few steps back to cur
		v := cur.Value + dir*float64(i)*d
		if v <= 0 || v > maxWeight(w.Unit) {
			break
		}
		if n := RoundWeight(model.Weight{Value: v, Unit: w.Unit}, r); (n.Value-cur.Value)*dir > 0 {
			return n
		}
	}
	return cur
}

// loadStep is the finest change r can load: a pair of the plates' GCD, the
// stack increment, or 2.5 kg / 5 lb for equipment that does not round.
func loadStep(r model.Rounding, u model.Unit) float64 {
	switch r.Kind {
	case model.RoundPlates:
		g := 0
		for _, p := range r.Plates {
			if p >= 0.001 {
				g = gcd(g, int(math.Round(p*1000)))
			}
		}
		if g > 0 {
			return float64(2*g) / 1000
		}
	case model.RoundStack:
		if r.Increment > 0 {
			return r.Increment
		}
	}
	if u == model.UnitLb {
		return 5
	}
	return 2.5
}

func isExercise(c *ctxValue, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &name); err != nil {
		return nil, err
	}
	return starlark.Bool(strings.EqualFold(strings.TrimSpace(name), c.planned.Name)), nil
}

// isProgramExercise matches "label: name" or "name" against the exercise's
// program identity, its first name and label.
func isProgramExercise(c *ctxValue, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s string
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &s); err != nil {
		return nil, err
	}
	label, name, ok := strings.Cut(s, ":")
	if !ok {
		label, name = "", s
	}
	return starlark.Bool(strings.EqualFold(strings.TrimSpace(label), c.key.Label) &&
		strings.EqualFold(strings.TrimSpace(name), c.key.Exercise)), nil
}

var funcDocs = docGroup("Script functions",
	BuiltinDoc{Name: "calc_1rm", Signature: "calc_1rm(weight, reps)",
		Doc:     "The estimated 1RM by the Epley formula, weight x (1 + reps / 30), as a weight. It works anywhere, also outside progressions.",
		Example: "# fragment\ndef estimate(ctx):\n    ctx.rm1 = calc_1rm(ctx.completed_weights[1], ctx.completed_reps[1])\n\nex(\"Squat\", sets=[s(1, 5, pct(80), amrap=True)], progress=custom(estimate))"},
	BuiltinDoc{Name: "training_max", Signature: "training_max(weight, reps)",
		Doc:     "90% of calc_1rm(weight, reps): a training max as 5/3/1 uses it. It works anywhere.",
		Example: "# fragment\nTM = training_max(kg(100), 5)\nex(\"Bench Press\", sets=[s(3, 5, TM * 0.75)])"},
	BuiltinDoc{Name: "rpe_multiplier", Signature: "rpe_multiplier(reps, rpe)",
		Doc: "The fraction of 1RM that reps at rpe take, from the RTS chart (reps 1-12, RPE 6-10, interpolated and clamped), " +
			"rounded to 4 decimals: rpe_multiplier(5, 8) is 0.811. It works anywhere.",
		Example: "# fragment\nex(\"Deadlift\", sets=[s(3, 5, pct(rpe_multiplier(5, 8) * 100))])"},
	BuiltinDoc{Name: "round_weight", Signature: "round_weight(weight)",
		Doc:     "Rounds weight to the nearest weight the exercise's equipment loads, in the weight's unit. Only inside progress and update.",
		Example: "# fragment\ndef heavier(ctx):\n    ctx.weights = round_weight(ctx.completed_weights[1] * 1.05)\n\nex(\"Bench Press\", sets=[s(3, 5, kg(80))], progress=custom(heavier))"},
	BuiltinDoc{Name: "increment", Signature: "increment(weight)",
		Doc: "The next weight above weight that the exercise's equipment loads, or 2.5 kg / 5 lb more when the equipment does not round. " +
			"Only inside progress and update.",
		Example: "# fragment\ndef step_up(ctx):\n    if ctx.completed_reps >= ctx.reps:\n        ctx.weights = increment(ctx.weights[1])\n\n" +
			"ex(\"Lateral Raise\", eq=\"Dumbbell\", warmup=[], sets=[s(3, 12, kg(10))], progress=custom(step_up))"},
	BuiltinDoc{Name: "decrement", Signature: "decrement(weight)",
		Doc:     "The next weight below weight that the exercise's equipment loads. Only inside progress and update.",
		Example: "# fragment\ndef step_down(ctx):\n    if not (ctx.completed_reps >= ctx.reps):\n        ctx.weights = decrement(ctx.weights[1])\n\nex(\"Bench Press\", sets=[s(3, 8, kg(60))], progress=custom(step_down))"},
	BuiltinDoc{Name: "is_exercise", Signature: "is_exercise(name)",
		Doc: "True when the active alternate is named name, the full name as written, ignoring case. Only inside progress and update; use it to branch in a function several exercises share.",
		Example: "# fragment\ndef main_lift(ctx):\n    if ctx.completed_reps >= ctx.reps:\n        ctx.weights += kg(5) if is_exercise(\"Deadlift\") else kg(2.5)\n\n" +
			"ex(\"Deadlift\", sets=[s(1, 5, kg(140))], progress=custom(main_lift))\nex(\"Bench Press\", sets=[s(3, 5, kg(80))], progress=custom(main_lift))"},
	BuiltinDoc{Name: "is_program_exercise", Signature: "is_program_exercise(name)",
		Doc: "True when the running program line is name, written \"label: Name\", or \"Name\" when it has no label: its first name and label, ignoring case. Only inside progress and update.",
		Example: "# fragment\ndef tiers(ctx):\n    if ctx.completed_reps >= ctx.reps:\n        ctx.weights += kg(5) if is_program_exercise(\"t1: Squat\") else kg(2.5)\n\n" +
			"ex(\"Squat\", label=\"t1\", sets=[s(5, 3, kg(100))], progress=custom(tiers))\nex(\"Squat\", label=\"t2\", sets=[s(3, 10, kg(70))], progress=custom(tiers))"},
)
