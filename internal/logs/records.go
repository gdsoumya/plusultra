package logs

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

const maxEntries = 100

// WorkoutRecord is one workout on one date. Day is the snapshot of the planned
// day the record started from; Entries are its exercises as done.
type WorkoutRecord struct {
	ID         uuid.UUID
	Date       string
	ProgramID  *uuid.UUID
	Day        model.ResolvedDay
	Entries    []model.RecordEntry
	StartedAt  *time.Time
	FinishedAt *time.Time
	Finalized  bool
	Modified   bool // the entries deviate from Day; computed, see modified
	Version    int32
	DeviceID   string
	UpdatedAt  time.Time
	DeletedAt  *time.Time // set only on tombstones, which only sync returns
}

// NewRecord says where a new record's day comes from: Week and Day of
// ProgramID (the active workout program when Nil), AdHoc for a workout outside
// any program, or neither for the active program's day of Date (anchored) or
// its next day (floating). A Nil ID is replaced by a new one. OnlyOne refuses
// the record (ErrConflict) when the date already has one.
type NewRecord struct {
	ID        uuid.UUID
	Date      string
	ProgramID uuid.UUID
	Week, Day int
	AdHoc     *AdHoc
	OnlyOne   bool
	DeviceID  string
}

// AdHoc is a workout outside any program, with its planned sets.
type AdHoc struct {
	Name    string
	Entries []model.RecordEntry
}

// RecordInput is what UpdateWorkoutRecord replaces.
type RecordInput struct {
	Entries    []model.RecordEntry
	StartedAt  *time.Time
	FinishedAt *time.Time
	DeviceID   string
}

