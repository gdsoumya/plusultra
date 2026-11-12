package engine

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/gdsoumya/plusultra/internal/model"
)

// Warning flags something a simulated program does that the author may not
// intend. Code is one of rounding_moved, rounding_source, stalled_progression,
// pct_without_rm1 and double_progression_in_week; Exercise is the exercise
// concerned and Week and Day where it was first seen.
type Warning struct {
	Code      string
	Week, Day int
	Exercise  string
	Message   string
}

// maxWarnings bounds the warnings of one simulation.
const maxWarnings = 200

// warner collects warnings, each distinct one once.
type warner struct {
	res  *SimResult
	seen map[string]bool
}

func (w *warner) add(code string, week, day int, exercise, msg string) {
	w.dedupe(code+"\x00"+exercise+"\x00"+msg, Warning{code, week, day, exercise, msg})
}

// once adds the first warning of code about exercise.
func (w *warner) once(code string, week, day int, exercise, msg string) {
	w.dedupe(code+"\x00"+exercise, Warning{code, week, day, exercise, msg})
}

func (w *warner) dedupe(key string, x Warning) {
	if w.seen[key] {
		return
	}
	w.seen[key] = true
	if len(w.res.Warnings) >= maxWarnings {
		if last := w.res.Warnings[len(w.res.Warnings)-1]; last.Code != "truncated" {
			w.res.Warnings = append(w.res.Warnings, Warning{Code: "truncated", Message: "more warnings left out"})
		}
		return
	}
	w.res.Warnings = append(w.res.Warnings, x)
}

// scan warns about what the program's structure shows before any day runs:
// percent loads without a 1RM, and one exercise progressing on several days
// of a week.
func (w *warner) scan(p *model.Program, st *model.UserState) {
	for week := 1; week <= p.Weeks; week++ {
		days := map[model.ProgKey][]int{}
		for _, num := range dayNums(p) {
			d := p.DayAt(week, num)
			if d == nil {
				continue
			}
			for i := range d.Exercises {
				e := &d.Exercises[i]
				key := model.ProgKey{Exercise: e.Names[0], Label: e.Label}
				if e.Progress != nil {
					days[key] = append(days[key], num)
				}
				name := e.Names[nth(cmp.Or(st.Overlay.ExerciseVariation[key], e.Current), len(e.Names))]
				if _, ok := st.Overlay.RM1[name]; ok {
					continue
				}
				if _, ok := st.RM1[name]; !ok && usesPct(e) {
					w.once("pct_without_rm1", week, num, name,
						fmt.Sprintf("%s loads with pct() but has no 1RM; set one with exercise_set_data or an rm1 override", name))
				}
			}
		}
		for _, key := range slices.SortedFunc(maps.Keys(days), func(a, b model.ProgKey) int { return strings.Compare(keyName(a), keyName(b)) }) {
			if nums := days[key]; len(nums) > 1 {
				w.once("double_progression_in_week", week, nums[0], keyName(key),
					fmt.Sprintf("%s progresses on days %s of week %d; label the instances or set progress=\"none\" on all but one", keyName(key), dayList(nums), week))
			}
		}
	}
}

func usesPct(e *model.Exercise) bool {
	groups := slices.Clone(e.Sets)
	for _, v := range e.Variations {
		groups = append(groups, v...)
	}
	return slices.ContainsFunc(groups, func(g model.SetGroup) bool { return g.Load.Kind == model.LoadPercent })
}

func dayList(nums []int) string {
	s := make([]string, len(nums))
	for i, n := range nums {
		s[i] = strconv.Itoa(n)
	}
	if len(s) > 1 {
		return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
	}
	return s[0]
}

// rounding warns, per exercise of rd, which rounding it uses and about every
// working weight the rounding moved; raw holds the unrounded working weights.
func (w *warner) rounding(rd *model.ResolvedDay, raw [][]model.Weight, st *model.UserState) {
	for i, e := range rd.Exercises {
		r, src := roundingSource(st, e.Name, e.Equipment)
		w.once("rounding_source", rd.Week, rd.Day, e.Name, fmt.Sprintf("%s rounds with %s: %s", e.Name, src, roundingStr(r, st.Units)))
		for j, s := range working(e.Sets) {
			if j < len(raw[i]) && raw[i][j].Value > 0 && math.Abs(raw[i][j].Value-s.Weight.Value) > 1e-9 {
				w.add("rounding_moved", rd.Week, rd.Day, e.Name, fmt.Sprintf("%s: %s rounds to %s", e.Name, weightStr(raw[i][j]), weightStr(s.Weight)))
			}
		}
	}
}

// stalled warns when the progression wrote a weight for this session's
// address but the weight still rounds to what it was.
func (w *warner) stalled(ex *model.Exercise, rd *model.ResolvedDay, e *model.ResolvedExercise, raw []model.Weight, delta model.Overlay, st *model.UserState) {
	here := model.SetAddr{Key: model.ProgKey{Exercise: ex.Names[0], Label: ex.Label}, Week: rd.Week, Day: rd.Day}
	touched := false
	for a := range delta.Weights {
		touched = touched || overlaps(a, here)
	}
	if !touched {
		return
	}
	after, rawAfter, err := resolveExercise(ex, rd.Week, rd.Day, st)
	before, now := working(e.Sets), working(after.Sets)
	if err != nil || len(before) == 0 || len(now) != len(before) || len(rawAfter) != len(raw) {
		return
	}
	moved := false
	for i := range before {
		if before[i].Weight != now[i].Weight {
			return
		}
		moved = moved || rawAfter[i] != raw[i]
	}
	if moved {
		w.add("stalled_progression", rd.Week, rd.Day, e.Name,
			fmt.Sprintf("%s: the progression moved the weight but it still rounds to %s; the step is smaller than the equipment loads", e.Name, weightStr(now[0].Weight)))
	}
}

func working(sets []model.ResolvedSet) []model.ResolvedSet {
	return slices.DeleteFunc(slices.Clone(sets), func(s model.ResolvedSet) bool { return s.Warmup })
}

// roundingStr describes r in unit u.
func roundingStr(r model.Rounding, u model.Unit) string {
	nums := func(xs []float64) string {
		s := make([]string, len(xs))
		for i, x := range xs {
			s[i] = num(x)
		}
		return strings.Join(s, " ")
	}
	switch r.Kind {
	case model.RoundPlates:
		return fmt.Sprintf("plates, bar %s %s, per side %s %s", num(r.Bar), u, nums(r.Plates), u)
	case model.RoundStack:
		return fmt.Sprintf("stack, steps of %s %s", num(r.Increment), u)
	case model.RoundFixed:
		return fmt.Sprintf("fixed weights %s %s", nums(r.Plates), u)
	}
	return "no rounding"
}
