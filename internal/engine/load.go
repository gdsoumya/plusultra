package engine

import "github.com/gdsoumya/plusultra/internal/model"

// loader turns one exercise's loads into weights in the user's units.
type loader struct {
	units      model.Unit
	bodyweight model.Weight
	rounding   model.Rounding
}

// weight is ld unrounded; a percent is of base.
func (l loader) weight(ld model.Load, base model.Weight) model.Weight {
	switch ld.Kind {
	case model.LoadPercent:
		return convert(model.Weight{Value: base.Value * ld.Pct / 100, Unit: base.Unit}, l.units)
	case model.LoadBodyweight:
		w := convert(l.bodyweight, l.units)
		w.Value += convert(ld.Delta, l.units).Value
		return w
	case model.LoadNone:
		return model.Weight{Unit: l.units}
	}
	return convert(ld.Weight, l.units)
}

// round rounds w to the equipment. A bodyweight load is the body plus a
// delta, not something the equipment loads, so it is left as is.
func (l loader) round(w model.Weight, kind model.LoadKind) model.Weight {
	if kind == model.LoadBodyweight {
		return w
	}
	return RoundWeight(w, l.rounding)
}

// warmups are e's declared warmups, or the default when it declares none,
// for the first working set.
func (l loader) warmups(e *model.Exercise, firstLoad model.LoadKind, first model.ResolvedSet) []model.ResolvedSet {
	var out []model.ResolvedSet
	switch {
	case e.WarmupSet:
		for _, g := range expand(e.Warmup) {
			s := resolvedSet(g, l.round(l.weight(g.Load, first.Weight), g.Load.Kind))
			s.Warmup = true
			out = append(out, s)
		}
	case firstLoad != model.LoadBodyweight && first.Weight.Value > l.rounding.Bar:
		out = DefaultWarmup(first)
		for i := range out {
			out[i].Weight = RoundWeight(out[i].Weight, l.rounding)
		}
	}
	return out
}

var defaultWarmup = []struct {
	pct  float64
	reps int
}{{50, 8}, {70, 5}, {85, 3}}

// DefaultWarmup is the warmup for an exercise that declares none: 50%×8,
// 70%×5 and 85%×3 of the first working set, unrounded. Resolution skips it
// for bodyweight loads and for weights at or below the bar.
func DefaultWarmup(first model.ResolvedSet) []model.ResolvedSet {
	out := make([]model.ResolvedSet, len(defaultWarmup))
	for i, d := range defaultWarmup {
		w := first.Weight
		w.Value *= d.pct / 100
		out[i] = model.ResolvedSet{Reps: d.reps, MinReps: d.reps, Weight: w, Warmup: true}
	}
	return out
}

func resolvedSet(g model.SetGroup, w model.Weight) model.ResolvedSet {
	return model.ResolvedSet{Reps: g.MaxReps, MinReps: g.MinReps, Weight: w, RPE: g.RPE, Rest: g.Rest, Work: g.Work,
		Amrap: g.Amrap, Ask: g.Ask, LogRPE: g.LogRPE, Auto: g.Auto, CountUp: g.CountUp, Label: g.Label}
}
