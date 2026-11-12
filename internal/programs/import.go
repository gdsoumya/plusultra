package programs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Import saves src as a draft like Create, with what an exported program
// carries: o is the state block of a workout program and foods the catalog
// snapshot of a meal program, keyed by ref or name. State of exercises the
// script does not have is dropped and named in the warnings.
func (s *Service) Import(ctx context.Context, userID uuid.UUID, kind, src string, o *model.Overlay, foods map[string]model.Food) (sqlc.Program, []string, error) {
	var p sqlc.Program
	var warnings []string
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		var sc script
		var err error
		if p, sc, err = create(ctx, q, userID, kind, src, foods); err != nil || o == nil || sc.workout == nil {
			return err
		}
		for _, k := range retain(o, sc.workout) {
			name := k.Exercise
			if k.Label != "" {
				name = k.Label + ": " + name
			}
			warnings = append(warnings, fmt.Sprintf("dropped the state of %q: the script has no such exercise", name))
		}
		return SaveOverlay(ctx, q, p.ID, *o)
	})
	return p, warnings, err
}
