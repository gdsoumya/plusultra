package sync

import (
	"context"
	"errors"
	"fmt"
	gosync "sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
)

// Reasons for rows that were not applied (R54).
const (
	reasonOutdated       = "outdated"        // the device had not seen the server's later change
	reasonDateTaken      = "date_taken"      // another row holds the date
	reasonFinalized      = "finalized"       // a finalize raced a REST finalize of the same record
	reasonFinalizeFailed = "finalize_failed" // the row was written but its finalize failed
	reasonInvalid        = "invalid"         // the row breaks a rule, or its id is another user's
	reasonUnknownProgram = "unknown_program"
)

// PushResult says what became of each pushed row: applied, or not applied with a reason, by
// pushed row id. Conflicts carry the server's row when there is one to show.
type PushResult struct {
	Applied   []uuid.UUID
	Conflicts Rows
	Reasons   map[uuid.UUID]string
}

// Push applies rows changed on deviceID. Each row is last-write-wins by UpdatedAt, except that
// a row whose Version is behind the server's and whose UpdatedAt is older than the server's
// comes back as a conflict. A day log, meal log or check-in on a date another row holds comes
// back as a conflict carrying that row. Rows are applied one at a time, so an error leaves the
// rows before it applied; pushing them again reports them as outdated.
//
// A user's pushes run one at a time, so a row's check and write never interleave with another
// push; the lock is in process because the server runs as a single replica.
func (s *Service) Push(ctx context.Context, userID uuid.UUID, deviceID string, rows Rows) (PushResult, error) {
	for _, c := range rows.Checkins {
		if len(c.Photo) > 0 {
			return PushResult{}, fmt.Errorf("%w: check-in photos are not synced; use the check-in API", ErrInvalid)
		}
	}
	lock, _ := s.pushing.LoadOrStore(userID, new(gosync.Mutex))
	lock.(*gosync.Mutex).Lock()
	defer lock.(*gosync.Mutex).Unlock()
	res := PushResult{Reasons: map[uuid.UUID]string{}}
	for _, r := range rows.DayLogs {
		srv, reason, err := s.pushDayLog(ctx, userID, deviceID, r)
		if err := settle(&res, r.ID, srv, reason, err, &res.Conflicts.DayLogs); err != nil {
			return res, err
		}
	}
	for _, r := range rows.WorkoutRecords {
		srv, reason, err := s.pushWorkoutRecord(ctx, userID, deviceID, r)
		if err := settle(&res, r.ID, srv, reason, err, &res.Conflicts.WorkoutRecords); err != nil {
			return res, err
		}
	}
	for _, r := range rows.MealLogs {
		srv, reason, err := s.pushMealLog(ctx, userID, deviceID, r)
		if err := settle(&res, r.ID, srv, reason, err, &res.Conflicts.MealLogs); err != nil {
			return res, err
		}
	}
	for _, r := range rows.Checkins {
		srv, reason, err := s.pushCheckin(ctx, userID, deviceID, r)
		if err := settle(&res, r.ID, srv, reason, err, &res.Conflicts.Checkins); err != nil {
			return res, err
		}
	}
	return res, nil
}

// settle records the outcome of pushing row id: applied unless it has a reason.
func settle[T any](res *PushResult, id uuid.UUID, srv *T, reason string, err error, conflicts *[]T) error {
	switch {
	case err != nil:
		return err
	case reason == "":
		res.Applied = append(res.Applied, id)
	case srv != nil:
		*conflicts = append(*conflicts, *srv)
		fallthrough
	default:
		res.Reasons[id] = reason
	}
	return nil
}

// stale reports whether a pushed row loses to the server's: the device had not seen the
// server's last change, and that change is the later one.
func stale(version int32, at time.Time, srvVersion int32, srvAt time.Time) bool {
	return version < srvVersion && srvAt.After(at)
}

