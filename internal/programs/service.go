// Package programs manages workout and meal programs: scripts and their
// revisions, activation, and the days they resolve to.
package programs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Program kinds and statuses.
const (
	KindWorkout    = "workout"
	KindMeal       = "meal"
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusFinished = "finished"
	StatusArchived = "archived"
)

// scriptFile names scripts in error positions.
const scriptFile = "program.star"

// Service implements the program life cycle. Build it with NewService.
type Service struct {
	Q *sqlc.Queries
	// Now is the clock; the user's time zone turns it into today's date.
	Now  func() time.Time
	pool *pgxpool.Pool
}

// NewService returns a Service backed by pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{Q: sqlc.New(pool), Now: time.Now, pool: pool}
}

func (s *Service) tx(ctx context.Context, f func(*sqlc.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return f(s.Q.WithTx(tx)) })
}

// Validation is the outcome of checking a script. The script is valid when
// Errors and NameErrors are both empty.
type Validation struct {
	Structure  *Structure // nil when the script does not evaluate
	Errors     []*engine.EvalError
	NameErrors []catalog.NameError
}

// Validate evaluates script as a program of kind and checks its exercise
// names against userID's catalog. The error is for lookup failures and an
// unknown kind, not for script errors.
func (s *Service) Validate(ctx context.Context, userID uuid.UUID, kind, script string) (Validation, error) {
	v, _, err := validate(ctx, s.Q, userID, kind, script, nil)
	return v, err
}

// validate evaluates src with foods, the catalog snapshot of an imported meal
// program, and checks its names. A state block is an error: export writes
// it, and only import reads it.
func validate(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, kind, src string, foods map[string]model.Food) (Validation, script, error) {
	sc, err := evaluate(kind, src, foods)
	if ee, ok := errors.AsType[*engine.EvalError](err); ok {
		return Validation{Errors: []*engine.EvalError{ee}}, sc, nil
	}
	if err != nil {
		return Validation{}, sc, err
	}
	if sc.state != nil {
		ee := &engine.EvalError{File: scriptFile, Line: sc.state.Line, Msg: "state() is written by export; import the exported file instead of saving it"}
		return Validation{Errors: []*engine.EvalError{ee}}, sc, nil
	}
	v := Validation{Structure: sc.structure()}
	if sc.workout != nil {
		v.NameErrors, err = catalog.ValidateProgramNames(ctx, q, userID, sc.workout)
	} else {
		v.NameErrors, err = catalog.ValidateMealNames(ctx, q, userID, sc.meal)
		// A ref the snapshot fills needs no catalog row.
		v.NameErrors = slices.DeleteFunc(v.NameErrors, func(n catalog.NameError) bool { _, ok := foods[n.Name]; return ok })
		if err == nil && len(v.NameErrors) == 0 {
			// Freezing checks each item's unit against the catalog, so validate and create agree.
			if sc, err = freeze(ctx, q, userID, src, sc); err != nil {
				if ee, ok := errors.AsType[*engine.EvalError](err); ok {
					v.Errors, err = []*engine.EvalError{ee}, nil
				}
			}
		}
	}
	return v, sc, err
}

// checked is the evaluated script, or an InvalidArgument error when it is not valid.
func checked(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, kind, src string, foods map[string]model.Food) (script, error) {
	v, sc, err := validate(ctx, q, userID, kind, src, foods)
	switch {
	case err != nil:
		return sc, err
	case len(v.Errors) > 0:
		return sc, invalidArgument(v.Errors[0])
	}
	what, hint := "exercise", ""
	if kind == KindMeal {
		what, hint = "food", "; add a custom food or remove the item"
	}
	errs := make([]error, len(v.NameErrors))
	for i, n := range v.NameErrors {
		msg := fmt.Sprintf("line %d: unknown %s %q", n.Line, what, n.Name)
		if len(n.Suggestions) > 0 {
			msg += "; did you mean " + strings.Join(n.Suggestions, ", ") + "?"
		}
		errs[i] = errors.New(msg + hint)
	}
	if err := errors.Join(errs...); err != nil {
		return sc, invalidArgument(err)
	}
	return sc, nil
}

// Create saves script as a new draft program of kind at revision 1.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, kind, script string) (sqlc.Program, error) {
	var p sqlc.Program
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		var err error
		p, _, err = create(ctx, q, userID, kind, script, nil)
		return err
	})
	return p, err
}

func create(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, kind, src string, foods map[string]model.Food) (sqlc.Program, script, error) {
	sc, err := checked(ctx, q, userID, kind, src, foods)
	if err != nil {
		return sqlc.Program{}, sc, err
	}
	st := sc.structure()
	raw, err := json.Marshal(st)
	if err != nil {
		return sqlc.Program{}, sc, err
	}
	p, err := q.CreateProgram(ctx, sqlc.CreateProgramParams{UserID: userID, Kind: kind, Name: st.Name,
		Script: src, Structure: raw, Status: StatusDraft})
	if err != nil {
		return p, sc, err
	}
	return p, sc, insertRevision(ctx, q, p)
}

func insertRevision(ctx context.Context, q *sqlc.Queries, p sqlc.Program) error {
	createdBy := "user"
	if pr, ok := auth.FromContext(ctx); ok && pr.Via == auth.ViaAPIKey {
		createdBy = "api"
	}
	_, err := q.InsertRevision(ctx, sqlc.InsertRevisionParams{ProgramID: p.ID, Revision: p.Revision,
		Script: p.Script, Structure: p.Structure, CreatedBy: createdBy})
	return err
}

// get returns userID's program id, or a NotFound error.
func get(ctx context.Context, q *sqlc.Queries, userID, id uuid.UUID) (sqlc.Program, error) {
	return found(q.GetProgram(ctx, sqlc.GetProgramParams{ID: id, UserID: userID}))
}

// lock is get for a write: the program's row stays locked until the transaction ends.
func lock(ctx context.Context, q *sqlc.Queries, userID, id uuid.UUID) (sqlc.Program, error) {
	return found(q.GetProgramForUpdate(ctx, sqlc.GetProgramForUpdateParams{ID: id, UserID: userID}))
}

func found(p sqlc.Program, err error) (sqlc.Program, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return p, connect.NewError(connect.CodeNotFound, errors.New("program not found"))
	}
	return p, err
}

// activeProgram is userID's active program of kind, or nil.
func activeProgram(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, kind string) (*sqlc.Program, error) {
	ps, err := q.ListProgramsByUserAndStatus(ctx, sqlc.ListProgramsByUserAndStatusParams{UserID: userID, Status: StatusActive})
	for i := range ps {
		if ps[i].Kind == kind {
			return &ps[i], nil
		}
	}
	return nil, err
}

func invalidArgument(err error) error { return connect.NewError(connect.CodeInvalidArgument, err) }

func failedPrecondition(err error) error {
	return connect.NewError(connect.CodeFailedPrecondition, err)
}
