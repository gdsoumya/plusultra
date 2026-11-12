package export

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// recordImporter stores imported workouts as records through the logs service.
type recordImporter struct {
	q    *sqlc.Queries
	logs *logs.Service
}

// ImportWorkout stores w as a record of date built from the parsed sets, not
// from the program's plan, so it holds what the text says was done. The
// workout starts at midnight and lasts w.Minutes. One completed warmup set
// stands for "warmup done", which is all the text keeps of the warmups. A
// record the date already has for the program is rewritten, so a log imports
// again without doubling; a finalized one is left alone with ErrFinalized.
func (r recordImporter) ImportWorkout(ctx context.Context, userID uuid.UUID, date string, w Workout, finalize bool) error {
	rec := logs.WorkoutRecord{ID: uuid.New(), Date: date, DeviceID: "import",
		Day: model.ResolvedDay{Week: w.Week, Day: w.Day, Name: w.DayName, Kind: cmp.Or(w.Kind, "training")}}
	if w.Program != "" {
		id, err := r.programID(ctx, userID, w.Program)
		if err != nil {
			return err
		}
		rec.ProgramID = &id
	}
	if old, ok, err := r.existing(ctx, userID, date, rec.ProgramID); err != nil {
		return err
	} else if ok {
		if old.Finalized {
			return logs.ErrFinalized
		}
		rec.ID = old.ID
	}
	if start, err := time.Parse(time.DateOnly, date); err == nil && w.Minutes > 0 {
		end := start.Add(time.Duration(w.Minutes) * time.Minute)
		rec.StartedAt, rec.FinishedAt = &start, &end
	}
	for _, e := range w.Entries {
		en := e.RecordEntry
		if e.WarmupDone {
			en.Sets = append([]model.RecordSet{{Planned: model.RecordPlanned{Warmup: true}, Completed: true, Warmup: true}}, en.Sets...)
		}
		rec.Entries = append(rec.Entries, en)
	}
	if _, err := r.logs.PutWorkoutRecord(ctx, userID, rec); err != nil {
		return err
	}
	if !finalize {
		return nil
	}
	return r.logs.Finalize(ctx, userID, rec.ID)
}

// existing is the date's record of the program (nil for an ad hoc workout),
// else its first record.
func (r recordImporter) existing(ctx context.Context, userID uuid.UUID, date string, programID *uuid.UUID) (sqlc.WorkoutRecord, bool, error) {
	day, _ := time.Parse(time.DateOnly, date)
	rows, err := r.q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: day, ToDate: day})
	if err != nil || len(rows) == 0 {
		return sqlc.WorkoutRecord{}, false, err
	}
	for _, row := range rows {
		if programID == nil && row.ProgramID == nil || programID != nil && row.ProgramID != nil && *row.ProgramID == *programID {
			return row, true, nil
		}
	}
	return rows[0], true, nil
}

// programID is the user's workout program called name: the active one when
// several share the name, else the newest, drafts included.
func (r recordImporter) programID(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	ps, err := r.q.ListProgramsByUser(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	var found *sqlc.Program
	for i := range ps {
		p := &ps[i]
		if p.Kind != programs.KindWorkout || p.Name != name {
			continue
		}
		if found == nil || p.Status == programs.StatusActive || found.Status != programs.StatusActive && p.CreatedAt.After(found.CreatedAt) {
			found = p
		}
	}
	if found == nil {
		return uuid.Nil, fmt.Errorf("%w: unknown program %q", logs.ErrInvalid, name)
	}
	return found.ID, nil
}
