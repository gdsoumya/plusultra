// Package export writes a user's programs and day logs as files and reads
// them back: scripts with their generated blocks, day logs as text, JSON or
// CSV, and a zip of everything.
package export

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// Import kinds.
const (
	KindProgram = "program"
	KindDayLog  = "daylog"
)

// Day log export formats.
const (
	FormatText = "text"
	FormatJSON = "json"
	FormatCSV  = "csv"
)

// Service exports and imports. Build it with NewService.
type Service struct {
	Q        *sqlc.Queries
	Programs *programs.Service
	Logs     *logs.Service
	Meals    *logs.MealService
	Checkins *logs.CheckinService
	// Workouts records the workouts of imported day logs, through Logs by
	// default; while nil, their workout lines are skipped with a warning.
	Workouts WorkoutImporter
}

// NewService returns a Service backed by pool that writes through the shared
// programs and logs services.
func NewService(pool *pgxpool.Pool, progs *programs.Service, logSvc *logs.Service) *Service {
	q := sqlc.New(pool)
	s := &Service{Q: q, Programs: progs, Logs: logSvc, Meals: logs.NewMealService(q, pool), Checkins: logs.NewCheckinService(q, pool)}
	if logSvc != nil {
		s.Workouts = recordImporter{q: q, logs: logSvc}
	}
	return s
}

// invalid is an InvalidArgument error about a line of imported text.
func invalid(line int, format string, args ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...)))
}

// logsError maps the logs sentinels to Connect codes, naming the line that caused them.
func logsError(line int, err error) error {
	code := connect.CodeInternal
	switch {
	case err == nil:
		return nil
	case errors.Is(err, logs.ErrInvalid):
		code = connect.CodeInvalidArgument
	case errors.Is(err, logs.ErrNotFound):
		code = connect.CodeNotFound
	case errors.Is(err, logs.ErrConflict):
		code = connect.CodeAlreadyExists
	default:
		return err
	}
	return connect.NewError(code, fmt.Errorf("line %d: %w", line, err))
}
