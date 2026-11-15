package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/programcatalog"
)

func catalogProgramPB(e programcatalog.Entry, script bool) *pb.CatalogProgram {
	out := &pb.CatalogProgram{
		Slug: e.Slug, Name: e.Name, Author: e.Author, Level: e.Level,
		DaysPerWeek: i32(e.Days), Weeks: i32(e.Weeks), Loop: e.Loop, Summary: e.Summary, Maxes: e.Maxes,
	}
	if script {
		out.Script = e.Script
	}
	return out
}

func (s *programServer) ListCatalog(ctx context.Context, _ *connect.Request[pb.ListCatalogRequest]) (*connect.Response[pb.ListCatalogResponse], error) {
	if _, err := callerID(ctx); err != nil {
		return nil, err
	}
	res := &pb.ListCatalogResponse{}
	for _, e := range programcatalog.All() {
		res.Programs = append(res.Programs, catalogProgramPB(e, false))
	}
	return connect.NewResponse(res), nil
}

func (s *programServer) GetCatalogProgram(ctx context.Context, req *connect.Request[pb.GetCatalogProgramRequest]) (*connect.Response[pb.GetCatalogProgramResponse], error) {
	if _, err := callerID(ctx); err != nil {
		return nil, err
	}
	e, ok := programcatalog.Get(req.Msg.GetSlug())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no catalog program %q", req.Msg.GetSlug()))
	}
	return connect.NewResponse(&pb.GetCatalogProgramResponse{Program: catalogProgramPB(e, true)}), nil
}
