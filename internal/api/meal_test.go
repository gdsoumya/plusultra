package api

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

func oats(qty float64) *pb.MealItem {
	return &pb.MealItem{Food: &pb.FrozenFood{Name: "Oats", Per100G: &pb.Macros{Kcal: 380, Protein: 13}}, Qty: qty, Unit: "g"}
}

func mealLog(date string, meals ...*pb.Meal) *pb.MealLog {
	return &pb.MealLog{Id: uuid.NewString(), Date: date, Target: &pb.Macros{Kcal: 2000}, Meals: meals, DeviceId: "phone"}
}

func TestMealLogsOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	c := plusultrav1connect.NewMealServiceClient(http.DefaultClient, e.url)

	egg := &pb.MealItem{Food: &pb.FrozenFood{Name: "Egg", PerUnit: &pb.Macros{Kcal: 70}, Unit: "egg"}, Qty: 2, Unit: "egg", Planned: true}
	skipped := &pb.MealItem{Food: &pb.FrozenFood{Name: "Toast", Per100G: &pb.Macros{Kcal: 250}}, Qty: 0, Planned: true} // planned, not eaten (R44)
	in := mealLog("2026-11-10", &pb.Meal{Name: "Breakfast", Time: "08:00", Items: []*pb.MealItem{oats(100), egg, skipped}})
	created, err := c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: in}))
	if err != nil {
		t.Fatal(err)
	}
	l := created.Msg.GetMealLog()
	if l.GetId() != in.GetId() || l.GetDate() != "2026-11-10" || l.GetVersion() != 1 || l.GetDeviceId() != "phone" || l.GetUpdatedAt() == nil ||
		l.GetTarget().GetKcal() != 2000 || len(l.GetMeals()) != 1 || l.GetMeals()[0].GetTime() != "08:00" || len(l.GetMeals()[0].GetItems()) != 3 {
		t.Fatalf("created %v", l)
	}
	items := l.GetMeals()[0].GetItems()
	if items[0].GetMacros().GetKcal() != 380 || items[0].GetMacros().GetProtein() != 13 || items[1].GetMacros().GetKcal() != 140 ||
		!items[1].GetPlanned() || items[2].GetQty() != 0 || items[2].GetMacros().GetKcal() != 0 || l.GetTotals().GetKcal() != 520 {
		t.Fatalf("macros not computed by the server: %v", items)
	}
	_, err = c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-10")}))
	assertCode(t, err, connect.CodeAlreadyExists)

	got, err := c.GetMealLog(ctx, as(tok, &pb.GetMealLogRequest{Id: l.GetId()}))
	if err != nil || got.Msg.GetMealLog().GetTotals().GetKcal() != 520 {
		t.Fatalf("get = %v, err %v", got, err)
	}

	in.Meals, in.Target = []*pb.Meal{{Name: "Lunch", Items: []*pb.MealItem{oats(50)}}}, &pb.Macros{Kcal: 1800}
	up := call(c.UpdateMealLog(ctx, as(tok, &pb.UpdateMealLogRequest{MealLog: in}))).ok(t).GetMealLog()
	if up.GetVersion() != 2 || up.GetTotals().GetKcal() != 190 || up.GetTarget().GetKcal() != 1800 {
		t.Fatalf("update = %v", up)
	}
	in.Date = "2026-11-11"
	_, err = c.UpdateMealLog(ctx, as(tok, &pb.UpdateMealLogRequest{MealLog: in}))
	assertCode(t, err, connect.CodeInvalidArgument)

	if _, err := c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-12")})); err != nil {
		t.Fatal(err)
	}
	dates := &pb.DateRange{From: "2026-11-01", To: "2026-11-30"}
	page := call(c.ListMealLogs(ctx, as(tok, &pb.ListMealLogsRequest{Dates: dates, Limit: 1}))).ok(t)
	if len(page.GetMealLogs()) != 1 || !page.GetHasMore() || page.GetNextCursor() == "" || page.GetMealLogs()[0].GetDate() != "2026-11-10" {
		t.Fatalf("first page = %v", page)
	}
	rest := call(c.ListMealLogs(ctx, as(tok, &pb.ListMealLogsRequest{Dates: dates, Limit: 1, Cursor: page.GetNextCursor()}))).ok(t)
	if len(rest.GetMealLogs()) != 1 || rest.GetHasMore() || rest.GetMealLogs()[0].GetDate() != "2026-11-12" {
		t.Fatalf("second page = %v", rest)
	}

	bob := e.invitee(t, tok, "bob").GetToken()
	_, err = c.GetMealLog(ctx, as(bob, &pb.GetMealLogRequest{Id: l.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.UpdateMealLog(ctx, as(bob, &pb.UpdateMealLogRequest{MealLog: in}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.DeleteMealLog(ctx, as(bob, &pb.DeleteMealLogRequest{Id: l.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	if list, err := c.ListMealLogs(ctx, as(bob, &pb.ListMealLogsRequest{Dates: dates})); err != nil || len(list.Msg.GetMealLogs()) != 0 {
		t.Fatalf("bob sees alice's logs: %v, err %v", list, err)
	}

	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = c.DeleteMealLog(ctx, as(ro, &pb.DeleteMealLogRequest{Id: l.GetId()}))
	assertCode(t, err, connect.CodePermissionDenied)
	_, err = c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-13-01")}))
	assertCode(t, err, connect.CodeInvalidArgument)

	if _, err := c.DeleteMealLog(ctx, as(tok, &pb.DeleteMealLogRequest{Id: l.GetId()})); err != nil {
		t.Fatal(err)
	}
	_, err = c.GetMealLog(ctx, as(tok, &pb.GetMealLogRequest{Id: l.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
}

// A food named by ref alone gets its macros from the catalog and frozen into the log.
func TestMealLogFreezesCatalogRefs(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	reg := e.register(t, "alice", "")
	tok := reg.GetToken()
	c := plusultrav1connect.NewMealServiceClient(http.DefaultClient, e.url)
	f, err := catalog.CreateCustomFood(ctx, sqlc.New(e.pool), uuid.MustParse(reg.GetUser().GetId()), catalog.CustomFood{
		Name: "Egg", Per100g: model.Macros{Kcal: 140, Protein: 12}, Units: []catalog.Unit{{Name: "egg", Grams: 50}},
	})
	if err != nil {
		t.Fatal(err)
	}

	byID := &pb.MealItem{Food: &pb.FrozenFood{Name: "Egg", Ref: f.ID.String()}, Qty: 2, Unit: "egg"}
	bySource := &pb.MealItem{Food: &pb.FrozenFood{Name: "Egg", Ref: "custom:" + f.SourceID}, Qty: 50}
	res, err := c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-10", &pb.Meal{Name: "Breakfast", Items: []*pb.MealItem{byID, bySource}})}))
	if err != nil {
		t.Fatal(err)
	}
	items := res.Msg.GetMealLog().GetMeals()[0].GetItems()
	if items[0].GetMacros().GetKcal() != 140 || items[0].GetFood().GetPerUnit().GetKcal() != 70 || items[0].GetFood().GetUnit() != "egg" ||
		items[0].GetFood().GetRef() != f.ID.String() || items[1].GetMacros().GetKcal() != 70 || items[1].GetFood().GetPer100G().GetProtein() != 12 {
		t.Fatalf("frozen items = %v", items)
	}

	// An item whose macros the user overrode keeps them: the catalog is not consulted.
	mine := &pb.MealItem{Food: &pb.FrozenFood{Name: "Egg", Ref: f.ID.String(), PerUnit: &pb.Macros{Kcal: 90}, Unit: "egg"}, Qty: 2, Unit: "egg", Edited: true}
	res, err = c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-12", &pb.Meal{Name: "Breakfast", Items: []*pb.MealItem{mine}})}))
	if err != nil {
		t.Fatal(err)
	}
	if it := res.Msg.GetMealLog().GetMeals()[0].GetItems()[0]; !it.GetEdited() || it.GetMacros().GetKcal() != 180 ||
		it.GetFood().GetPerUnit().GetKcal() != 90 || it.GetFood().GetPer100G().GetKcal() != 0 || it.GetFood().GetRef() != f.ID.String() {
		t.Fatalf("edited item = %v", it)
	}

	unknown := &pb.Meal{Name: "B", Items: []*pb.MealItem{{Food: &pb.FrozenFood{Name: "X", Ref: "custom:nope"}, Qty: 1}}}
	_, err = c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-11", unknown)}))
	assertCode(t, err, connect.CodeInvalidArgument)
	bob := e.invitee(t, tok, "bob").GetToken()
	_, err = c.CreateMealLog(ctx, as(bob, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-10", &pb.Meal{Name: "B", Items: []*pb.MealItem{byID}})}))
	assertCode(t, err, connect.CodeInvalidArgument)
}

// A log created without a target takes the active meal plan's target for its date.
func TestMealLogDefaultsTargetFromPlan(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok, pc := e.programUser(t, "alice", "")
	c := plusultrav1connect.NewMealServiceClient(http.DefaultClient, e.url)

	p := create(t, pc, tok, pb.ProgramKind_PROGRAM_KIND_MEAL, mealScript)
	call(pc.ActivateProgram(ctx, as(tok, &pb.ActivateProgramRequest{Id: p.GetId(), StartDate: "2026-11-09"}))).ok(t)

	// No workout program, so every date is a rest day: oats(50).
	planned := call(c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: &pb.MealLog{Id: uuid.NewString(), Date: "2026-11-10"}}))).ok(t).GetMealLog()
	if planned.GetTarget().GetKcal() != 190 {
		t.Fatalf("target = %v, want the plan's 190 kcal", planned.GetTarget())
	}
	explicit := call(c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: mealLog("2026-11-11")}))).ok(t).GetMealLog()
	if explicit.GetTarget().GetKcal() != 2000 {
		t.Fatalf("explicit target = %v", explicit.GetTarget())
	}
	outside := call(c.CreateMealLog(ctx, as(tok, &pb.CreateMealLogRequest{MealLog: &pb.MealLog{Id: uuid.NewString(), Date: "2026-12-25"}}))).ok(t).GetMealLog()
	if outside.GetTarget().GetKcal() != 0 {
		t.Fatalf("target outside the plan = %v", outside.GetTarget())
	}
}
