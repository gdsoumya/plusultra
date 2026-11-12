package api

import (
	"cmp"
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/export"
)

const (
	maxImportBytes  = 8 << 20   // an Import request
	maxExportBytes  = 64 << 20  // an ExportAll response
	maxProgramBytes = 256 << 10 // a program script inside an Import
)

var (
	formatNames = [...]string{
		pb.ExportFormat_EXPORT_FORMAT_TEXT: export.FormatText,
		pb.ExportFormat_EXPORT_FORMAT_JSON: export.FormatJSON,
		pb.ExportFormat_EXPORT_FORMAT_CSV:  export.FormatCSV,
	}
	contentTypes = map[string]string{export.FormatText: "text/plain", export.FormatJSON: "application/json", export.FormatCSV: "text/csv"}
	importKinds  = [...]string{
		pb.ImportKind_IMPORT_KIND_PROGRAM: export.KindProgram,
		pb.ImportKind_IMPORT_KIND_DAY_LOG: export.KindDayLog,
	}
)

type exportServer struct{ export *export.Service }

func (s *exportServer) ExportProgram(ctx context.Context, req *connect.Request[pb.ExportProgramRequest]) (*connect.Response[pb.ExportProgramResponse], error) {
	userID, id, err := owned(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	text, name, err := s.export.ExportProgram(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ExportProgramResponse{Script: text, Filename: name}), nil
}

func (s *exportServer) ExportDays(ctx context.Context, req *connect.Request[pb.ExportDaysRequest]) (*connect.Response[pb.ExportDaysResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	from, err1 := time.Parse(time.DateOnly, req.Msg.GetDates().GetFrom())
	to, err2 := time.Parse(time.DateOnly, req.Msg.GetDates().GetTo())
	if err := cmp.Or(err1, err2); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("dates must be YYYY-MM-DD"))
	}
	format := cmp.Or(formatNames[req.Msg.GetFormat()], export.FormatText)
	b, err := s.export.ExportDays(ctx, userID, from, to, format)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ExportDaysResponse{Content: string(b), ContentType: contentTypes[format]}), nil
}

func (s *exportServer) ExportAll(ctx context.Context, _ *connect.Request[pb.ExportAllRequest]) (*connect.Response[pb.ExportAllResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	b, err := s.export.ExportAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ExportAllResponse{Zip: b, Filename: "plusultra-" + time.Now().UTC().Format(time.DateOnly) + ".zip"}), nil
}

func (s *exportServer) Import(ctx context.Context, req *connect.Request[pb.ImportRequest]) (*connect.Response[pb.ImportResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	if int(req.Msg.GetKind()) >= len(importKinds) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown import kind"))
	}
	kind := importKinds[req.Msg.GetKind()]
	if kind == export.KindProgram && len(req.Msg.GetContent()) > maxProgramBytes {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a program script is at most 256 KiB"))
	}
	res, err := s.export.Import(ctx, userID, kind, req.Msg.GetContent())
	if err != nil {
		return nil, err
	}
	out := &pb.ImportResponse{Dates: res.Dates, Warnings: res.Warnings}
	if res.Program != nil {
		if out.Program, err = programPB(*res.Program, true); err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(out), nil
}
