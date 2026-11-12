package programs

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// LoadOverlay returns a program's progression state. Like SaveOverlay it takes
// no user: callers pass a program id they have already scoped to its user.
func LoadOverlay(ctx context.Context, q *sqlc.Queries, programID uuid.UUID) (model.Overlay, error) {
	rows, err := q.ListProgressionState(ctx, programID)
	var o model.Overlay
	for _, r := range rows {
		x, err := engine.UnmarshalOverlay(r.State)
		if err != nil {
			return o, err
		}
		o.Merge(x)
	}
	return o, err
}

// SaveOverlay replaces a program's progression state. It is kept whole in the
// progression_state row with an empty exercise and label.
func SaveOverlay(ctx context.Context, q *sqlc.Queries, programID uuid.UUID, o model.Overlay) error {
	raw, err := engine.MarshalOverlay(o)
	if err != nil {
		return err
	}
	return q.UpsertProgressionState(ctx, sqlc.UpsertProgressionStateParams{ProgramID: programID, State: raw})
}

// slugs maps exercise names to catalog slugs.
type slugs map[string]string

// set fills in the slug of every exercise of d.
func (sl slugs) set(d *model.ResolvedDay) {
	for i := range d.Exercises {
		d.Exercises[i].Slug = sl[d.Exercises[i].Name]
	}
}

// userState is what resolving p needs about userID, without progression
// state: units, latest bodyweight, the rounding of each equipment, and per
// exercise name the 1RM (both stored in kilograms, whatever the user's units),
// the user's own rounding and the catalog equipment. It also returns the slugs
// of the names the catalog knows.
func userState(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, p *model.Program) (*model.UserState, slugs, error) {
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	units := model.Unit(u.Units)
	st := &model.UserState{Units: units, RM1: map[string]model.Weight{}, Unilateral: map[string]bool{}, Equipment: map[string]string{}, Rounding: map[string]model.Rounding{}}
	bw, err := q.LatestBodyweight(ctx, userID)
	switch {
	case err == nil:
		st.Bodyweight = model.Weight{Value: bw, Unit: model.UnitKg}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, nil, err
	}
	eqs, err := catalog.GetEquipment(ctx, q, userID)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range eqs {
		st.Rounding[e.Name] = e.Config
	}
	sl := slugs{}
	for _, d := range p.Days {
		for _, e := range d.Exercises {
			for _, name := range e.Names {
				if _, seen := sl[name]; seen {
					continue
				}
				if err := exerciseState(ctx, q, userID, name, st, sl); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	return st, sl, nil
}

func exerciseState(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, name string, st *model.UserState, sl slugs) error {
	slug, err := catalog.ResolveName(ctx, q, userID, name)
	sl[name] = slug // empty for an unknown name
	if _, ok := errors.AsType[*catalog.UnknownNameError](err); ok {
		return nil
	}
	if err != nil {
		return err
	}
	ex, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: slug, UserID: &userID})
	if err != nil {
		return err
	}
	st.Equipment[name] = ex.Equipment
	d, err := catalog.GetExerciseData(ctx, q, userID, slug)
	if errors.Is(err, catalog.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if d.RM1 != nil {
		st.RM1[name] = model.Weight{Value: *d.RM1, Unit: model.UnitKg}
	}
	if d.Unilateral {
		st.Unilateral[name] = true
	}
	if d.Rounding != nil {
		st.Rounding[name] = *d.Rounding
	}
	return nil
}
