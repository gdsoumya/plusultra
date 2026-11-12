package api

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// seedBench adds a catalog exercise with one image file under the env's images dir.
func (e *testEnv) seedBench(t *testing.T) {
	t.Helper()
	if _, err := sqlc.New(e.pool).UpsertExercise(t.Context(), sqlc.UpsertExerciseParams{
		Slug: "bench-press-barbell", Name: "Bench Press, Barbell", Equipment: "barbell", Aliases: []string{"bench press"},
		PrimaryMuscles: []string{"chest"}, SecondaryMuscles: []string{"triceps"}, Category: "strength",
		Images: []string{"bench-press-barbell/0.jpg"}, Instructions: []string{"Lie on the bench.", "Press."}, Source: "curated",
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(e.images, "bench-press-barbell")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0.jpg"), []byte("jpeg bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExerciseCatalogOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	e.seedBench(t)
	tok := e.register(t, "alice", "").GetToken()
	c := plusultrav1connect.NewExerciseServiceClient(http.DefaultClient, e.url)

	found, err := c.SearchExercises(ctx, as(tok, &pb.SearchExercisesRequest{Query: "bench", Equipment: "barbell"}))
	if err != nil || len(found.Msg.GetExercises()) != 1 {
		t.Fatalf("search = %v, err %v", found, err)
	}
	bench := found.Msg.GetExercises()[0]
	if bench.GetSlug() != "bench-press-barbell" || bench.GetId() == "" || bench.GetCustom() || bench.GetPrimaryMuscles()[0] != "chest" ||
		bench.GetCategory() != "strength" || len(bench.GetImages()) != 1 || bench.GetImages()[0] != httpsURL+Prefix+"/exercises/images/bench-press-barbell/0.jpg" {
		t.Fatalf("bench = %v", bench)
	}
	if code, body := e.get(t, Prefix+"/exercises/images/bench-press-barbell/0.jpg"); code != http.StatusOK || body != "jpeg bytes" {
		t.Fatalf("image = %d %q", code, body)
	}
	for _, p := range []string{"/exercises/images/bench-press-barbell/0.jpg", Prefix + "/exercises/images/bench-press-barbell/sub", Prefix + "/exercises/images/bench-press-barbell/", Prefix + "/exercises/images/bench-press-barbell/1.jpg"} {
		if code, _ := e.get(t, p); code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, code)
		}
	}
	if none, err := c.SearchExercises(ctx, as(tok, &pb.SearchExercisesRequest{Query: "bench", Muscle: "quads"})); err != nil || len(none.Msg.GetExercises()) != 0 {
		t.Fatalf("muscle filter = %v, err %v", none, err)
	}
	got, err := c.GetExercise(ctx, as(tok, &pb.GetExerciseRequest{Slug: "bench-press-barbell"}))
	if err != nil || got.Msg.GetExercise().GetName() != "Bench Press, Barbell" || !slices.Equal(got.Msg.GetExercise().GetInstructions(), []string{"Lie on the bench.", "Press."}) {
		t.Fatalf("get = %v, err %v", got, err)
	}
	_, err = c.GetExercise(ctx, as(tok, &pb.GetExerciseRequest{Slug: "nope"}))
	assertCode(t, err, connect.CodeNotFound)

	steps := []string{"Hold the bar in the crooks of the elbows.", "Squat."}
	custom := &pb.Exercise{Name: "Zercher Squat", Equipment: "Barbell", PrimaryMuscles: []string{"quads"}, Category: "strength", Instructions: steps}
	created, err := c.CreateCustomExercise(ctx, as(tok, &pb.CreateCustomExerciseRequest{Exercise: custom}))
	if err != nil {
		t.Fatal(err)
	}
	z := created.Msg.GetExercise()
	if !z.GetCustom() || z.GetEquipment() != "barbell" || !strings.HasPrefix(z.GetSlug(), "custom-") || !strings.HasSuffix(z.GetSlug(), "-zercher-squat-barbell") || z.GetId() == "" ||
		!slices.Equal(z.GetInstructions(), steps) {
		t.Fatalf("custom = %v", z)
	}
	_, err = c.CreateCustomExercise(ctx, as(tok, &pb.CreateCustomExerciseRequest{Exercise: custom}))
	assertCode(t, err, connect.CodeAlreadyExists)
	for _, bad := range [][]string{slices.Repeat([]string{"step"}, 21), {strings.Repeat("x", 2001)}} {
		_, err = c.CreateCustomExercise(ctx, as(tok, &pb.CreateCustomExerciseRequest{Exercise: &pb.Exercise{Name: "Too much", Instructions: bad}}))
		assertCode(t, err, connect.CodeInvalidArgument)
	}
	if hits, err := c.SearchExercises(ctx, as(tok, &pb.SearchExercisesRequest{Query: "zercher"})); err != nil || len(hits.Msg.GetExercises()) != 1 {
		t.Fatalf("search custom = %v, err %v", hits, err)
	}

	bob := e.invitee(t, tok, "bob").GetToken()
	if hits, err := c.SearchExercises(ctx, as(bob, &pb.SearchExercisesRequest{Query: "zercher"})); err != nil || len(hits.Msg.GetExercises()) != 0 {
		t.Fatalf("bob sees alice's exercise: %v, err %v", hits, err)
	}
	_, err = c.GetExercise(ctx, as(bob, &pb.GetExerciseRequest{Slug: z.GetSlug()}))
	assertCode(t, err, connect.CodeNotFound)
	z.Name = "Zercher Squat (low)"
	_, err = c.UpdateCustomExercise(ctx, as(bob, &pb.UpdateCustomExerciseRequest{Exercise: z}))
	assertCode(t, err, connect.CodeNotFound)
	up, err := c.UpdateCustomExercise(ctx, as(tok, &pb.UpdateCustomExerciseRequest{Exercise: z}))
	if err != nil || up.Msg.GetExercise().GetName() != "Zercher Squat (low)" || up.Msg.GetExercise().GetSlug() != z.GetSlug() {
		t.Fatalf("update = %v, err %v", up, err)
	}
	_, err = c.UpdateCustomExercise(ctx, as(tok, &pb.UpdateCustomExerciseRequest{Exercise: &pb.Exercise{Slug: "bench-press-barbell", Name: "Mine now"}}))
	assertCode(t, err, connect.CodeNotFound)

	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = c.DeleteCustomExercise(ctx, as(ro, &pb.DeleteCustomExerciseRequest{Slug: z.GetSlug()}))
	assertCode(t, err, connect.CodePermissionDenied)
	if _, err := c.DeleteCustomExercise(ctx, as(tok, &pb.DeleteCustomExerciseRequest{Slug: z.GetSlug()})); err != nil {
		t.Fatal(err)
	}
	_, err = c.GetExercise(ctx, as(tok, &pb.GetExerciseRequest{Slug: z.GetSlug()}))
	assertCode(t, err, connect.CodeNotFound)
}

func TestExerciseDataAndEquipmentOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	e.seedBench(t)
	tok := e.register(t, "alice", "").GetToken()
	e.setUnits(t, tok, pb.Unit_UNIT_LB)
	c := plusultrav1connect.NewExerciseServiceClient(http.DefaultClient, e.url)
	const slug = "bench-press-barbell"

	empty := call(c.GetExerciseData(ctx, as(tok, &pb.GetExerciseDataRequest{Slug: slug}))).ok(t).GetData()
	if empty.GetSlug() != slug || empty.GetRm1() != nil || empty.GetRounding() != nil || empty.GetUpdatedAt() != nil {
		t.Fatalf("empty data = %v", empty)
	}
	in := &pb.ExerciseData{Slug: slug, Rm1: &pb.Weight{Value: 220}, Rounding: &pb.Rounding{Kind: "plates", Bar: 45, Plates: []float64{45, 25}}, Notes: "touch and go", Unilateral: true}
	set := call(c.SetExerciseData(ctx, as(tok, &pb.SetExerciseDataRequest{Data: in}))).ok(t).GetData()
	if math.Abs(set.GetRm1().GetValue()-220) > 1e-6 || set.GetRm1().GetUnit() != pb.Unit_UNIT_LB || set.GetRounding().GetBar() != 45 ||
		len(set.GetRounding().GetPlates()) != 2 || set.GetNotes() != "touch and go" || set.GetUpdatedAt() == nil || !set.GetUnilateral() {
		t.Fatalf("set = %v", set)
	}
	e.setUnits(t, tok, pb.Unit_UNIT_KG)
	got := call(c.GetExerciseData(ctx, as(tok, &pb.GetExerciseDataRequest{Slug: slug}))).ok(t).GetData()
	if math.Abs(got.GetRm1().GetValue()-99.79) > 0.01 || got.GetRm1().GetUnit() != pb.Unit_UNIT_KG {
		t.Fatalf("1RM in kg = %v", got)
	}
	list, err := c.ListExerciseData(ctx, as(tok, &pb.ListExerciseDataRequest{}))
	if err != nil || len(list.Msg.GetItems()) != 1 || list.Msg.GetItems()[0].GetSlug() != slug || list.Msg.GetItems()[0].GetRm1() == nil || !list.Msg.GetItems()[0].GetUnilateral() {
		t.Fatalf("list = %v, err %v", list, err)
	}
	cleared := call(c.SetExerciseData(ctx, as(tok, &pb.SetExerciseDataRequest{Data: &pb.ExerciseData{Slug: slug, Notes: "x"}}))).ok(t).GetData()
	if cleared.GetRm1() != nil || cleared.GetRounding() != nil || cleared.GetNotes() != "x" || cleared.GetUnilateral() {
		t.Fatalf("cleared = %v", cleared)
	}
	_, err = c.SetExerciseData(ctx, as(tok, &pb.SetExerciseDataRequest{Data: &pb.ExerciseData{Slug: "nope", Notes: "x"}}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.SetExerciseData(ctx, as(tok, &pb.SetExerciseDataRequest{Data: &pb.ExerciseData{Slug: slug, Rm1: &pb.Weight{Value: -5}}}))
	assertCode(t, err, connect.CodeInvalidArgument)
	bob := e.invitee(t, tok, "bob").GetToken()
	if list, err := c.ListExerciseData(ctx, as(bob, &pb.ListExerciseDataRequest{})); err != nil || len(list.Msg.GetItems()) != 0 {
		t.Fatalf("bob sees alice's data: %v, err %v", list, err)
	}

	none, err := c.GetEquipment(ctx, as(tok, &pb.GetEquipmentRequest{}))
	if err != nil || len(none.Msg.GetEquipment()) != 0 {
		t.Fatalf("initial equipment = %v, err %v", none, err)
	}
	eq := []*pb.Equipment{
		{Name: "Barbell", Rounding: &pb.Rounding{Kind: "plates", Bar: 20, Plates: []float64{20, 10, 5}}},
		{Name: "Stack", Rounding: &pb.Rounding{Kind: "stack", Increment: 5}},
	}
	set2, err := c.SetEquipment(ctx, as(tok, &pb.SetEquipmentRequest{Equipment: eq}))
	if err != nil || len(set2.Msg.GetEquipment()) != 2 || set2.Msg.GetEquipment()[0].GetRounding().GetBar() != 20 {
		t.Fatalf("set equipment = %v, err %v", set2, err)
	}
	again := call(c.SetEquipment(ctx, as(tok, &pb.SetEquipmentRequest{Equipment: eq[1:]}))).ok(t).GetEquipment()
	if len(again) != 1 || again[0].GetName() != "Stack" || again[0].GetRounding().GetIncrement() != 5 {
		t.Fatalf("replace equipment = %v", again)
	}
	_, err = c.SetEquipment(ctx, as(tok, &pb.SetEquipmentRequest{Equipment: []*pb.Equipment{{Name: "Dumbbell", Rounding: &pb.Rounding{Kind: "fixed"}}}}))
	assertCode(t, err, connect.CodeInvalidArgument)
	if got, err := c.GetEquipment(ctx, as(tok, &pb.GetEquipmentRequest{})); err != nil || len(got.Msg.GetEquipment()) != 1 || got.Msg.GetEquipment()[0].GetName() != "Stack" {
		t.Fatalf("a failed set changed the list: %v, err %v", got, err)
	}
	if got, err := c.GetEquipment(ctx, as(bob, &pb.GetEquipmentRequest{})); err != nil || len(got.Msg.GetEquipment()) != 0 {
		t.Fatalf("bob sees alice's equipment: %v, err %v", got, err)
	}
	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = c.SetEquipment(ctx, as(ro, &pb.SetEquipmentRequest{Equipment: eq}))
	assertCode(t, err, connect.CodePermissionDenied)
}

// A new 1RM re-resolves the active workout program, so percentage weights update on the calendar.
func TestSetExerciseDataReresolvesActivePrograms(t *testing.T) {
	e := newEnv(t, httpsURL+"/")
	ctx := t.Context()
	tok, pc := e.programUser(t, "alice", "")
	c := plusultrav1connect.NewExerciseServiceClient(http.DefaultClient, e.url)

	today := time.Now().UTC()
	weekday := (int(today.Weekday())+6)%7 + 1 // Monday is 1
	script := fmt.Sprintf("program(\"Pct\", weeks=1)\nday(1, %d, \"Lower\")\nex(\"Squat\", warmup=[], sets=[s(3, 5, pct(80))])\n", weekday)
	me, err := e.me(t, tok)
	if err != nil {
		t.Fatal(err)
	}
	slug := "custom-" + me.GetId()[:8] + "-squat"
	rm1 := func(kg float64) {
		call(c.SetExerciseData(ctx, as(tok, &pb.SetExerciseDataRequest{Data: &pb.ExerciseData{Slug: slug, Rm1: &pb.Weight{Value: kg, Unit: pb.Unit_UNIT_KG}}}))).ok(t)
	}
	rm1(50) // activation needs a 1RM for pct() sets
	p := create(t, pc, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, script)
	call(pc.ActivateProgram(ctx, as(tok, &pb.ActivateProgramRequest{Id: p.GetId(), StartDate: today.Format(time.DateOnly)}))).ok(t)

	squat := func() *pb.ResolvedExercise {
		for _, d := range call(pc.GetResolvedDays(ctx, as(tok, &pb.GetResolvedDaysRequest{ProgramId: p.GetId(), Week: 1}))).ok(t).GetDays() {
			if d.GetDate() == today.Format(time.DateOnly) && len(d.GetExercises()) == 1 {
				return d.GetExercises()[0]
			}
		}
		t.Fatal("no resolved training day for today")
		return nil
	}
	before := squat()
	if before.GetSlug() != slug || before.GetSets()[0].GetWeight().GetValue() != 40 {
		t.Fatalf("with a 50 kg 1RM: %v", before)
	}
	rm1(100)
	if w := squat().GetSets()[0].GetWeight(); w.GetValue() != 80 || w.GetUnit() != pb.Unit_UNIT_KG {
		t.Fatalf("after a 100 kg 1RM, pct(80) = %v, want 80 kg", w)
	}
	call(c.SetEquipment(ctx, as(tok, &pb.SetEquipmentRequest{Equipment: []*pb.Equipment{{Name: "Barbell", Rounding: &pb.Rounding{Kind: "stack", Increment: 5}}}}))).ok(t)
	if w := squat().GetSets()[0].GetWeight(); w.GetValue() != 80 {
		t.Fatalf("after equipment change = %v", w)
	}

	// R76: exercise data applies to every stored week, past ones included.
	call(pc.FinishProgram(ctx, as(tok, &pb.FinishProgramRequest{Id: p.GetId()}))).ok(t)
	lastWeek := today.AddDate(0, 0, -7)
	p2 := create(t, pc, tok, pb.ProgramKind_PROGRAM_KIND_WORKOUT, strings.Replace(script, "weeks=1", "weeks=2", 1))
	call(pc.ActivateProgram(ctx, as(tok, &pb.ActivateProgramRequest{Id: p2.GetId(), StartDate: lastWeek.Format(time.DateOnly)}))).ok(t)
	pastSquat := func() *pb.ResolvedExercise {
		for _, d := range call(pc.GetResolvedDays(ctx, as(tok, &pb.GetResolvedDaysRequest{ProgramId: p2.GetId(), Week: 1}))).ok(t).GetDays() {
			if d.GetDate() == lastWeek.Format(time.DateOnly) {
				return d.GetExercises()[0]
			}
		}
		t.Fatal("no resolved training day last week")
		return nil
	}
	if pastSquat().GetUnilateral() {
		t.Fatal("unilateral before it was set")
	}
	call(c.SetExerciseData(ctx, as(tok, &pb.SetExerciseDataRequest{Data: &pb.ExerciseData{Slug: slug, Rm1: &pb.Weight{Value: 100, Unit: pb.Unit_UNIT_KG}, Unilateral: true}}))).ok(t)
	if !pastSquat().GetUnilateral() {
		t.Fatal("last week's stored day still lacks the unilateral flag")
	}

	// A trailing slash on the public URL does not double up in image URLs.
	e.seedBench(t)
	bench := call(c.GetExercise(ctx, as(tok, &pb.GetExerciseRequest{Slug: "bench-press-barbell"}))).ok(t).GetExercise()
	if bench.GetImages()[0] != httpsURL+Prefix+"/exercises/images/bench-press-barbell/0.jpg" {
		t.Fatalf("image url = %q", bench.GetImages()[0])
	}
}
