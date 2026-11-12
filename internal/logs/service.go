package logs

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// Day log statuses. A planned day that passed with no record and no log is
// skipped; done is set by finalizing its workout; rest is the user's call.
const (
	StatusPlanned = "planned"
	StatusDone    = "done"
	StatusPartial = "partial"
	StatusSkipped = "skipped"
	StatusRest    = "rest"
)

var statuses = []string{StatusPlanned, StatusDone, StatusPartial, StatusSkipped, StatusRest}

// Service keeps the calendar: day logs and workout records against the
// active programs' plans.
type Service struct {
	q        *sqlc.Queries
	pool     *pgxpool.Pool
	programs *programs.Service
	meals    *MealService
	// Now is the clock; the user's time zone turns it into today's date.
	Now func() time.Time
}

// NewService returns a Service on pool that plans with progs.
func NewService(pool *pgxpool.Pool, progs *programs.Service) *Service {
	q := sqlc.New(pool)
	return &Service{q: q, pool: pool, programs: progs, meals: NewMealService(q, pool), Now: time.Now}
}

// Readiness is how the user felt, each 1 (bad) to 5 (good), nil when not given.
type Readiness struct {
	Sleep    *int `json:"sleep,omitempty"`
	Soreness *int `json:"soreness,omitempty"`
	Stress   *int `json:"stress,omitempty"`
}

// DayLog is the user's log of one date.
type DayLog struct {
	ID        uuid.UUID
	Date      string
	Status    string
	Notes     string
	Readiness *Readiness
	Version   int32
	DeviceID  string
	UpdatedAt time.Time
	DeletedAt *time.Time // set only on tombstones, which only sync returns
}

// DayLogInput is what UpdateDay sets. An empty Status keeps the log's status,
// planned for a new log. ID is kept for a new log, so devices keep their ids;
// a date's existing log keeps its own.
type DayLogInput struct {
	ID        uuid.UUID
	Date      string
	Status    string
	Notes     string
	Readiness *Readiness
	DeviceID  string
}

// UpdateDay sets a date's status, notes and readiness, creating the log.
func (s *Service) UpdateDay(ctx context.Context, userID uuid.UUID, in DayLogInput) (DayLog, error) {
	date, err := parseDate(in.Date)
	if err != nil {
		return DayLog{}, err
	}
	if in.Status != "" && !slices.Contains(statuses, in.Status) {
		return DayLog{}, invalid("status %q is not one of %v", in.Status, statuses)
	}
	if len(in.Notes) > 10000 {
		return DayLog{}, invalid("notes must be at most 10000 characters")
	}
	if r := in.Readiness; r != nil {
		for _, v := range []*int{r.Sleep, r.Soreness, r.Stress} {
			if v != nil && (*v < 1 || *v > 5) {
				return DayLog{}, invalid("readiness values must be 1-5")
			}
		}
	}
	var readiness []byte
	if in.Readiness != nil {
		if readiness, err = json.Marshal(in.Readiness); err != nil {
			return DayLog{}, err
		}
	}
	var row sqlc.DayLog
	err = inDayTx(ctx, s.pool, s.q, userID, date, "day", func(q *sqlc.Queries) error {
		status := in.Status
		if status == "" {
			status = StatusPlanned
			old, err := q.GetDay(ctx, sqlc.GetDayParams{UserID: userID, Date: date})
			if err == nil {
				status = old.Status
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		p := sqlc.UpsertDayLogParams{UserID: userID, Date: date, Status: status, Notes: in.Notes, Readiness: readiness, DeviceID: in.DeviceID}
		if in.ID != uuid.Nil {
			p.ID = &in.ID
		}
		var err error
		row, err = q.UpsertDayLog(ctx, p)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation { // the id is another log's
			return ErrConflict
		}
		return err
	})
	if err != nil {
		return DayLog{}, err
	}
	return DecodeDayLog(row)
}

// markDay sets the status of date's log, keeping its notes and readiness.
func markDay(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, date time.Time, status string) error {
	p := sqlc.UpsertDayLogParams{UserID: userID, Date: date, Status: status}
	old, err := q.GetDay(ctx, sqlc.GetDayParams{UserID: userID, Date: date})
	switch {
	case err == nil:
		p.Notes, p.Readiness, p.DeviceID = old.Notes, old.Readiness, old.DeviceID
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	_, err = q.UpsertDayLog(ctx, p)
	return err
}

// DecodeDayLog converts a stored row.
func DecodeDayLog(r sqlc.DayLog) (DayLog, error) {
	l := DayLog{ID: r.ID, Date: r.Date.Format(time.DateOnly), Status: r.Status, Notes: r.Notes,
		Version: r.Version, DeviceID: r.DeviceID, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt}
	if len(r.Readiness) == 0 {
		return l, nil
	}
	l.Readiness = new(Readiness)
	return l, json.Unmarshal(r.Readiness, l.Readiness)
}

// today is the date in userID's time zone, as midnight UTC like the database's dates.
func (s *Service) today(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		loc = time.UTC
	}
	t := s.Now().In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}
