package engine

import (
	"fmt"
	"maps"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// view is ctx.at(week=, day=, variation=), a progression's handle on the
// exercise's sets at that address. Its fields read the sets resolved there,
// taking the workout's own week, day or variation where one is zero, and
// write to the address, which covers all where a field is zero:
// ctx.at(week=2).weights += kg(5), ctx.at(week=2).weights[1] = kg(60).
type view struct {
	opaque
	c *ctxValue
	a model.SetAddr
}

func (*view) Type() string { return "view" }
func (v *view) String() string {
	return fmt.Sprintf("view(week=%d, day=%d, variation=%d)", v.a.Week, v.a.Day, v.a.Variation)
}
func (v *view) AttrNames() []string {
	return slices.Sorted(slices.Values(append(slices.Collect(maps.Keys(setFields)), "add_sets", "number_of_sets")))
}

// count is the view's set count address.
func (v *view) count() model.DayAddr {
	return model.DayAddr{Key: v.a.Key, Week: v.a.Week, Day: v.a.Day, Variation: v.a.Variation}
}

func (v *view) Attr(name string) (starlark.Value, error) {
	if _, ok := setFields[name]; ok {
		elems, err := v.c.current(name, v.a)
		if err != nil {
			return nil, err
		}
		return &vec{elems: elems, ctx: v.c, name: name, scope: v.a}, nil
	}
	switch name {
	case "number_of_sets":
		elems, err := v.c.current("reps", v.a)
		return starlark.MakeInt(len(elems)), err
	case "add_sets":
		return v.c.addSets(v.count()), nil
	}
	return nil, nil
}

func (v *view) SetField(name string, x starlark.Value) error {
	if _, ok := setFields[name]; ok {
		return v.c.setVector(name, v.a, x)
	}
	if name == "number_of_sets" {
		return v.c.setNumSets(v.count(), x)
	}
	return starlark.NoSuchAttrError(fmt.Sprintf("view has no .%s field", name))
}

func (c *ctxValue) at(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	a := model.SetAddr{Key: c.key}
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "week?", &a.Week, "day?", &a.Day, "variation?", &a.Variation); err != nil {
		return nil, err
	}
	if c.update {
		return nil, fmt.Errorf("%s: only changeable in progress; an update changes this workout", b.Name())
	}
	for _, f := range []int{a.Week, a.Day, a.Variation} {
		if f < 0 || f > maxListLen {
			return nil, fmt.Errorf("%s: week, day and variation must be 0-%d", b.Name(), maxListLen)
		}
	}
	if _, err := c.current("weights", a); err != nil { // the exercise must be there
		return nil, err
	}
	return &view{c: c, a: a}, nil
}

// addSets is add_sets(n) for the set count at a: a relative change, so each
// week and day keeps its own count plus n.
func (c *ctxValue) addSets(a model.DayAddr) *starlark.Builtin {
	return starlark.NewBuiltin("add_sets", func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var n int
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &n); err != nil {
			return nil, err
		}
		if n < -maxListLen || n > maxListLen {
			return nil, fmt.Errorf("%s: n must be within ±%d", b.Name(), maxListLen)
		}
		return starlark.None, c.setCount(a, edit{v: float64(n), scale: 1})
	})
}

// setNumSets is number_of_sets = x at a, an absolute set count.
func (c *ctxValue) setNumSets(a model.DayAddr, x starlark.Value) error {
	n, err := toCount(x, maxListLen)
	if err != nil {
		return fmt.Errorf("number_of_sets: %w", err)
	}
	return c.setCount(a, edit{abs: true, v: float64(n)})
}

// setCount applies e to the set count at a: in the overlay for a
// progression, and to this workout when a covers it, keeping completed sets.
func (c *ctxValue) setCount(a model.DayAddr, e edit) error {
	if !c.update {
		if err := assign(c.delta.NumSets, c.st.Overlay.NumSets, a, dayAddr, joinDay,
			func(o model.IntOverride) model.IntOverride { return composeNum(o, e, maxListLen) }); err != nil {
			return err
		}
	}
	if coversSet(dayAddr(a), model.SetAddr{Key: c.key, Week: c.res.Week, Day: c.res.Day, Variation: c.variation}) {
		n := applyNum(composeNum(model.IntOverride{}, e, maxListLen), len(c.sets), maxListLen)
		c.sets = resize(c.sets, max(n, c.setIndex))
	}
	return nil
}
