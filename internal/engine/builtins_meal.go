package engine

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

const gram = "g"

// EvaluateMeal runs a meal program script and returns its structure. Errors are *EvalError.
func EvaluateMeal(src, filename string) (*model.MealProgram, error) {
	return EvaluateMealWith(src, filename, nil)
}

// EvaluateMealWith is EvaluateMeal with catalog foods, keyed by ref or name,
// for the foods the script gives by ref or name alone. The script's
// catalog_snapshot wins over foods. A food neither has keeps zero macros and
// is Unresolved.
func EvaluateMealWith(src, filename string, foods map[string]model.Food) (*model.MealProgram, error) {
	b, err := evalMeal(src, filename, foods)
	if err != nil {
		return nil, err
	}
	return b.prog, nil
}

// EvaluateSnapshot is EvaluateMeal that also returns the foods of the
// script's catalog_snapshot blocks, keyed as they are written.
func EvaluateSnapshot(src, filename string) (*model.MealProgram, map[string]model.Food, error) {
	b, err := evalMeal(src, filename, nil)
	if err != nil {
		return nil, nil, err
	}
	return b.prog, b.snapshot, nil
}

func evalMeal(src, filename string, foods map[string]model.Food) (*mealBuilder, error) {
	b := &mealBuilder{snapshot: map[string]model.Food{}}
	maps.Copy(b.snapshot, foods)
	if _, err := exec(src, filename, b.builtins()); err != nil {
		return nil, err
	}
	if b.prog == nil {
		return nil, &EvalError{File: filename, Msg: "meal_program() was never called"}
	}
	for _, a := range b.pending {
		if err := b.fillItem(&b.prog.Days[a.day].Meals[a.meal].Items[a.item]); err != nil {
			return nil, &EvalError{File: filename, Line: a.line, Msg: err.Error()}
		}
	}
	for i := range b.prog.Days {
		d := &b.prog.Days[i]
		ResolveMealTotals(d)
		if d.Target == nil {
			t := d.Totals()
			d.Target = &t
		}
	}
	return b, nil
}

// ResolveMealTotals sets each item's macros from its food and quantity.
func ResolveMealTotals(d *model.MealDay) {
	for i := range d.Meals {
		for j := range d.Meals[i].Items {
			it := &d.Meals[i].Items[j]
			it.Macros = it.Food.Per100g.Scale(it.Qty / 100)
			if it.Food.PerUnit != nil && it.Unit == it.Food.Unit {
				it.Macros = it.Food.PerUnit.Scale(it.Qty)
			}
		}
	}
}

// mealBuilder is the meal program under construction during one evaluation.
type mealBuilder struct {
	prog     *model.MealProgram
	open     *model.MealDay        // the day meal() attaches to
	snapshot map[string]model.Food // by ref or name
	pending  []itemAddr            // items of foods given by ref or name alone, filled from snapshot at the end
}

// fillItem gives the item's food the macros snapshot has for its ref, else
// its name, or marks it Unresolved. The item's unit, left open by item() for
// such a food, is settled here: grams unless the item named the filled
// food's unit. Grams stay the default so one counted item cannot change what
// the food's other items mean.
func (b *mealBuilder) fillItem(it *model.Item) error {
	f := &it.Food
	s, ok := b.lookup(f)
	if !ok {
		f.Unresolved = true
		return nil
	}
	f.Per100g, f.PerUnit, f.Unit, f.Ref = s.Per100g, s.PerUnit, s.Unit, cmp.Or(f.Ref, s.Ref)
	switch {
	case it.Unit == "":
		it.Unit = gram
	case it.Unit != gram && (f.PerUnit == nil || it.Unit != f.Unit):
		return fmt.Errorf("%s has no unit %q; give qty in %s", f.Name, it.Unit, cmp.Or(f.Unit, gram))
	}
	return nil
}

func (b *mealBuilder) lookup(f *model.Food) (model.Food, bool) {
	for _, k := range []string{f.Ref, f.Name} {
		if s, ok := b.snapshot[k]; ok && k != "" {
			return s, true
		}
	}
	return model.Food{}, false
}

// itemAddr locates an item in prog.Days; line is the meal() call's.
type itemAddr struct{ day, meal, item, line int }

