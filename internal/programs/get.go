package programs

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// Get returns userID's program id.
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (sqlc.Program, error) {
	return get(ctx, s.Q, userID, id)
}

// Cursor returns the next day of p when it is an active floating workout
// program; ok is false otherwise.
func (s *Service) Cursor(ctx context.Context, p sqlc.Program) (c sqlc.ProgramCursor, ok bool, err error) {
	if p.Kind != KindWorkout || p.Status != StatusActive || p.StartDate != nil {
		return c, false, nil
	}
	c, err = s.Q.GetCursor(ctx, p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, false, nil
	}
	return c, err == nil, err
}

// Delete deletes program id; logs that point at it stay. Deleting the active
// workout program re-resolves the active meal program from today, since its
// calendar is gone.
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.tx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockUserPrograms(ctx, userID); err != nil {
			return err
		}
		p, err := lock(ctx, q, userID, id)
		if err != nil {
			return err
		}
		if _, err := q.DeleteProgram(ctx, sqlc.DeleteProgramParams{ID: id, UserID: userID}); err != nil {
			return err
		}
		if p.Kind != KindWorkout || p.Status != StatusActive {
			return nil
		}
		from, err := s.today(ctx, q, userID)
		if err != nil {
			return err
		}
		return s.resolveMeals(ctx, q, userID, from)
	})
}
