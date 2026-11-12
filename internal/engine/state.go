package engine

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// maxStateDepth bounds nesting in state values; a list can contain itself.
const maxStateDepth = 32

// stateValue is ctx.state or ctx.shared(tag): named values a script reads and
// sets. Unless open, only existing names can be set.
type stateValue struct {
	opaque
	vars     map[string]starlark.Value
	open     bool
	readonly bool
}

func (*stateValue) Type() string                               { return "state" }
func (s *stateValue) String() string                           { return "state(" + strings.Join(s.AttrNames(), ", ") + ")" }
func (s *stateValue) AttrNames() []string                      { return slices.Sorted(maps.Keys(s.vars)) }
func (s *stateValue) Attr(name string) (starlark.Value, error) { return s.vars[name], nil }

func (s *stateValue) SetField(name string, v starlark.Value) error {
	if s.readonly {
		return errors.New("state is only changeable in progress")
	}
	if _, ok := s.vars[name]; !ok && !s.open {
		return starlark.NoSuchAttrError(fmt.Sprintf("state has no %s; declare it in custom() or state=", name))
	}
	if _, err := copyState(v, 0); err != nil {
		return fmt.Errorf("state %s: %w", name, err)
	}
	s.vars[name] = v
	return nil
}

func (c *ctxValue) sharedState(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var tag int
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &tag); err != nil {
		return nil, err
	}
	if s, ok := c.shared[tag]; ok {
		return s, nil
	}
	vars, err := loadState(nil, c.st.Overlay.State[tagKey(tag)], nil)
	if err != nil {
		return nil, err
	}
	c.shared[tag] = &stateValue{vars: vars, open: true, readonly: c.update}
	return c.shared[tag], nil
}

// tagKey is where the overlay keeps a tag's shared state.
func tagKey(tag int) model.ProgKey { return model.ProgKey{Exercise: fmt.Sprintf("tag:%d", tag)} }

// SharedKey reports whether k keys a tag's shared state rather than an exercise.
func SharedKey(k model.ProgKey) bool { return strings.HasPrefix(k.Exercise, "tag:") && k.Label == "" }

// loadState is the declared state overridden by saved values and answered
// prompts, as mutable copies: the program's and overlay's values are frozen.
func loadState(decl map[string]model.StateVar, saved, prompts map[string]starlark.Value) (map[string]starlark.Value, error) {
	vars := make(map[string]starlark.Value, len(decl)+len(saved))
	for k, sv := range decl {
		vars[k] = sv.Value
	}
	maps.Copy(vars, saved)
	for k, sv := range decl {
		if v, ok := prompts[k]; ok && sv.Prompt {
			vars[k] = v
		}
	}
	for k, v := range vars {
		c, err := copyState(v, 0)
		if err != nil {
			return nil, fmt.Errorf("state %s: %w", k, err)
		}
		vars[k] = c
	}
	return vars, nil
}

// commitState records vars as key's state in the delta, frozen.
func (c *ctxValue) commitState(key model.ProgKey, vars map[string]starlark.Value) error {
	if len(vars) == 0 {
		return nil
	}
	out := make(map[string]starlark.Value, len(vars))
	for k, v := range vars {
		f, err := copyState(v, 0)
		if err != nil {
			return fmt.Errorf("state %s: %w", k, err)
		}
		f.Freeze()
		out[k] = f
	}
	c.delta.State[key] = out
	return nil
}

// copyState deep-copies a state value, rejecting what state cannot persist.
func copyState(v starlark.Value, depth int) (starlark.Value, error) {
	if depth > maxStateDepth {
		return nil, fmt.Errorf("nested deeper than %d", maxStateDepth)
	}
	each := func(xs []starlark.Value) ([]starlark.Value, error) {
		out := make([]starlark.Value, len(xs))
		for i, x := range xs {
			c, err := copyState(x, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	}
	switch v := v.(type) {
	case starlark.NoneType, starlark.Bool, starlark.Int, starlark.Float, starlark.String, *weightValue, *pctValue, *bwValue:
		return v, nil
	case *starlark.List:
		xs, err := each(slices.Collect(starlark.Elements(v)))
		return starlark.NewList(xs), err
	case starlark.Tuple:
		xs, err := each(v)
		return starlark.Tuple(xs), err
	case *starlark.Dict:
		d := starlark.NewDict(v.Len())
		for _, kv := range v.Items() {
			kv, err := each(kv[:])
			if err != nil {
				return nil, err
			}
			if err := d.SetKey(kv[0], kv[1]); err != nil {
				return nil, err
			}
		}
		return d, nil
	}
	return nil, fmt.Errorf("cannot hold %s", v.Type())
}