// CreateWorkoutRecord starts a workout for in.Date with no set completed. The
// program's update hooks run once, before the first set, as they do after
// every set. It returns ErrConflict when the id is taken.
func (s *Service) CreateWorkoutRecord(ctx context.Context, userID uuid.UUID, in NewRecord) (WorkoutRecord, error) {
	date, err := parseDate(in.Date)
	if err != nil {
		return WorkoutRecord{}, err
	}
	if in.AdHoc != nil {
		if err := checkEntries(in.AdHoc.Entries); err != nil {
			return WorkoutRecord{}, err
		}
	}
	id := in.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	var row sqlc.WorkoutRecord
	err = inDayTx(ctx, s.pool, s.q, userID, date, "workout", func(q *sqlc.Queries) error {
		if _, err := q.GetWorkoutRecord(ctx, sqlc.GetWorkoutRecordParams{ID: id, UserID: userID}); err == nil {
			return ErrConflict
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if in.OnlyOne { // the day lock makes this check and the insert one step
			rows, err := q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: date, ToDate: date})
			if err != nil {
				return err
			}
			if len(rows) > 0 {
				return ErrConflict
			}
		}
		p, day, err := s.plan(ctx, q, userID, in, date)
		if err != nil {
			return err
		}
		params := sqlc.UpsertWorkoutRecordParams{ID: id, UserID: userID, Date: date, DeviceID: in.DeviceID}
		var entries []model.RecordEntry
		if p == nil {
			entries = in.AdHoc.Entries
			if err := fillSlugs(ctx, q, userID, entries); err != nil {
				return err
			}
			for i := range entries {
				for j := range entries[i].Sets {
					entries[i].Sets[j].Completed = false
				}
			}
		} else {
			params.ProgramID = &p.ID
			if entries, err = entriesFrom(ctx, q, userID, day); err != nil {
				return err
			}
			if err := s.runHooks(ctx, q, *p, day, entries); err != nil {
				return err
			}
		}
		now := s.Now()
		params.StartedAt = &now
		if params.ResolvedDay, err = json.Marshal(day); err != nil {
			return err
		}
		if params.Entries, err = json.Marshal(entries); err != nil {
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

// plan picks the day a new record snapshots and the program it belongs to,
// nil for an ad hoc workout.
func (s *Service) plan(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, in NewRecord, date time.Time) (*sqlc.Program, *model.ResolvedDay, error) {
	if in.AdHoc != nil {
		return nil, &model.ResolvedDay{Name: in.AdHoc.Name, Kind: "training"}, nil
	}
	var p sqlc.Program
	if in.ProgramID != uuid.Nil {
		var err error
		if p, err = q.GetProgram(ctx, sqlc.GetProgramParams{ID: in.ProgramID, UserID: userID}); err != nil {
			return nil, nil, notFound(err)
		}
	} else {
		ps, err := q.ListProgramsByUserAndStatus(ctx, sqlc.ListProgramsByUserAndStatusParams{UserID: userID, Status: programs.StatusActive})
		if err != nil {
			return nil, nil, err
		}
		i := 0
		for i < len(ps) && ps[i].Kind != programs.KindWorkout {
			i++
		}
		if i == len(ps) {
			return nil, nil, invalid("no active workout program")
		}
		p = ps[i]
	}
	if p.Kind != programs.KindWorkout {
		return nil, nil, invalid("%s is a meal program", p.Name)
	}
	week, day := in.Week, in.Day
	if week == 0 {
		switch {
		case p.StartDate != nil:
			rows, err := q.ListResolvedDaysByDateRange(ctx, sqlc.ListResolvedDaysByDateRangeParams{ProgramID: p.ID, FromDate: &date, ToDate: &date})
			if err != nil {
				return nil, nil, err
			}
			if len(rows) == 0 {
				return nil, nil, invalid("%s is not a day of %s", date.Format(time.DateOnly), p.Name)
			}
			week, day = int(rows[0].Week), int(rows[0].Day)
		default:
			c, err := q.GetCursor(ctx, p.ID)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil, invalid("%s has no next day", p.Name)
			}
			if err != nil {
				return nil, nil, err
			}
			week, day = int(c.NextWeek), int(c.NextDay)
		}
	}
	d, err := resolvedDay(ctx, q, p, week, day)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, invalid("week %d day %d is not a day of %s", week, day, p.Name)
	}
	return &p, d, err
}

// resolvedDay is day (week, day) of workout program p as resolved for its
// user; pgx.ErrNoRows when the program has no such day.
func resolvedDay(ctx context.Context, q *sqlc.Queries, p sqlc.Program, week, day int) (*model.ResolvedDay, error) {
	row, err := q.GetResolvedDay(ctx, sqlc.GetResolvedDayParams{ProgramID: p.ID, Week: int32(week), Day: int32(day)}) //nolint:gosec // weeks and days are small
	if err != nil {
		return nil, err
	}
	d, err := programs.DayFrom(p.Kind, row)
	if err != nil {
		return nil, err
	}
	return d.Workout, nil
}

// entriesFrom snapshots the planned day's exercises with no set completed.
func entriesFrom(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, day *model.ResolvedDay) ([]model.RecordEntry, error) {
	out := make([]model.RecordEntry, len(day.Exercises))
	for i, e := range day.Exercises {
		out[i] = model.RecordEntry{Name: e.Name, Slug: e.Slug, Equipment: e.Equipment, Label: e.Label, Alternates: e.Alternates,
			Current: e.Current, Desc: e.Desc, Superset: e.Superset, Unilateral: e.Unilateral, Sets: make([]model.RecordSet, len(e.Sets))}
		for j, s := range e.Sets {
			out[i].Sets[j] = model.RecordSet{Planned: model.PlanSet(s), Warmup: s.Warmup}
		}
	}
	return out, fillSlugs(ctx, q, userID, out)
}

// fillSlugs resolves the slug of every entry that has none, by its name. Clients
// send an empty slug after switching an alternate or adding an exercise by name
// (R80); a name the catalog does not know stays empty.
func fillSlugs(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, entries []model.RecordEntry) error {
	for i := range entries {
		if entries[i].Slug != "" {
			continue
		}
		slug, err := catalog.ResolveName(ctx, q, userID, entries[i].Name)
		if _, unknown := errors.AsType[*catalog.UnknownNameError](err); err != nil && !unknown {
			return err
		}
		entries[i].Slug = slug
	}
	return nil
}

// UpdateWorkoutRecord replaces the record's entries and times. The program's
// update hooks run and may change the sets still to do; completed sets stay
// as logged. A finalized record takes the entries and replays its
// progression instead (R79).
func (s *Service) UpdateWorkoutRecord(ctx context.Context, userID, id uuid.UUID, in RecordInput) (WorkoutRecord, error) {
	if err := checkEntries(in.Entries); err != nil {
		return WorkoutRecord{}, err
	}
	var row sqlc.WorkoutRecord
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Finalize holds this lock, so the finalized flag read here is the committed one.
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		old, err := q.GetWorkoutRecord(ctx, sqlc.GetWorkoutRecordParams{ID: id, UserID: userID})
		if err != nil {
			return notFound(err)
		}
		if err := fillSlugs(ctx, q, userID, in.Entries); err != nil {
			return err
		}
		if old.Finalized {
			row, err = s.updateFinalized(ctx, q, old, in.Entries, in.StartedAt, in.DeviceID)
			return err
		}
		rec, err := DecodeRecord(old)
		if err != nil {
			return err
		}
		if old.ProgramID != nil {
			p, err := q.GetProgram(ctx, sqlc.GetProgramParams{ID: *old.ProgramID, UserID: userID})
			switch {
			case err == nil:
				err = s.runHooks(ctx, q, p, &rec.Day, in.Entries)
			case errors.Is(err, pgx.ErrNoRows): // the program is gone; the snapshot stands
				err = nil
			}
			if err != nil {
				return err
			}
		}
		entries, err := json.Marshal(in.Entries)
		if err != nil {
			return err
		}
		row, err = q.UpsertWorkoutRecord(ctx, sqlc.UpsertWorkoutRecordParams{ID: id, UserID: userID, Date: old.Date,
			ProgramID: old.ProgramID, ResolvedDay: old.ResolvedDay, Entries: entries, StartedAt: in.StartedAt,
			FinishedAt: in.FinishedAt, DeviceID: in.DeviceID})
		return err
	})
	if err != nil {
		return WorkoutRecord{}, err
	}
	return DecodeRecord(row)
}

