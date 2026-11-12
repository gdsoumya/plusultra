package engine

import (
	"cmp"
	"fmt"
	"maps"
	"math/bits"
	"slices"
	"strconv"
	"strings"

	"github.com/gdsoumya/plusultra/internal/model"
)

// Resolve resolves every declared day of every week of p for the user, in week
// then day order. Undeclared days are rest days and are left out.
func Resolve(p *model.Program, st *model.UserState) ([]model.ResolvedDay, error) {
	nums := dayNums(p)
	var days []model.ResolvedDay
	for week := 1; week <= p.Weeks; week++ {
		for _, num := range nums {
			d := p.DayAt(week, num)
			if d == nil {
				continue
			}
			rd, _, err := resolveDay(d, st)
			if err != nil {
				return nil, err
			}
			days = append(days, *rd)
		}
	}
	return days, nil
}

// dayNums are the day numbers p declares, ascending.
func dayNums(p *model.Program) []int {
	var nums []int
	for _, d := range p.Days {
		nums = append(nums, d.Num)
	}
	slices.Sort(nums)
	return slices.Compact(nums)
}

// ResolveDay resolves day of week for the user. An undeclared day 1-7 is a
// rest day.
func ResolveDay(p *model.Program, st *model.UserState, week, day int) (*model.ResolvedDay, error) {
	var d *model.Day
	if week >= 1 && week <= p.Weeks && day >= 1 {
		d = p.DayAt(week, day)
	}
	switch {
	case d != nil:
		rd, _, err := resolveDay(d, st)
		return rd, err
	case week < 1 || week > p.Weeks || day < 1 || day > 7: // only floating programs declare days above 7
		return nil, fmt.Errorf("week %d day %d is outside the program", week, day)
	}
	return &model.ResolvedDay{Week: week, Day: day, Kind: "rest"}, nil
}

// resolveDay also returns each exercise's working weights before rounding.
func resolveDay(d *model.Day, st *model.UserState) (*model.ResolvedDay, [][]model.Weight, error) {
	out := &model.ResolvedDay{Week: d.Week, Day: d.Num, Name: d.Name, Desc: d.Desc, Kind: d.Kind}
	raw := make([][]model.Weight, len(d.Exercises))
	for i := range d.Exercises {
		e, r, err := resolveExercise(&d.Exercises[i], d.Week, d.Num, st)
		if err != nil {
			return nil, nil, fmt.Errorf("week %d day %d: %w", d.Week, d.Num, err)
		}
		out.Exercises = append(out.Exercises, e)
		raw[i] = r
	}
	return out, raw, nil
}

// resolveExercise resolves e on week, day for the user. It also returns the
// working sets' weights before rounding.
func resolveExercise(e *model.Exercise, week, day int, st *model.UserState) (model.ResolvedExercise, []model.Weight, error) {
	ov := &st.Overlay
	key := model.ProgKey{Exercise: e.Names[0], Label: e.Label}
	cur := nth(cmp.Or(ov.ExerciseVariation[key], e.Current), len(e.Names)) + 1
	out := model.ResolvedExercise{Name: e.Names[cur-1], Label: e.Label,
		Alternates: slices.Clone(e.Names), Current: cur, Superset: e.Superset}
	out.Equipment = cmp.Or(equipmentOf(out.Name), e.Equipment)
	out.Unilateral = st.Unilateral[out.Name]
	out.Progression = progressionSummary(e.Progress)
	if len(e.Desc) > 0 {
		out.Desc = e.Desc[nth(cmp.Or(ov.DescIndex[key], 1), len(e.Desc))]
	}
	variation := setVariation(e, ov, key)
	planned := resize(expand(declaredSets(e, variation)), plannedCount(e, ov, week, day, variation))
	if len(planned) == 0 {
		return out, nil, nil
	}

	l := loader{units: cmp.Or(st.Units, model.UnitKg), bodyweight: st.Bodyweight, rounding: roundingFor(st, out.Name, out.Equipment)}
	rm1, hasRM1 := ov.RM1[out.Name]
	if !hasRM1 {
		rm1, hasRM1 = st.RM1[out.Name]
	}
	sets := make([]model.ResolvedSet, len(planned))
	raw := make([]model.Weight, len(planned))
	for i, g := range planned {
		addr := model.SetAddr{Key: key, Week: week, Day: day, Variation: variation, Set: i + 1}
		s := &sets[i]
		*s = resolvedSet(g, model.Weight{})
		applySetOverrides(ov, addr, s)
		ld := g.Load
		if ld.Kind == model.LoadNone && hasRM1 && s.RPE > 0 { // an RPE set loads what its reps and RPE give
			ld = model.Load{Kind: model.LoadPercent, Pct: rpeMult(float64(s.Reps), s.RPE) * 100}
		}
		s.Ask = s.Ask || ld.Kind == model.LoadNone
		o, ok := setValue(ov.Weights, addr)
		switch {
		case ok && o.Abs != nil:
			raw[i] = convert(*o.Abs, l.units)
		case ld.Kind == model.LoadPercent && !hasRM1:
			return out, nil, fmt.Errorf("no 1RM for %s", out.Name)
		default:
			raw[i] = l.weight(ld, rm1)
			if ok {
				raw[i] = applyWeight(o, raw[i])
			}
		}
		s.Weight = l.round(raw[i], ld.Kind)
	}
	out.Sets = append(l.warmups(e, planned[0].Load.Kind, sets[0]), sets...)
	return out, raw, nil
}

