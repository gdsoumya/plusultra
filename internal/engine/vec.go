package engine

import (
	"cmp"
	"errors"
	"fmt"
	"math"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/gdsoumya/plusultra/internal/model"
)

// vec is a 1-indexed vector of per-set values. Arithmetic applies per element,
// broadcasting a scalar, and never writes. A comparison holds when it holds
// for every element.
//
// A live vector is a field read from ctx or a ctx.at(...) view. Index
// assignment writes through to its address: ctx.weights[2] = kg(60) sets set
// 2 there to the value given, so indexed augmented assignment is absolute to
// the current value. Assigning the field as a whole goes through setVector.
// Other vectors are read-only.
//
// Indexing goes through Mapping and HasSetKey because Starlark bounds-checks
// Indexable as 0-based. A vector compares only with a vector: Starlark does
// not let a type define comparisons with other types.
type vec struct {
	opaque
	elems []starlark.Value
	ctx   *ctxValue // nil: read-only
	name  string
	scope model.SetAddr // a live vector's address
	rel   *affine       // set when this is a live vector * scale + delta
}

// affine says a vector is src*scale + delta, delta in unit for weights, so
// assigning it back to src's field can write a relative override.
type affine struct {
	src          *vec
	scale, delta float64
	unit         model.Unit
}

var (
	_ starlark.Mapping    = (*vec)(nil)
	_ starlark.HasSetKey  = (*vec)(nil)
	_ starlark.Sequence   = (*vec)(nil)
	_ starlark.HasBinary  = (*vec)(nil)
	_ starlark.HasUnary   = (*vec)(nil)
	_ starlark.Comparable = (*vec)(nil)
)

func (*vec) Type() string                 { return "vector" }
func (v *vec) String() string             { return starlark.NewList(v.elems).String() }
func (v *vec) Len() int                   { return len(v.elems) }
func (v *vec) Iterate() starlark.Iterator { return starlark.Tuple(v.elems).Iterate() }

func (v *vec) index(k starlark.Value) (int, error) {
	i, err := starlark.AsInt32(k)
	if err != nil {
		return 0, fmt.Errorf("vector index: %w", err)
	}
	if i < 1 || i > len(v.elems) {
		return 0, fmt.Errorf("vector index %d out of range 1-%d", i, len(v.elems))
	}
	return i, nil
}

func (v *vec) Get(k starlark.Value) (starlark.Value, bool, error) {
	i, err := v.index(k)
	if err != nil {
		return nil, false, err
	}
	return v.elems[i-1], true, nil
}

func (v *vec) SetKey(k, x starlark.Value) error {
	i, err := v.index(k)
	switch {
	case err != nil:
		return err
	case v.ctx == nil:
		return errors.New("vector is read-only")
	}
	if err := v.ctx.setSet(v.name, v.scope, i, x); err != nil {
		return err
	}
	v.elems, err = v.ctx.current(v.name, v.scope)
	return err
}

func (v *vec) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	yv, isVec := y.(*vec)
	if isVec && len(yv.elems) != len(v.elems) {
		return nil, fmt.Errorf("vector lengths differ: %d and %d", len(v.elems), len(yv.elems))
	}
	out := make([]starlark.Value, len(v.elems))
	for i, x := range v.elems {
		o := y
		if isVec {
			o = yv.elems[i]
		}
		l, r := x, o
		if side == starlark.Right {
			l, r = o, x
		}
		z, err := starlark.Binary(op, l, r)
		if err != nil {
			return nil, err
		}
		out[i] = z
	}
	step, affineOp := affineStep(op, y, side)
	switch {
	case v.ctx != nil && affineOp:
		step.src = v
		return &vec{elems: out, rel: &step}, nil
	case v.rel != nil && affineOp:
		r := affine{src: v.rel.src, scale: v.rel.scale * step.scale, delta: v.rel.delta*step.scale + step.delta,
			unit: cmp.Or(v.rel.unit, step.unit)}
		return &vec{elems: out, rel: &r}, nil
	}
	return &vec{elems: out}, nil
}

// affineStep is x op y as x*scale + delta, when it is one.
func affineStep(op syntax.Token, y starlark.Value, side starlark.Side) (affine, bool) {
	var d float64
	var unit model.Unit
	if w, ok := y.(*weightValue); ok {
		d, unit = w.v, w.unit
	} else if f, ok := starlark.AsFloat(y); ok {
		d = f
	} else {
		return affine{}, false
	}
	if math.IsNaN(d) || math.IsInf(d, 0) {
		return affine{}, false
	}
	switch {
	case op == syntax.PLUS:
		return affine{scale: 1, delta: d, unit: unit}, true
	case op == syntax.MINUS && side == starlark.Left:
		return affine{scale: 1, delta: -d, unit: unit}, true
	case op == syntax.STAR && unit == "":
		return affine{scale: d}, true
	case op == syntax.SLASH && side == starlark.Left && unit == "" && d != 0:
		return affine{scale: 1 / d}, true
	}
	return affine{}, false
}

// asEdit is a as a relative edit of field f, if f can hold it: only weights
// scale, and an int field takes whole deltas. A bare number added to weights
// is in unit u.
func (a affine) asEdit(f setField, u model.Unit) (edit, bool) {
	if f.kind == kindWeight {
		return edit{v: a.delta, unit: cmp.Or(a.unit, u), scale: a.scale}, true
	}
	whole := f.kind != kindInt || a.delta == math.Trunc(a.delta)
	if a.scale != 1 || a.unit != "" || f.kind == kindBool || !whole {
		return edit{}, false
	}
	return edit{v: a.delta, scale: 1}, true
}

func (v *vec) Unary(op syntax.Token) (starlark.Value, error) {
	out := make([]starlark.Value, len(v.elems))
	for i, x := range v.elems {
		z, err := starlark.Unary(op, x)
		if err != nil {
			return nil, err
		}
		out[i] = z
	}
	return &vec{elems: out}, nil
}

// CompareSameType holds when op holds for every pair of elements, so !=
// means every element differs. Vectors of different
// lengths are unequal and have no order.
func (v *vec) CompareSameType(op syntax.Token, y starlark.Value, depth int) (bool, error) {
	yv := y.(*vec)
	if len(yv.elems) != len(v.elems) {
		if op == syntax.EQL || op == syntax.NEQ {
			return op == syntax.NEQ, nil
		}
		return false, fmt.Errorf("vector lengths differ: %d and %d", len(v.elems), len(yv.elems))
	}
	for i := range v.elems {
		if ok, err := starlark.CompareDepth(op, v.elems[i], yv.elems[i], depth-1); err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}
