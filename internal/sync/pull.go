package sync

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

const (
	maxLimit = 500
	planDays = 28
)

// Changes is one page of a pull.
type Changes struct {
	Rows
	Programs []sqlc.Program // the active programs
	// Cursors are the next days of the active floating programs that have one.
	Cursors map[uuid.UUID]sqlc.ProgramCursor
	// ResolvedDays and ResolvedMealDays are the active workout and meal programs' next planDays
	// days: dated from today on when anchored, from the cursor on when floating, and for a
	// floating meal program from the workout's week on.
	ResolvedDays, ResolvedMealDays []PlanDay
	Cursor                         string // to send on the next pull, once the rows are applied
	HasMore                        bool
}

// PlanDay is a resolved day of an active program.
type PlanDay struct {
	ProgramID uuid.UUID
	Day       programs.Day
}

// change is a pulled row of any table, at its place in the change order.
type change struct {
	at  time.Time
	id  uuid.UUID
	add func(*Rows)
}

// Pull returns at most limit rows changed after cursor (limit 0 means 500), oldest first, with
// the active programs and their next days from the user's local date at now.
//
// The tables share one cursor, so each is read up to limit+1 rows, the union is sorted by
// (updated_at, id) and cut at limit, and the cursor is the last row kept. Every row left out
// sorts after it and comes in a later pull; cutting each table on its own would skip rows.
//
// A row is stamped before its transaction commits, so the changed-since queries leave out rows
// stamped in the last 5 seconds: a later-stamped row that commits first would otherwise move the
// cursor past one still uncommitted. The lag must exceed the longest transaction that stamps
// updated_at, and the four reads share one snapshot and one now(). The longest is a finalize,
// which re-resolves the program's later days; TestFinalizeFitsPullLag times one.
func (s *Service) Pull(ctx context.Context, userID uuid.UUID, cursor string, limit int, now time.Time) (Changes, error) {
	if limit < 0 || limit > maxLimit {
		return Changes{}, fmt.Errorf("%w: limit must be 0-%d", ErrInvalid, maxLimit)
	}
	if limit == 0 {
		limit = maxLimit
	}
	since, sinceID, err := decodeCursor(cursor)
	if err != nil {
		return Changes{}, err
	}
	p := sqlc.ListDaysChangedSinceParams{
		UserID: userID, Since: since, SinceID: sinceID, MaxRows: int32(limit + 1), //nolint:gosec // limit <= maxLimit
	}
	var changed []change
	err = pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		changed, err = changedSince(ctx, s.q.WithTx(tx), p)
		return err
	})
	if err != nil {
		return Changes{}, err
	}
	slices.SortFunc(changed, func(a, b change) int {
		return cmp.Or(a.at.Compare(b.at), bytes.Compare(a.id[:], b.id[:])) // uuid order as in Postgres
	})
	out := Changes{Cursor: cursor, HasMore: len(changed) > limit}
	changed = changed[:min(len(changed), limit)]
	for _, c := range changed {
		c.add(&out.Rows)
	}
	if n := len(changed); n > 0 {
		out.Cursor = encodeCursor(changed[n-1].at, changed[n-1].id)
	}
	return out, s.plan(ctx, userID, now, &out)
}

// changedSince reads up to p.MaxRows changed rows of each table.
func changedSince(ctx context.Context, q *sqlc.Queries, p sqlc.ListDaysChangedSinceParams) ([]change, error) {
	var out []change
	days, err := q.ListDaysChangedSince(ctx, p)
	if err != nil {
		return nil, err
	}
	for _, r := range days {
		l, err := logs.DecodeDayLog(r)
		if err != nil {
			return nil, err
		}
		out = append(out, change{r.UpdatedAt, r.ID, func(rs *Rows) { rs.DayLogs = append(rs.DayLogs, l) }})
	}
	recs, err := q.ListWorkoutRecordsChangedSince(ctx, sqlc.ListWorkoutRecordsChangedSinceParams(p))
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		w, err := logs.DecodeRecord(r)
		if err != nil {
			return nil, err
		}
		out = append(out, change{r.UpdatedAt, r.ID, func(rs *Rows) { rs.WorkoutRecords = append(rs.WorkoutRecords, w) }})
	}
	meals, err := q.ListMealLogsChangedSince(ctx, sqlc.ListMealLogsChangedSinceParams(p))
	if err != nil {
		return nil, err
	}
	for _, r := range meals {
		l, err := logs.DecodeMealLog(r)
		if err != nil {
			return nil, err
		}
		out = append(out, change{r.UpdatedAt, r.ID, func(rs *Rows) { rs.MealLogs = append(rs.MealLogs, l) }})
	}
	checkins, err := q.ListCheckinsChangedSince(ctx, sqlc.ListCheckinsChangedSinceParams(p))
	if err != nil {
		return nil, err
	}
	for _, r := range checkins {
		c, err := logs.DecodeCheckin(r)
		if err != nil {
			return nil, err
		}
		out = append(out, change{r.UpdatedAt, r.ID, func(rs *Rows) { rs.Checkins = append(rs.Checkins, c) }})
	}
	return out, nil
}

