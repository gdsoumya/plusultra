package engine

import (
	"time"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// RunProgress runs the progression of the exercise res reports, after its
// workout. It returns the overlay entries the progression wrote, for the
// caller to merge into st.Overlay, and the script's print output. Run a
// workout's exercises in order and merge each delta before the next, so
// shared tag state carries over. Script errors are *EvalError.
func RunProgress(p *model.Program, st *model.UserState, res model.ExerciseResult) (model.Overlay, []string, error) {
	c, err := newCtx(p, st, res, false, 0)
	if err != nil {
		return model.Overlay{}, nil, err
	}
	var out []string
	switch pr := c.e.Progress; {
	case pr == nil:
	case pr.Kind == model.ProgressCustom:
		out, err = c.run(pr.Fn)
	default:
		err = c.builtin(pr)
	}
	if err == nil {
		err = c.commit()
	}
	if err != nil {
		return model.Overlay{}, out, err
	}
	return c.delta, out, nil
}

func (c *ctxValue) commit() error {
	if err := c.commitState(c.key, c.state.vars); err != nil {
		return err
	}
	for tag, s := range c.shared {
		if err := c.commitState(tagKey(tag), s.vars); err != nil {
			return err
		}
	}
	return nil
}

// maxPrintBytes bounds the print output kept from one run.
const maxPrintBytes = 64 << 10

// run calls fn(ctx) in the sandbox and returns its print output, which stops
// at maxPrintBytes.
func (c *ctxValue) run(fn *starlark.Function) ([]string, error) {
	var out []string
	size := 0
	t := newThread(fn.Name())
	defer t.Local(timerKey).(*time.Timer).Stop()
	t.SetLocal(ctxKey, c)
	t.Print = func(_ *starlark.Thread, msg string) {
		size += len(msg)
		if size <= maxPrintBytes && len(out) < maxListLen {
			out = append(out, msg)
		}
	}
	if _, err := starlark.Call(t, fn, starlark.Tuple{c}, nil); err != nil {
		return out, toEvalError(fn.Position().Filename(), err)
	}
	return out, nil
}

// builtin runs lp, dp or sum_reps, which change every address of the
// exercise relatively. A workout succeeds when every set is completed with
// its target reps; a set past the plan has none. Without results nothing
// changes. lp keeps its streaks in the exercise state.
func (c *ctxValue) builtin(pr *model.Progression) error {
	if len(c.res.Sets) == 0 {
		return nil
	}
	all := model.SetAddr{Key: c.key}
	inc := edit{v: pr.Inc.Value, unit: pr.Inc.Unit, scale: 1}
	if pr.IncPct > 0 {
		inc = edit{scale: 1 + pr.IncPct/100}
	}
	ok, total := true, 0
	for i, r := range c.res.Sets {
		ok = ok && r.Completed && (i >= len(c.sets) || r.Reps >= c.sets[i].Reps)
		if r.Completed {
			total += r.Reps
		}
	}
	switch pr.Kind {
	case model.ProgressLP:
		wins, losses := c.counter("successes"), c.counter("failures")
		if ok {
			wins, losses = wins+1, 0
		} else {
			wins, losses = 0, losses+1
		}
		fail := edit{v: -pr.FailDec.Value, unit: pr.FailDec.Unit, scale: 1}
		if pr.FailReset > 0 {
			fail = edit{scale: pr.FailReset}
		}
		var err error
		switch {
		case wins >= pr.After:
			wins, err = 0, c.edit("weights", all, inc)
		case losses >= pr.FailAfter && (pr.FailDec.Value > 0 || pr.FailReset > 0):
			losses, err = 0, c.edit("weights", all, fail)
		}
		c.state.vars["successes"], c.state.vars["failures"] = starlark.MakeInt(wins), starlark.MakeInt(losses)
		return err
	case model.ProgressDP:
		if !ok || len(c.sets) == 0 {
			return nil
		}
		if c.sets[0].Reps < pr.Hi {
			return c.edit("reps", all, edit{v: 1, scale: 1})
		}
		if err := c.edit("reps", all, edit{abs: true, v: float64(pr.Lo)}); err != nil {
			return err
		}
		return c.edit("weights", all, inc)
	case model.ProgressSumReps:
		if total >= pr.Target {
			return c.edit("weights", all, inc)
		}
	}
	return nil
}

func (c *ctxValue) counter(name string) int {
	v, ok := c.state.vars[name]
	if !ok {
		return 0
	}
	n, _ := starlark.AsInt32(v)
	return n
}
