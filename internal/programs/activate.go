package programs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// Activate starts program id. With a start date it is anchored: day numbers
// are weekdays, week 1 is the Monday-based week holding start, and it ends on
// the Sunday of its last week. Without one it is floating, a sequence of days;
// a floating workout program's cursor starts at its first day. A user runs one
// program of each kind at a time.
func (s *Service) Activate(ctx context.Context, userID, id uuid.UUID, start *time.Time) (sqlc.Program, error) {
	var p sqlc.Program
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		cur, err := lock(ctx, q, userID, id)
		if err != nil {
			return err
		}
		if cur.Status == StatusActive {
			return failedPrecondition(errors.New("program is already active"))
		}
		other, err := activeProgram(ctx, q, userID, cur.Kind)
		if err != nil {
			return err
		}
		if other != nil {
			return failedPrecondition(fmt.Errorf("%s program %q is active; finish it first", cur.Kind, other.Name))
		}
		sc, err := load(cur)
		if err != nil {
			return err
		}
		params := sqlc.SetProgramScheduleParams{Status: StatusActive, ID: id, UserID: userID}
		if start != nil {
			if err := sc.checkAnchored(); err != nil {
				return err
			}
			d := dateOf(*start)
			end := endDate(d, sc.weeks())
			params.StartDate, params.EndDate = &d, &end
		}
		if p, err = q.SetProgramSchedule(ctx, params); err != nil {
			return err
		}
		days, err := s.resolveFrom(ctx, q, p, sc, time.Time{})
		if err == nil && start == nil && sc.workout != nil && len(days) > 0 {
			_, err = q.UpsertCursor(ctx, sqlc.UpsertCursorParams{ProgramID: p.ID, NextWeek: int32(days[0].Week), NextDay: int32(days[0].Day)}) //nolint:gosec // weeks and days are small
		}
		return err
	})
	return p, err
}

// Finish marks an active program finished.
func (s *Service) Finish(ctx context.Context, userID, id uuid.UUID) (sqlc.Program, error) {
	var p sqlc.Program
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		cur, err := lock(ctx, q, userID, id)
		if err != nil {
			return err
		}
		if cur.Status != StatusActive {
			return failedPrecondition(errors.New("program is not active"))
		}
		if p, err = q.SetProgramStatus(ctx, sqlc.SetProgramStatusParams{Status: StatusFinished, ID: id, UserID: userID}); err != nil {
			return err
		}
		if p.Kind != KindWorkout {
			return nil
		}
		from, err := s.today(ctx, q, userID)
		if err != nil {
			return err
		}
		return s.resolveMeals(ctx, q, userID, from)
	})
	return p, err
}

// schedule lists the dated days of an anchored program of weeks weeks from
// start. Dates before start are not scheduled.
func schedule(start time.Time, weeks int) []Day {
	first := monday(start)
	var days []Day
	for i := range weeks * 7 {
		d := first.AddDate(0, 0, i)
		if !d.Before(start) {
			days = append(days, Day{Week: i/7 + 1, Day: i%7 + 1, Date: &d})
		}
	}
	return days
}

func monday(d time.Time) time.Time { return d.AddDate(0, 0, -(int(d.Weekday())+6)%7) }

// endDate is the Sunday of the last week of an anchored program of weeks weeks from start.
func endDate(start time.Time, weeks int) time.Time { return monday(start).AddDate(0, 0, 7*weeks-1) }

// dateOf is t's calendar date as midnight UTC, the form dates take in the database.
func dateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// today is the date in userID's time zone.
func (s *Service) today(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (time.Time, error) {
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return dateOf(s.Now().In(loc)), nil
}
