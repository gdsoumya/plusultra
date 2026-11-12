package export

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Day is everything logged on one date, as the JSON export carries it and
// the text export shows it.
type Day struct {
	Date      string          `json:"date"`
	Status    string          `json:"status,omitempty"`
	Notes     string          `json:"notes,omitempty"`
	Readiness json.RawMessage `json:"readiness,omitempty"`
	Workout   *Workout        `json:"workout,omitempty"`
	// Workouts are the date's further records, which the text has no room for.
	Workouts []*Workout `json:"workouts,omitempty"`
	Meals    *Meals     `json:"meals,omitempty"`
	Checkin  *Checkin   `json:"checkin,omitempty"`
}

// Workout is a workout record. Program is the program's name, "" for an ad
// hoc workout. Minutes is its length when the timestamps are unknown, as in
// imported text.
type Workout struct {
	Program    string     `json:"program"`
	ProgramID  string     `json:"program_id,omitempty"`
	Week       int        `json:"week"`
	Day        int        `json:"day"`
	DayName    string     `json:"day_name"`
	Kind       string     `json:"kind,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Minutes    int        `json:"minutes,omitempty"`
	Finalized  bool       `json:"finalized"`
	Modified   bool       `json:"modified"` // the entries deviate from the planned day
	Entries    []Entry    `json:"entries"`
}

// Entry is an exercise of a workout record. WarmupDone is all the text keeps
// of the warmup sets.
type Entry struct {
	model.RecordEntry
	WarmupDone bool `json:"warmup_done,omitempty"`
}

// Meals is a meal log.
type Meals struct {
	ProgramID string       `json:"program_id,omitempty"`
	Target    model.Macros `json:"target"`
	Totals    model.Macros `json:"totals"`
	Meals     []model.Meal `json:"meals"`
}

// Checkin is a body check-in, weight in kg.
type Checkin struct {
	WeightKg     *float64           `json:"weight_kg,omitempty"`
	BodyFat      *float64           `json:"body_fat,omitempty"`
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

func (w *Workout) minutes() int {
	if w.StartedAt != nil && w.FinishedAt != nil {
		return int(math.Round(w.FinishedAt.Sub(*w.StartedAt).Minutes()))
	}
	return w.Minutes
}

// ExportDays writes userID's day logs from from to to, inclusive, in format.
func (s *Service) ExportDays(ctx context.Context, userID uuid.UUID, from, to time.Time, format string) ([]byte, error) {
	days, units, err := s.loadDays(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	switch format {
	case FormatText, "":
		return []byte(formatDays(days, units)), nil
	case FormatJSON:
		return json.MarshalIndent(days, "", "  ")
	case FormatCSV:
		return formatCSV(days), nil
	}
	return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("format must be %s, %s or %s", FormatText, FormatJSON, FormatCSV))
}

// loadDays collects the user's logs between from and to, oldest first, and
// the user's weight unit.
func (s *Service) loadDays(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]Day, model.Unit, error) {
	u, err := s.Q.GetUser(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	byDate := map[string]*Day{}
	day := func(date string) *Day {
		if byDate[date] == nil {
			byDate[date] = &Day{Date: date}
		}
		return byDate[date]
	}
	dayLogs, err := s.Q.ListDays(ctx, sqlc.ListDaysParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return nil, "", err
	}
	for _, r := range dayLogs {
		d := day(r.Date.Format(time.DateOnly))
		d.Status, d.Notes, d.Readiness = r.Status, r.Notes, r.Readiness
	}
	if err := s.loadWorkouts(ctx, userID, from, to, day); err != nil {
		return nil, "", err
	}
	fromS, toS := from.Format(time.DateOnly), to.Format(time.DateOnly)
	for cursor, more := "", true; more; {
		page, err := s.Meals.List(ctx, userID, fromS, toS, cursor, 500)
		if err != nil {
			return nil, "", err
		}
		for _, l := range page.Items {
			m := &Meals{Target: l.Target, Totals: l.Totals, Meals: l.Meals}
			if l.ProgramID != nil {
				m.ProgramID = l.ProgramID.String()
			}
			day(l.Date).Meals = m
		}
		cursor, more = page.NextCursor, page.HasMore
	}
	for cursor, more := "", true; more; {
		page, err := s.Checkins.List(ctx, userID, fromS, toS, cursor, 500, false)
		if err != nil {
			return nil, "", err
		}
		for _, c := range page.Items {
			day(c.Date).Checkin = &Checkin{WeightKg: c.WeightKg, BodyFat: c.BodyFat, Measurements: c.Measurements}
		}
		cursor, more = page.NextCursor, page.HasMore
	}
	days := make([]Day, 0, len(byDate))
	for _, d := range byDate {
		days = append(days, *d)
	}
	slices.SortFunc(days, func(a, b Day) int { return strings.Compare(a.Date, b.Date) })
	return days, model.Unit(u.Units), nil
}

// loadWorkouts attaches each date's workout records: the first as Workout,
// the rest as Workouts.
func (s *Service) loadWorkouts(ctx context.Context, userID uuid.UUID, from, to time.Time, day func(string) *Day) error {
	names, err := s.Q.ListProgramNames(ctx, userID)
	if err != nil {
		return err
	}
	name := map[uuid.UUID]string{}
	for _, n := range names {
		name[n.ID] = n.Name
	}
	records, err := s.Q.ListWorkoutRecordsByDate(ctx, sqlc.ListWorkoutRecordsByDateParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return err
	}
	for _, r := range records {
		d := day(r.Date.Format(time.DateOnly))
		rec, err := logs.DecodeRecord(r)
		if err != nil {
			return err
		}
		rd := rec.Day
		w := &Workout{Week: rd.Week, Day: rd.Day, DayName: rd.Name, Kind: rd.Kind, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
			Finalized: r.Finalized, Modified: rec.Modified}
		if r.ProgramID != nil {
			w.Program, w.ProgramID = name[*r.ProgramID], r.ProgramID.String()
		}
		for _, e := range rec.Entries {
			w.Entries = append(w.Entries, Entry{RecordEntry: e, WarmupDone: warmupDone(e.Sets)})
		}
		if d.Workout == nil {
			d.Workout = w
		} else {
			d.Workouts = append(d.Workouts, w)
		}
	}
	return nil
}

// workouts is every workout record of d.
func (d *Day) workouts() []*Workout {
	if d.Workout == nil {
		return nil
	}
	return append([]*Workout{d.Workout}, d.Workouts...)
}

// warmupDone reports whether sets has warmups and every one was completed.
func warmupDone(sets []model.RecordSet) bool {
	n := 0
	for _, s := range sets {
		if s.Warmup || s.Planned.Warmup {
			if !s.Completed {
				return false
			}
			n++
		}
	}
	return n > 0
}
