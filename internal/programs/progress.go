package programs

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Load evaluates workout program p with its user's state and progression
// overlay, ready for the engine.
func Load(ctx context.Context, q *sqlc.Queries, p sqlc.Program) (*model.Program, *model.UserState, error) {
	sc, err := evaluate(p.Kind, p.Script, nil)
	if err != nil {
		return nil, nil, err
	}
	if sc.workout == nil {
		return nil, nil, errors.New("not a workout program")
	}
	st, _, err := userState(ctx, q, p.UserID, sc.workout)
	if err != nil {
		return nil, nil, err
	}
	st.Overlay, err = LoadOverlay(ctx, q, p.ID)
	return sc.workout, st, err
}

// OnDay reports whether the exercise res reports is on its day of p.
func OnDay(p *model.Program, res model.ExerciseResult) bool {
	d := p.DayAt(res.Week, res.Day)
	return d != nil && slices.ContainsFunc(d.Exercises, func(e model.Exercise) bool {
		return model.ProgKey{Exercise: e.Names[0], Label: e.Label} == res.Key
	})
}

// Finalize applies a workout done on date at (week, day) of active program p:
// Progress, then a floating program's cursor moves to the next day, wrapping
// after the last unless the program does not loop, which finishes it.
func (s *Service) Finalize(ctx context.Context, q *sqlc.Queries, p sqlc.Program, date time.Time, week, day int, results []model.ExerciseResult) ([]string, error) {
	prog, days, out, err := s.Progress(ctx, q, p, date, results)
	if err != nil || p.StartDate != nil {
		return out, err
	}
	return out, advance(ctx, q, p, prog.Loop, days, week, day)
}

// Progress runs each result's progression in order, so shared state carries
// over, saves the overlay and re-resolves the days after the workout done on
// date: the dates after it when anchored and every day when floating.
// Exercises no longer on their day are skipped. It returns the program, its
// days and the scripts' print output; script errors are InvalidArgument.
func (s *Service) Progress(ctx context.Context, q *sqlc.Queries, p sqlc.Program, date time.Time, results []model.ExerciseResult) (*model.Program, []Day, []string, error) {
	prog, st, err := Load(ctx, q, p)
	if err != nil {
		return nil, nil, nil, err
	}
	var out []string
	for _, r := range results {
		if !OnDay(prog, r) {
			continue
		}
		delta, prints, err := engine.RunProgress(prog, st, r)
		out = append(out, prints...)
		if err != nil {
			return nil, nil, out, invalidArgument(err)
		}
		st.Overlay.Merge(delta)
	}
	if err := SaveOverlay(ctx, q, p.ID, st.Overlay); err != nil {
		return nil, nil, out, err
	}
	var from time.Time
	if p.StartDate != nil {
		from = date.AddDate(0, 0, 1)
	}
	days, err := s.resolveFrom(ctx, q, p, script{workout: prog}, from)
	return prog, days, out, err
}

func advance(ctx context.Context, q *sqlc.Queries, p sqlc.Program, loop bool, days []Day, week, day int) error {
	i := slices.IndexFunc(days, func(d Day) bool { return d.Week == week && d.Day == day })
	if i < 0 { // not a day of the program any more; the cursor stays
		return nil
	}
	next := i + 1
	if next == len(days) {
		if !loop {
			_, err := q.SetProgramStatus(ctx, sqlc.SetProgramStatusParams{Status: StatusFinished, ID: p.ID, UserID: p.UserID})
			return err
		}
		next = 0
	}
	_, err := q.UpsertCursor(ctx, sqlc.UpsertCursorParams{ProgramID: p.ID, NextWeek: int32(days[next].Week), NextDay: int32(days[next].Day)}) //nolint:gosec // weeks and days are small
	return err
}