// GetWorkoutRecord returns one record.
func (s *Service) GetWorkoutRecord(ctx context.Context, userID, id uuid.UUID) (WorkoutRecord, error) {
	row, err := s.q.GetWorkoutRecord(ctx, sqlc.GetWorkoutRecordParams{ID: id, UserID: userID})
	if err != nil {
		return WorkoutRecord{}, notFound(err)
	}
	return DecodeRecord(row)
}

// ListWorkoutRecords returns the records from..to (inclusive), oldest first,
// one page at a time, only those of programID when it is set.
func (s *Service) ListWorkoutRecords(ctx context.Context, userID uuid.UUID, from, to string, programID uuid.UUID, cursor string, limit int) (Page[WorkoutRecord], error) {
	pq, err := newPageQuery(from, to, cursor, limit)
	if err != nil {
		return Page[WorkoutRecord]{}, err
	}
	params := sqlc.ListWorkoutRecordsPageParams{UserID: userID, FromDate: pq.from, ToDate: pq.to,
		AfterDate: pq.afterDate, AfterID: pq.afterID, MaxRows: pq.fetch()}
	if programID != uuid.Nil {
		params.ProgramID = &programID
	}
	rows, err := s.q.ListWorkoutRecordsPage(ctx, params)
	if err != nil {
		return Page[WorkoutRecord]{}, err
	}
	return paginate(pq, rows, func(r sqlc.WorkoutRecord) (time.Time, uuid.UUID) { return r.Date, r.ID }, DecodeRecord)
}

// DeleteWorkoutRecord soft-deletes a record. A finalized record goes too
// (R75): the progression and cursor it moved stay, and the day marked done
// at finalize goes back to planned.
func (s *Service) DeleteWorkoutRecord(ctx context.Context, userID, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		old, err := q.GetWorkoutRecord(ctx, sqlc.GetWorkoutRecordParams{ID: id, UserID: userID})
		if err != nil {
			return notFound(err)
		}
		n, err := q.DeleteWorkoutRecord(ctx, sqlc.DeleteWorkoutRecordParams{ID: id, UserID: userID})
		if err != nil || n == 0 {
			return cmp.Or(err, ErrNotFound)
		}
		if !old.Finalized {
			return nil
		}
		log, err := q.GetDay(ctx, sqlc.GetDayParams{UserID: userID, Date: old.Date})
		if err != nil || log.Status != StatusDone {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		return markDay(ctx, q, userID, old.Date, StatusPlanned)
	})
}

// DecodeRecord converts a stored row.
func DecodeRecord(r sqlc.WorkoutRecord) (WorkoutRecord, error) {
	w := WorkoutRecord{ID: r.ID, Date: r.Date.Format(time.DateOnly), ProgramID: r.ProgramID, StartedAt: r.StartedAt,
		FinishedAt: r.FinishedAt, Finalized: r.Finalized, Version: r.Version, DeviceID: r.DeviceID, UpdatedAt: r.UpdatedAt,
		DeletedAt: r.DeletedAt}
	if err := json.Unmarshal(r.ResolvedDay, &w.Day); err != nil {
		return w, err
	}
	if err := json.Unmarshal(r.Entries, &w.Entries); err != nil {
		return w, err
	}
	w.Modified = modified(&w.Day, w.Entries)
	return w, nil
}

func checkEntries(entries []model.RecordEntry) error {
	if len(entries) > maxEntries {
		return invalid("at most %d exercises", maxEntries)
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	for i, e := range entries {
		if e.Name == "" || len(e.Name) > 200 || len(e.Sets) > maxEntries || len(e.PromptValues) > 50 {
			return invalid("exercise %d: name must be 1-200 characters, at most %d sets and 50 prompt values", i+1, maxEntries)
		}
		for _, v := range e.PromptValues {
			if !finite(v) {
				return invalid("exercise %d: prompt values must be finite", i+1)
			}
		}
		for j, s := range e.Sets {
			switch {
			case !finite(s.Weight.Value) || !finite(s.Planned.Weight.Value) || !finite(s.RPE) || !finite(s.Planned.RPE):
				return invalid("exercise %d set %d: weights and RPE must be finite", i+1, j+1)
			case s.Reps < 0 || s.Reps > 1000 || s.RepsLeft < 0 || s.RepsLeft > 1000 || s.Planned.Reps < 0 || s.Planned.Reps > 1000:
				return invalid("exercise %d set %d: reps must be 0-1000", i+1, j+1)
			case s.RPE < 0 || s.RPE > 10 || s.WorkTime < 0 || s.WorkTime > 86400 || s.Planned.Rest < 0:
				return invalid("exercise %d set %d: RPE must be 0-10 and times 0-86400 seconds", i+1, j+1)
			}
		}
	}
	return nil
}
