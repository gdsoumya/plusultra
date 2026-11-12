package engine

import (
	"cmp"
	"fmt"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// setVector writes field name as a whole at scope, the address of ctx.<name>
// or of a view. A live vector of that field and address combined with
// numbers or weights by + - * / is relative: every address scope covers
// changes the same way, keeping its own script value. Anything else, or a
// relative vector whose base has changed since it was read, is absolute: one
// value or equal values set all sets, other sequences set each set.
func (c *ctxValue) setVector(name string, scope model.SetAddr, v starlark.Value) error {
	cur, err := c.current(name, scope)
	if err != nil {
		return err
	}
	if x, ok := v.(*vec); ok && x.rel != nil {
		src := x.rel.src
		if e, ok := x.rel.asEdit(setFields[name], c.units); ok && src.ctx == c && src.name == name && src.scope == scope &&
			allEqualTo(src.elems, cur) {
			return c.edit(name, scope, e)
		}
	}
	if _, ok := v.(starlark.Sequence); !ok {
		return c.setSet(name, scope, 0, v)
	}
	elems, err := values(v)
	switch {
	case err != nil:
		return fmt.Errorf("ctx.%s: %w", name, err)
	case len(elems) != len(cur):
		return fmt.Errorf("ctx.%s: got %d values for %d sets", name, len(elems), len(cur))
	case len(elems) > 0 && allEqualTo(elems[1:], elems[:len(elems)-1]):
		return c.setSet(name, scope, 0, elems[0])
	}
	for i, x := range elems {
		if err := c.setSet(name, scope, i+1, x); err != nil {
			return err
		}
	}
	return nil
}

// allEqualTo reports whether xs and ys are equal element by element.
func allEqualTo(xs, ys []starlark.Value) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if eq, err := starlark.Equal(xs[i], ys[i]); err != nil || !eq {
			return false
		}
	}
	return true
}

// setSet sets set i at scope to v, or every set there when i is 0.
func (c *ctxValue) setSet(name string, scope model.SetAddr, i int, v starlark.Value) error {
	cur, err := c.current(name, scope)
	if err != nil {
		return err
	}
	if i > len(cur) {
		return fmt.Errorf("ctx.%s: set %d out of range 1-%d", name, i, len(cur))
	}
	e, err := setFields[name].parse(v)
	if err != nil {
		return fmt.Errorf("ctx.%s: %w", name, err)
	}
	scope.Set = i
	return c.edit(name, scope, e)
}

// current is field name at scope as it reads now: the workout's own sets for
// ctx.<name>, else the sets resolved there with this run's writes.
func (c *ctxValue) current(name string, scope model.SetAddr) ([]starlark.Value, error) {
	if scope == (model.SetAddr{Key: c.key}) {
		return c.vector(name, c.sets).elems, nil
	}
	sets, err := c.resolveAt(scope)
	if err != nil {
		return nil, err
	}
	return c.vector(name, sets).elems, nil
}

// edit applies e to field name at a. A progression records it in the
// overlay. The workout's own sets that a covers change too, except the
// completed ones in an update.
func (c *ctxValue) edit(name string, a model.SetAddr, e edit) error {
	f := setFields[name]
	if !c.update {
		if err := f.persist(&c.delta, &c.st.Overlay, a, e); err != nil {
			return err
		}
	}
	for j := c.setIndex; j < len(c.sets); j++ {
		if coversSet(a, model.SetAddr{Key: c.key, Week: c.res.Week, Day: c.res.Day, Variation: c.variation, Set: j + 1}) {
			f.apply(&c.sets[j], e)
		}
	}
	return nil
}

// resolveAt is the exercise's working sets at a's week, day and variation
// (the workout's own where zero), with this run's writes applied.
func (c *ctxValue) resolveAt(a model.SetAddr) ([]model.ResolvedSet, error) {
	week, day := cmp.Or(a.Week, c.res.Week), cmp.Or(a.Day, c.res.Day)
	e, err := findExercise(c.p, model.ExerciseResult{Key: c.key, Week: week, Day: day})
	if err != nil {
		return nil, err
	}
	st := *c.st
	st.Overlay = model.Overlay{}
	st.Overlay.Merge(c.st.Overlay)
	st.Overlay.Merge(c.delta)
	if a.Variation > 0 {
		st.Overlay.SetVariation = map[model.ProgKey]int{c.key: a.Variation}
	}
	ex, _, err := resolveExercise(e, week, day, &st)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(ex.Sets, func(s model.ResolvedSet) bool { return s.Warmup }), nil
}

func (c *ctxValue) SetField(name string, v starlark.Value) error {
	if _, ok := setFields[name]; ok {
		return c.setVector(name, model.SetAddr{Key: c.key}, v)
	}
	switch name {
	case "number_of_sets":
		return c.setNumSets(model.DayAddr{Key: c.key}, v)
	case "rm1", "set_variation", "exercise_variation", "description_index":
		if c.update {
			return fmt.Errorf("ctx.%s is only changeable in progress", name)
		}
		if name == "rm1" {
			w, err := toWeight(v)
			if err != nil {
				return fmt.Errorf("ctx.%s: %w", name, err)
			}
			c.rm1, c.delta.RM1[c.planned.Name] = w, w
			return nil
		}
		return c.setIndexVar(name, v)
	}
	if slices.Contains(c.AttrNames(), name) {
		return fmt.Errorf("ctx.%s is read-only", name)
	}
	return starlark.NoSuchAttrError(fmt.Sprintf("ctx has no .%s field", name))
}

// setIndexVar stores a 1-based index, wrapped to the exercise's count.
func (c *ctxValue) setIndexVar(name string, v starlark.Value) error {
	n, err := starlark.AsInt32(v)
	if err != nil {
		return fmt.Errorf("ctx.%s: %w", name, err)
	}
	count, dst := len(c.e.Variations), c.delta.SetVariation
	switch name {
	case "exercise_variation":
		count, dst = len(c.e.Names), c.delta.ExerciseVariation
	case "description_index":
		count, dst = len(c.e.Desc), c.delta.DescIndex
	}
	c.index[name] = nth(n, max(count, 1)) + 1
	dst[c.key] = c.index[name]
	return nil
}