func (b *mealBuilder) builtins() starlark.StringDict {
	fns := map[string]builtinFn{
		"meal_program": b.mealProgram, "day": b.day, "meal": b.meal, "food": b.food,
		"item": item, "target": target, "catalog_snapshot": b.catalogSnapshot,
	}
	d := make(starlark.StringDict, len(fns))
	for name, fn := range fns {
		d[name] = starlark.NewBuiltin(name, fn)
	}
	return d
}

func (b *mealBuilder) mealProgram(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if b.prog != nil {
		return nil, fmt.Errorf("%s: called more than once", fn.Name())
	}
	p := &model.MealProgram{Loop: true}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &p.Name, "weeks", &p.Weeks,
		"loop?", &p.Loop, "notes?", &p.Notes); err != nil {
		return nil, err
	}
	if p.Weeks < 1 || p.Weeks > maxWeeks {
		return nil, fmt.Errorf("%s: weeks must be 1-%d", fn.Name(), maxWeeks)
	}
	b.prog = p
	return starlark.None, nil
}

func (b *mealBuilder) day(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if b.prog == nil {
		return nil, fmt.Errorf("%s: call meal_program() first", fn.Name())
	}
	d := model.MealDay{Kind: "training"}
	var n starlark.Value // nil: a kind day
	var weeks intList
	var tgt *targetValue
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "week", &d.Week, "n??", &n, "name?", &d.Name,
		"desc?", &d.Desc, "kind?", &d.Kind, "target??", &tgt, "weeks??", &weeks); err != nil {
		return nil, err
	}
	if n != nil {
		num, err := starlark.AsInt32(n)
		if err != nil || num < 1 {
			return nil, fmt.Errorf("%s: n must be an int of at least 1; omit n for a kind day", fn.Name())
		}
		d.Num = num
	}
	d.Weeks = weeks
	if err := cmp.Or(b.checkWeeks(fn, []int{d.Week}), b.checkWeeks(fn, d.Weeks)); err != nil {
		return nil, err
	}
	if !slices.Contains(dayKinds, d.Kind) {
		return nil, fmt.Errorf("%s: kind must be one of %v, got %q", fn.Name(), dayKinds, d.Kind)
	}
	if tgt != nil {
		t := tgt.m
		d.Target = &t
	}
	b.prog.Days = append(b.prog.Days, d)
	b.open = &b.prog.Days[len(b.prog.Days)-1]
	return starlark.None, nil
}

// checkWeeks applies the workout builder's week checks against this program's length.
func (b *mealBuilder) checkWeeks(fn *starlark.Builtin, weeks []int) error {
	return (&builder{prog: &model.Program{Weeks: b.prog.Weeks}}).checkWeeks(fn, weeks)
}

