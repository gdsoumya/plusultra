// Package sync keeps the app's offline store in step with the server: Pull sends the rows
// changed since a cursor, Push applies the rows changed on a device.
package sync

import (
	"context"
	"errors"
	"log/slog"
	gosync "sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// ErrInvalid is returned for a bad cursor or limit, or a push carrying photo bytes.
var ErrInvalid = errors.New("sync: invalid")

// Finalizer runs a workout record's progressions; the logs Service is one. Push calls it for a
// record pushed with Finalized set that the server has not finalized yet.
type Finalizer interface {
	Finalize(ctx context.Context, userID, recordID uuid.UUID) error
}

// Rows are rows of the synced tables. Rows with DeletedAt set are tombstones.
type Rows struct {
	DayLogs        []logs.DayLog
	WorkoutRecords []logs.WorkoutRecord
	MealLogs       []logs.MealLog
	Checkins       []logs.Checkin
}

// Service pulls and pushes rows of the synced tables.
type Service struct {
	Logger *slog.Logger // receives the errors behind finalize_failed reasons

	q        *sqlc.Queries
	pool     *pgxpool.Pool
	logs     *logs.Service
	programs *programs.Service
	meals    *logs.MealService
	checkins *logs.CheckinService
	fin      Finalizer
	pushing  gosync.Map // user id to the *gosync.Mutex held by that user's push
}

// New returns a Service. Rows are written through the logs services, so their validation,
// ownership checks and finalized-record rules match the rest of the API.
func New(q *sqlc.Queries, pool *pgxpool.Pool, ls *logs.Service, ps *programs.Service, f Finalizer) *Service {
	return &Service{
		Logger: slog.Default(), q: q, pool: pool, logs: ls, programs: ps, fin: f,
		meals: logs.NewMealService(q, pool), checkins: logs.NewCheckinService(q, pool),
	}
}
