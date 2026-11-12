package logs

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// FinalizeResult is a finalized record and its progressions' print output.
type FinalizeResult struct {
	Record WorkoutRecord
	Output []string
}

// FinalizeWorkoutRecord ends a workout: it runs each exercise's progression
// once, updates the program's later days and cursor, and marks the day done.
// An ad hoc record is only marked. A second finalize returns ErrFinalized.
// The program state before the progressions is kept on the record (R79).
func (s *Service) FinalizeWorkoutRecord(ctx context.Context, userID, id uuid.UUID) (FinalizeResult, error) {
	var res FinalizeResult
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Finalizes apply in the order they arrive, so they queue per user like activations.
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		old, err := q.GetWorkoutRecord(ctx, sqlc.GetWorkoutRecordParams{ID: id, UserID: userID})
		if err != nil {
			return notFound(err)
		}
		if old.Finalized {
			return ErrFinalized
		}
		rec, err := DecodeRecord(old)
		if err != nil {
			return err
		}
		var before []byte
		if old.ProgramID != nil {
			p, err := q.GetProgramForUpdate(ctx, sqlc.GetProgramForUpdateParams{ID: *old.ProgramID, UserID: userID})
			switch {
			case errors.Is(err, pgx.ErrNoRows): // the program is gone; nothing to progress
			case err != nil:
				return err
			case p.Status == programs.StatusActive:
				if before, err = snapshot(ctx, q, p.ID); err != nil {
					return err
				}
				if res.Output, err = s.programs.Finalize(ctx, q, p, old.Date, rec.Day.Week, rec.Day.Day, planResults(rec)); err != nil {
					return err
				}
			}
		}
		finished := old.FinishedAt
		if finished == nil {
			now := s.Now()
			finished = &now
		}
		row, err := q.UpsertWorkoutRecord(ctx, sqlc.UpsertWorkoutRecordParams{ID: id, UserID: userID, Date: old.Date,
			ProgramID: old.ProgramID, ResolvedDay: old.ResolvedDay, Entries: old.Entries, StartedAt: old.StartedAt,
			FinishedAt: finished, Finalized: true, DeviceID: old.DeviceID})
		if err != nil {
			return err
		}
		if res.Record, err = DecodeRecord(row); err != nil {
			return err
		}
		if err := q.SetWorkoutRecordStateBefore(ctx, sqlc.SetWorkoutRecordStateBeforeParams{ID: id, UserID: userID, StateBefore: before}); err != nil {
			return err
		}
		return markDay(ctx, q, userID, old.Date, StatusDone)
	})
	return res, err
}

// Finalize is FinalizeWorkoutRecord for callers that only need the outcome,
// such as sync.
func (s *Service) Finalize(ctx context.Context, userID, recordID uuid.UUID) error {
	_, err := s.FinalizeWorkoutRecord(ctx, userID, recordID)
	return err
}
