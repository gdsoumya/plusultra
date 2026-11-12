package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// maxSimulateOutput bounds the print output Simulate keeps over all its runs,
// which each have their own maxPrintBytes.
const maxSimulateOutput = 256 << 10

// SimOptions says how Simulate runs a program.
type SimOptions struct {
	// Weeks to run, 1 to 104, from week FromWeek day FromDay; zero means the start.
	Weeks, FromWeek, FromDay int
	// Reps done, and completed, on every working set of the exercise named, instead of the plan.
	Reps map[string]int
	// Scenario is what the user did in particular sessions; everything else goes as planned.
	Scenario []Logged
	// Trace records every progression run in the result.
	Trace bool
}

// Logged is what the user did for one exercise of one session. Sets, when
// given, are the results as logged. Otherwise the plan is completed, with
// Weight on every set, WeightDelta added to every set and MissLastSetBy
// fewer reps on the last set. Skip leaves the exercise unlogged, so its
// progression does not run. Prompts answer the progression's prompt() state.
type Logged struct {
	Week, Day           int
	Exercise, Label     string
	Sets                []model.SetResult
	Weight, WeightDelta *model.Weight
	MissLastSetBy       int
	Skip                bool
	Prompts             map[string]starlark.Value
}

func (l *Logged) matches(week, day int, e *model.ResolvedExercise) bool {
	return l.Week == week && l.Day == day && strings.EqualFold(l.Label, e.Label) &&
		slices.ContainsFunc(e.Alternates, func(n string) bool { return strings.EqualFold(n, l.Exercise) })
}

// SimResult is what Simulate produced: the days in order, each resolved after
// the progressions of the days before it, the scripts' print output cut at
// maxSimulateOutput, a trace per progression run when asked, and warnings.
type SimResult struct {
	Days     []model.ResolvedDay
	Output   []string
	Trace    []SessionTrace
	Warnings []Warning
}

// Simulate runs workout program p forward for opts.Weeks weeks, starting
// again at week 1 when the program loops, as if every workout went as opts
// says. st is the user's state, nil for none; Simulate does not change it.
// It stops when ctx ends. On an error the result holds the output and the
// warnings so far. Script errors are *EvalError.
func Simulate(ctx context.Context, p *model.Program, st *model.UserState, opts SimOptions) (*SimResult, error) {
	res := &SimResult{}
	if opts.Weeks < 1 || opts.Weeks > maxWeeks {
		return res, fmt.Errorf("weeks must be 1-%d", maxWeeks)
	}
	from := max(opts.FromWeek, 1)
	if from > p.Weeks {
		return res, fmt.Errorf("week %d is outside the %d-week program", from, p.Weeks)
	}
	sim := model.UserState{Units: p.Units}
	if st != nil {
		sim = *st
		sim.Overlay = model.Overlay{}
		sim.Overlay.Merge(st.Overlay) // copies the maps that progressions write into
	}
	w := &warner{res: res, seen: map[string]bool{}}
	w.scan(p, &sim)
	out := &output{}
	defer func() { res.Output = out.lines }()
	for i := range opts.Weeks {
		week := from + i
		if week > p.Weeks {
			if !p.Loop {
				break
			}
			week = (week-1)%p.Weeks + 1
		}
		for _, num := range dayNums(p) {
			if i == 0 && num < opts.FromDay {
				continue
			}
			d := p.DayAt(week, num)
			if d == nil {
				continue
			}
			rd, raw, err := resolveDay(d, &sim)
			if err != nil {
				return res, err
			}
			res.Days = append(res.Days, *rd)
			w.rounding(rd, raw, &sim)
			for j := range rd.Exercises {
				if err := ctx.Err(); err != nil {
					return res, fmt.Errorf("simulation stopped in week %d: %w", week, err)
				}
				if err := runSession(p, &sim, rd, j, raw[j], &opts, res, w, out); err != nil {
					return res, err
				}
			}
		}
	}
	return res, nil
}

// runSession logs exercise i of rd as opts says and runs its progression.
func runSession(p *model.Program, sim *model.UserState, rd *model.ResolvedDay, i int, raw []model.Weight, opts *SimOptions, res *SimResult, w *warner, out *output) error {
	e := &rd.Exercises[i]
	r, ok := logged(rd, e, opts)
	if !ok {
		return nil
	}
	before := sim.Overlay.State[r.Key]
	delta, prints, err := RunProgress(p, sim, r)
	out.keep(prints)
	if err != nil {
		return err
	}
	ex, err := findExercise(p, r)
	if err != nil {
		return err
	}
	if opts.Trace && ex.Progress != nil {
		res.Trace = append(res.Trace, sessionTrace(rd, e, ex.Progress, before, delta, prints))
	}
	sim.Overlay.Merge(delta)
	w.stalled(ex, rd, e, raw, delta, sim)
	return nil
}

// logged is the result opts gives exercise e of day rd, and whether it was
// logged at all.
func logged(rd *model.ResolvedDay, e *model.ResolvedExercise, opts *SimOptions) (model.ExerciseResult, bool) {
	res := model.ExerciseResult{Key: model.ProgKey{Exercise: e.Alternates[0], Label: e.Label}, Week: rd.Week, Day: rd.Day}
	var lg *Logged
	if i := slices.IndexFunc(opts.Scenario, func(l Logged) bool { return l.matches(rd.Week, rd.Day, e) }); i >= 0 {
		lg = &opts.Scenario[i]
		res.PromptValues = lg.Prompts
		switch {
		case lg.Skip:
			return res, false
		case len(lg.Sets) > 0:
			res.Sets = lg.Sets
			return res, true
		}
	}
	reps, given := opts.Reps[e.Name]
	work := slices.DeleteFunc(slices.Clone(e.Sets), func(s model.ResolvedSet) bool { return s.Warmup })
	for i, s := range work {
		r := model.SetResult{Reps: s.Reps, MinReps: s.MinReps, Weight: s.Weight, RPE: s.RPE, Rest: s.Rest, Completed: true}
		if given {
			r.Reps = reps
		}
		if lg != nil {
			if lg.Weight != nil {
				r.Weight = convert(*lg.Weight, s.Weight.Unit)
			}
			if lg.WeightDelta != nil {
				r.Weight.Value += convert(*lg.WeightDelta, s.Weight.Unit).Value
			}
			if i == len(work)-1 {
				r.Reps = max(r.Reps-lg.MissLastSetBy, 0)
			}
		}
		res.Sets = append(res.Sets, r)
	}
	return res, true
}

// output collects print lines up to maxSimulateOutput.
type output struct {
	lines []string
	size  int
}

func (o *output) keep(lines []string) {
	for _, l := range lines {
		if o.size > maxSimulateOutput {
			return
		}
		if o.size += len(l) + 1; o.size > maxSimulateOutput { // +1 so empty lines count too
			o.lines = append(o.lines, "output truncated")
			return
		}
		o.lines = append(o.lines, l)
	}
}
