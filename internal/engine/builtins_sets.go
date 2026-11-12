package engine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// maxListLen bounds list arguments; a range() is lazy and would otherwise
// let a script allocate without limit in Go code the step limit cannot see.
const maxListLen = 1000

// opaque supplies the Value methods of host values that scripts only pass around.
type opaque struct{}

func (opaque) Freeze()               {}
func (opaque) Truth() starlark.Bool  { return true }
func (opaque) Hash() (uint32, error) { return 0, errors.New("unhashable") }

type setGroupValue struct {
	opaque
	g model.SetGroup
}

func (*setGroupValue) Type() string     { return "set_group" }
func (v *setGroupValue) String() string { return fmt.Sprintf("set_group(%+v)", v.g) }

func setGroup(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		g    model.SetGroup
		reps repsArg
		load = loadArg{Kind: model.LoadNone}
		rpe  number
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "n", &g.Count, "reps", &reps, "weight??", &load,
		"rest??", &g.Rest, "label??", &g.Label, "rpe??", &rpe, "amrap?", &g.Amrap, "ask?", &g.Ask,
		"work??", &g.Work, "auto?", &g.Auto, "count_up?", &g.CountUp, "log_rpe?", &g.LogRPE); err != nil {
		return nil, err
	}
	g.MinReps, g.MaxReps, g.Load, g.RPE = reps.lo, reps.hi, model.Load(load), float64(rpe)
	switch {
	case g.Load.Kind == model.LoadNone && g.RPE == 0:
		return nil, fmt.Errorf("%s: weight required: give kg, lb, pct, bw or rpe", b.Name())
	case !(g.RPE >= 0 && g.RPE <= 10):
		return nil, fmt.Errorf("%s: rpe must be between 0 and 10", b.Name())
	case g.Rest < 0 || g.Work < 0:
		return nil, fmt.Errorf("%s: rest and work must not be negative", b.Name())
	}
	return newSetGroup(b, g)
}

func warmupGroup(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		g    model.SetGroup
		reps repsArg
		load = loadArg{Kind: model.LoadNone}
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "n", &g.Count, "reps", &reps, "weight??", &load); err != nil {
		return nil, err
	}
	if load.Kind == model.LoadNone {
		return nil, fmt.Errorf("%s: weight required: give kg, lb, pct or bw", b.Name())
	}
	g.MinReps, g.MaxReps, g.Load = reps.lo, reps.hi, model.Load(load)
	return newSetGroup(b, g)
}

func newSetGroup(b *starlark.Builtin, g model.SetGroup) (starlark.Value, error) {
	if g.Count < 1 || g.Count > maxListLen {
		return nil, fmt.Errorf("%s: n must be 1-%d", b.Name(), maxListLen)
	}
	return &setGroupValue{g: g}, nil
}

func repsRange(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var lo, hi int
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "lo", &lo, "hi", &hi); err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.MakeInt(lo), starlark.MakeInt(hi)}, nil
}

func validWeight(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= 0 }

var (
	errWeight   = errors.New("weight must be a finite non-negative number")
	errTooLarge = errors.New("weight too large: the limit is 2000 kg or 4400 lb")
)

// loadArg unpacks a kg, lb, pct or bw value.
type loadArg model.Load

func (l *loadArg) Unpack(v starlark.Value) error {
	lv, ok := v.(interface{ Load() model.Load })
	if !ok {
		return fmt.Errorf("got %s, want kg, lb, pct or bw", v.Type())
	}
	ld := lv.Load()
	valid := validWeight(ld.Weight.Value)
	switch ld.Kind {
	case model.LoadPercent:
		valid = validWeight(ld.Pct)
	case model.LoadBodyweight:
		valid = validWeight(math.Abs(ld.Delta.Value)) // an assisted delta is negative
	}
	if !valid {
		return errWeight
	}
	if ld.Kind == model.LoadAbsolute && ld.Weight.Value > maxWeight(ld.Weight.Unit) {
		return errTooLarge
	}
	*l = loadArg(ld)
	return nil
}

// repsArg unpacks an int or a (lo, hi) tuple.
type repsArg struct{ lo, hi int }

func (r *repsArg) Unpack(v starlark.Value) error {
	var err error
	if t, ok := v.(starlark.Tuple); ok && len(t) == 2 {
		r.lo, err = starlark.AsInt32(t[0])
		if err == nil {
			r.hi, err = starlark.AsInt32(t[1])
		}
	} else {
		r.lo, err = starlark.AsInt32(v)
		r.hi = r.lo
	}
	if err != nil {
		return fmt.Errorf("got %s, want int or (lo, hi)", v)
	}
	if r.lo < 1 || r.lo > r.hi {
		return fmt.Errorf("reps %s: want at least 1 and lo <= hi", v)
	}
	return nil
}

// setList unpacks a list of s() or w() values.
type setList []model.SetGroup

