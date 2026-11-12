package api

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/logs"
)

type workoutServer struct{ Deps }

func (s *workoutServer) ListWorkoutRecords(ctx context.Context, req *connect.Request[pb.ListWorkoutRecordsRequest]) (*connect.Response[pb.ListWorkoutRecordsResponse], error) {
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	var programID uuid.UUID
	if m.GetProgramId() != "" {
		if programID, err = parseID(m.GetProgramId()); err != nil {
			return nil, err
		}
	}
	page, err := s.Logs.ListWorkoutRecords(ctx, id, m.GetDates().GetFrom(), m.GetDates().GetTo(), programID, m.GetCursor(), int(m.GetLimit()))
	if err != nil {
		return nil, logsError(err)
	}
	res := &pb.ListWorkoutRecordsResponse{Records: make([]*pb.WorkoutRecord, len(page.Items)), NextCursor: page.NextCursor, HasMore: page.HasMore}
	for i, r := range page.Items {
		res.Records[i] = recordPB(r)
	}
	return connect.NewResponse(res), nil
}

func (s *workoutServer) GetWorkoutRecord(ctx context.Context, req *connect.Request[pb.GetWorkoutRecordRequest]) (*connect.Response[pb.GetWorkoutRecordResponse], error) {
	id, recordID, err := owned(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	r, err := s.Logs.GetWorkoutRecord(ctx, id, recordID)
	if err != nil {
		return nil, logsError(err)
	}
	return connect.NewResponse(&pb.GetWorkoutRecordResponse{Record: recordPB(r)}), nil
}

func (s *workoutServer) CreateWorkoutRecord(ctx context.Context, req *connect.Request[pb.CreateWorkoutRecordRequest]) (*connect.Response[pb.CreateWorkoutRecordResponse], error) {
	m := req.Msg
	id, recordID, err := ownedWrite(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	in := logs.NewRecord{ID: recordID, Date: m.GetDate()}
	switch src := m.GetSource().(type) {
	case *pb.CreateWorkoutRecordRequest_ProgramDay:
		if pid := src.ProgramDay.GetProgramId(); pid != "" {
			if in.ProgramID, err = parseID(pid); err != nil {
				return nil, err
			}
		}
		in.Week, in.Day = int(src.ProgramDay.GetWeek()), int(src.ProgramDay.GetDay())
	case *pb.CreateWorkoutRecordRequest_AdHoc:
		unit, err := s.userUnit(ctx)
		if err != nil {
			return nil, err
		}
		in.AdHoc = &logs.AdHoc{Name: src.AdHoc.GetDayName(), Entries: entriesDB(src.AdHoc.GetEntries(), unit)}
	}
	r, err := s.Logs.CreateWorkoutRecord(ctx, id, in)
	if err != nil {
		return nil, logsError(err)
	}
	return connect.NewResponse(&pb.CreateWorkoutRecordResponse{Record: recordPB(r)}), nil
}

func (s *workoutServer) UpdateWorkoutRecord(ctx context.Context, req *connect.Request[pb.UpdateWorkoutRecordRequest]) (*connect.Response[pb.UpdateWorkoutRecordResponse], error) {
	m := req.Msg.GetRecord()
	id, recordID, err := ownedWrite(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	unit, err := s.userUnit(ctx)
	if err != nil {
		return nil, err
	}
	r, err := s.Logs.UpdateWorkoutRecord(ctx, id, recordID, logs.RecordInput{Entries: entriesDB(m.GetEntries(), unit),
		StartedAt: timePtr(m.GetStartedAt()), FinishedAt: timePtr(m.GetFinishedAt()), DeviceID: m.GetDeviceId()})
	if err != nil {
		return nil, logsError(err)
	}
	return connect.NewResponse(&pb.UpdateWorkoutRecordResponse{Record: recordPB(r)}), nil
}

func (s *workoutServer) DeleteWorkoutRecord(ctx context.Context, req *connect.Request[pb.DeleteWorkoutRecordRequest]) (*connect.Response[pb.DeleteWorkoutRecordResponse], error) {
	id, recordID, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := s.Logs.DeleteWorkoutRecord(ctx, id, recordID); err != nil {
		return nil, logsError(err)
	}
	return connect.NewResponse(&pb.DeleteWorkoutRecordResponse{}), nil
}

func (s *workoutServer) FinalizeWorkoutRecord(ctx context.Context, req *connect.Request[pb.FinalizeWorkoutRecordRequest]) (*connect.Response[pb.FinalizeWorkoutRecordResponse], error) {
	id, recordID, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	res, err := s.Logs.FinalizeWorkoutRecord(ctx, id, recordID)
	if err != nil {
		return nil, logsError(err)
	}
	return connect.NewResponse(&pb.FinalizeWorkoutRecordResponse{Record: recordPB(res.Record), Output: res.Output}), nil
}
