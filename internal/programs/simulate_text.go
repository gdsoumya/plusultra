package programs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Filter keeps part of a simulation's output: the weeks and days listed, the
// exercises named (any alternate, ignoring case) and, with WorkingOnly, no
// warmups. Days left without an exercise are dropped.
type Filter struct {
	Exercises   []string
	Weeks, Days []int
	WorkingOnly bool
}

func (f Filter) apply(days []Day, trace []engine.SessionTrace) ([]Day, []engine.SessionTrace) {
	var outDays []Day
	var outTrace []engine.SessionTrace
	ti := 0
	for _, d := range days {
		for ti < len(trace) && trace[ti].Week == d.Week && trace[ti].Day == d.Day {
			if f.day(d) && (len(f.Exercises) == 0 || f.named(trace[ti].Exercise)) {
				outTrace = append(outTrace, trace[ti])
			}
			ti++
		}
		if !f.day(d) {
			continue
		}
		rd := *d.Workout
		rd.Exercises = slices.Clone(rd.Exercises)
		if len(f.Exercises) > 0 {
			rd.Exercises = slices.DeleteFunc(rd.Exercises, func(e model.ResolvedExercise) bool { return !f.named(e.Alternates...) })
			if len(rd.Exercises) == 0 {
				continue
			}
		}
		if f.WorkingOnly {
			for i := range rd.Exercises {
				rd.Exercises[i].Sets = working(rd.Exercises[i].Sets)
			}
		}
		d.Workout = &rd
		outDays = append(outDays, d)
	}
	return outDays, outTrace
}

func (f Filter) day(d Day) bool {
	return (len(f.Weeks) == 0 || slices.Contains(f.Weeks, d.Week)) && (len(f.Days) == 0 || slices.Contains(f.Days, d.Day))
}

func (f Filter) named(names ...string) bool {
	return slices.ContainsFunc(names, func(n string) bool {
		return slices.ContainsFunc(f.Exercises, func(x string) bool { return strings.EqualFold(x, n) })
	})
}

func working(sets []model.ResolvedSet) []model.ResolvedSet {
	return slices.DeleteFunc(slices.Clone(sets), func(s model.ResolvedSet) bool { return s.Warmup })
}

// compactLine is one session on one line: "W2D3 Wed 2026-11-25 Lower B | Deadlift 115x5 x3, 100x5 x2 | Hack Squat 45x8 x3".
func compactLine(d Day) string {
	var b strings.Builder
	fmt.Fprintf(&b, "W%dD%d", d.Week, d.Day)
	if d.Date != nil {
		b.WriteString(d.Date.Format(" Mon 2006-01-02"))
	}
	if d.Workout.Name != "" {
		b.WriteString(" " + d.Workout.Name)
	}
	for _, e := range d.Workout.Exercises {
		name := e.Name
		if e.Label != "" {
			name += ":" + e.Label
		}
		b.WriteString(" | " + name + " " + setGroups(working(e.Sets)))
	}
	return b.String()
}

// setGroups writes consecutive equal sets once with a count: "100x5 x3, 90x5 x2".
func setGroups(sets []model.ResolvedSet) string {
	var groups []string
	for i := 0; i < len(sets); {
		j := i + 1
		for j < len(sets) && setStr(sets[j]) == setStr(sets[i]) {
			j++
		}
		g := setStr(sets[i])
		if j-i > 1 {
			g += " x" + strconv.Itoa(j-i)
		}
		groups = append(groups, g)
		i = j
	}
	return strings.Join(groups, ", ")
}

func setStr(s model.ResolvedSet) string {
	w := num(s.Weight.Value)
	if s.Ask && s.Weight.Value == 0 {
		w = "?"
	}
	reps := strconv.Itoa(s.Reps)
	if s.MinReps > 0 && s.MinReps != s.Reps {
		reps = strconv.Itoa(s.MinReps) + "-" + reps
	}
	if s.Amrap {
		reps += "+"
	}
	return w + "x" + reps
}

// num prints v with the fewest digits that round-trip as a float32.
func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 32) }

func weightStr(w model.Weight) string { return num(w.Value) + " " + string(w.Unit) }

// Expectation is a check on one working set (Set, 1-based, 0 for the first)
// of an exercise on a day: its Weight when set and its Reps when positive.
type Expectation struct {
	Week, Day int
	Exercise  string
	Set       int
	Weight    *model.Weight
	Reps      int
}

// Assertion answers an Expectation: Actual is the set as simulated, such as
// "102.5 kg x5", and Message why it failed.
type Assertion struct {
	Expect          Expectation
	OK              bool
	Actual, Message string
}

