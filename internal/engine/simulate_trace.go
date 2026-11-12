package engine

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// SessionTrace is one progression run of a simulation: the function that ran,
// its exercise state before and after as script values, every overlay write,
// and its print output.
type SessionTrace struct {
	Week, Day       int
	Exercise, Label string
	Function        string
	StateBefore     map[string]string
	StateAfter      map[string]string
	Writes          []Write
	Prints          []string
}

// Write is one overlay entry a progression wrote: the field and address, such
// as "weights Squat[w2 s1]", whether it is absolute or relative, and the value.
type Write struct{ Address, Kind, Value string }

func sessionTrace(rd *model.ResolvedDay, e *model.ResolvedExercise, pr *model.Progression, before map[string]starlark.Value, delta model.Overlay, prints []string) SessionTrace {
	key := model.ProgKey{Exercise: e.Alternates[0], Label: e.Label}
	t := SessionTrace{Week: rd.Week, Day: rd.Day, Exercise: e.Name, Label: e.Label, Function: pr.Kind, Prints: prints,
		StateBefore: stateRepr(before), StateAfter: stateRepr(delta.State[key]), Writes: writes(delta, key)}
	if pr.Kind == model.ProgressCustom {
		t.Function = pr.Fn.Name()
	}
	if seen, err := loadState(pr.State, before, nil); err == nil { // what the script read, defaults included
		t.StateBefore = stateRepr(seen)
	}
	return t
}

func stateRepr(vars map[string]starlark.Value) map[string]string {
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		out[k] = v.String()
	}
	return out
}

// writes lists delta's entries, sorted by address. The state of key itself is
// in the trace's StateAfter; other keys' state (shared tags) is listed here.
func writes(d model.Overlay, key model.ProgKey) []Write {
	var out []Write
	for a, o := range d.Weights {
		out = append(out, Write{addrString("weights", a), kindOf(o.Abs == nil), weightOverride(o)})
	}
	out = append(out, numWrites("reps", d.Reps)...)
	out = append(out, numWrites("min_reps", d.MinReps)...)
	out = append(out, numWrites("rest", d.Rest)...)
	out = append(out, numWrites("work", d.Work)...)
	out = append(out, numWrites("rpe", d.RPE)...)
	for name, m := range map[string]map[model.SetAddr]model.BoolOverride{"amrap": d.Amrap, "ask": d.Ask, "log_rpe": d.LogRPE} {
		for a, o := range m {
			out = append(out, Write{addrString(name, a), "absolute", strconv.FormatBool(*o.Abs)})
		}
	}
	for a, o := range d.NumSets {
		out = append(out, Write{addrString("number_of_sets", dayAddr(a)), kindOf(o.Abs == nil), numOverride(o)})
	}
	for name, m := range map[string]map[model.ProgKey]int{"set_variation": d.SetVariation, "exercise_variation": d.ExerciseVariation, "description_index": d.DescIndex} {
		for k, v := range m {
			out = append(out, Write{name + " " + keyName(k), "absolute", strconv.Itoa(v)})
		}
	}
	for name, w := range d.RM1 {
		out = append(out, Write{"rm1 " + name, "absolute", weightStr(w)})
	}
	for k, vars := range d.State {
		if k == key {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(vars)) {
			out = append(out, Write{"state " + keyName(k) + "." + name, "absolute", vars[name].String()})
		}
	}
	slices.SortFunc(out, func(a, b Write) int { return cmp.Or(cmp.Compare(a.Address, b.Address), cmp.Compare(a.Value, b.Value)) })
	return out
}

func numWrites[T int | float64](name string, m map[model.SetAddr]model.Override[T]) []Write {
	var out []Write
	for a, o := range m {
		out = append(out, Write{addrString(name, a), kindOf(o.Abs == nil), numOverride(o)})
	}
	return out
}

func kindOf(relative bool) string {
	if relative {
		return "relative"
	}
	return "absolute"
}

// keyName is the exercise of k as scripts name it: "Name" or "Name:label".
func keyName(k model.ProgKey) string {
	if k.Label == "" {
		return k.Exercise
	}
	return k.Exercise + ":" + k.Label
}

// addrString is field at a: "weights Squat" for every set, "weights Squat[w2 d1 s3]" narrowed.
func addrString(field string, a model.SetAddr) string {
	var parts []string
	for _, p := range []struct {
		tag string
		n   int
	}{{"w", a.Week}, {"d", a.Day}, {"v", a.Variation}, {"s", a.Set}} {
		if p.n != 0 {
			parts = append(parts, p.tag+strconv.Itoa(p.n))
		}
	}
	s := field + " " + keyName(a.Key)
	if len(parts) > 0 {
		s += "[" + strings.Join(parts, " ") + "]"
	}
	return s
}

func weightOverride(o model.WeightOverride) string {
	if o.Abs != nil {
		return weightStr(*o.Abs)
	}
	var parts []string
	if o.Scale != 0 && o.Scale != 1 {
		parts = append(parts, "x"+num(o.Scale))
	}
	if o.Delta.Value != 0 || len(parts) == 0 {
		parts = append(parts, signed(o.Delta.Value)+" "+string(o.Delta.Unit))
	}
	return strings.Join(parts, " ")
}

func numOverride[T int | float64](o model.Override[T]) string {
	if o.Abs != nil {
		return num(float64(*o.Abs))
	}
	return signed(float64(o.Delta))
}

func signed(v float64) string {
	if v >= 0 {
		return "+" + num(v)
	}
	return num(v)
}
