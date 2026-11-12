package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// NameError is an exercise or food name (or food ref) in a program that the catalog does not know.
type NameError struct {
	Line        int
	Name        string
	Suggestions []string
}

// ValidateProgramNames resolves every exercise name and alternate in p. The
// error is for lookup failures, not for unknown names.
func ValidateProgramNames(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, p *model.Program) ([]NameError, error) {
	var out []NameError
	for _, d := range p.Days {
		for _, ex := range d.Exercises {
			for _, name := range ex.Names {
				_, err := ResolveName(ctx, q, userID, name)
				var un *UnknownNameError
				switch {
				case errors.As(err, &un):
					out = append(out, NameError{Line: ex.Line, Name: name, Suggestions: un.Suggestions})
				case err != nil:
					return nil, err
				}
			}
		}
	}
	return out, nil
}