func (l *setList) Unpack(v starlark.Value) error {
	gs, err := listOf(v, func(x starlark.Value) (model.SetGroup, error) {
		if g, ok := x.(*setGroupValue); ok {
			return g.g, nil
		}
		return model.SetGroup{}, fmt.Errorf("got %s, want s(...)", x.Type())
	})
	*l = gs
	return err
}

// variationList unpacks a list of set lists.
type variationList [][]model.SetGroup

func (l *variationList) Unpack(v starlark.Value) error {
	vs, err := listOf(v, func(x starlark.Value) ([]model.SetGroup, error) {
		var sl setList
		if err := sl.Unpack(x); err != nil {
			return nil, err
		}
		if len(sl) == 0 {
			return nil, errors.New("empty variation")
		}
		return sl, nil
	})
	*l = vs
	return err
}

// strList unpacks a string or a list of strings. "" is the empty list.
type strList []string

func (l *strList) Unpack(v starlark.Value) error {
	if s, ok := v.(starlark.String); ok {
		*l = []string{}
		if s != "" {
			*l = []string{string(s)}
		}
		return nil
	}
	ss, err := listOf(v, func(x starlark.Value) (string, error) {
		if s, ok := starlark.AsString(x); ok {
			return s, nil
		}
		return "", fmt.Errorf("got %s, want string", x.Type())
	})
	*l = ss
	return err
}

// intList unpacks a list of ints, or a "n" or "lo-hi" string.
type intList []int

func (l *intList) Unpack(v starlark.Value) error {
	s, ok := starlark.AsString(v)
	if !ok {
		ns, err := listOf(v, starlark.AsInt32)
		*l = ns
		return err
	}
	from, to, isRange := strings.Cut(s, "-")
	lo, err := strconv.Atoi(strings.TrimSpace(from))
	hi := lo
	if err == nil && isRange {
		hi, err = strconv.Atoi(strings.TrimSpace(to))
	}
	if err != nil || lo > hi || hi-lo >= maxListLen {
		return fmt.Errorf("%q: want \"n\" or \"lo-hi\"", s)
	}
	*l = []int{}
	for n := lo; n <= hi; n++ {
		*l = append(*l, n)
	}
	return nil
}

// listOf converts each element of an iterable. Empty input gives an empty, non-nil slice.
func listOf[T any](v starlark.Value, conv func(starlark.Value) (T, error)) ([]T, error) {
	iter := starlark.Iterate(v)
	if iter == nil {
		return nil, fmt.Errorf("got %s, want list", v.Type())
	}
	defer iter.Done()
	out := []T{}
	var x starlark.Value
	for iter.Next(&x) {
		if len(out) == maxListLen {
			return nil, fmt.Errorf("more than %d items", maxListLen)
		}
		t, err := conv(x)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

var setDocs = docGroup("Sets",
	BuiltinDoc{Name: "s", Signature: `s(n, reps, weight=None, rest=None, label=None, rpe=None, amrap=False, ask=False, work=None, auto=False, count_up=False, log_rpe=False)`,
		Doc: "A set group: n sets of reps, where reps is an int or a (lo, hi) range that targets hi with lo as the minimum. " +
			"Every group needs a load: kg(), lb(), pct() of 1RM, bw(), or rpe= alone, which loads 1RM x rpe_multiplier(reps, rpe) " +
			"(an ask set at 0 when the user has no 1RM). rest is the rest timer and work the active set timer, in seconds; " +
			"auto advances to the next set when the rest ends; count_up records time past work. amrap=True makes reps a minimum (the 5+ notation). " +
			"ask=True asks the user for the weight; log_rpe=True asks for the RPE done. rpe is 0-10. label names the group, such as \"Backoff\".",
		Example: "# fragment\nex(\"Squat\", sets=[s(1, 5, kg(100), rest=180, label=\"Top\"), s(3, (8, 10), kg(80), rest=120), s(1, 5, kg(70), amrap=True)])\n" +
			"ex(\"Bench Press\", sets=[s(3, 8, rpe=8, log_rpe=True)])\nex(\"Plank\", warmup=[], sets=[s(3, 1, bw(), work=45, rest=30, auto=True)])"},
	BuiltinDoc{Name: "w", Signature: "w(n, reps, weight)",
		Doc: "A warmup set group for ex(warmup=[...]). pct() here is a percent of the first working set, not of 1RM. " +
			"Without warmup= an exercise gets a default ramp of 50% x 8, 70% x 5 and 85% x 3 of its first working set, " +
			"skipped for bodyweight loads and for weights at or below the bar; warmup=[] means none.",
		Example: "# fragment\nex(\"Deadlift\", warmup=[w(1, 5, kg(60)), w(1, 3, pct(80))], sets=[s(3, 5, kg(140))])"},
	BuiltinDoc{Name: "reps_range", Signature: "reps_range(lo, hi)",
		Doc:     "A rep range, the same as the tuple (lo, hi): the set targets hi reps with lo as the minimum.",
		Example: "# fragment\nex(\"Lat Pulldown\", sets=[s(3, reps_range(8, 12), kg(50))])"},
)
