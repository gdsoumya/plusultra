package engine

import (
	"cmp"
	"fmt"
	"math/bits"

	"github.com/gdsoumya/plusultra/internal/model"
)

const (
	// maxOverlayWrites bounds the overlay entries one run may hold. Each write
	// scans them, so it also bounds the Go work a script can cause.
	maxOverlayWrites = 5000
	maxScale         = 1000
)

// edit is one write to a set field: abs sets it to v, otherwise it becomes
// x*scale + v. unit is v's unit for weights. Only weights scale.
type edit struct {
	abs   bool
	v     float64
	unit  model.Unit
	scale float64
}

func clamp[T int | float64](x, lo, hi T) T { return min(max(x, lo), hi) }

// composeWeight is o followed by e. Relative writes stay relative, so each
// address keeps its own script weight underneath.
func composeWeight(o model.WeightOverride, e edit) model.WeightOverride {
	d := model.Weight{Value: e.v, Unit: e.unit}
	switch {
	case e.abs:
		return model.WeightOverride{Abs: &d}
	case o.Abs != nil:
		w := *o.Abs
		w.Value = clamp(w.Value*e.scale+convert(d, w.Unit).Value, 0, maxWeight(w.Unit))
		return model.WeightOverride{Abs: &w}
	}
	delta := cmp.Or(o.Delta, model.Weight{Unit: e.unit})
	lim := maxWeight(delta.Unit)
	delta.Value = clamp(delta.Value*e.scale+convert(d, delta.Unit).Value, -lim, lim)
	scale := clamp(cmp.Or(o.Scale, 1)*e.scale, 0, maxScale)
	if scale == 0 { // a zero Scale reads as 1, and x*0 + delta is just delta
		w := model.Weight{Value: max(delta.Value, 0), Unit: delta.Unit}
		return model.WeightOverride{Abs: &w}
	}
	return model.WeightOverride{Delta: delta, Scale: scale}
}

// composeNum is o followed by e, which does not scale, bounded by hi.
func composeNum[T int | float64](o model.Override[T], e edit, hi T) model.Override[T] {
	v := T(e.v)
	switch {
	case e.abs:
	case o.Abs != nil:
		v = clamp(*o.Abs+v, 0, hi)
	default:
		o.Delta = clamp(o.Delta+v, -hi, hi)
		return o
	}
	return model.Override[T]{Abs: &v}
}

// applyWeight is script weight w changed by o, in w's unit.
func applyWeight(o model.WeightOverride, w model.Weight) model.Weight {
	if o.Abs != nil {
		w = convert(*o.Abs, w.Unit)
	} else {
		w.Value = w.Value*cmp.Or(o.Scale, 1) + convert(o.Delta, w.Unit).Value
	}
	w.Value = clamp(w.Value, 0, maxWeight(w.Unit))
	return w
}

// applyNum is script value x changed by o, bounded by hi.
func applyNum[T int | float64](o model.Override[T], x, hi T) T {
	if o.Abs != nil {
		x = *o.Abs
	} else {
		x += o.Delta
	}
	return clamp(x, 0, hi)
}

// assign applies f at address a of an overlay, m being this run's delta over
// base; addr and join map keys to set addresses and combine two overlapping
// keys. Every address x that a matches must end up as f of x's value before.
// After the write, resolution picks at x the join of a with every entry that
// matches x, and the entries in force at that join are exactly those, so f
// goes to each join of a with a set of overlapping entries, starting from the
// value in force there now. Joins with no entry are a itself and the entries
// a covers, so a broadcast moves older, more specific overrides instead of
// losing to them, and a specific write builds on broader ones.
func assign[K comparable, V any](m, base map[K]V, a K, addr func(K) model.SetAddr, join func(K, K) K, f func(V) V) error {
	targets, seen := []K{a}, map[K]bool{a: true}
	for _, src := range []map[K]V{base, m} {
		for b := range src {
			ab := addr(b)
			if !overlaps(addr(a), ab) {
				continue
			}
			for _, t := range targets { // the targets so far; joins added here already hold b
				if j := join(t, b); overlaps(addr(t), ab) && !seen[j] {
					targets, seen[j] = append(targets, j), true
				}
			}
			if len(targets) > maxOverlayWrites {
				return fmt.Errorf("more than %d overlay writes", maxOverlayWrites)
			}
		}
	}
	starts := make([]V, len(targets))
	for i, t := range targets {
		starts[i] = inForce(m, base, t, addr)
	}
	for i, t := range targets {
		m[t] = f(starts[i])
	}
	if len(m) > maxOverlayWrites {
		return fmt.Errorf("more than %d overlay writes", maxOverlayWrites)
	}
	return nil
}

// inForce is the override resolution applies at k: the most specific entry
// covering it, m's over base's for the same key, or zero.
func inForce[K comparable, V any](m, base map[K]V, k K, addr func(K) model.SetAddr) V {
	if v, ok := m[k]; ok {
		return v
	}
	if v, ok := base[k]; ok {
		return v
	}
	var v V
	best, ka := -1, addr(k)
	for _, src := range []map[K]V{base, m} {
		for b, x := range src {
			if r := specificity(addr(b)); r >= best && coversSet(addr(b), ka) {
				v, best = x, r
			}
		}
	}
	return v
}

func matches(x, y int) bool { return x == 0 || x == y }

// overlaps reports whether some set address matches both a and b.
func overlaps(a, b model.SetAddr) bool {
	agree := func(x, y int) bool { return x == 0 || y == 0 || x == y }
	return a.Key == b.Key && agree(a.Week, b.Week) && agree(a.Day, b.Day) &&
		agree(a.Variation, b.Variation) && agree(a.Set, b.Set)
}

// coversSet reports whether a matches every address b does.
func coversSet(a, b model.SetAddr) bool {
	return a.Key == b.Key && matches(a.Week, b.Week) && matches(a.Day, b.Day) &&
		matches(a.Variation, b.Variation) && matches(a.Set, b.Set)
}

// specificity ranks a as resolution prefers it: more fields set, then the
// earlier fields (week, day, variation, set).
func specificity(a model.SetAddr) int {
	mask := 0
	for _, f := range []int{a.Week, a.Day, a.Variation, a.Set} {
		mask <<= 1
		if f != 0 {
			mask |= 1
		}
	}
	return bits.OnesCount(uint(mask))<<4 | mask
}

func setAddr(a model.SetAddr) model.SetAddr { return a }

func dayAddr(a model.DayAddr) model.SetAddr {
	return model.SetAddr{Key: a.Key, Week: a.Week, Day: a.Day, Variation: a.Variation}
}

// joinSet and joinDay are the addresses two overlapping addresses both match.
func joinSet(a, b model.SetAddr) model.SetAddr {
	return model.SetAddr{Key: a.Key, Week: cmp.Or(a.Week, b.Week), Day: cmp.Or(a.Day, b.Day),
		Variation: cmp.Or(a.Variation, b.Variation), Set: cmp.Or(a.Set, b.Set)}
}

func joinDay(a, b model.DayAddr) model.DayAddr {
	j := joinSet(dayAddr(a), dayAddr(b))
	return model.DayAddr{Key: j.Key, Week: j.Week, Day: j.Day, Variation: j.Variation}
}
