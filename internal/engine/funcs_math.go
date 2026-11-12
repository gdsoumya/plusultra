package engine

import (
	"fmt"
	"math"
	"slices"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// roundFn applies f to a number, giving an int, or to a weight in its unit.
func roundFn(f func(float64) float64) builtinFn {
	return func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var x starlark.Value
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &x); err != nil {
			return nil, err
		}
		if w, ok := x.(*weightValue); ok {
			return &weightValue{quantity{f(w.v), w.unit}}, nil
		}
		n, ok := starlark.AsFloat(x)
		if !ok {
			return nil, fmt.Errorf("%s: got %s, want a number or weight", b.Name(), x.Type())
		}
		i, err := starlark.NumberToInt(starlark.Float(f(n)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name(), err)
		}
		return i, nil
	}
}

// roundHalfUp rounds half away from zero, halves towards +Inf: -2.5 gives -2.
func roundHalfUp(x float64) float64 {
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	return r
}

// sum adds a vector's or list's elements; an empty one sums to 0.
func sum(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var v starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &v); err != nil {
		return nil, err
	}
	xs, err := values(v)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.Name(), err)
	}
	var total starlark.Value = starlark.MakeInt(0) // 0 + kg(x) is kg(x)
	for _, x := range xs {
		if total, err = starlark.Binary(syntax.PLUS, total, x); err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name(), err)
		}
	}
	return total, nil
}

// zeroOrGte reports whether every element of a is 0 or at least the
// matching element of b, or b itself when b is a single value.
func zeroOrGte(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var av, bv starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 2, &av, &bv); err != nil {
		return nil, err
	}
	as, err := values(av)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.Name(), err)
	}
	bs := slices.Repeat([]starlark.Value{bv}, len(as))
	if _, ok := bv.(starlark.Iterable); ok {
		if bs, err = values(bv); err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name(), err)
		}
	}
	if len(bs) != len(as) {
		return nil, fmt.Errorf("%s: lengths differ: %d and %d", b.Name(), len(as), len(bs))
	}
	for i, x := range as {
		if !x.Truth() {
			continue
		}
		ge, err := starlark.Compare(syntax.GE, x, bs[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name(), err)
		}
		if !ge {
			return starlark.False, nil
		}
	}
	return starlark.True, nil
}

// values lists the elements of a vector or list, at most maxListLen.
func values(v starlark.Value) ([]starlark.Value, error) {
	return listOf(v, func(x starlark.Value) (starlark.Value, error) { return x, nil })
}

var mathDocs = docGroup("Math functions",
	BuiltinDoc{Name: "floor", Signature: "floor(x)",
		Doc:     "Rounds a number down to an int, or a weight down to a whole number in its unit.",
		Example: "# fragment\ndef half(ctx):\n    ctx.reps = floor(ctx.completed_reps[1] / 2)\n\nex(\"Squat\", sets=[s(3, 5, kg(100))], progress=custom(half))"},
	BuiltinDoc{Name: "ceil", Signature: "ceil(x)",
		Doc:     "Rounds a number up to an int, or a weight up to a whole number in its unit.",
		Example: "# fragment\nex(\"Squat\", sets=[s(ceil(10 / 3), 5, ceil(kg(101.2)))])"},
	BuiltinDoc{Name: "round", Signature: "round(x)",
		Doc:     "Rounds a number to an int, or a weight to a whole number in its unit, halves towards +Inf: round(2.5) is 3 and round(-2.5) is -2.",
		Example: "# fragment\nex(\"Squat\", sets=[s(3, 5, round(kg(101.5)))])"},
	BuiltinDoc{Name: "sum", Signature: "sum(xs)",
		Doc:     "Adds the elements of a vector or list; an empty one sums to 0. sum(ctx.completed_reps) is the reps done.",
		Example: "# fragment\ndef total(ctx):\n    if sum(ctx.completed_reps) >= 40:\n        ctx.weights += kg(2.5)\n\nex(\"Pull Up\", sets=[s(4, 10, bw(), amrap=True)], progress=custom(total))"},
	BuiltinDoc{Name: "zero_or_gte", Signature: "zero_or_gte(a, b)",
		Doc: "True when every element of a is 0 or at least the matching element of b, or b itself when b is one value. " +
			"Sets not done read 0, so zero_or_gte(ctx.completed_reps, ctx.reps) passes when every set done hit its reps.",
		Example: "# fragment\ndef lenient(ctx):\n    if zero_or_gte(ctx.completed_reps, ctx.reps):\n        ctx.weights += kg(2.5)\n\nex(\"Bench Press\", sets=[s(3, 5, kg(80))], progress=custom(lenient))"},
)
