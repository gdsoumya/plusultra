package export_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/export"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func overlayText(t *testing.T, o model.Overlay) string {
	t.Helper()
	b, err := engine.MarshalOverlay(o)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (e env) loadOverlay(id uuid.UUID) model.Overlay {
	e.t.Helper()
	o, err := programs.LoadOverlay(e.ctx, e.q, id)
	e.fatal(err)
	return o
}

func sampleOverlay(extra ...model.ProgKey) model.Overlay {
	abs := model.Weight{Value: 70, Unit: model.UnitKg}
	squat, heavy := model.ProgKey{Exercise: "Squat"}, model.ProgKey{Exercise: "Bench Press", Label: "heavy"}
	o := model.Overlay{
		Weights: map[model.SetAddr]model.WeightOverride{
			{Key: squat}:                  {Delta: model.Weight{Value: 5, Unit: model.UnitKg}, Scale: 1},
			{Key: heavy, Week: 2, Set: 1}: {Abs: &abs},
		},
		NumSets: map[model.DayAddr]model.IntOverride{{Key: squat, Week: 2}: {Delta: 1}},
		State: map[model.ProgKey]map[string]starlark.Value{
			squat:               {"successes": starlark.MakeInt(3), "failures": starlark.MakeInt(0)},
			{Exercise: "tag:1"}: {"rating": starlark.MakeInt(5)},
		},
		RM1: map[string]model.Weight{"Squat": {Value: 140, Unit: model.UnitKg}},
	}
	for _, k := range extra {
		o.Weights[model.SetAddr{Key: k}] = model.WeightOverride{Delta: model.Weight{Value: 2.5, Unit: model.UnitKg}, Scale: 1}
		o.State[k] = map[string]starlark.Value{"x": starlark.MakeInt(1)}
	}
	return o
}

func TestExportProgramRoundTrip(t *testing.T) {
	e := setup(t)
	p := e.create(programs.KindWorkout, workoutScript)
	o := sampleOverlay()
	e.fatal(programs.SaveOverlay(e.ctx, e.q, p.ID, o))

	text, name, err := e.s.ExportProgram(e.ctx, e.user, p.ID)
	e.fatal(err)
	if name != "base.star" || !strings.HasPrefix(text, workoutScript+"\nstate({\n") || !strings.HasSuffix(text, "})\n") {
		t.Fatalf("export %q:\n%s", name, text)
	}
	res, err := e.s.Import(e.ctx, e.user, export.KindProgram, text)
	e.fatal(err)
	np := res.Program
	if np == nil || np.ID == p.ID || np.Script != workoutScript || np.Revision != 1 || np.Status != programs.StatusDraft ||
		np.Kind != programs.KindWorkout || np.Name != "Base" || len(res.Warnings) != 0 || len(res.Dates) != 0 {
		t.Fatalf("imported %+v, %+v", np, res)
	}
	if got, want := overlayText(t, e.loadOverlay(np.ID)), overlayText(t, o); got != want {
		t.Fatalf("overlay:\n%s\nwant:\n%s", got, want)
	}
	revs, err := e.q.ListRevisions(e.ctx, np.ID)
	if err != nil || len(revs) != 1 || revs[0].Script != workoutScript {
		t.Fatalf("revisions %+v, %v", revs, err)
	}

	_, err = e.s.Programs.Create(e.ctx, e.user, programs.KindWorkout, text)
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "state()") {
		t.Fatalf("Create with a state block: %v", err)
	}
	_, err = e.s.Programs.Update(e.ctx, e.user, p.ID, text)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("Update with a state block: %v", err)
	}

	other, err := e.q.CreateUser(e.ctx, sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	e.fatal(err)
	_, _, err = e.s.ExportProgram(e.ctx, other.ID, p.ID)
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("other user's export: %v", err)
	}
	if text, _, err := e.s.ExportProgram(e.ctx, e.user, e.create(programs.KindWorkout, workoutScript).ID); err != nil || text != workoutScript {
		t.Fatalf("export without state: %q, %v", text, err)
	}
}

