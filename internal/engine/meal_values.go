package engine

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// maxMealValue caps quantities and macro amounts so item and day totals stay finite.
const maxMealValue = 100_000

// foodValue is a food(); calling it, food(qty, ...), is item(food, qty, ...).
type foodValue struct {
	opaque
	f       model.Food
	per100g bool // per100g was given, even if all zero
}

func (*foodValue) Type() string     { return "food" }
func (v *foodValue) String() string { return fmt.Sprintf("food(%q)", v.f.Name) }
func (v *foodValue) Name() string   { return v.f.Name }
func (v *foodValue) CallInternal(t *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.Call(t, itemBuiltin, append(starlark.Tuple{v}, args...), kwargs)
}

// refOnly reports whether the food's macros come from catalog_snapshot or the catalog.
func (v *foodValue) refOnly() bool { return !v.per100g && v.f.PerUnit == nil }

type itemValue struct {
	opaque
	it      model.Item
	refOnly bool
}

func (*itemValue) Type() string { return "item" }
func (v *itemValue) String() string {
	return fmt.Sprintf("item(%q, %g %s)", v.it.Food.Name, v.it.Qty, v.it.Unit)
}

type targetValue struct {
	opaque
	m model.Macros
}

func (*targetValue) Type() string     { return "target" }
func (v *targetValue) String() string { return fmt.Sprintf("target(%+v)", v.m) }

var macroFields = map[string]func(*model.Macros) *float64{
	"kcal":     func(m *model.Macros) *float64 { return &m.Kcal },
	"protein":  func(m *model.Macros) *float64 { return &m.Protein },
	"carbs":    func(m *model.Macros) *float64 { return &m.Carbs },
	"fat":      func(m *model.Macros) *float64 { return &m.Fat },
	"fiber":    func(m *model.Macros) *float64 { return &m.Fiber },
	"water_ml": func(m *model.Macros) *float64 { return &m.WaterML },
}

// setMacro sets the field named key from v. None leaves it unset.
func setMacro(m *model.Macros, key string, v starlark.Value) error {
	field, ok := macroFields[key]
	if !ok {
		return fmt.Errorf("unknown macro %q; want kcal, protein, carbs, fat, fiber or water_ml", key)
	}
	if v == starlark.None {
		return nil
	}
	f, ok := starlark.AsFloat(v)
	if !ok {
		return fmt.Errorf("%s: got %s, want number", key, v.Type())
	}
	if err := checkMealValue(key, f); err != nil {
		return err
	}
	*field(m) = f
	return nil
}

func checkMealValue(name string, x float64) error {
	switch {
	case !validWeight(x):
		return fmt.Errorf("%s must be a finite non-negative number", name)
	case x > maxMealValue:
		return fmt.Errorf("%s value too large: the limit is %d", name, maxMealValue)
	}
	return nil
}

// macrosArg unpacks a dict of macros such as {"kcal": 120, "protein": 23}.
type macrosArg struct {
	m   model.Macros
	set bool
}

func (a *macrosArg) Unpack(v starlark.Value) error {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return fmt.Errorf("got %s, want dict of macros", v.Type())
	}
	for _, kv := range d.Items() {
		k, ok := starlark.AsString(kv[0])
		if !ok {
			return fmt.Errorf("macro keys must be strings, got %s", kv[0].Type())
		}
		if err := setMacro(&a.m, k, kv[1]); err != nil {
			return err
		}
	}
	a.set = true
	return nil
}
