package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/programs"
)

type programServer struct{ programs *programs.Service }

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	return id, nil
}

// owned is the caller's id and the parsed id of the program a request names.
func owned(ctx context.Context, id string) (uuid.UUID, uuid.UUID, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return userID, uuid.Nil, err
	}
	pid, err := parseID(id)
	return userID, pid, err
}

// program is p with its cursor when it runs floating, and its script unless listed.
func (s *programServer) program(ctx context.Context, p sqlc.Program, script bool) (*pb.Program, error) {
	out, err := programPB(p, script)
	if err != nil {
		return nil, err
	}
	c, ok, err := s.programs.Cursor(ctx, p)
	if ok {
		out.Cursor = &pb.ProgramCursor{NextWeek: c.NextWeek, NextDay: c.NextDay}
	}
	return out, err
}

func (s *programServer) ListPrograms(ctx context.Context, req *connect.Request[pb.ListProgramsRequest]) (*connect.Response[pb.ListProgramsResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	page, err := s.programs.List(ctx, userID, kindNames[m.GetKind()], statusNames[m.GetStatus()], int(m.GetLimit()), m.GetCursor())
	if err != nil {
		return nil, err
	}
	res := &pb.ListProgramsResponse{NextCursor: page.NextCursor, HasMore: page.HasMore}
	for _, p := range page.Items {
		pp, err := s.program(ctx, p, false)
		if err != nil {
			return nil, err
		}
		res.Programs = append(res.Programs, pp)
	}
	return connect.NewResponse(res), nil
}

func (s *programServer) GetProgram(ctx context.Context, req *connect.Request[pb.GetProgramRequest]) (*connect.Response[pb.GetProgramResponse], error) {
	userID, id, err := owned(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	p, err := s.programs.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	out, err := s.program(ctx, p, true)
	return connect.NewResponse(&pb.GetProgramResponse{Program: out}), err
}

func (s *programServer) CreateProgram(ctx context.Context, req *connect.Request[pb.CreateProgramRequest]) (*connect.Response[pb.CreateProgramResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.programs.Create(ctx, userID, kindNames[req.Msg.GetKind()], req.Msg.GetScript())
	if err != nil {
		return nil, err
	}
	out, err := s.program(ctx, p, true)
	return connect.NewResponse(&pb.CreateProgramResponse{Program: out}), err
}

func (s *programServer) UpdateProgram(ctx context.Context, req *connect.Request[pb.UpdateProgramRequest]) (*connect.Response[pb.UpdateProgramResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	p, err := s.programs.Update(ctx, userID, id, req.Msg.GetScript())
	if err != nil {
		return nil, err
	}
	out, err := s.program(ctx, p, true)
	return connect.NewResponse(&pb.UpdateProgramResponse{Program: out}), err
}

func (s *programServer) DeleteProgram(ctx context.Context, req *connect.Request[pb.DeleteProgramRequest]) (*connect.Response[pb.DeleteProgramResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteProgramResponse{}), s.programs.Delete(ctx, userID, id)
}

func (s *programServer) ValidateProgram(ctx context.Context, req *connect.Request[pb.ValidateProgramRequest]) (*connect.Response[pb.ValidateProgramResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.programs.Validate(ctx, userID, kindNames[req.Msg.GetKind()], req.Msg.GetScript())
	if err != nil {
		return nil, err
	}
	res := &pb.ValidateProgramResponse{}
	for _, e := range v.Errors {
		res.Errors = append(res.Errors, scriptErrorPB(e))
	}
	for _, n := range v.NameErrors {
		res.NameErrors = append(res.NameErrors, nameErrorPB(n))
	}
	if len(res.Errors)+len(res.NameErrors) == 0 {
		res.Structure = structureOf(v.Structure)
	}
	return connect.NewResponse(res), nil
}

// SimulateProgram reports script and resolution errors in the response, not as a failed call.
func (s *programServer) SimulateProgram(ctx context.Context, req *connect.Request[pb.SimulateProgramRequest]) (*connect.Response[pb.SimulateProgramResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	in, err := simulateInput(req.Msg)
	if err != nil {
		return nil, err
	}
	out, err := s.programs.Simulate(ctx, userID, in)
	if connect.CodeOf(err) == connect.CodeInvalidArgument {
		out.Days = nil
		res := simulateResponse(req.Msg, out)
		res.Errors = []*pb.ScriptError{scriptErrorPB(err)}
		return connect.NewResponse(res), nil
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(simulateResponse(req.Msg, out)), nil
}

func (s *programServer) ActivateProgram(ctx context.Context, req *connect.Request[pb.ActivateProgramRequest]) (*connect.Response[pb.ActivateProgramResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	var start *time.Time
	if d := req.Msg.GetStartDate(); d != "" {
		t, err := time.Parse(time.DateOnly, d)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("start_date must be YYYY-MM-DD"))
		}
		start = &t
	}
	p, err := s.programs.Activate(ctx, userID, id, start)
	if err != nil {
		return nil, err
	}
	out, err := s.program(ctx, p, true)
	return connect.NewResponse(&pb.ActivateProgramResponse{Program: out}), err
}

func (s *programServer) FinishProgram(ctx context.Context, req *connect.Request[pb.FinishProgramRequest]) (*connect.Response[pb.FinishProgramResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	p, err := s.programs.Finish(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	out, err := s.program(ctx, p, true)
	return connect.NewResponse(&pb.FinishProgramResponse{Program: out}), err
}

func (s *programServer) ListRevisions(ctx context.Context, req *connect.Request[pb.ListRevisionsRequest]) (*connect.Response[pb.ListRevisionsResponse], error) {
	m := req.Msg
	userID, id, err := owned(ctx, m.GetProgramId())
	if err != nil {
		return nil, err
	}
	page, err := s.programs.ListRevisions(ctx, userID, id, int(m.GetLimit()), m.GetCursor())
	if err != nil {
		return nil, err
	}
	res := &pb.ListRevisionsResponse{NextCursor: page.NextCursor, HasMore: page.HasMore}
	for _, r := range page.Items {
		rp, err := revisionPB(r, m.GetIncludeScript())
		if err != nil {
			return nil, err
		}
		res.Revisions = append(res.Revisions, rp)
	}
	return connect.NewResponse(res), nil
}

func (s *programServer) GetResolvedDays(ctx context.Context, req *connect.Request[pb.GetResolvedDaysRequest]) (*connect.Response[pb.GetResolvedDaysResponse], error) {
	m := req.Msg
	userID, id, err := owned(ctx, m.GetProgramId())
	if err != nil {
		return nil, err
	}
	week := int(m.GetWeek())
	days, err := s.programs.ResolvedDays(ctx, userID, id, programs.Range{FromWeek: week, ToWeek: week})
	if err != nil {
		return nil, err
	}
	res := &pb.GetResolvedDaysResponse{}
	for _, d := range days {
		if d.Workout != nil {
			rd := resolvedDayPB(d.Workout)
			rd.ProgramId, rd.Date, rd.Week, rd.Day, rd.Kind = m.GetProgramId(), dateString(d.Date), i32(d.Week), i32(d.Day), d.Kind
			res.Days = append(res.Days, rd)
			continue
		}
		md := mealDayPB(d.Meal)
		md.ProgramId, md.Date, md.Week, md.Day, md.Kind = m.GetProgramId(), dateString(d.Date), i32(d.Week), i32(d.Day), d.Kind
		res.MealDays = append(res.MealDays, md)
	}
	return connect.NewResponse(res), nil
}

// writer is the caller's id when the caller may modify data.
func writer(ctx context.Context) (uuid.UUID, error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return uuid.Nil, err
	}
	return callerID(ctx)
}

func ownedWrite(ctx context.Context, id string) (uuid.UUID, uuid.UUID, error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return owned(ctx, id)
}