func assert(exps []Expectation, days []Day) []Assertion {
	out := make([]Assertion, len(exps))
	for i, x := range exps {
		out[i] = Assertion{Expect: x, Message: fmt.Sprintf("no session W%dD%d", x.Week, x.Day)}
		di := slices.IndexFunc(days, func(d Day) bool { return d.Week == x.Week && d.Day == x.Day })
		if di < 0 {
			continue
		}
		ei := slices.IndexFunc(days[di].Workout.Exercises, func(e model.ResolvedExercise) bool {
			return slices.ContainsFunc(e.Alternates, func(n string) bool { return strings.EqualFold(n, x.Exercise) })
		})
		if ei < 0 {
			out[i].Message = fmt.Sprintf("W%dD%d has no %s", x.Week, x.Day, x.Exercise)
			continue
		}
		e := days[di].Workout.Exercises[ei]
		sets, n := working(e.Sets), max(x.Set, 1)
		if n > len(sets) {
			out[i].Message = fmt.Sprintf("%s has %d sets, not %d", e.Name, len(sets), n)
			continue
		}
		s := sets[n-1]
		out[i].Actual, out[i].OK, out[i].Message = weightStr(s.Weight)+" x"+strconv.Itoa(s.Reps), true, ""
		var fails []string
		if x.Weight != nil && !sameWeight(s.Weight, *x.Weight) {
			fails = append(fails, fmt.Sprintf("weight %s, want %s", weightStr(s.Weight), weightStr(withUnit(*x.Weight, s.Weight.Unit))))
		}
		if x.Reps > 0 && s.Reps != x.Reps {
			fails = append(fails, fmt.Sprintf("reps %d, want %d", s.Reps, x.Reps))
		}
		if len(fails) > 0 {
			out[i].OK, out[i].Message = false, strings.Join(fails, "; ")
		}
	}
	return out
}

// sameWeight compares a and b, b without a unit taken in a's.
func sameWeight(a, b model.Weight) bool {
	return abs(a.Kg()-withUnit(b, a.Unit).Kg()) < 1e-6
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// DiffCell is one value that differs between two simulations: Field is
// weight or reps of working set Set, "sets" for the set count, or exercise
// or day when one run lacks it.
type DiffCell struct {
	Week, Day   int
	Exercise    string
	Set         int
	Field, A, B string
}

func diff(a, b []Day) []DiffCell {
	var out []DiffCell
	same := func(x Day) func(Day) bool { return func(y Day) bool { return x.Week == y.Week && x.Day == y.Day } }
	for _, da := range a {
		bi := slices.IndexFunc(b, same(da))
		if bi < 0 {
			out = append(out, DiffCell{Week: da.Week, Day: da.Day, Field: "day", A: "present", B: "-"})
			continue
		}
		out = append(out, diffDay(da, b[bi])...)
	}
	for _, db := range b {
		if !slices.ContainsFunc(a, same(db)) {
			out = append(out, DiffCell{Week: db.Week, Day: db.Day, Field: "day", A: "-", B: "present"})
		}
	}
	return out
}

func diffDay(a, b Day) []DiffCell {
	var out []DiffCell
	key := func(e model.ResolvedExercise) model.ProgKey {
		return model.ProgKey{Exercise: e.Alternates[0], Label: e.Label}
	}
	cell := func(e model.ResolvedExercise, set int, field, x, y string) DiffCell {
		return DiffCell{Week: a.Week, Day: a.Day, Exercise: e.Name, Set: set, Field: field, A: x, B: y}
	}
	for _, ea := range a.Workout.Exercises {
		bi := slices.IndexFunc(b.Workout.Exercises, func(e model.ResolvedExercise) bool { return key(e) == key(ea) })
		if bi < 0 {
			out = append(out, cell(ea, 0, "exercise", "present", "-"))
			continue
		}
		sa, sb := working(ea.Sets), working(b.Workout.Exercises[bi].Sets)
		if len(sa) != len(sb) {
			out = append(out, cell(ea, 0, "sets", strconv.Itoa(len(sa)), strconv.Itoa(len(sb))))
		}
		for i := range min(len(sa), len(sb)) {
			if !sameWeight(sa[i].Weight, sb[i].Weight) {
				out = append(out, cell(ea, i+1, "weight", weightStr(sa[i].Weight), weightStr(sb[i].Weight)))
			}
			if sa[i].Reps != sb[i].Reps {
				out = append(out, cell(ea, i+1, "reps", strconv.Itoa(sa[i].Reps), strconv.Itoa(sb[i].Reps)))
			}
		}
	}
	for _, eb := range b.Workout.Exercises {
		if !slices.ContainsFunc(a.Workout.Exercises, func(e model.ResolvedExercise) bool { return key(e) == key(eb) }) {
			out = append(out, cell(eb, 0, "exercise", "-", "present"))
		}
	}
	return out
}
