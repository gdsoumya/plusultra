package engine

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// maxWeeks bounds program length so week ranges stay cheap to expand and resolve.
const maxWeeks = 104

var dayKinds = []string{"training", "conditioning", "rest", "note"}

type builtinFn = func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)

// Evaluate runs a workout program script and returns its structure. Errors are *EvalError.
func Evaluate(src, filename string) (*model.Program, error) {
	p, _, err := EvaluateState(src, filename)
	return p, err
}

// builder is the program under construction during one evaluation.
type builder struct {
	prog  *model.Program
	open  *model.Day // the day ex() attaches to
	state *State     // the exported state block, once seen
}

func (b *builder) builtins() starlark.StringDict {
	d := builtins()
	fns := map[string]builtinFn{
		"program": b.program, "week": b.needProgram(b.week), "day": b.needProgram(b.day), "ex": b.needProgram(b.ex),
		"s": setGroup, "w": warmupGroup, "reps_range": repsRange, "template": template,
		"lp": lp, "dp": dp, "sum_reps": sumReps, "custom": custom, "prompt": prompt, "state": b.stateBlock,
	}
	maps.Copy(fns, scriptFuncs) // predeclared here because functions resolve names where they are defined
	for name, fn := range fns {
		d[name] = starlark.NewBuiltin(name, fn)
	}
	return d
}

func (b *builder) needProgram(f builtinFn) builtinFn {
	return func(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if b.prog == nil {
			return nil, fmt.Errorf("%s: call program() first", fn.Name())
		}
		return f(t, fn, args, kwargs)
	}
}

func (b *builder) program(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if b.prog != nil {
		return nil, fmt.Errorf("%s: called more than once", fn.Name())
	}
	p := &model.Program{Units: model.UnitKg, Loop: true, WeekDesc: map[int]string{}}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &p.Name, "weeks", &p.Weeks,
		"units?", (*string)(&p.Units), "loop?", &p.Loop, "notes?", &p.Notes); err != nil {
		return nil, err
	}
	if p.Weeks < 1 || p.Weeks > maxWeeks {
		return nil, fmt.Errorf("%s: weeks must be 1-%d", fn.Name(), maxWeeks)
	}
	if !p.Units.Valid() {
		return nil, fmt.Errorf("%s: units must be kg or lb, got %q", fn.Name(), p.Units)
	}
	b.prog = p
	return starlark.None, nil
}

func (b *builder) week(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var n int
	var desc string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "n", &n, "desc?", &desc); err != nil {
		return nil, err
	}
	if err := b.checkWeeks(fn, []int{n}); err != nil {
		return nil, err
	}
	if desc != "" {
		b.prog.WeekDesc[n] = desc
	}
	b.open = nil
	return starlark.None, nil
}

func (b *builder) day(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	d := model.Day{Kind: "training", Line: callerLine(t)}
	var weeks intList
	var desc starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "week", &d.Week, "n", &d.Num, "name", &d.Name,
		"desc??", &desc, "kind?", &d.Kind, "weeks??", &weeks); err != nil {
		return nil, err
	}
	d.Weeks = weeks
	if desc != nil {
		var ok bool
		if d.Desc, ok = starlark.AsString(desc); !ok {
			return nil, fmt.Errorf("%s: desc must be a string, got %s", fn.Name(), desc.Type())
		}
		d.DescSet = true
	}
	if err := cmp.Or(b.checkWeeks(fn, []int{d.Week}), b.checkWeeks(fn, d.Weeks)); err != nil {
		return nil, err
	}
	if d.Num < 1 {
		return nil, fmt.Errorf("%s: day number must be at least 1", fn.Name())
	}
	if !slices.Contains(dayKinds, d.Kind) {
		return nil, fmt.Errorf("%s: kind must be one of %v, got %q", fn.Name(), dayKinds, d.Kind)
	}
	b.prog.Days = append(b.prog.Days, d)
	b.open = &b.prog.Days[len(b.prog.Days)-1]
	return starlark.None, nil
}

