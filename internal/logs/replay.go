package logs

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// planResults is the results of rec's plan exercises; ad hoc and removed exercises get no progression.
func planResults(rec WorkoutRecord) []model.ExerciseResult {
	var results []model.ExerciseResult
	for i, plan := range PlanEntries(&rec.Day, rec.Entries) {
		if plan {
			results = append(results, result(&rec.Day, rec.Entries[i]))
		}
	}
	return results
}

// snapshot is program programID's progression state as stored.
func snapshot(ctx context.Context, q *sqlc.Queries, programID uuid.UUID) ([]byte, error) {
	o, err := programs.LoadOverlay(ctx, q, programID)
	if err != nil {
		return nil, err
	}
	return engine.MarshalOverlay(o)
}

// updateFinalized stores entries, started time and device on finalized record old; its date,
// finish time, program and day stay. When the record's program is active, its progression and
// every later finalized record's replay with the edited results, so the program's next sessions
// follow the edit (R79). The cursor stays: the sessions were done.
func (s *Service) updateFinalized(ctx context.Context, q *sqlc.Queries, old sqlc.WorkoutRecord, entries []model.RecordEntry, startedAt *time.Time, deviceID string) (sqlc.WorkoutRecord, error) {
	raw, err := json.Marshal(entries)
	if err != nil {
		return sqlc.WorkoutRecord{}, err
	}
	row, err := q.UpsertWorkoutRecord(ctx, sqlc.UpsertWorkoutRecordParams{ID: old.ID, UserID: old.UserID, Date: old.Date,
		ProgramID: old.ProgramID, ResolvedDay: old.ResolvedDay, Entries: raw, StartedAt: startedAt,
		FinishedAt: old.FinishedAt, Finalized: true, DeviceID: deviceID})
	if err != nil {
		return row, err
	}
	if old.ProgramID == nil { // ad hoc
		return row, nil
	}
	p, err := q.GetProgramForUpdate(ctx, sqlc.GetProgramForUpdateParams{ID: *old.ProgramID, UserID: old.UserID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && p.Status != programs.StatusActive) {
		return row, nil
	}
	if err != nil {
		return row, err
	}
	return row, s.replay(ctx, q, p, row)
}

// replay re-applies the progressions of from and the finalized records of p after it, in
// (date, week, day) order, starting from the state stored before from's. A record finalized
// before snapshots existed has none, so the whole program replays from its activation baseline,
// an empty overlay (R81), and every record gets its snapshot on the way.
func (s *Service) replay(ctx context.Context, q *sqlc.Queries, p sqlc.Program, from sqlc.WorkoutRecord) error {
	rows, err := q.ListFinalizedProgramRecords(ctx, sqlc.ListFinalizedProgramRecordsParams{UserID: p.UserID, ProgramID: &p.ID})
	if err != nil {
		return err
	}
	type item struct {
		row sqlc.WorkoutRecord
		rec WorkoutRecord
	}
	items := make([]item, len(rows))
	for i, r := range rows {
		if items[i].rec, err = DecodeRecord(r); err != nil {
			return err
		}
		items[i].row = r
	}
	slices.SortStableFunc(items, func(a, b item) int {
		if c := a.row.Date.Compare(b.row.Date); c != 0 {
			return c
		}
		if c := a.rec.Day.Week - b.rec.Day.Week; c != 0 {
			return c
		}
		return a.rec.Day.Day - b.rec.Day.Day
	})
	start := slices.IndexFunc(items, func(it item) bool { return it.row.ID == from.ID })
	if start < 0 {
		return nil
	}
	var o model.Overlay
	if from.StateBefore == nil {
		start = 0
	} else if o, err = engine.UnmarshalOverlay(from.StateBefore); err != nil {
		return err
	}
	if err := programs.SaveOverlay(ctx, q, p.ID, o); err != nil {
		return err
	}
	for _, it := range items[start:] {
		before, err := snapshot(ctx, q, p.ID)
		if err != nil {
			return err
		}
		if _, _, _, err := s.programs.Progress(ctx, q, p, it.row.Date, planResults(it.rec)); err != nil {
			return err
		}
		if err := q.SetWorkoutRecordStateBefore(ctx, sqlc.SetWorkoutRecordStateBeforeParams{ID: it.row.ID, UserID: p.UserID, StateBefore: before}); err != nil {
			return err
		}
	}
	return nil
}
