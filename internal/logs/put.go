package logs

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// PutWorkoutRecord stores a record as a device holds it, for sync. A new
// record keeps its id, program and snapshot, the program's resolved day when
// the device sent only the header; an existing one takes the date,
// entries, times and device. No update hook runs, as offline. Finalized stays
// as stored and only Finalize sets it; a finalized record takes the entries
// and replays its progression (R79). A program that is not the user's is
// ErrInvalid.
func (s *Service) PutWorkoutRecord(ctx context.Context, userID uuid.UUID, rec WorkoutRecord) (WorkoutRecord, error) {
	date, err := parseDate(rec.Date)
	if err != nil {
		return WorkoutRecord{}, err
	}
	if err := checkEntries(rec.Entries); err != nil {
		return WorkoutRecord{}, err
	}
	if len(rec.Day.Exercises) > maxEntries {
		return WorkoutRecord{}, invalid("at most %d planned exercises", maxEntries)
	}
	var row sqlc.WorkoutRecord
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockUserPrograms(ctx, userID); err != nil { // see UpdateWorkoutRecord
			return err
		}
		if err := fillSlugs(ctx, q, userID, rec.Entries); err != nil {
			return err
		}
		params := sqlc.UpsertWorkoutRecordParams{ID: rec.ID, UserID: userID, Date: date, ProgramID: rec.ProgramID,
			StartedAt: rec.StartedAt, FinishedAt: rec.FinishedAt, DeviceID: rec.DeviceID}
		old, err := q.GetWorkoutRecord(ctx, sqlc.GetWorkoutRecordParams{ID: rec.ID, UserID: userID})
		switch {
		case err == nil:
			if old.Finalized {
				row, err = s.updateFinalized(ctx, q, old, rec.Entries, rec.StartedAt, rec.DeviceID)
				return err
			}
			params.ProgramID, params.ResolvedDay = old.ProgramID, old.ResolvedDay
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		default:
			if rec.ProgramID != nil {
				p, err := q.GetProgram(ctx, sqlc.GetProgramParams{ID: *rec.ProgramID, UserID: userID})
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrUnknownProgram
				} else if err != nil {
					return err
				}
				if len(rec.Day.Exercises) == 0 { // devices and imports send the day's header; the plan is the server's
					switch d, err := resolvedDay(ctx, q, p, rec.Day.Week, rec.Day.Day); {
					case err == nil && d != nil:
						rec.Day = *d
					case err != nil && !errors.Is(err, pgx.ErrNoRows):
						return err
					}
				}
			}
			if params.ResolvedDay, err = json.Marshal(rec.Day); err != nil {
				return err
			}
		}
		if params.Entries, err = json.Marshal(rec.Entries); err != nil {
			return err
		}
		row, err = q.UpsertWorkoutRecord(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) { // the id belongs to another user
			return ErrConflict
		}
		return err
	})
	if err != nil {
		return WorkoutRecord{}, err
	}
	return DecodeRecord(row)
}