func (b *builder) ex(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if b.open == nil {
		return nil, fmt.Errorf("%s: call day() before ex()", fn.Name())
	}
	e := model.Exercise{Current: 1, Line: callerLine(t)}
	var (
		name             string
		alts, desc       strList
		weeks, tags      intList
		sets, warmup     setList
		variations       variationList
		progress, update progressArg
		state            *starlark.Dict
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name??", &name, "alts??", &alts, "eq??", &e.Equipment,
		"label??", &e.Label, "weeks??", &weeks, "current?", &e.Current, "sets??", &sets, "warmup??", &warmup,
		"desc??", &desc, "superset??", &e.Superset, "tags??", &tags, "progress??", &progress,
		"update??", &update, "state??", &state, "variations??", &variations, "set_variation??", &e.SetVariation); err != nil {
		return nil, err
	}
	if name != "" {
		e.Names = []string{name}
	}
	e.Names = append(e.Names, alts...)
	e.Weeks, e.Sets, e.Variations, e.Desc, e.Tags = weeks, sets, variations, desc, tags
	e.Warmup, e.WarmupSet = warmup, warmup != nil
	switch {
	case len(e.Names) == 0:
		return nil, fmt.Errorf("%s: name or alts required", fn.Name())
	case e.Current < 1 || e.Current > len(e.Names):
		return nil, fmt.Errorf("%s: current %d out of range 1-%d", fn.Name(), e.Current, len(e.Names))
	case len(sets) > 0 && variations != nil:
		return nil, fmt.Errorf("%s: give sets or variations, not both", fn.Name())
	case len(sets) == 0 && len(variations) == 0:
		return nil, fmt.Errorf("%s: sets or variations required", fn.Name())
	case e.SetVariation < 0 || e.SetVariation > len(variations):
		return nil, fmt.Errorf("%s: set_variation %d is not one of the %d variations", fn.Name(), e.SetVariation, len(variations))
	}
	if err := b.checkWeeks(fn, e.Weeks); err != nil {
		return nil, err
	}
	if err := attachProgress(&e, progress, update, state); err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	for i := range b.open.Exercises {
		o := &b.open.Exercises[i]
		ow := b.open.WeeksOf(o)
		if o.Names[0] == e.Names[0] && o.Label == e.Label &&
			slices.ContainsFunc(b.open.WeeksOf(&e), func(w int) bool { return slices.Contains(ow, w) }) {
			return nil, fmt.Errorf("%s: %q is already on this day; give one a label", fn.Name(), e.Names[0])
		}
	}
	b.open.Exercises = append(b.open.Exercises, e)
	return starlark.None, nil
}

// checkWeeks validates week numbers against the program; nil means absent.
func (b *builder) checkWeeks(fn *starlark.Builtin, weeks []int) error {
	if weeks != nil && len(weeks) == 0 {
		return fmt.Errorf("%s: weeks is empty", fn.Name())
	}
	for _, w := range weeks {
		if w < 1 || w > b.prog.Weeks {
			return fmt.Errorf("%s: week %d out of range 1-%d", fn.Name(), w, b.prog.Weeks)
		}
	}
	return nil
}