// applySetOverrides applies the overrides at addr other than weight to s.
// Reps go first: setting them can move min_reps.
func applySetOverrides(ov *model.Overlay, addr model.SetAddr, s *model.ResolvedSet) {
	if o, ok := setValue(ov.Reps, addr); ok {
		setReps(s, applyNum(o, s.Reps, maxListLen))
	}
	if o, ok := setValue(ov.MinReps, addr); ok {
		s.MinReps = applyNum(o, s.MinReps, maxListLen)
	}
	if o, ok := setValue(ov.Rest, addr); ok {
		s.Rest = applyNum(o, s.Rest, maxSeconds)
	}
	if o, ok := setValue(ov.Work, addr); ok {
		s.Work = applyNum(o, s.Work, maxSeconds)
	}
	if o, ok := setValue(ov.RPE, addr); ok {
		s.RPE = applyNum(o, s.RPE, 10)
	}
	applyFlag(ov.Amrap, addr, &s.Amrap)
	applyFlag(ov.Ask, addr, &s.Ask)
	applyFlag(ov.LogRPE, addr, &s.LogRPE)
}

func applyFlag(m map[model.SetAddr]model.BoolOverride, addr model.SetAddr, flag *bool) {
	if o, ok := setValue(m, addr); ok && o.Abs != nil {
		*flag = *o.Abs
	}
}

// setVariation is e's active 1-based set variation, 1 when it has none.
func setVariation(e *model.Exercise, ov *model.Overlay, key model.ProgKey) int {
	if len(e.Variations) == 0 {
		return 1
	}
	return nth(cmp.Or(ov.SetVariation[key], e.SetVariation, 1), len(e.Variations)) + 1
}

// declaredSets are e's set groups in 1-based set variation v.
func declaredSets(e *model.Exercise, v int) []model.SetGroup {
	if len(e.Variations) > 0 {
		return e.Variations[nth(v, len(e.Variations))]
	}
	return e.Sets
}

// equipmentOf is the equipment named by a "Name, Equipment" exercise name, or "".
func equipmentOf(name string) string {
	if i := strings.LastIndex(name, ","); i >= 0 {
		return strings.TrimSpace(name[i+1:])
	}
	return ""
}

// plannedCount is the number of working sets e plans at week, day and set
// variation: as declared, or as the overlay sets it.
func plannedCount(e *model.Exercise, ov *model.Overlay, week, day, variation int) int {
	n := 0
	for _, g := range declaredSets(e, variation) {
		n += g.Count
	}
	key := model.ProgKey{Exercise: e.Names[0], Label: e.Label}
	if o, ok := mostSpecific(ov.NumSets, []int{week, day, variation}, func(f []int) model.DayAddr {
		return model.DayAddr{Key: key, Week: f[0], Day: f[1], Variation: f[2]}
	}); ok {
		n = applyNum(o, n, maxListLen)
	}
	return n
}

