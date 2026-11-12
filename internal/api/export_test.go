package api

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
)

func TestExportAndImportOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, pc := e.programUser(t, "alice", "")
	ec := plusultrav1connect.NewExportServiceClient(http.DefaultClient, e.url)
	ctx := t.Context()
	p := create(t, pc, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, workoutScript)

	ex, err := ec.ExportProgram(ctx, as(tok, &pb.ExportProgramRequest{Id: p.GetId()}))
	if err != nil || ex.Msg.GetScript() != workoutScript || ex.Msg.GetFilename() != "base.star" {
		t.Fatalf("export program %v, %v", ex, err)
	}
	imp, err := ec.Import(ctx, as(tok, &pb.ImportRequest{Kind: pb.ImportKind_IMPORT_KIND_PROGRAM, Content: ex.Msg.GetScript()}))
	if err != nil {
		t.Fatal(err)
	}
	if ip := imp.Msg.GetProgram(); ip.GetId() == p.GetId() || ip.GetRevision() != 1 || ip.GetStatus() != pb.ProgramStatus_PROGRAM_STATUS_DRAFT ||
		ip.GetScript() != workoutScript || len(imp.Msg.GetDates()) != 0 {
		t.Fatalf("imported %v", imp.Msg)
	}
	_, err = ec.Import(ctx, as(tok, &pb.ImportRequest{Kind: pb.ImportKind_IMPORT_KIND_PROGRAM, Content: "program(\"x\",\n"}))
	assertCode(t, err, connect.CodeInvalidArgument)
	_, err = ec.Import(ctx, as(tok, &pb.ImportRequest{Kind: pb.ImportKind_IMPORT_KIND_PROGRAM, Content: strings.Repeat("#", 256<<10+1)}))
	assertCode(t, err, connect.CodeInvalidArgument)

	const text = "2026-11-11 / status: rest / notes: \"easy\"\n"
	imp, err = ec.Import(ctx, as(tok, &pb.ImportRequest{Kind: pb.ImportKind_IMPORT_KIND_DAY_LOG, Content: text}))
	if err != nil || imp.Msg.GetProgram() != nil || strings.Join(imp.Msg.GetDates(), ",") != "2026-11-11" {
		t.Fatalf("import day log %v, %v", imp, err)
	}
	dates := &pb.DateRange{From: "2026-11-11", To: "2026-11-11"}
	days, err := ec.ExportDays(ctx, as(tok, &pb.ExportDaysRequest{Dates: dates}))
	if err != nil || days.Msg.GetContent() != text || days.Msg.GetContentType() != "text/plain" {
		t.Fatalf("export text %v, %v", days, err)
	}
	days, err = ec.ExportDays(ctx, as(tok, &pb.ExportDaysRequest{Dates: dates, Format: pb.ExportFormat_EXPORT_FORMAT_JSON}))
	if err != nil || !strings.Contains(days.Msg.GetContent(), `"status": "rest"`) || days.Msg.GetContentType() != "application/json" {
		t.Fatalf("export json %v, %v", days, err)
	}
	days, err = ec.ExportDays(ctx, as(tok, &pb.ExportDaysRequest{Dates: dates, Format: pb.ExportFormat_EXPORT_FORMAT_CSV}))
	if err != nil || !strings.HasPrefix(days.Msg.GetContent(), "date,status,notes,readiness,row,") || days.Msg.GetContentType() != "text/csv" {
		t.Fatalf("export csv %v, %v", days, err)
	}

	all, err := ec.ExportAll(ctx, as(tok, &pb.ExportAllRequest{}))
	if err != nil || !strings.HasPrefix(all.Msg.GetFilename(), "plusultra-") || !strings.HasSuffix(all.Msg.GetFilename(), ".zip") {
		t.Fatalf("export all %v, %v", all, err)
	}
	zr, err := zip.NewReader(bytes.NewReader(all.Msg.GetZip()), int64(len(all.Msg.GetZip())))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["programs/base.star"] || !names["programs/base-2.star"] || !names["days/2026/11/11.txt"] || !names["days.json"] || !names["checkins.csv"] {
		t.Fatalf("zip files %v", names)
	}

	read := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = ec.Import(ctx, as(read, &pb.ImportRequest{Kind: pb.ImportKind_IMPORT_KIND_DAY_LOG, Content: text}))
	assertCode(t, err, connect.CodePermissionDenied)
	if _, err := ec.ExportProgram(ctx, as(read, &pb.ExportProgramRequest{Id: p.GetId()})); err != nil {
		t.Fatalf("read key export: %v", err)
	}
	inv, err := e.auth.CreateInvite(ctx, as(tok, &pb.CreateInviteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	bob, _ := e.programUser(t, "bob", inv.Msg.GetInvite().GetCode())
	_, err = ec.ExportProgram(ctx, as(bob, &pb.ExportProgramRequest{Id: p.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
}
