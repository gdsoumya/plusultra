package engine

import (
	"fmt"
	"math"
	"strconv"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/gdsoumya/plusultra/internal/model"
)

func builtins() starlark.StringDict {
	return starlark.StringDict{
		"kg":  weightBuiltin(model.UnitKg),
		"lb":  weightBuiltin(model.UnitLb),
		"pct": starlark.NewBuiltin("pct", pct),
		"bw":  starlark.NewBuiltin("bw", bw),
	}
}

// number unpacks a Starlark int or float.
type number float64

func (n *number) Unpack(v starlark.Value) error {
	f, ok := starlark.AsFloat(v)
	if !ok {
		return fmt.Errorf("got %s, want number", v.Type())
	}
	*n = number(f)
	return nil
}

func weightBuiltin(unit model.Unit) *starlark.Builtin {
	return starlark.NewBuiltin(string(unit), func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var x number
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x); err != nil {
			return nil, err
		}
		if math.Abs(float64(x)) > maxWeight(unit) {
			return nil, fmt.Errorf("%s: %w", b.Name(), errTooLarge)
		}
		return &weightValue{quantity{float64(x), unit}}, nil
	})
}

func pct(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x number
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "x", &x); err != nil {
		return nil, err
	}
	return &pctValue{quantity{v: float64(x)}}, nil
}

func bw(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var delta number
	unit := string(model.UnitKg)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "delta?", &delta, "unit?", &unit); err != nil {
		return nil, err
	}
	if !model.Unit(unit).Valid() {
		return nil, fmt.Errorf("%s: unit must be kg or lb, got %q", b.Name(), unit)
	}
	return &bwValue{quantity{float64(delta), model.Unit(unit)}}, nil
}

// quantity is the immutable core of weight, pct and bw values. unit is empty for pct.
type quantity struct {
	v    float64
	unit model.Unit
}

type quantityValue interface {
	starlark.Value
	qty() quantity
}

func (q quantity) qty() quantity        { return q }
func (q quantity) Freeze()              {}
func (q quantity) Truth() starlark.Bool { return q.v != 0 }
func (q quantity) Hash() (uint32, error) {
	return starlark.Tuple{starlark.Float(q.v), starlark.String(q.unit)}.Hash()
}
func (q quantity) num() string { return strconv.FormatFloat(q.v, 'g', -1, 64) }

// CompareSameType is only called with y of the same type as q's owner.
func (q quantity) CompareSameType(op syntax.Token, y starlark.Value, _ int) (bool, error) {
	o := y.(quantityValue).qty()
	if q.unit != o.unit {
		switch op {
		case syntax.EQL:
			return false, nil
		case syntax.NEQ:
			return true, nil
		}
		return false, fmt.Errorf("cannot compare %s and %s", q.unit, o.unit)
	}
	return starlark.Compare(op, starlark.Float(q.v), starlark.Float(o.v))
}

// binary implements + - * / for x. A number operand is taken in x's unit; a
// same-kind operand must share x's unit, and dividing by one gives a ratio.
func binary(x quantityValue, op syntax.Token, y starlark.Value, side starlark.Side, mk func(quantity) starlark.Value) (starlark.Value, error) {
	q := x.qty()
	var other float64
	sameKind := false
	if f, ok := starlark.AsFloat(y); ok {
		other = f
	} else if o, ok := y.(quantityValue); ok && o.Type() == x.Type() {
		if o.qty().unit != q.unit {
			return nil, fmt.Errorf("cannot mix %s and %s", q.unit, o.qty().unit)
		}
		other, sameKind = o.qty().v, true
	} else {
		return nil, nil
	}
	l, r := q.v, other
	if side == starlark.Right {
		l, r = r, l
	}
	switch op {
	case syntax.PLUS:
		return mk(quantity{l + r, q.unit}), nil
	case syntax.MINUS:
		return mk(quantity{l - r, q.unit}), nil
	case syntax.STAR:
		if !sameKind {
			return mk(quantity{l * r, q.unit}), nil
		}
	case syntax.SLASH:
		if side == starlark.Right {
			return nil, nil // number / quantity has no meaning
		}
		if r == 0 {
			return nil, fmt.Errorf("%s division by zero", x.Type())
		}
		if sameKind {
			return starlark.Float(l / r), nil
		}
		return mk(quantity{l / r, q.unit}), nil
	}
	return nil, nil
}

