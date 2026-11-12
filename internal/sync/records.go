package sync

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
)

// pushDayLog applies a pushed day log. A date's log keeps the id it was first written with, so
// a push with another id for a taken date, tombstone or not, gets the date's log back.
func (s *Service) pushDayLog(ctx context.Context, userID uuid.UUID, deviceID string, in logs.DayLog) (*logs.DayLog, string, error) {
	date, err := time.Parse(time.DateOnly, in.Date)
	if err != nil {
		return nil, reasonInvalid, nil
	}
	row, err := s.q.GetDayLogForSync(ctx, sqlc.GetDayLogForSyncParams{UserID: userID, Date: date})
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	var srv logs.DayLog
	if found {
		if srv, err = logs.DecodeDayLog(row); err != nil {
			return nil, "", err
		}
	}
	switch {
	case found && srv.ID != in.ID:
		return &srv, reasonDateTaken, nil
	case found && stale(in.Version, in.UpdatedAt, srv.Version, srv.UpdatedAt):
		return &srv, reasonOutdated, nil
	case in.DeletedAt != nil:
		_, err = s.q.DeleteDayLog(ctx, sqlc.DeleteDayLogParams{UserID: userID, Date: date}) // no row when never stored
		return nil, "", err
	}
	_, err = s.logs.UpdateDay(ctx, userID, logs.DayLogInput{
		ID: in.ID, Date: in.Date, Status: in.Status, Notes: in.Notes, Readiness: in.Readiness, DeviceID: deviceID,
	})
	return outcome(err, none[logs.DayLog])
}

// pushWorkoutRecord applies a pushed workout record through the workout service, which keeps a
// record's program and planned day as created and replays the progression of an edited
// finalized record (R79); a tombstone deletes one (R75). A new record of a program day the date
// already has a record of gets that record back, so devices that each started the planned
// workout share one (P2-R41). Finalized on a push of a record not yet finalized asks for a
// finalize after the write; its error comes back as the written row with finalize_failed.
func (s *Service) pushWorkoutRecord(ctx context.Context, userID uuid.UUID, deviceID string, in logs.WorkoutRecord) (*logs.WorkoutRecord, string, error) {
	row, err := s.q.GetWorkoutRecordForSync(ctx, sqlc.GetWorkoutRecordForSyncParams{ID: in.ID, UserID: userID})
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	var srv logs.WorkoutRecord
	if found {
		if srv, err = logs.DecodeRecord(row); err != nil {
			return nil, "", err
		}
	}
	switch {
	case found && stale(in.Version, in.UpdatedAt, srv.Version, srv.UpdatedAt):
		return &srv, reasonOutdated, nil
	case in.DeletedAt != nil:
		err := s.logs.DeleteWorkoutRecord(ctx, userID, in.ID)
		if errors.Is(err, logs.ErrNotFound) { // never stored, or deleted already
			err = nil
		}
		return outcome(err, none[logs.WorkoutRecord])
	case !found && in.ProgramID != nil:
		if other, err := s.plannedDayRecord(ctx, userID, in); other != nil || err != nil {
			return other, reasonDateTaken, err
		}
	}
	in.DeviceID = deviceID
	rec, err := s.logs.PutWorkoutRecord(ctx, userID, in)
	if err != nil || !in.Finalized || (found && srv.Finalized) {
		return outcome(err, none[logs.WorkoutRecord])
	}
	if err := s.fin.Finalize(ctx, userID, in.ID); err != nil {
		s.Logger.ErrorContext(ctx, "sync: finalize on push", "user_id", userID, "record_id", in.ID, "err", err)
		return &rec, reasonFinalizeFailed, nil
	}
	return nil, "", nil
}

// plannedDayRecord returns the live record on in's date of in's program day, or nil.
func (s *Service) plannedDayRecord(ctx context.Context, userID uuid.UUID, in logs.WorkoutRecord) (*logs.WorkoutRecord, error) {
	date, err := time.Parse(time.DateOnly, in.Date)
	if err != nil {
		return nil, nil // the write reports it
	}
	rows, err := s.q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: date, ToDate: date})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ProgramID == nil || *row.ProgramID != *in.ProgramID {
			continue
		}
		rec, err := logs.DecodeRecord(row)
		if err != nil {
			return nil, err
		}
		if rec.Day.Week == in.Day.Week && rec.Day.Day == in.Day.Day {
			return &rec, nil
		}
	}
	return nil, nil
}
