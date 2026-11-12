package logs

import (
	"context"
	"math"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// runHooks runs program p's update hook for each plan entry done on day: it
// sees the completed sets and changes only the plans of pending sets the user
// did not edit, which the entries carry between runs. Ad hoc entries and
// entries no longer on the day keep their plan.
func (s *Service) runHooks(ctx context.Context, q *sqlc.Queries, p sqlc.Program, day *model.ResolvedDay, entries []model.RecordEntry) error {
	prog, st, err := programs.Load(ctx, q, p)
	if err != nil {
		return err
	}
	plan := PlanEntries(day, entries)
	for i := range entries {
		en := &entries[i]
		res := result(day, *en)
		if !plan[i] || !programs.OnDay(prog, res) {
			continue
		}
		done := 0
		for _, r := range res.Sets {
			if r.Completed {
				done++
			}
		}
		out, err := engine.RunUpdate(prog, st, res, done)
		if err != nil {
			return invalid("%s update hook: %v", en.Name, err)
		}
		var work []int // indexes of the entry's working sets
		for j, set := range en.Sets {
			if !set.Warmup && !set.Planned.Warmup {
				work = append(work, j)
			}
		}
		for _, set := range out.Sets {
			switch {
			case set.Warmup:
			case len(work) > 0:
				if cur := &en.Sets[work[0]]; !cur.Completed && !cur.Edited {
					cur.Planned = model.PlanSet(set)
				}
				work = work[1:]
			default:
				en.Sets = append(en.Sets, model.RecordSet{Planned: model.PlanSet(set)})
			}
		}
		// Pending sets the hook dropped (number_of_sets lowered) go; completed and edited ones stay.
		gone := make(map[int]bool, len(work))
		for _, j := range work {
			gone[j] = !en.Sets[j].Completed && !en.Sets[j].Edited
		}
		k := 0
		for j := range en.Sets {
			if !gone[j] {
				en.Sets[k] = en.Sets[j]
				k++
			}
		}
		en.Sets = en.Sets[:k]
	}
	return nil
}

// result is entry en of day as the engine reads it: completed working sets
// as done, pending ones as currently planned.
func result(day *model.ResolvedDay, en model.RecordEntry) model.ExerciseResult {
	res := model.ExerciseResult{Key: progKey(en.Alternates, en.Name, en.Label), Week: day.Week, Day: day.Day, Variation: en.Variation}
	for _, s := range en.Sets {
		if s.Warmup || s.Planned.Warmup {
			continue
		}
		r := model.SetResult{Completed: s.Completed, WorkTime: s.WorkTime, RepsLeft: s.RepsLeft}
		if s.Completed {
			r.Reps, r.Weight, r.RPE = s.Reps, model.Weight(s.Weight), s.RPE
		} else {
			p := s.Planned
			r.Reps, r.MinReps, r.Weight, r.RPE, r.Rest = p.Reps, p.MinReps, model.Weight(p.Weight), p.RPE, p.Rest
		}
		res.Sets = append(res.Sets, r)
	}
	if len(en.PromptValues) > 0 {
		res.PromptValues = make(map[string]starlark.Value, len(en.PromptValues))
		for k, v := range en.PromptValues {
			if v == math.Trunc(v) {
				res.PromptValues[k] = starlark.MakeInt(int(v))
			} else {
				res.PromptValues[k] = starlark.Float(v)
			}
		}
	}
	return res
}