func template(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("%s: takes keyword arguments only", b.Name())
	}
	d := starlark.NewDict(len(kwargs))
	for _, kv := range kwargs {
		if err := d.SetKey(kv[0], kv[1]); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// callerLine is the script line that called the running builtin.
func callerLine(t *starlark.Thread) int { return int(t.CallFrame(1).Pos.Line) }

var programDocs = docGroup("Program structure",
	BuiltinDoc{Name: "program", Signature: `program(name, weeks, units="kg", loop=True, notes="")`,
		Doc: "Starts a workout program; call it once, before anything else. weeks is the length, 1 to 104. " +
			"units (\"kg\" or \"lb\") is the unit ctx reads weights in; the user sees resolved weights in their own units. " +
			"loop=False finishes a floating program after its last week instead of wrapping to week 1; anchored programs always end. " +
			"The start date and scheduling mode are chosen at activation, not in the script.",
		Example: `program("Upper/lower", weeks=4, units="kg", loop=True, notes="Four days a week")`},
	BuiltinDoc{Name: "week", Signature: `week(n, desc="")`,
		Doc: "Sets the description of week n, shown as a phase note such as \"Deload\"; it applies to that week only. " +
			"week() also closes the open day, so call day() again before more ex() calls.",
		Example: "# fragment\nweek(4, desc=\"Deload: same lifts, half the sets\")"},
	BuiltinDoc{Name: "day", Signature: `day(week, n, name, desc=None, kind="training", weeks=None)`,
		Doc: "Opens day n of week; every following ex() attaches to it until the next day() or week(). " +
			"In an anchored program n is the weekday (1 Monday to 7 Sunday); in a floating program days are a sequence and n may pass 7. " +
			"kind is training, conditioning, rest or note. weeks (\"1-8\", a list, or range(1, 9)) repeats the day and its exercises in every listed week. " +
			"A later week's day() with the same n overrides the name and kind for the weeks it covers and adds or replaces exercises by name and label; exercises a range carries stay. " +
			"desc carries forward to later weeks until a declaration gives a new one; desc=\"\" stops it.",
		Example: "# fragment\nday(1, 2, \"Upper A\", desc=\"Heavy pressing\", weeks=\"1-4\")\nex(\"Bench Press\", sets=[s(3, 5, kg(80))])"},
	BuiltinDoc{Name: "ex", Signature: `ex(name=None, alts=None, eq=None, label=None, weeks=None, current=1, sets=None, warmup=None, desc=None, superset=None, tags=None, progress=None, update=None, state=None, variations=None, set_variation=None)`,
		Doc: "Adds an exercise to the open day. name is a catalog name, optionally with an equipment suffix (\"Bench Press, Dumbbell\"). " +
			"alts are alternatives that share sets, weights and progression; current (1-based over name, then alts) picks the active one. " +
			"Give sets (a list of s() groups) or variations (a list of set lists, switched by ctx.set_variation, starting at set_variation). " +
			"eq names the equipment used for rounding when the active name has no suffix. label tells two instances of one exercise apart: " +
			"the program identifies an exercise by its first name and label. weeks places it on this day number in the listed weeks only. " +
			"warmup is a list of w() groups, [] for none, or left out for the default ramp. desc is a string, or a list switched by ctx.description_index; " +
			"it carries forward to later declarations until redeclared, and \"\" stops it. superset groups exercises of a day by a shared string. " +
			"tags are ints, kept as metadata. progress takes lp(), dp(), sum_reps(), custom() or \"none\"; update takes custom(fn) or \"none\"; " +
			"state is a dict of extra state variables for the progression.",
		Example: "# fragment\nex(\"Bench Press\", alts=[\"Bench Press, Dumbbell\"], label=\"heavy\", desc=\"Pause on the chest.\",\n" +
			"   sets=[s(3, 5, kg(80), rest=180)], progress=lp(kg(2.5)))"},
	BuiltinDoc{Name: "template", Signature: "template(**kwargs)",
		Doc: "Returns a dict of ex() arguments to reuse as ex(name, **t). Keyword arguments only. " +
			"Each exercise that uses it gets its own progression and state.",
		Example: "# fragment\nMAIN = template(sets=[s(3, 5, pct(75), rest=180)], progress=lp(kg(2.5)))\nex(\"Squat\", **MAIN)\nex(\"Bench Press\", **MAIN)"},
	BuiltinDoc{Name: "state", Signature: "state(overlay)",
		Doc: "The progression state of an exported program, as export writes it after the script. Import reads it back and keeps the state " +
			"of exercises the script still has. Never write it by hand: saving a script that contains it is rejected; import the exported file instead.",
		Example: `# fragment
state({"Weights": {"\"Squat\" \"\" 0 0 0 0": {"Delta": {"Value": 2.5, "Unit": "kg"}, "Scale": 1}}})`},
)
