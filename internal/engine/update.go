package engine

import (
	"cmp"
	"fmt"
	"slices"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

// RunUpdate runs the update hook of the exercise res reports once its first
// setIndex working sets are done. It returns the exercise for the rest of the
// workout: the plan sized to res.Sets and seeded with the weights and pending
// targets res carries, with the hook's changes to the sets after setIndex.
// The overlay is not changed. Script errors are *EvalError.
func RunUpdate(p *model.Program, st *model.UserState, res model.ExerciseResult, setIndex int) (*model.ResolvedExercise, error) {
	c, err := newCtx(p, st, res, true, setIndex)
	if err != nil {
		return nil, err
	}
	if h := c.e.Update; h != nil {
		if _, err := c.run(h.Fn); err != nil {
			return nil, err
		}
	}
	for i := setIndex; i < len(c.sets); i++ {
		if i >= len(c.orig) || c.sets[i].Weight != c.orig[i].Weight {
			c.sets[i].Weight = RoundWeight(c.sets[i].Weight, c.rounding)
		}
	}
	out := c.planned
	out.Sets = slices.Concat(c.warmups, c.sets)
	return &out, nil
}

// setRange is ctx.sets(frm, to, min_reps, max_reps, amrap, weight, rest, rpe,
// log_rpe): it sets sets frm to to, adding sets up to to as copies of the last.
func (c *ctxValue) setRange(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		from, to, minReps, maxReps, rest int
		amrap, logRPE                    bool
		weight, rpe                      starlark.Value
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "frm", &from, "to", &to, "min_reps", &minReps,
		"max_reps", &maxReps, "amrap", &amrap, "weight", &weight, "rest", &rest, "rpe", &rpe, "log_rpe", &logRPE); err != nil {
		return nil, err
	}
	if from < 1 || from > to || to > maxListLen {
		return nil, fmt.Errorf("%s: want 1 <= frm <= to <= %d", b.Name(), maxListLen)
	}
	if len(c.sets) == 0 {
		c.sets = []model.ResolvedSet{{Weight: model.Weight{Unit: cmp.Or(c.st.Units, model.UnitKg)}}}
	}
	c.sets = resize(c.sets, max(to, len(c.sets)))
	for _, f := range []struct {
		name string
		v    starlark.Value
	}{ // reps before min_reps: setting reps can lower min_reps
		{"reps", starlark.MakeInt(maxReps)}, {"min_reps", starlark.MakeInt(minReps)}, {"amrap", starlark.Bool(amrap)},
		{"weights", weight}, {"rest", starlark.MakeInt(rest)}, {"rpe", rpe}, {"log_rpe", starlark.Bool(logRPE)},
	} {
		for i := from; i <= to; i++ {
			if err := c.setSet(f.name, model.SetAddr{Key: c.key}, i, f.v); err != nil {
				return nil, err
			}
		}
	}
	return starlark.None, nil
}
