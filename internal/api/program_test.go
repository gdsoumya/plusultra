package api

import (
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const workoutScript = `program("Base", weeks=2)
day(1, 1, "Lower", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)))
day(1, 3, "Upper", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(60))])
`

const mealScript = `meal_program("Eat", weeks=1)
oats = food("Oats", per100g={"kcal": 380})
day(1, kind="training", name="Train")
meal("Breakfast", oats(100))
day(1, kind="rest", name="Rest")
meal("Breakfast", oats(50))
`

// programUser registers a user who knows the exercises the scripts use.
func (e *testEnv) programUser(t *testing.T, name, invite string) (string, plusultrav1connect.ProgramServiceClient) {
	t.Helper()
	reg := e.register(t, name, invite)
	id := uuid.MustParse(reg.GetUser().GetId())
	for _, ex := range []string{"Squat", "Bench Press"} {
		if _, err := catalog.CreateCustomExercise(t.Context(), sqlc.New(e.pool), id, catalog.Custom{Name: ex}); err != nil {
			t.Fatal(err)
		}
	}
	return reg.GetToken(), plusultrav1connect.NewProgramServiceClient(http.DefaultClient, e.url)
}

func create(t *testing.T, c plusultrav1connect.ProgramServiceClient, tok string, kind pb.ProgramKind, script string) *pb.Program {
	t.Helper()
	res, err := c.CreateProgram(t.Context(), as(tok, &pb.CreateProgramRequest{Kind: kind, Script: script}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return res.Msg.GetProgram()
}

func TestProgramLifecycleOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	ctx := t.Context()

	p := create(t, c, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, workoutScript)
	if p.GetStatus() != pb.ProgramStatus_PROGRAM_STATUS_DRAFT || p.GetRevision() != 1 || p.GetName() != "Base" ||
		p.GetScript() != workoutScript || p.GetMode() != pb.ProgramMode_PROGRAM_MODE_UNSPECIFIED ||
		p.GetStructure().GetWeeks() != 2 || len(p.GetStructure().GetDays()) != 4 || p.GetStructure().GetDays()[1].GetDay() != 3 {
		t.Fatalf("created %v", p)
	}
	_, err := c.CreateProgram(ctx, as(tok, &pb.CreateProgramRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Script: "program(\"x\",\n"}))
	assertCode(t, err, connect.CodeInvalidArgument)

	up, err := c.UpdateProgram(ctx, as(tok, &pb.UpdateProgramRequest{Id: p.GetId(), Script: strings.Replace(workoutScript, "kg(60)", "kg(70)", 1)}))
	if err != nil || up.Msg.GetProgram().GetRevision() != 2 {
		t.Fatalf("update: %v, %v", up, err)
	}

	act, err := c.ActivateProgram(ctx, as(tok, &pb.ActivateProgramRequest{Id: p.GetId(), StartDate: "2026-11-09"}))
	if err != nil {
		t.Fatal(err)
	}
	if a := act.Msg.GetProgram(); a.GetStatus() != pb.ProgramStatus_PROGRAM_STATUS_ACTIVE || a.GetMode() != pb.ProgramMode_PROGRAM_MODE_ANCHORED ||
		a.GetStartDate() != "2026-11-09" || a.GetEndDate() != "2026-11-22" || a.GetCursor() != nil {
		t.Fatalf("activated %v", a)
	}
	days, err := c.GetResolvedDays(ctx, as(tok, &pb.GetResolvedDaysRequest{ProgramId: p.GetId(), Week: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if d := days.Msg.GetDays(); len(d) != 7 || d[0].GetDate() != "2026-11-16" || d[0].GetProgramId() != p.GetId() || d[1].GetKind() != "rest" ||
		d[2].GetExercises()[0].GetSets()[0].GetWeight().GetValue() != 70 || !strings.HasPrefix(d[2].GetExercises()[0].GetSlug(), "custom-") ||
		len(days.Msg.GetMealDays()) != 0 {
		t.Fatalf("week 2 days %v", d)
	}

	other := create(t, c, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, workoutScript)
	_, err = c.ActivateProgram(ctx, as(tok, &pb.ActivateProgramRequest{Id: other.GetId()}))
	assertCode(t, err, connect.CodeFailedPrecondition)
	if fin, err := c.FinishProgram(ctx, as(tok, &pb.FinishProgramRequest{Id: p.GetId()})); err != nil ||
		fin.Msg.GetProgram().GetStatus() != pb.ProgramStatus_PROGRAM_STATUS_FINISHED {
		t.Fatalf("finish: %v, %v", fin, err)
	}
	act, err = c.ActivateProgram(ctx, as(tok, &pb.ActivateProgramRequest{Id: other.GetId()}))
	if err != nil {
		t.Fatal(err)
	}
	if a := act.Msg.GetProgram(); a.GetMode() != pb.ProgramMode_PROGRAM_MODE_FLOATING || a.GetStartDate() != "" ||
		a.GetCursor().GetNextWeek() != 1 || a.GetCursor().GetNextDay() != 1 {
		t.Fatalf("floating %v", a)
	}

	if _, err := c.DeleteProgram(ctx, as(tok, &pb.DeleteProgramRequest{Id: p.GetId()})); err != nil {
		t.Fatal(err)
	}
	_, err = c.GetProgram(ctx, as(tok, &pb.GetProgramRequest{Id: p.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.DeleteProgram(ctx, as(tok, &pb.DeleteProgramRequest{Id: p.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
}

func TestProgramListsPaginate(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	ctx := t.Context()
	var ids []string
	for range 3 {
		ids = append(ids, create(t, c, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, workoutScript).GetId())
	}
	create(t, c, tok, pb.ProgramKind_PROGRAM_KIND_MEAL, mealScript)

	first, err := c.ListPrograms(ctx, as(tok, &pb.ListProgramsRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Limit: 2}))
	if err != nil {
		t.Fatal(err)
	}
	ps := first.Msg.GetPrograms()
	if len(ps) != 2 || !first.Msg.GetHasMore() || ps[0].GetId() != ids[2] || ps[1].GetId() != ids[1] || ps[0].GetScript() != "" ||
		ps[0].GetStructure().GetWeeks() != 2 {
		t.Fatalf("first page %v", first.Msg)
	}
	next, err := c.ListPrograms(ctx, as(tok, &pb.ListProgramsRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Limit: 2, Cursor: first.Msg.GetNextCursor()}))
	if err != nil || len(next.Msg.GetPrograms()) != 1 || next.Msg.GetPrograms()[0].GetId() != ids[0] || next.Msg.GetHasMore() || next.Msg.GetNextCursor() != "" {
		t.Fatalf("second page %v, %v", next, err)
	}
	all, err := c.ListPrograms(ctx, as(tok, &pb.ListProgramsRequest{Status: pb.ProgramStatus_PROGRAM_STATUS_DRAFT}))
	if err != nil || len(all.Msg.GetPrograms()) != 4 {
		t.Fatalf("drafts %v, %v", all, err)
	}
	_, err = c.ListPrograms(ctx, as(tok, &pb.ListProgramsRequest{Cursor: "nope"}))
	assertCode(t, err, connect.CodeInvalidArgument)

	for _, s := range []string{"kg(61)", "kg(62)"} {
		if _, err := c.UpdateProgram(ctx, as(tok, &pb.UpdateProgramRequest{Id: ids[0], Script: strings.Replace(workoutScript, "kg(60)", s, 1)})); err != nil {
			t.Fatal(err)
		}
	}
	revs, err := c.ListRevisions(ctx, as(tok, &pb.ListRevisionsRequest{ProgramId: ids[0], Limit: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if r := revs.Msg.GetRevisions(); len(r) != 2 || r[0].GetRevision() != 3 || r[0].GetScript() != "" || r[0].GetCreatedBy() != "user" ||
		r[0].GetStructure().GetWeeks() != 2 || !revs.Msg.GetHasMore() {
		t.Fatalf("revisions %v", revs.Msg)
	}
	revs, err = c.ListRevisions(ctx, as(tok, &pb.ListRevisionsRequest{ProgramId: ids[0], IncludeScript: true, Cursor: revs.Msg.GetNextCursor()}))
	if err != nil || len(revs.Msg.GetRevisions()) != 1 || revs.Msg.GetRevisions()[0].GetScript() != workoutScript || revs.Msg.GetHasMore() {
		t.Fatalf("last revision page %v, %v", revs, err)
	}
}

func TestValidateAndSimulateOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	ctx := t.Context()

	v, err := c.ValidateProgram(ctx, as(tok, &pb.ValidateProgramRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT,
		Script: strings.Replace(workoutScript, `"Squat"`, `"Squatt"`, 1)}))
	if err != nil {
		t.Fatal(err)
	}
	if ne := v.Msg.GetNameErrors(); len(ne) != 1 || ne[0].GetLine() != 3 || ne[0].GetName() != "Squatt" || len(v.Msg.GetErrors()) != 0 || v.Msg.GetStructure() != nil {
		t.Fatalf("name errors %v", v.Msg)
	}
	v, err = c.ValidateProgram(ctx, as(tok, &pb.ValidateProgramRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Script: "program(\"x\", weeks=1)\nday(1, 1, \"A\")\nex(\"Squat\", sets=[])\n"}))
	if err != nil || len(v.Msg.GetErrors()) != 1 || v.Msg.GetErrors()[0].GetLine() != 3 || v.Msg.GetErrors()[0].GetMessage() == "" {
		t.Fatalf("script errors %v, %v", v, err)
	}
	v, err = c.ValidateProgram(ctx, as(tok, &pb.ValidateProgramRequest{Kind: pb.ProgramKind_PROGRAM_KIND_MEAL, Script: mealScript}))
	if err != nil || v.Msg.GetStructure().GetWeeks() != 1 || len(v.Msg.GetStructure().GetDays()) != 2 {
		t.Fatalf("valid meal %v, %v", v, err)
	}

	sim, err := c.SimulateProgram(ctx, as(tok, &pb.SimulateProgramRequest{Script: workoutScript, Weeks: 2,
		Assumptions: &pb.SimulateAssumptions{CompleteAll: true}}))
	if err != nil {
		t.Fatal(err)
	}
	if d := sim.Msg.GetDays(); len(d) != 4 || d[2].GetWeek() != 2 || d[2].GetExercises()[0].GetSets()[0].GetWeight().GetValue() != 102.5 ||
		d[2].GetExercises()[0].GetSlug() == "" {
		t.Fatalf("simulated %v", d)
	}
	sim, err = c.SimulateProgram(ctx, as(tok, &pb.SimulateProgramRequest{Script: "program(\"x\", weeks=1)\nday(1, 1, \"A\")\nex(\"Squat\", sets=[s(1, 5, pct(80))])\n", Weeks: 1}))
	if err != nil || len(sim.Msg.GetDays()) != 0 || len(sim.Msg.GetErrors()) != 1 || !strings.Contains(sim.Msg.GetErrors()[0].GetMessage(), "1RM") ||
		len(sim.Msg.GetWarnings()) != 1 || sim.Msg.GetWarnings()[0].GetCode() != "pct_without_rm1" {
		t.Fatalf("simulate without a 1RM: %v, %v", sim, err)
	}
}

func TestSimulateVerificationOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	sim, err := c.SimulateProgram(t.Context(), as(tok, &pb.SimulateProgramRequest{Script: workoutScript, Weeks: 2, Compact: true, Trace: true,
		AnchorStartDate: "2026-11-09",
		Scenario:        []*pb.SimulateSession{{Week: 1, Day: 1, Exercise: "Squat", MissLastSetBy: 2}},
		Filter:          &pb.SimulateFilter{Exercises: []string{"Squat"}},
		Expect:          []*pb.SimulateExpectation{{Week: 2, Day: 1, Exercise: "Squat", Weight: &pb.Weight{Value: 100}}},
		CompareScenario: []*pb.SimulateSession{{Week: 1, Day: 1, Exercise: "Squat", Skip: true}},
		Overrides:       &pb.SimulateOverrides{Rounding: map[string]*pb.Rounding{"Squat": {Kind: "stack", Increment: 5}}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	m := sim.Msg
	want := []string{"W1D1 Mon 2026-11-09 Lower | Squat 100x5 x3", "W2D1 Mon 2026-11-16 Lower | Squat 100x5 x3"}
	if len(m.GetDays()) != 0 || !slices.Equal(m.GetCompact(), want) || len(m.GetErrors()) != 0 {
		t.Fatalf("compact %q, days %d, errors %v", m.GetCompact(), len(m.GetDays()), m.GetErrors())
	}
	if tr := m.GetTrace(); len(tr) != 2 || tr[0].GetDate() != "2026-11-09" || tr[0].GetFunction() != "lp" || len(tr[0].GetWrites()) != 0 ||
		tr[0].GetStateAfter()["failures"] != "1" || tr[1].GetWrites()[0].GetValue() != "+2.5 kg" {
		t.Fatalf("trace %v", tr)
	}
	if a := m.GetAssertions(); len(a) != 1 || !a[0].GetOk() || a[0].GetExpect().GetExercise() != "Squat" || a[0].GetActual() != "100 kg x5" {
		t.Fatalf("assertions %v", a)
	}
	if len(m.GetDiff()) != 0 { // skipping and failing week 1 both leave week 2 at 100
		t.Fatalf("diff %v", m.GetDiff())
	}
	codes := map[string]int{}
	for _, w := range m.GetWarnings() {
		codes[w.GetCode()]++
	}
	if codes["rounding_source"] != 2 || codes["stalled_progression"] != 0 {
		t.Fatalf("warnings %v", m.GetWarnings())
	}
	bad := &pb.SimulateProgramRequest{Weeks: 1}
	_, err = c.SimulateProgram(t.Context(), as(tok, bad))
	assertCode(t, err, connect.CodeInvalidArgument) // neither script nor program_id
}

func TestProgramMealDaysOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	m := create(t, c, tok, pb.ProgramKind_PROGRAM_KIND_MEAL, mealScript)
	if _, err := c.ActivateProgram(t.Context(), as(tok, &pb.ActivateProgramRequest{Id: m.GetId()})); err != nil {
		t.Fatal(err)
	}
	res, err := c.GetResolvedDays(t.Context(), as(tok, &pb.GetResolvedDaysRequest{ProgramId: m.GetId()}))
	if err != nil {
		t.Fatal(err)
	}
	md := res.Msg.GetMealDays()
	if len(md) != 2 || len(res.Msg.GetDays()) != 0 || md[0].GetKind() != "training" || md[0].GetDay() != 0 || md[0].GetName() != "Train" ||
		md[1].GetTotals().GetKcal() != 190 || md[0].GetTarget().GetKcal() != 380 || md[0].GetMeals()[0].GetItems()[0].GetFood().GetPer100G().GetKcal() != 380 {
		t.Fatalf("meal days %v", md)
	}
}

func TestProgramWritesNeedScopeAndOwnership(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	p := create(t, c, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, workoutScript)
	read := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err := c.CreateProgram(t.Context(), as(read, &pb.CreateProgramRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Script: workoutScript}))
	assertCode(t, err, connect.CodePermissionDenied)
	_, err = c.ActivateProgram(t.Context(), as(read, &pb.ActivateProgramRequest{Id: p.GetId()}))
	assertCode(t, err, connect.CodePermissionDenied)
	if _, err := c.GetProgram(t.Context(), as(read, &pb.GetProgramRequest{Id: p.GetId()})); err != nil {
		t.Fatalf("read key get: %v", err)
	}

	write := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ, pb.ApiKeyScope_API_KEY_SCOPE_WRITE).GetKey()
	if _, err := c.UpdateProgram(t.Context(), as(write, &pb.UpdateProgramRequest{Id: p.GetId(), Script: workoutScript})); err != nil {
		t.Fatal(err)
	}
	revs, err := c.ListRevisions(t.Context(), as(tok, &pb.ListRevisionsRequest{ProgramId: p.GetId()}))
	if err != nil || revs.Msg.GetRevisions()[0].GetCreatedBy() != "api" {
		t.Fatalf("revisions %v, %v", revs, err)
	}

	inv, err := e.auth.CreateInvite(t.Context(), as(tok, &pb.CreateInviteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	bob, _ := e.programUser(t, "bob", inv.Msg.GetInvite().GetCode())
	_, err = c.GetProgram(t.Context(), as(bob, &pb.GetProgramRequest{Id: p.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.GetResolvedDays(t.Context(), as(bob, &pb.GetResolvedDaysRequest{ProgramId: p.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
}

func TestSimulateMealProgramOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok, c := e.programUser(t, "alice", "")
	src, err := os.ReadFile("../engine/testdata/examples/meals/recomp.star")
	if err != nil {
		t.Fatal(err)
	}
	sim, err := c.SimulateProgram(t.Context(), as(tok, &pb.SimulateProgramRequest{Script: string(src), Weeks: 2}))
	if err != nil {
		t.Fatal(err)
	}
	md := sim.Msg.GetMealDays()
	if len(sim.Msg.GetDays()) != 0 || len(sim.Msg.GetErrors()) != 0 || len(md) != 6 || md[0].GetWeek() != 1 || md[0].GetKind() != "training" ||
		md[0].GetTarget().GetKcal() != 2600 || md[0].GetTotals().GetKcal() == 0 || len(md[0].GetMeals()) != 4 || md[5].GetWeek() != 2 || md[5].GetKind() != "rest" {
		t.Fatalf("simulated meal days %v", sim.Msg)
	}
	if ps, err := c.ListPrograms(t.Context(), as(tok, &pb.ListProgramsRequest{})); err != nil || len(ps.Msg.GetPrograms()) != 0 {
		t.Fatalf("simulate saved a program: %v, %v", ps, err)
	}
}
