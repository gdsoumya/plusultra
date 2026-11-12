package engine

import (
	"cmp"
	"math"
	"slices"

	"github.com/gdsoumya/plusultra/internal/model"
)

const lbInKg = 0.45359237

// convert expresses w in unit to. A weight without a unit is taken as already in to.
func convert(w model.Weight, to model.Unit) model.Weight {
	switch {
	case w.Unit == model.UnitLb && to == model.UnitKg:
		w.Value *= lbInKg
	case w.Unit == model.UnitKg && to == model.UnitLb:
		w.Value /= lbInKg
	}
	w.Unit = to
	return w
}

// maxWeight is the heaviest weight in unit u that a script may load and that
// rounding considers. It bounds the plate search.
func maxWeight(u model.Unit) float64 {
	if u == model.UnitLb {
		return 4400
	}
	return 2000
}

// RoundWeight rounds w to the nearest weight the equipment r can load, after
// clamping it to maxWeight. A zero weight (an ask set) and bodyweight
// equipment are left as they are.
func RoundWeight(w model.Weight, r model.Rounding) model.Weight {
	if w.Value == 0 {
		return w
	}
	limit := maxWeight(w.Unit)
	w.Value = min(w.Value, limit)
	switch r.Kind {
	case model.RoundPlates:
		w.Value = roundPlates(w.Value, limit, r)
	case model.RoundStack:
		if r.Increment > 0 {
			w.Value = max(r.Increment, math.Round(w.Value/r.Increment)*r.Increment)
		}
	case model.RoundFixed:
		if len(r.Plates) > 0 {
			w.Value = slices.MinFunc(r.Plates, func(a, b float64) int {
				return cmp.Compare(math.Abs(a-w.Value), math.Abs(b-w.Value))
			})
		}
	}
	return w
}

// roundPlates finds the loadable weight nearest v, the lighter on a tie. It
// works in steps of 0.001 and searches per-side sums in steps of the plates'
// GCD, ignoring plates heavier than limit. With no bar, a positive weight
// gets at least one pair of the lightest plate.
func roundPlates(v, limit float64, r model.Rounding) float64 {
	var plates []int
	unit := 0
	for _, p := range r.Plates {
		if p >= 0.001 && p <= limit {
			g := int(math.Round(p * 1000))
			plates = append(plates, g)
			unit = gcd(unit, g)
		}
	}
	bar := max(r.Bar, 0)
	if !(v > bar) || len(plates) == 0 {
		return bar
	}
	slices.Sort(plates)
	plates = slices.Compact(plates)
	side := int(math.Round((v - bar) * 500))
	// The nearest sum above side is at most one lightest plate past the best below it.
	reach := make([]bool, (side+plates[0])/unit+1)
	for i := range plates {
		plates[i] /= unit
	}
	dist := func(j int) int { return max(j*unit-side, side-j*unit) }
	reach[0] = true
	best := 0
	if bar == 0 {
		best = plates[0]
	}
	for j := 1; j < len(reach); j++ {
		for _, p := range plates {
			if p <= j && reach[j-p] {
				reach[j] = true
				break
			}
		}
		if reach[j] && dist(j) < dist(best) {
			best = j
		}
	}
	return bar + float64(2*best*unit)/1000
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
