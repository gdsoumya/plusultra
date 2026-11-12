package api

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

type checkinServer struct {
	checkins *logs.CheckinService
	q        *sqlc.Queries
}

func (s *checkinServer) ListCheckins(ctx context.Context, req *connect.Request[pb.ListCheckinsRequest]) (*connect.Response[pb.ListCheckinsResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	unit, err := userUnit(ctx, s.q, userID)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	page, err := s.checkins.List(ctx, userID, m.GetDates().GetFrom(), m.GetDates().GetTo(), m.GetCursor(), int(m.GetLimit()), m.GetIncludePhotos())
	if err != nil {
		return nil, err
	}
	res := &pb.ListCheckinsResponse{NextCursor: page.NextCursor, HasMore: page.HasMore}
	for _, c := range page.Items {
		res.Checkins = append(res.Checkins, checkinPB(c, unit))
	}
	return connect.NewResponse(res), nil
}

func (s *checkinServer) CreateCheckin(ctx context.Context, req *connect.Request[pb.CreateCheckinRequest]) (*connect.Response[pb.CreateCheckinResponse], error) {
	userID, unit, in, err := s.input(ctx, req.Msg.GetCheckin())
	if err != nil {
		return nil, err
	}
	c, err := s.checkins.Create(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateCheckinResponse{Checkin: checkinPB(c, unit)}), nil
}

func (s *checkinServer) UpdateCheckin(ctx context.Context, req *connect.Request[pb.UpdateCheckinRequest]) (*connect.Response[pb.UpdateCheckinResponse], error) {
	userID, unit, in, err := s.input(ctx, req.Msg.GetCheckin())
	if err != nil {
		return nil, err
	}
	c, err := s.checkins.Update(ctx, userID, in.ID, in)
	if err == nil && req.Msg.GetClearPhoto() {
		c, err = s.checkins.ClearPhoto(ctx, userID, in.ID)
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateCheckinResponse{Checkin: checkinPB(c, unit)}), nil
}

func (s *checkinServer) DeleteCheckin(ctx context.Context, req *connect.Request[pb.DeleteCheckinRequest]) (*connect.Response[pb.DeleteCheckinResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteCheckinResponse{}), s.checkins.Delete(ctx, userID, id)
}

// input is the writing caller, their unit, and the service input for m read in that unit.
func (s *checkinServer) input(ctx context.Context, m *pb.Checkin) (uuid.UUID, model.Unit, logs.CheckinInput, error) {
	userID, err := writer(ctx)
	if err != nil {
		return userID, "", logs.CheckinInput{}, err
	}
	unit, err := userUnit(ctx, s.q, userID)
	if err != nil {
		return userID, unit, logs.CheckinInput{}, err
	}
	in, err := checkinInput(m, unit)
	return userID, unit, in, err
}
