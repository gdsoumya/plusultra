package export

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/gdsoumya/plusultra/internal/model"
)

const kgPerLb = 0.45359237

// formatDays writes days in the text format of the log-format reference, one
// block per day, blocks separated by a blank line.
func formatDays(days []Day, units model.Unit) string {
	var b strings.Builder
	for i, d := range days {
		if i > 0 {
			b.WriteByte('\n')
		}
		writeDay(&b, d, units)
	}
	return b.String()
}

func writeDay(b *strings.Builder, d Day, units model.Unit) {
	b.WriteString(d.Date)
	if d.Status != "" {
		b.WriteString(" / status: " + d.Status)
	}
	if d.Checkin != nil && d.Checkin.WeightKg != nil {
		b.WriteString(" / weight: " + bodyweight(*d.Checkin.WeightKg, units))
	}
	if d.Notes != "" {
		b.WriteString(" / notes: " + strconv.Quote(d.Notes))
	}
	b.WriteByte('\n')
	if w := d.Workout; w != nil {
		fmt.Fprintf(b, "workout: %q week %d day %d %q", w.Program, w.Week, w.Day, w.DayName)
		if m := w.minutes(); m > 0 {
			fmt.Fprintf(b, " / %dmin", m)
		}
		b.WriteByte('\n')
		for _, e := range w.Entries {
			writeEntry(b, e)
		}
	}
	if m := d.Meals; m != nil {
		fmt.Fprintf(b, "meals: target %s / actual %s\n", macrosText(m.Target), macrosText(m.Totals))
		for _, meal := range m.Meals {
			writeMeal(b, meal)
		}
	}
}

// num is x to one decimal, without a trailing zero.
func num(x float64) string { return strconv.FormatFloat(math.Round(x*10)/10, 'f', -1, 64) }

func bodyweight(kg float64, units model.Unit) string {
	if units == model.UnitLb {
		return num(kg/kgPerLb) + "lb"
	}
	return num(kg) + "kg"
}

func macrosText(m model.Macros) string {
	return fmt.Sprintf("%skcal %sp %sc %sf", num(m.Kcal), num(m.Protein), num(m.Carbs), num(m.Fat))
}

func weightText(w model.RecordWeight) string {
	return strconv.FormatFloat(w.Value, 'f', -1, 64) + cmp.Or(string(w.Unit), string(model.UnitKg))
}

func writeEntry(b *strings.Builder, e Entry) {
	b.WriteString("  ")
	if e.Label != "" {
		b.WriteString(e.Label + ": ")
	}
	b.WriteString(e.Name)
	for _, g := range groups(e.Sets) {
		b.WriteString(" / " + groupText(g))
	}
	if e.WarmupDone {
		b.WriteString(" / warmup done")
	}
	b.WriteByte('\n')
}

// groups splits the working sets into runs with the same plan.
func groups(sets []model.RecordSet) [][]model.RecordSet {
	var out [][]model.RecordSet
	for _, s := range sets {
		if s.Warmup || s.Planned.Warmup {
			continue
		}
		if n := len(out); n > 0 && samePlan(out[n-1][0].Planned, s.Planned) {
			out[n-1] = append(out[n-1], s)
			continue
		}
		out = append(out, []model.RecordSet{s})
	}
	return out
}

// samePlan ignores the timers and flags the text leaves out.
func samePlan(a, b model.RecordPlanned) bool {
	for _, p := range []*model.RecordPlanned{&a, &b} {
		p.Rest, p.Work, p.LogRPE, p.Auto, p.CountUp = 0, 0, false, false, false
	}
	return a == b
}

// asked reports whether the set's weight was asked for and the plan has none.
func asked(p model.RecordPlanned) bool { return p.Ask && p.Weight.Value == 0 }

func groupText(g []model.RecordSet) string {
	p := g[0].Planned
	reps := strconv.Itoa(p.Reps)
	if p.MinReps != 0 && p.MinReps != p.Reps {
		reps = fmt.Sprintf("%d-%d", p.MinReps, p.Reps)
	}
	if p.Amrap {
		reps += "+"
	}
	weight := "?"
	if !asked(p) {
		weight = weightText(p.Weight)
	}
	s := fmt.Sprintf("%dx%s %s", len(g), reps, weight)
	if p.RPE != 0 {
		s += " @" + strconv.FormatFloat(p.RPE, 'f', -1, 64)
	}
	if p.Label != "" {
		s += " (" + p.Label + ")"
	}
	ok := !asked(p) && !slices.ContainsFunc(g, func(x model.RecordSet) bool {
		return !x.Completed || x.Weight != p.Weight || x.Reps < p.Reps || p.Amrap && x.Reps != p.Reps
	})
	if ok {
		return s + " " + strings.Repeat("✓", len(g))
	}
	results := make([]string, len(g))
	for i, x := range g {
		switch {
		case !x.Completed:
			results[i] = "-"
		case x.Weight != p.Weight || asked(p):
			results[i] = fmt.Sprintf("%d@%s", x.Reps, weightText(x.Weight))
		default:
			results[i] = strconv.Itoa(x.Reps)
		}
	}
	return s + " -> " + strings.Join(results, ",")
}

func writeMeal(b *strings.Builder, m model.Meal) {
	b.WriteString("  " + m.Name)
	if !slices.ContainsFunc(m.Items, func(it model.Item) bool { return it.Planned }) {
		b.WriteString(" (ad hoc)")
	}
	for _, it := range m.Items {
		qty := strconv.FormatFloat(it.Qty, 'f', -1, 64)
		if it.Unit == "" || it.Unit == "g" {
			fmt.Fprintf(b, " / %s %sg", it.Food.Name, qty)
		} else {
			fmt.Fprintf(b, " / %s %s %s", it.Food.Name, qty, it.Unit)
		}
	}
	b.WriteByte('\n')
}