// roundingFor is the rounding of exercise name: its own, its equipment's (the
// script's, then the catalog's), the user's default, or a 2.5 kg / 5 lb
// stack. Keys match in any case, an exact match first.
func roundingFor(st *model.UserState, name, equipment string) model.Rounding {
	r, _ := roundingSource(st, name, equipment)
	return r
}

// roundingSource is roundingFor with a description of where the rounding came from.
func roundingSource(st *model.UserState, name, equipment string) (model.Rounding, string) {
	keys := slices.Sorted(maps.Keys(st.Rounding)) // keys differing only in case resolve the same way each time
	for _, c := range []struct{ kind, key string }{{"exercise", name}, {"equipment", equipment}, {"catalog equipment", st.Equipment[name]}} {
		if c.key == "" {
			continue
		}
		if r, ok := st.Rounding[c.key]; ok {
			return r, c.kind + " " + strconv.Quote(c.key)
		}
		if i := slices.IndexFunc(keys, func(x string) bool { return strings.EqualFold(x, c.key) }); i >= 0 {
			return st.Rounding[keys[i]], c.kind + " " + strconv.Quote(keys[i])
		}
	}
	if st.DefaultRounding.Kind != "" {
		return st.DefaultRounding, "the default rounding"
	}
	if st.Units == model.UnitLb {
		return model.Rounding{Kind: model.RoundStack, Increment: 5}, "the built-in 5 lb stack"
	}
	return model.Rounding{Kind: model.RoundStack, Increment: 2.5}, "the built-in 2.5 kg stack"
}

// setReps sets s's target reps. A fixed-reps set stays fixed, and a range
// widens down to reps.
func setReps(s *model.ResolvedSet, reps int) {
	if s.MinReps == s.Reps || reps < s.MinReps {
		s.MinReps = reps
	}
	s.Reps = reps
}

// expand lists one group per set.
func expand(groups []model.SetGroup) []model.SetGroup {
	var out []model.SetGroup
	for _, g := range groups {
		for range g.Count {
			out = append(out, g)
		}
	}
	return out
}

// resize truncates sets to n, at most maxListLen, or extends them with copies
// of the last set.
func resize[T any](sets []T, n int) []T {
	n = min(n, maxListLen)
	if n <= len(sets) {
		return sets[:max(n, 0)]
	}
	for len(sets) > 0 && len(sets) < n {
		sets = append(sets, sets[len(sets)-1])
	}
	return sets
}

// nth is the 0-based index of 1-based i among n items, wrapping around.
func nth(i, n int) int { return ((i-1)%n + n) % n }

func setValue[V any](m map[model.SetAddr]V, a model.SetAddr) (V, bool) {
	return mostSpecific(m, []int{a.Week, a.Day, a.Variation, a.Set}, func(f []int) model.SetAddr {
		return model.SetAddr{Key: a.Key, Week: f[0], Day: f[1], Variation: f[2], Set: f[3]}
	})
}

// mostSpecific returns m's value at the most specific address matching
// fields, where a zero field in an address matches all. Between addresses
// with as many fields set, the one setting earlier fields wins.
func mostSpecific[K comparable, V any](m map[K]V, fields []int, addr func([]int) K) (V, bool) {
	var zero V
	if len(m) == 0 {
		return zero, false
	}
	n := len(fields)
	f := make([]int, n)
	for set := n; set >= 0; set-- {
		for mask := uint(1)<<n - 1; ; mask-- {
			if bits.OnesCount(mask) == set {
				for i := range f {
					f[i] = 0
					if mask&(1<<(n-1-i)) != 0 {
						f[i] = fields[i]
					}
				}
				if v, ok := m[addr(f)]; ok {
					return v, true
				}
			}
			if mask == 0 {
				break
			}
		}
	}
	return zero, false
}
