package api

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// imagesPath is the public route under Prefix that serves Deps.ImagesDir.
const imagesPath = "/exercises/images/"

type exerciseServer struct {
	q         *sqlc.Queries
	pool      *pgxpool.Pool
	programs  *programs.Service
	publicURL string
}

// reresolve recomputes every week of the caller's active workout programs, whose
// weights and flags depend on 1RMs, equipment and unilateral settings (R76).
func (s *exerciseServer) reresolve(ctx context.Context, userID uuid.UUID) error {
	page, err := s.programs.List(ctx, userID, programs.KindWorkout, programs.StatusActive, 0, "")
	if err != nil {
		return err
	}
	for _, p := range page.Items {
		if err := s.programs.ResolveAll(ctx, userID, p.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *exerciseServer) exercisePB(e sqlc.Exercise) *pb.Exercise {
	out := &pb.Exercise{Id: e.ID.String(), Slug: e.Slug, Name: e.Name, Equipment: e.Equipment, Aliases: e.Aliases,
		PrimaryMuscles: e.PrimaryMuscles, SecondaryMuscles: e.SecondaryMuscles, Category: e.Category, Instructions: e.Instructions, Custom: e.UserID != nil}
	for _, img := range e.Images {
		out.Images = append(out.Images, s.publicURL+Prefix+imagesPath+img)
	}
	return out
}

func customOf(e *pb.Exercise) catalog.Custom {
	return catalog.Custom{Name: e.GetName(), Equipment: e.GetEquipment(), Category: e.GetCategory(),
		PrimaryMuscles: e.GetPrimaryMuscles(), SecondaryMuscles: e.GetSecondaryMuscles(), Instructions: e.GetInstructions()}
}

func (s *exerciseServer) SearchExercises(ctx context.Context, req *connect.Request[pb.SearchExercisesRequest]) (*connect.Response[pb.SearchExercisesResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	rows, err := catalog.SearchExercises(ctx, s.q, userID, m.GetQuery(), m.GetEquipment(), m.GetMuscle(), int(m.GetLimit()))
	if err != nil {
		return nil, err
	}
	res := &pb.SearchExercisesResponse{}
	for _, r := range rows {
		res.Exercises = append(res.Exercises, s.exercisePB(r))
	}
	return connect.NewResponse(res), nil
}

func (s *exerciseServer) GetExercise(ctx context.Context, req *connect.Request[pb.GetExerciseRequest]) (*connect.Response[pb.GetExerciseResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	ex, err := s.q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: req.Msg.GetSlug(), UserID: &userID})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetExerciseResponse{Exercise: s.exercisePB(ex)}), nil
}

func (s *exerciseServer) CreateCustomExercise(ctx context.Context, req *connect.Request[pb.CreateCustomExerciseRequest]) (*connect.Response[pb.CreateCustomExerciseResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	ex, err := catalog.CreateCustomExercise(ctx, s.q, userID, customOf(req.Msg.GetExercise()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateCustomExerciseResponse{Exercise: s.exercisePB(ex)}), nil
}

func (s *exerciseServer) UpdateCustomExercise(ctx context.Context, req *connect.Request[pb.UpdateCustomExerciseRequest]) (*connect.Response[pb.UpdateCustomExerciseResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg.GetExercise()
	ex, err := catalog.UpdateCustomExercise(ctx, s.q, userID, m.GetSlug(), customOf(m))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateCustomExerciseResponse{Exercise: s.exercisePB(ex)}), nil
}

func (s *exerciseServer) DeleteCustomExercise(ctx context.Context, req *connect.Request[pb.DeleteCustomExerciseRequest]) (*connect.Response[pb.DeleteCustomExerciseResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteCustomExerciseResponse{}), catalog.DeleteCustomExercise(ctx, s.q, userID, req.Msg.GetSlug())
}

func (s *exerciseServer) GetExerciseData(ctx context.Context, req *connect.Request[pb.GetExerciseDataRequest]) (*connect.Response[pb.GetExerciseDataResponse], error) {
	userID, unit, err := s.reader(ctx)
	if err != nil {
		return nil, err
	}
	slug := req.Msg.GetSlug()
	row, err := s.q.GetUserExerciseData(ctx, sqlc.GetUserExerciseDataParams{UserID: userID, ExerciseSlug: slug})
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewResponse(&pb.GetExerciseDataResponse{Data: &pb.ExerciseData{Slug: slug}}), nil
	}
	if err != nil {
		return nil, err
	}
	d, err := exerciseDataPB(row, unit)
	return connect.NewResponse(&pb.GetExerciseDataResponse{Data: d}), err
}

func (s *exerciseServer) SetExerciseData(ctx context.Context, req *connect.Request[pb.SetExerciseDataRequest]) (*connect.Response[pb.SetExerciseDataResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	unit, err := userUnit(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	m := req.Msg.GetData()
	in := catalog.ExerciseData{Slug: m.GetSlug(), RM1: kgOf(m.GetRm1(), unit), Rounding: roundingOf(m.GetRounding()), Notes: m.GetNotes(), Unilateral: m.GetUnilateral()}
	if err := catalog.SetExerciseData(ctx, s.q, userID, in); err != nil {
		return nil, err
	}
	if err := s.reresolve(ctx, userID); err != nil {
		return nil, err
	}
	row, err := s.q.GetUserExerciseData(ctx, sqlc.GetUserExerciseDataParams{UserID: userID, ExerciseSlug: m.GetSlug()})
	if err != nil {
		return nil, err
	}
	d, err := exerciseDataPB(row, unit)
	return connect.NewResponse(&pb.SetExerciseDataResponse{Data: d}), err
}

func (s *exerciseServer) ListExerciseData(ctx context.Context, _ *connect.Request[pb.ListExerciseDataRequest]) (*connect.Response[pb.ListExerciseDataResponse], error) {
	userID, unit, err := s.reader(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListUserExerciseData(ctx, userID)
	if err != nil {
		return nil, err
	}
	res := &pb.ListExerciseDataResponse{}
	for _, r := range rows {
		d, err := exerciseDataPB(r, unit)
		if err != nil {
			return nil, err
		}
		res.Items = append(res.Items, d)
	}
	return connect.NewResponse(res), nil
}

// reader is the caller and the unit their weights come back in.
func (s *exerciseServer) reader(ctx context.Context) (uuid.UUID, model.Unit, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return userID, "", err
	}
	unit, err := userUnit(ctx, s.q, userID)
	return userID, unit, err
}

func exerciseDataPB(r sqlc.UserExerciseDatum, u model.Unit) (*pb.ExerciseData, error) {
	out := &pb.ExerciseData{Slug: r.ExerciseSlug, Notes: r.Notes, Unilateral: r.Unilateral, UpdatedAt: timestamppb.New(r.UpdatedAt)}
	rm1, err := r.Rm1.Float64Value()
	if err != nil {
		return nil, err
	}
	if rm1.Valid {
		out.Rm1 = weightPB(fromKg(rm1.Float64, u))
	}
	if r.Rounding != nil {
		var rd model.Rounding
		if err := json.Unmarshal(r.Rounding, &rd); err != nil {
			return nil, err
		}
		out.Rounding = roundingPB(rd)
	}
	return out, nil
}

func (s *exerciseServer) GetEquipment(ctx context.Context, _ *connect.Request[pb.GetEquipmentRequest]) (*connect.Response[pb.GetEquipmentResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	eqs, err := catalog.GetEquipment(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetEquipmentResponse{Equipment: equipmentPB(eqs)}), nil
}

// SetEquipment replaces the list in one transaction, so a bad entry leaves the old list in place.
func (s *exerciseServer) SetEquipment(ctx context.Context, req *connect.Request[pb.SetEquipmentRequest]) (*connect.Response[pb.SetEquipmentResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	var eqs []catalog.Equipment
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		old, err := catalog.GetEquipment(ctx, q, userID)
		if err != nil {
			return err
		}
		for _, e := range old {
			if err := catalog.DeleteEquipment(ctx, q, userID, e.Name); err != nil {
				return err
			}
		}
		for _, e := range req.Msg.GetEquipment() {
			if err := catalog.SetEquipment(ctx, q, userID, e.GetName(), *roundingOf(e.GetRounding())); err != nil {
				return err
			}
		}
		eqs, err = catalog.GetEquipment(ctx, q, userID)
		return err
	})
	if err == nil {
		err = s.reresolve(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SetEquipmentResponse{Equipment: equipmentPB(eqs)}), nil
}

func equipmentPB(eqs []catalog.Equipment) []*pb.Equipment {
	out := make([]*pb.Equipment, len(eqs))
	for i, e := range eqs {
		out[i] = &pb.Equipment{Name: e.Name, Rounding: roundingPB(e.Config)}
	}
	return out
}
