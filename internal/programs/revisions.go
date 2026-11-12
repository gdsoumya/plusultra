package programs

import (
	"cmp"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Update saves script as program id's next revision. Progression state stays
// for exercises (name and label) the new script still has. An active program
// re-resolves from today and keeps its start date or cursor.
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, script string) (sqlc.Program, error) {
	var p sqlc.Program
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		cur, err := lock(ctx, q, userID, id)
		if err != nil {
			return err
		}
		sc, err := checked(ctx, q, userID, cur.Kind, script, nil)
		if err != nil {
			return err
		}
		active := cur.Status == StatusActive
		if active && cur.StartDate != nil {
			if err := sc.checkAnchored(); err != nil {
				return err
			}
		}
		st := sc.structure()
		raw, err := json.Marshal(st)
		if err != nil {
			return err
		}
		p, err = q.UpdateProgramScript(ctx, sqlc.UpdateProgramScriptParams{Name: st.Name, Script: script, Structure: raw, ID: id, UserID: userID})
		if err != nil {
			return err
		}
		if err := insertRevision(ctx, q, p); err != nil {
			return err
		}
		if sc.workout != nil {
			if err := keepExercises(ctx, q, id, sc.workout); err != nil {
				return err
			}
		}
		if !active {
			return nil
		}
		if p.StartDate != nil {
			end := endDate(*p.StartDate, sc.weeks())
			if p, err = q.SetProgramSchedule(ctx, sqlc.SetProgramScheduleParams{Status: p.Status,
				StartDate: p.StartDate, EndDate: &end, ID: id, UserID: userID}); err != nil {
				return err
			}
		}
		from, err := s.today(ctx, q, userID)
		if err != nil {
			return err
		}
		_, err = s.resolveFrom(ctx, q, p, sc, from)
		return err
	})
	return p, err
}

// keepExercises drops the progression state of exercises p no longer has.
func keepExercises(ctx context.Context, q *sqlc.Queries, programID uuid.UUID, p *model.Program) error {
	o, err := LoadOverlay(ctx, q, programID)
	if err != nil {
		return err
	}
	retain(&o, p)
	return SaveOverlay(ctx, q, programID, o)
}

// retain drops from o the progression state of exercises p does not have and
// returns their keys, sorted. Shared tag state and 1RMs are not tied to one
// exercise and stay.
func retain(o *model.Overlay, p *model.Program) []model.ProgKey {
	keep := map[model.ProgKey]bool{}
	for _, d := range p.Days {
		for _, e := range d.Exercises {
			keep[model.ProgKey{Exercise: e.Names[0], Label: e.Label}] = true
		}
	}
	dropped := map[model.ProgKey]bool{}
	gone := func(k model.ProgKey) bool {
		g := !keep[k] && !engine.SharedKey(k)
		if g {
			dropped[k] = true
		}
		return g
	}
	set := func(a model.SetAddr) bool { return gone(a.Key) }
	day := func(a model.DayAddr) bool { return gone(a.Key) }
	drop(o.Weights, set)
	drop(o.Reps, set)
	drop(o.MinReps, set)
	drop(o.Rest, set)
	drop(o.Work, set)
	drop(o.RPE, set)
	drop(o.Amrap, set)
	drop(o.Ask, set)
	drop(o.LogRPE, set)
	drop(o.NumSets, day)
	drop(o.SetVariation, gone)
	drop(o.ExerciseVariation, gone)
	drop(o.DescIndex, gone)
	drop(o.State, gone)
	return slices.SortedFunc(maps.Keys(dropped), func(a, b model.ProgKey) int {
		return cmp.Or(strings.Compare(a.Exercise, b.Exercise), strings.Compare(a.Label, b.Label))
	})
}

func drop[K comparable, V any](m map[K]V, gone func(K) bool) {
	maps.DeleteFunc(m, func(k K, _ V) bool { return gone(k) })
}
