package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/programs"
	syncsvc "github.com/gdsoumya/plusultra/internal/sync"
)

const maxSyncBytes = 4 << 20 // a push carries up to 1000 rows per table

type syncServer struct {
	Deps
	svc *syncsvc.Service
}

func newSyncServer(d Deps, progs *programs.Service) *syncServer {
	svc := syncsvc.New(d.Queries, d.Pool, d.Logs, progs, d.Finalizer)
	svc.Logger = d.Logger
	return &syncServer{d, svc}
}

func (s *syncServer) Pull(ctx context.Context, req *connect.Request[pb.PullRequest]) (*connect.Response[pb.PullResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	unit, err := s.userUnit(ctx)
	if err != nil {
		return nil, err
	}
	c, err := s.svc.Pull(ctx, userID, req.Msg.GetCursor(), int(req.Msg.GetLimit()), time.Now())
	if err != nil {
		return nil, syncError(err)
	}
	res, err := syncPullPB(c, unit)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (s *syncServer) Push(ctx context.Context, req *connect.Request[pb.PushRequest]) (*connect.Response[pb.PushResponse], error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return nil, err
	}
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	unit, err := s.userUnit(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := syncRowsIn(req.Msg.GetRows(), unit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	res, err := s.svc.Push(ctx, userID, req.Msg.GetDeviceId(), rows)
	if err != nil {
		return nil, syncError(err)
	}
	out := &pb.PushResponse{
		Conflicts: syncRowsPB(res.Conflicts, unit), AppliedIds: make([]string, len(res.Applied)), Reasons: make(map[string]string, len(res.Reasons)),
	}
	for i, id := range res.Applied {
		out.AppliedIds[i] = id.String()
	}
	for id, reason := range res.Reasons {
		out.Reasons[id.String()] = reason
	}
	return connect.NewResponse(out), nil
}

// syncError makes the service's ErrInvalid an InvalidArgument; publicError hides the rest.
func syncError(err error) error {
	if errors.Is(err, syncsvc.ErrInvalid) {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return err
}
