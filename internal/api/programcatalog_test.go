package api

import (
	"testing"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
)

func TestProgramCatalogOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "cat", "")
	ctx := t.Context()

	list := call(c.ListCatalog(ctx, as(tok, &pb.ListCatalogRequest{}))).ok(t)
	if len(list.GetPrograms()) < 3 {
		t.Fatalf("catalog: %v", list)
	}
	first := list.GetPrograms()[0]
	if first.GetScript() != "" || first.GetLevel() != "beginner" || first.GetWeeks() == 0 || first.GetDaysPerWeek() == 0 {
		t.Fatalf("listed %v", first)
	}

	got := call(c.GetCatalogProgram(ctx, as(tok, &pb.GetCatalogProgramRequest{Slug: first.GetSlug()}))).ok(t).GetProgram()
	if got.GetName() != first.GetName() || got.GetScript() == "" || len(got.GetMaxes()) != len(first.GetMaxes()) {
		t.Fatalf("got %v", got)
	}

	_, err := c.GetCatalogProgram(ctx, as(tok, &pb.GetCatalogProgramRequest{Slug: "nope"}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.ListCatalog(ctx, connect.NewRequest(&pb.ListCatalogRequest{}))
	assertCode(t, err, connect.CodeUnauthenticated)
}