func (b *mealBuilder) meal(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if b.open == nil {
		return nil, fmt.Errorf("%s: call day() before meal()", fn.Name())
	}
	n := min(len(args), 1) // name; the rest are items
	var m model.Meal
	if err := starlark.UnpackArgs(fn.Name(), args[:n], kwargs, "name", &m.Name, "time??", &m.Time, "desc?", &m.Desc); err != nil {
		return nil, err
	}
	items, err := listOf(args[n:], func(x starlark.Value) (*itemValue, error) {
		if it, ok := x.(*itemValue); ok {
			return it, nil
		}
		return nil, fmt.Errorf("got %s, want item", x.Type())
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	total := len(items)
	for _, o := range b.open.Meals {
		total += len(o.Items)
	}
	if total > maxListLen {
		return nil, fmt.Errorf("%s: more than %d items on this day", fn.Name(), maxListLen)
	}
	m.Items = make([]model.Item, len(items))
	for i, it := range items {
		m.Items[i] = it.it
		if it.refOnly {
			b.pending = append(b.pending, itemAddr{len(b.prog.Days) - 1, len(b.open.Meals), i, callerLine(t)})
		}
	}
	b.open.Meals = append(b.open.Meals, m)
	return starlark.None, nil
}

func (b *mealBuilder) food(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var f model.Food
	var per100g, perUnit macrosArg
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &f.Name, "per100g??", &per100g,
		"per_unit??", &perUnit, "unit??", &f.Unit, "ref??", &f.Ref); err != nil {
		return nil, err
	}
	switch {
	case perUnit.set != (f.Unit != ""):
		return nil, fmt.Errorf("%s: per_unit and unit go together", fn.Name())
	case perUnit.set && f.Unit == gram:
		return nil, fmt.Errorf("%s: per_unit needs a unit other than %q; use per100g for grams", fn.Name(), gram)
	}
	f.Per100g, f.Line = per100g.m, callerLine(t)
	if perUnit.set {
		f.PerUnit = &perUnit.m
	}
	return &foodValue{f: f, per100g: per100g.set}, nil
}

func (b *mealBuilder) catalogSnapshot(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var foods *starlark.Dict
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "foods", &foods); err != nil {
		return nil, err
	}
	for _, kv := range foods.Items() {
		ref, ok := starlark.AsString(kv[0])
		if !ok {
			return nil, fmt.Errorf("%s: refs must be strings, got %s", fn.Name(), kv[0].Type())
		}
		f, err := snapshotFood(kv[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", fn.Name(), ref, err)
		}
		b.snapshot[ref] = f
	}
	return starlark.None, nil
}

// snapshotFood reads a snapshot entry: a macro dict, or a dict with per100g
// and, for a counted food, per_unit and unit.
func snapshotFood(v starlark.Value) (model.Food, error) {
	var f model.Food
	d, ok := v.(*starlark.Dict)
	var per100g starlark.Value
	if ok {
		per100g, ok, _ = d.Get(starlark.String("per100g"))
	}
	var m macrosArg
	if !ok {
		err := m.Unpack(v)
		f.Per100g = m.m
		return f, err
	}
	if err := m.Unpack(per100g); err != nil {
		return f, fmt.Errorf("per100g: %w", err)
	}
	f.Per100g = m.m
	if pu, ok, _ := d.Get(starlark.String("per_unit")); ok {
		var m macrosArg
		if err := m.Unpack(pu); err != nil {
			return f, fmt.Errorf("per_unit: %w", err)
		}
		f.PerUnit = &m.m
	}
	if u, ok, _ := d.Get(starlark.String("unit")); ok {
		if f.Unit, ok = starlark.AsString(u); !ok {
			return f, fmt.Errorf("unit: got %s, want string", u.Type())
		}
	}
	if (f.PerUnit != nil) != (f.Unit != "") {
		return f, errors.New("per_unit and unit go together")
	}
	return f, nil
}

var itemBuiltin = starlark.NewBuiltin("item", item)

func item(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var f *foodValue
	var qty number
	var it model.Item
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "food", &f, "qty", &qty, "unit??", &it.Unit, "note?", &it.Note); err != nil {
		return nil, err
	}
	it.Food, it.Qty = f.f, float64(qty)
	if err := checkMealValue("qty", it.Qty); err != nil {
		return nil, fmt.Errorf("%s: %w", b.Name(), err)
	}
	if f.refOnly() { // the unit is settled once the food is filled, in fillItem
		return &itemValue{it: it, refOnly: true}, nil
	}
	perUnit := it.Food.PerUnit != nil
	if it.Unit == "" {
		it.Unit = gram
		if perUnit {
			it.Unit = it.Food.Unit
		}
	}
	switch {
	case perUnit && it.Unit == it.Food.Unit:
	case it.Unit != gram:
		return nil, fmt.Errorf("%s: unit must be %q or the food's unit, got %q", b.Name(), gram, it.Unit)
	case perUnit && !f.per100g:
		return nil, fmt.Errorf("%s: %s has no per100g; give qty in %s", b.Name(), it.Food.Name, it.Food.Unit)
	}
	return &itemValue{it: it}, nil
}

func target(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%s: takes keyword arguments only", b.Name())
	}
	v := &targetValue{}
	for _, kv := range kwargs {
		if err := setMacro(&v.m, string(kv[0].(starlark.String)), kv[1]); err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name(), err)
		}
	}
	return v, nil
}