// plan fills in the active programs, their cursors and their next days.
func (s *Service) plan(ctx context.Context, userID uuid.UUID, now time.Time, out *Changes) error {
	today, err := s.localDate(ctx, userID, now)
	if err != nil {
		return err
	}
	if out.Programs, err = s.q.ListProgramsByUserAndStatus(ctx, sqlc.ListProgramsByUserAndStatusParams{UserID: userID, Status: programs.StatusActive}); err != nil {
		return err
	}
	out.Cursors = map[uuid.UUID]sqlc.ProgramCursor{}
	last := today.AddDate(0, 0, planDays-1)
	var floatingMeals []sqlc.Program
	for _, p := range out.Programs {
		var rows []sqlc.ResolvedDay
		switch {
		case p.StartDate != nil:
			rows, err = s.q.ListResolvedDaysByDateRange(ctx, sqlc.ListResolvedDaysByDateRangeParams{ProgramID: p.ID, FromDate: &today, ToDate: &last})
		case p.Kind == programs.KindMeal: // after the workout days, whose week it follows
			floatingMeals = append(floatingMeals, p)
			continue
		default:
			rows, err = s.floatingDays(ctx, p, out.Cursors)
		}
		if err != nil {
			return err
		}
		for _, r := range rows {
			d, err := programs.DayFrom(p.Kind, r)
			if err != nil {
				return err
			}
			out.add(p, d)
		}
	}
	week := 1 // the workout's current week: its cursor's when floating, today's when anchored
	if len(out.ResolvedDays) > 0 {
		week = out.ResolvedDays[0].Day.Week
	}
	for _, p := range floatingMeals {
		days, err := s.floatingMealDays(ctx, userID, p, week)
		if err != nil {
			return err
		}
		for _, d := range days {
			out.add(p, d)
		}
	}
	return nil
}

func (c *Changes) add(p sqlc.Program, d programs.Day) {
	if p.Kind == programs.KindMeal {
		c.ResolvedMealDays = append(c.ResolvedMealDays, PlanDay{ProgramID: p.ID, Day: d})
	} else {
		c.ResolvedDays = append(c.ResolvedDays, PlanDay{ProgramID: p.ID, Day: d})
	}
}

// floatingMealDays returns floating meal program p's days from week on, then the weeks before
// it, planDays at most. A floating meal day is picked by the workout's week and kind, and the
// weeks before follow when the workout loops.
func (s *Service) floatingMealDays(ctx context.Context, userID uuid.UUID, p sqlc.Program, week int) ([]programs.Day, error) {
	days, err := s.programs.ResolvedDays(ctx, userID, p.ID, programs.Range{})
	if err != nil {
		return nil, err
	}
	var later, earlier []programs.Day
	for _, d := range days {
		if d.Week >= week {
			later = append(later, d)
		} else {
			earlier = append(earlier, d)
		}
	}
	later = append(later, earlier...)
	return later[:min(len(later), planDays)], nil
}

// floatingDays returns floating program p's next planDays days from its cursor (its first day
// when it has none), wrapping to week 1 when it loops, and records the cursor in cursors.
func (s *Service) floatingDays(ctx context.Context, p sqlc.Program, cursors map[uuid.UUID]sqlc.ProgramCursor) ([]sqlc.ResolvedDay, error) {
	c, err := s.q.GetCursor(ctx, p.ID)
	switch {
	case err == nil:
		cursors[p.ID] = c
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	var st struct{ Loop bool }
	if err := json.Unmarshal(p.Structure, &st); err != nil {
		return nil, err
	}
	return s.q.ListResolvedDaysFromCursor(ctx, sqlc.ListResolvedDaysFromCursorParams{
		ProgramID: p.ID, Wrap: st.Loop, NextWeek: c.NextWeek, NextDay: c.NextDay, MaxRows: planDays,
	})
}

// localDate returns the user's calendar date at now, at UTC midnight like the date columns.
func (s *Service) localDate(ctx context.Context, userID uuid.UUID, now time.Time) (time.Time, error) {
	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
}