func (s *Service) pushMealLog(ctx context.Context, userID uuid.UUID, deviceID string, in logs.MealLog) (*logs.MealLog, string, error) {
	srv, err := s.q.GetMealLogForSync(ctx, sqlc.GetMealLogForSyncParams{ID: in.ID, UserID: userID})
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	if found && stale(in.Version, in.UpdatedAt, srv.Version, srv.UpdatedAt) {
		l, err := logs.DecodeMealLog(srv)
		return &l, reasonOutdated, err
	}
	live := found && srv.DeletedAt == nil
	if in.DeletedAt != nil {
		if !live {
			return nil, "", nil
		}
		if err := s.meals.Delete(ctx, userID, in.ID); !errors.Is(err, logs.ErrNotFound) {
			return nil, "", err
		}
		return nil, "", nil // deleted meanwhile
	}
	li := logs.MealLogInput{ID: in.ID, Date: in.Date, ProgramID: in.ProgramID, Target: &in.Target, Meals: in.Meals, DeviceID: deviceID}
	if live {
		_, err = s.meals.Update(ctx, userID, in.ID, li)
	} else {
		_, err = s.meals.Create(ctx, userID, li)
	}
	return outcome(err, func() (*logs.MealLog, error) {
		row, err := s.q.GetMealLogByDate(ctx, sqlc.GetMealLogByDateParams{UserID: userID, Date: mustDate(in.Date)})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		l, err := logs.DecodeMealLog(row)
		return &l, err
	})
}

func (s *Service) pushCheckin(ctx context.Context, userID uuid.UUID, deviceID string, in logs.Checkin) (*logs.Checkin, string, error) {
	srv, err := s.q.GetCheckinForSync(ctx, sqlc.GetCheckinForSyncParams{ID: in.ID, UserID: userID})
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	if found && stale(in.Version, in.UpdatedAt, srv.Version, srv.UpdatedAt) {
		c, err := logs.DecodeCheckin(srv)
		return &c, reasonOutdated, err
	}
	live := found && srv.DeletedAt == nil
	if in.DeletedAt != nil {
		if !live {
			return nil, "", nil
		}
		if err := s.checkins.Delete(ctx, userID, in.ID); !errors.Is(err, logs.ErrNotFound) {
			return nil, "", err
		}
		return nil, "", nil // deleted meanwhile
	}
	on := func() (*logs.Checkin, error) {
		row, err := s.q.GetCheckinByDate(ctx, sqlc.GetCheckinByDateParams{UserID: userID, Date: mustDate(in.Date)})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		c, err := logs.DecodeCheckin(row)
		return &c, err
	}
	// The pushed photo id is ignored: Update keeps the stored photo and Create starts without one.
	ci := logs.CheckinInput{ID: in.ID, Date: in.Date, WeightKg: in.WeightKg, BodyFat: in.BodyFat, Measurements: in.Measurements, DeviceID: deviceID}
	if live {
		_, err = s.checkins.Update(ctx, userID, in.ID, ci)
		return outcome(err, on)
	}
	// Create would take over the check-in already on the date, so a taken date is a conflict here.
	if _, perr := time.Parse(time.DateOnly, in.Date); perr == nil {
		if other, err := on(); other != nil || err != nil {
			return other, reasonDateTaken, err
		}
	}
	_, err = s.checkins.Create(ctx, userID, ci)
	return outcome(err, on)
}

// outcome turns a logs service error into a push outcome; on returns the row holding the
// pushed date, or nil when there is none.
func outcome[T any](err error, on func() (*T, error)) (*T, string, error) {
	switch {
	case errors.Is(err, logs.ErrConflict):
		other, err := on()
		if other == nil && err == nil {
			return nil, reasonInvalid, nil // no row holds the date, so the id is another user's
		}
		return other, reasonDateTaken, err
	case errors.Is(err, logs.ErrNotFound): // deleted by a REST call between the read and the write
		return nil, reasonOutdated, nil
	case errors.Is(err, logs.ErrFinalized): // finalized by a REST call between the read and the write
		return nil, reasonFinalized, nil
	case errors.Is(err, logs.ErrUnknownProgram):
		return nil, reasonUnknownProgram, nil
	case errors.Is(err, logs.ErrInvalid):
		return nil, reasonInvalid, nil
	}
	return nil, "", err
}

// none is outcome's on for rows that no other row can hold the date of.
func none[T any]() (*T, error) { return nil, nil }

// mustDate parses a date the logs services have already accepted.
func mustDate(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}