var mealDocs = docGroup(MealGroup,
	BuiltinDoc{Name: "meal_program", Signature: `meal_program(name, weeks, loop=True, notes="")`,
		Doc:     "Starts a meal program; call it once, before anything else. weeks is the length, 1 to 104; loop works as in program().",
		Example: "# meal\nmeal_program(\"Lean bulk\", weeks=4, notes=\"2600 kcal on training days\")"},
	BuiltinDoc{Name: "day", Signature: `day(week, n=None, name="", desc="", kind="training", target=None, weeks=None)`,
		Doc: "Opens a meal day; meal() calls attach to it. With n it is a numbered day, used for that day position of the week. " +
			"Without n it is a kind day, used on dates whose workout day has that kind: training, conditioning, rest or note. " +
			"target is the day's goal; left out, it is the sum of the day's items. weeks repeats the day in the listed weeks.",
		Example: "# meal fragment\nday(1, kind=\"rest\", weeks=\"1-4\", target=target(kcal=2200, protein=170))"},
	BuiltinDoc{Name: "meal", Signature: `meal(name, *items, time=None, desc="")`,
		Doc:     "Adds a meal to the open day. Items are item() values or food calls: oats(80) is item(oats, 80). time is free text such as \"07:30\".",
		Example: "# meal fragment\noats = food(\"Oats\", per100g={\"kcal\": 379, \"protein\": 13, \"carbs\": 68, \"fat\": 6.5, \"fiber\": 10})\nmeal(\"Breakfast\", oats(80), time=\"07:30\")"},
	BuiltinDoc{Name: "food", Signature: "food(name, per100g=None, per_unit=None, unit=None, ref=None)",
		Doc: "Declares a food and freezes its macros. per100g and per_unit are dicts with the keys kcal, protein, carbs, fat, fiber and water_ml. " +
			"per_unit with unit, such as \"egg\", is for foods counted in units. ref is a catalog food as \"<source>:<source_id>\", copied from food_search, " +
			"such as \"usda_sr:169705\" or \"custom:<uuid>\"; a food with ref and no macros takes them from the catalog when the program is saved. " +
			"A food with only a name is matched against the catalog by name or alias when the program is saved; an unknown name fails " +
			"with suggestions, so add a custom food or remove the item. Explicit macros are never replaced. Calling a food makes an item: eggs(2).",
		Example: "# meal fragment\negg = food(\"Egg, whole\", per100g={\"kcal\": 143, \"protein\": 12.6, \"fat\": 9.5},\n" +
			"           per_unit={\"kcal\": 72, \"protein\": 6.3, \"fat\": 4.8}, unit=\"egg\")\n" +
			"oats = food(\"Oats\")  # macros from the catalog\nmeal(\"Breakfast\", egg(3), item(egg, 50, unit=\"g\"), oats(60))"},
	BuiltinDoc{Name: "item", Signature: `item(food, qty, unit=None, note="")`,
		Doc: "qty of food, in grams, or in the food's unit when it was declared with per_unit (then the default). " +
			"A catalog food given by name or ref alone is in grams unless unit names one of its catalog units, such as egg(2, unit=\"egg\"). food(qty) is the short form.",
		Example: "# meal fragment\nrice = food(\"Basmati rice, raw\", per100g={\"kcal\": 356, \"protein\": 8, \"carbs\": 78, \"fat\": 0.6})\nmeal(\"Lunch\", item(rice, 80, note=\"weighed raw\"))"},
	BuiltinDoc{Name: "target", Signature: "target(kcal=None, protein=None, carbs=None, fat=None, fiber=None, water_ml=None)",
		Doc:     "A day's goal for day(target=...): kcal, grams of protein, carbs, fat and fiber, and ml of water. Keywords only; a field left out has no goal.",
		Example: "# meal fragment\nday(1, kind=\"training\", target=target(kcal=2600, protein=180, carbs=300, fat=70, water_ml=3000))"},
	BuiltinDoc{Name: "catalog_snapshot", Signature: "catalog_snapshot(foods)",
		Doc: "The macros of foods given by ref or name alone, a dict from the ref or name to a macro dict, or to {\"per100g\": ..., \"per_unit\": ..., \"unit\": ...} for a counted food; a ref key wins over a name. " +
			"Export appends this block so an exported meal program evaluates without the catalog; scripts rarely write it by hand.",
		Example: "# meal fragment\ncatalog_snapshot({\"usda_sr:169705\": {\"kcal\": 389, \"protein\": 16.9, \"carbs\": 66.3, \"fat\": 6.9, \"fiber\": 10.6}, " +
			"\"Banana\": {\"kcal\": 89, \"carbs\": 22.8}})\n" +
			"oats = food(\"Oats\", ref=\"usda_sr:169705\")\nbanana = food(\"Banana\")\nmeal(\"Breakfast\", oats(80), banana(120))"},
)
