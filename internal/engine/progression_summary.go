package engine

import (
	"fmt"
	"strconv"

	"github.com/gdsoumya/plusultra/internal/model"
)

// progressionSummary is a one-line human description of p; empty for none.
func progressionSummary(p *model.Progression) string {
	if p == nil {
		return ""
	}
	switch p.Kind {
	case model.ProgressLP:
		inc := "+" + num(p.IncPct) + "%"
		if p.IncPct == 0 {
			inc = "+" + weightStr(p.Inc)
		}
		s := "Linear: " + inc + " after " + count(p.After, "a successful workout", "successes")
		switch {
		case p.FailReset > 0:
			s += ", -" + num((1-p.FailReset)*100) + "% after " + count(p.FailAfter, "a failure", "failures")
		case p.FailDec.Value > 0:
			s += ", -" + weightStr(p.FailDec) + " after " + count(p.FailAfter, "a failure", "failures")
		}
		return s
	case model.ProgressDP:
		return fmt.Sprintf("Double progression %d-%d reps, then +%s", p.Lo, p.Hi, weightStr(p.Inc))
	case model.ProgressSumReps:
		return fmt.Sprintf("Sum of reps >= %d: +%s", p.Target, weightStr(p.Inc))
	case model.ProgressCustom:
		return "Custom (" + p.Fn.Name() + ")"
	}
	return ""
}

func count(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return strconv.Itoa(n) + " " + many
}

func weightStr(w model.Weight) string { return num(w.Value) + " " + string(w.Unit) }

// num prints v with the fewest digits that round-trip, avoiding 9.999999999 from percent math.
func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 32) }
