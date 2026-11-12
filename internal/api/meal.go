package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

type mealServer struct {
	meals    *logs.MealService
	programs *programs.Service
	q        *sqlc.Queries
}

func (s *mealServer) ListMealLogs(ctx context.Context, req *connect.Request[pb.ListMealLogsRequest]) (*connect.Response[pb.ListMealLogsResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	page, err := s.meals.List(ctx, userID, m.GetDates().GetFrom(), m.GetDates().GetTo(), m.GetCursor(), int(m.GetLimit()))
	if err != nil {
		return nil, err
	}
	res := &pb.ListMealLogsResponse{NextCursor: page.NextCursor, HasMore: page.HasMore}
	for _, l := range page.Items {
		res.MealLogs = append(res.MealLogs, mealLogPB(l))
	}
	return connect.NewResponse(res), nil
}

func (s *mealServer) GetMealLog(ctx context.Context, req *connect.Request[pb.GetMealLogRequest]) (*connect.Response[pb.GetMealLogResponse], error) {
	userID, id, err := owned(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	l, err := s.meals.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMealLogResponse{MealLog: mealLogPB(l)}), nil
}

func (s *mealServer) CreateMealLog(ctx context.Context, req *connect.Request[pb.CreateMealLogRequest]) (*connect.Response[pb.CreateMealLogResponse], error) {
	userID, in, err := s.input(ctx, req.Msg.GetMealLog())
	if err != nil {
		return nil, err
	}
	if in.Target == nil {
		if in.Target, err = s.planTarget(ctx, userID, in.Date); err != nil {
			return nil, err
		}
	}
	l, err := s.meals.Create(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateMealLogResponse{MealLog: mealLogPB(l)}), nil
}

func (s *mealServer) UpdateMealLog(ctx context.Context, req *connect.Request[pb.UpdateMealLogRequest]) (*connect.Response[pb.UpdateMealLogResponse], error) {
	userID, in, err := s.input(ctx, req.Msg.GetMealLog())
	if err != nil {
		return nil, err
	}
	l, err := s.meals.Update(ctx, userID, in.ID, in)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateMealLogResponse{MealLog: mealLogPB(l)}), nil
}

func (s *mealServer) DeleteMealLog(ctx context.Context, req *connect.Request[pb.DeleteMealLogRequest]) (*connect.Response[pb.DeleteMealLogResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteMealLogResponse{}), s.meals.Delete(ctx, userID, id)
}

// input is the writing caller and the service input for m, with ref-only foods frozen.
func (s *mealServer) input(ctx context.Context, m *pb.MealLog) (uuid.UUID, logs.MealLogInput, error) {
	userID, err := writer(ctx)
	if err != nil {
		return userID, logs.MealLogInput{}, err
	}
	if err := s.freezeRefs(ctx, userID, m.GetMeals()); err != nil {
		return userID, logs.MealLogInput{}, err
	}
	in, err := mealLogInput(m)
	return userID, in, err
}

// planTarget is the target of the active anchored meal program's day on date, nil without one.
func (s *mealServer) planTarget(ctx context.Context, userID uuid.UUID, date string) (*model.Macros, error) {
	page, err := s.programs.List(ctx, userID, programs.KindMeal, programs.StatusActive, 1, "")
	if err != nil || len(page.Items) == 0 || page.Items[0].StartDate == nil {
		return nil, err
	}
	d, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	days, err := s.programs.ResolvedDays(ctx, userID, page.Items[0].ID, programs.Range{From: d, To: d})
	if err != nil || len(days) == 0 || days[0].Meal == nil {
		return nil, err
	}
	return days[0].Meal.Target, nil
}

// freezeRefs fills the macros of items whose food is given by ref alone, as FrozenFood.ref promises;
// an edited item carries the user's own macros and is left as sent.
func (s *mealServer) freezeRefs(ctx context.Context, userID uuid.UUID, meals []*pb.Meal) error {
	for _, m := range meals {
		for _, it := range m.GetItems() {
			f := it.GetFood()
			if f.GetRef() == "" || f.GetPer100G() != nil || it.GetEdited() {
				continue
			}
			cf, err := foodByRef(ctx, s.q, userID, f.GetRef())
			if err != nil {
				return err
			}
			var per100g model.Macros
			var units []catalog.Unit
			if err := json.Unmarshal(cf.Per100g, &per100g); err != nil {
				return err
			}
			if err := json.Unmarshal(cf.Units, &units); err != nil {
				return err
			}
			f.Per100G = macrosPB(per100g)
			for _, u := range units {
				if u.Name == it.GetUnit() {
					f.PerUnit, f.Unit = macrosPB(per100g.Scale(u.Grams/100)), u.Name
				}
			}
		}
	}
	return nil
}

// foodByRef resolves an opaque meal item ref (R53): a food id, or source:source_id as scripts write it.
func foodByRef(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, ref string) (catalog.Food, error) {
	var f catalog.Food
	var err error
	if id, perr := uuid.Parse(ref); perr == nil {
		f, err = catalog.GetFood(ctx, q, userID, id)
	} else if source, sid, ok := strings.Cut(ref, ":"); ok {
		f, err = q.GetFoodBySource(ctx, sqlc.GetFoodBySourceParams{Source: source, SourceID: sid, UserID: &userID})
	} else {
		err = pgx.ErrNoRows
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, catalog.ErrNotFound) {
		return f, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown food ref "+ref))
	}
	return f, err
}