func TestExportMealProgramRoundTrip(t *testing.T) {
	e := setup(t)
	_, err := e.q.UpsertFood(e.ctx, sqlc.UpsertFoodParams{Source: "usda_sr", SourceID: "169705", Name: "Oats",
		Per100g: []byte(`{"Kcal": 389, "Protein": 16.9}`), Units: []byte("[]")})
	e.fatal(err)
	_, err = catalog.CreateCustomFood(e.ctx, e.q, e.user, catalog.CustomFood{Name: "Egg, whole", Per100g: model.Macros{Kcal: 140, Protein: 12}, Units: []catalog.Unit{{Name: "egg", Grams: 50}}})
	e.fatal(err)
	script := "meal_program(\"Cut\", weeks=1)\noats = food(\"Oats\", ref=\"usda_sr:169705\")\negg = food(\"Egg, whole\")\n" +
		"day(1, kind=\"training\", name=\"Train\")\nmeal(\"Breakfast\", oats(80), egg(2, unit=\"egg\"))\n"
	p := e.create(programs.KindMeal, script)

	text, name, err := e.s.ExportProgram(e.ctx, e.user, p.ID)
	e.fatal(err)
	if name != "cut.star" || !strings.HasPrefix(text, script+"\ncatalog_snapshot({\n") || !strings.Contains(text, "\"per_unit\": {\"kcal\": 70, \"protein\": 6}, \"unit\": \"egg\"") ||
		!strings.Contains(text, "\"usda_sr:169705\": {\"kcal\": 389, \"protein\": 16.9}") {
		t.Fatalf("export %q:\n%s", name, text)
	}

	// Another server has none of these foods.
	if _, err := e.pool.Exec(e.ctx, "DELETE FROM foods"); err != nil {
		t.Fatal(err)
	}
	res, err := e.s.Import(e.ctx, e.user, export.KindProgram, text)
	e.fatal(err)
	np := res.Program
	if np == nil || np.Script != script || np.Kind != programs.KindMeal || len(res.Warnings) != 0 {
		t.Fatalf("imported %+v, %+v", np, res)
	}
	var st programs.Structure
	e.fatal(json.Unmarshal(np.Structure, &st))
	if len(st.Foods) != 2 || st.Foods["usda_sr:169705"].Per100g.Kcal != 389 || st.Foods["Egg, whole"].PerUnit.Kcal != 70 {
		t.Fatalf("structure foods %+v", st.Foods)
	}
	days, err := e.s.Programs.ResolvedDays(e.ctx, e.user, np.ID, programs.Range{})
	e.fatal(err)
	if len(days) != 1 || math.Abs(days[0].Meal.Totals().Kcal-451.2) > 1e-9 {
		t.Fatalf("days %+v", days)
	}
	if text2, _, err := e.s.ExportProgram(e.ctx, e.user, np.ID); err != nil || text2 != text {
		t.Fatalf("second export differs:\n%s\n%v", text2, err)
	}
}

func TestStaleStateDropped(t *testing.T) {
	e := setup(t)
	p := e.create(programs.KindWorkout, workoutScript)
	dead := model.ProgKey{Exercise: "Deadlift"}
	e.fatal(programs.SaveOverlay(e.ctx, e.q, p.ID, sampleOverlay(dead)))
	text, _, err := e.s.ExportProgram(e.ctx, e.user, p.ID)
	e.fatal(err)

	res, err := e.s.Import(e.ctx, e.user, export.KindProgram, text)
	e.fatal(err)
	if len(res.Warnings) != 1 || res.Warnings[0] != `dropped the state of "Deadlift": the script has no such exercise` {
		t.Fatalf("warnings %q", res.Warnings)
	}
	if got, want := overlayText(t, e.loadOverlay(res.Program.ID)), overlayText(t, sampleOverlay()); got != want {
		t.Fatalf("overlay:\n%s\nwant:\n%s", got, want)
	}
}