// unary implements -x and +x.
func unary(x quantityValue, op syntax.Token, mk func(quantity) starlark.Value) (starlark.Value, error) {
	q := x.qty()
	switch op {
	case syntax.MINUS:
		return mk(quantity{-q.v, q.unit}), nil
	case syntax.PLUS:
		return x, nil
	}
	return nil, nil
}

type weightValue struct{ quantity }

func (w *weightValue) Type() string   { return "weight" }
func (w *weightValue) String() string { return fmt.Sprintf("%s(%s)", w.unit, w.num()) }
func (w *weightValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	return binary(w, op, y, side, func(q quantity) starlark.Value { return &weightValue{q} })
}
func (w *weightValue) Unary(op syntax.Token) (starlark.Value, error) {
	return unary(w, op, func(q quantity) starlark.Value { return &weightValue{q} })
}

func weightOf(w model.Weight) *weightValue { return &weightValue{quantity{w.Value, w.Unit}} }

// Weight returns the value as a model weight.
func (w *weightValue) Weight() model.Weight { return model.Weight{Value: w.v, Unit: w.unit} }

// Load returns the value as an absolute load.
func (w *weightValue) Load() model.Load {
	return model.Load{Kind: model.LoadAbsolute, Weight: w.Weight()}
}

type pctValue struct{ quantity }

func (p *pctValue) Type() string   { return "pct" }
func (p *pctValue) String() string { return fmt.Sprintf("pct(%s)", p.num()) }
func (p *pctValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	return binary(p, op, y, side, func(q quantity) starlark.Value { return &pctValue{q} })
}
func (p *pctValue) Unary(op syntax.Token) (starlark.Value, error) {
	return unary(p, op, func(q quantity) starlark.Value { return &pctValue{q} })
}

// Load returns the value as a percent-of-1RM load.
func (p *pctValue) Load() model.Load { return model.Load{Kind: model.LoadPercent, Pct: p.v} }

type bwValue struct{ quantity }

func (b *bwValue) Type() string   { return "bw" }
func (b *bwValue) String() string { return fmt.Sprintf("bw(%s, unit=%q)", b.num(), b.unit) }
func (b *bwValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	return binary(b, op, y, side, func(q quantity) starlark.Value { return &bwValue{q} })
}
func (b *bwValue) Unary(op syntax.Token) (starlark.Value, error) {
	return unary(b, op, func(q quantity) starlark.Value { return &bwValue{q} })
}

// Load returns the value as a bodyweight load with its delta.
func (b *bwValue) Load() model.Load {
	return model.Load{Kind: model.LoadBodyweight, Delta: model.Weight{Value: b.v, Unit: b.unit}}
}

var valueDocs = docGroup("Weights",
	BuiltinDoc{Name: "kg", Signature: "kg(x)",
		Doc: "An absolute weight in kilograms, at most 2000. Weights add and subtract with numbers and weights of the same unit, " +
			"multiply and divide by numbers (kg(100) * 0.9), divide by a weight to give a ratio, and compare with weights.",
		Example: "# fragment\nex(\"Squat\", sets=[s(5, 5, kg(100) * 0.9)])"},
	BuiltinDoc{Name: "lb", Signature: "lb(x)",
		Doc:     "An absolute weight in pounds, at most 4400. Mixing kg and lb in one sum is an error; resolution converts to the user's units.",
		Example: "# fragment\nex(\"Bench Press\", sets=[s(3, 8, lb(135))])"},
	BuiltinDoc{Name: "pct", Signature: "pct(x)",
		Doc: "x percent of the active exercise's 1RM; resolution fails when the user has no 1RM for it. " +
			"In w() it is a percent of the first working set, and lp(pct(5)) is a 5% increment.",
		Example: "# fragment\nex(\"Squat\", sets=[s(3, 5, pct(75))])"},
	BuiltinDoc{Name: "bw", Signature: `bw(delta=0, unit="kg")`,
		Doc: "The user's bodyweight plus delta: bw() for bodyweight, bw(10) with 10 kg on a belt, bw(-25) on an assisted machine. " +
			"Bodyweight loads are not rounded and get no default warmup. Use kg(0) to log a bodyweight exercise without the body's weight.",
		Example: "# fragment\nex(\"Pull Up\", sets=[s(3, 8, bw(10))])\nex(\"Pull Up, Leverage Machine\", sets=[s(3, 8, bw(-25))])"},
)
