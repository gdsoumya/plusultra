package export_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/export"
	"github.com/gdsoumya/plusultra/internal/llmdocs"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func (e env) mealLog(day string, target model.Macros, meals ...model.Meal) {
	e.t.Helper()
	_, err := e.s.Meals.Create(e.ctx, e.user, logs.MealLogInput{Date: day, Target: &target, Meals: meals})
	e.fatal(err)
}

func logsCheckin(day string, weightKg float64) logs.CheckinInput {
	return logs.CheckinInput{Date: day, WeightKg: &weightKg}
}

func golden(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/days.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (e env) exportText(from, to string) string {
	e.t.Helper()
	b, err := e.s.ExportDays(e.ctx, e.user, date(from), date(to), export.FormatText)
	e.fatal(err)
	return string(b)
}

// TestDayLogTextGolden pins the text format to the log-format reference: the
// golden file is the reference's example, and the example data exports to it.
func TestDayLogTextGolden(t *testing.T) {
	want := golden(t)
	doc := llmdocs.Build("https://x.test").LogFormat
	if m := regexp.MustCompile("(?s)```text\n(.*?)```").FindStringSubmatch(doc); m == nil || m[1] != want {
		t.Fatal("testdata/days.txt is not the log-format reference's example")
	}
	e := setup(t)
	e.seedGolden()
	if got := e.exportText("2026-11-01", "2026-11-30"); got != want {
		t.Fatalf("text export:\n%s\nwant:\n%s", got, want)
	}
	_, second, _ := strings.Cut(want, "\n\n")
	if got := e.exportText("2026-11-12", "2026-11-12"); got != second {
		t.Fatalf("one day:\n%s", got)
	}

	raw, err := e.s.ExportDays(e.ctx, e.user, date("2026-11-11"), date("2026-11-12"), export.FormatJSON)
	e.fatal(err)
	var days []export.Day
	e.fatal(json.Unmarshal(raw, &days))
	if len(days) != 2 || days[0].Status != "done" || days[0].Notes != "slept badly" || *days[0].Checkin.WeightKg != 82.4 ||
		days[0].Workout.Program != "16-Week Strength" || days[0].Workout.Week != 1 || len(days[0].Workout.Entries) != 2 ||
		!days[0].Workout.Entries[0].WarmupDone || len(days[0].Workout.Entries[0].Sets) != 9 || days[0].Workout.Entries[0].Sets[3].Planned.Label != "Work" ||
		days[0].Meals.Target.Kcal != 2600 || days[0].Meals.Totals.Protein < 175.9 || len(days[0].Meals.Meals) != 2 ||
		days[1].Workout.Entries[2].Sets[0].Planned.RPE != 8 || !days[1].Workout.Entries[3].Sets[0].Planned.Ask ||
		days[1].Meals.Meals[0].Items[2].Qty != 0 || !days[1].Meals.Meals[0].Items[2].Planned {
		t.Fatalf("json %s", raw)
	}

	csvOut, err := e.s.ExportDays(e.ctx, e.user, date("2026-11-11"), date("2026-11-12"), export.FormatCSV)
	e.fatal(err)
	lines := strings.Split(strings.TrimSpace(string(csvOut)), "\n")
	// A header, 9+4 and 3+5+3+3 set rows, 3+3 item rows and 2 check-in rows.
	if !strings.HasPrefix(lines[0], "date,status,notes,readiness,row,") || len(lines) != 1+13+14+6+2 {
		t.Fatalf("csv has %d lines:\n%s", len(lines), csvOut)
	}
	for _, want := range []string{",set,", ",item,", ",checkin,", "82.4", "Bent Over Row, Dumbbell", "slept badly", "Backoff"} {
		if !strings.Contains(string(csvOut), want) {
			t.Errorf("csv lacks %q", want)
		}
	}
}

// fakeWorkouts records what the importer is handed.
type fakeWorkouts struct {
	got      []export.Workout
	finalize []bool
}

func (f *fakeWorkouts) ImportWorkout(_ context.Context, _ uuid.UUID, _ string, w export.Workout, finalize bool) error {
	f.got, f.finalize = append(f.got, w), append(f.finalize, finalize)
	return nil
}

// TestParsedWorkouts checks the sets the text parses into, through a fake importer.
func TestParsedWorkouts(t *testing.T) {
	e := setup(t)
	fake := &fakeWorkouts{}
	e.s.Workouts = fake
	var blocks []string // the golden without its meals
	for _, b := range strings.Split(golden(t), "\n\n") {
		workout, _, _ := strings.Cut(b, "meals:")
		blocks = append(blocks, workout)
	}
	if _, err := e.s.Import(e.ctx, e.user, export.KindDayLog, strings.Join(blocks, "\n")); err != nil {
		t.Fatal(err)
	}
	if len(fake.got) != 2 || !fake.finalize[0] || fake.finalize[1] {
		t.Fatalf("workouts %+v finalize %v", fake.got, fake.finalize)
	}
	w := fake.got[0]
	if w.Program != "16-Week Strength" || w.Week != 1 || w.Day != 2 || w.DayName != "Tue Upper A bench" || w.Minutes != 58 || len(w.Entries) != 2 ||
		!w.Entries[0].WarmupDone || len(w.Entries[0].Sets) != 6 || w.Entries[0].Sets[4].Planned.Label != "Backoff" || w.Entries[0].Sets[4].Planned.Weight != kg(62.5) ||
		!w.Entries[0].Sets[4].Completed || w.Entries[0].Sets[4].Reps != 5 || w.Entries[1].Label != "heavy" || w.Entries[1].Sets[3].Reps != 5 || w.Entries[1].Sets[3].Weight != kg(70) {
		t.Fatalf("first workout %+v", w)
	}
	w = fake.got[1]
	pull, bench, ohp, row := w.Entries[0].Sets, w.Entries[1].Sets, w.Entries[2].Sets, w.Entries[3].Sets
	if pull[2].Completed || pull[1].Reps != 7 || bench[0].Planned.MinReps != 8 || bench[0].Planned.Reps != 12 || bench[0].Reps != 11 ||
		ohp[0].Planned.RPE != 8 || !ohp[2].Completed || !row[0].Planned.Ask || row[0].Weight != kg(30) || row[2].Reps != 9 || w.Entries[3].Name != "Bent Over Row, Dumbbell" {
		t.Fatalf("second workout %+v", w)
	}
}

func TestDayLogTextImport(t *testing.T) {
	e := setup(t)
	for _, f := range []model.Food{chicken, rice, banana, oats} {
		_, err := catalog.CreateCustomFood(e.ctx, e.q, e.user, catalog.CustomFood{Name: f.Name, Per100g: f.Per100g})
		e.fatal(err)
	}
	_, err := catalog.CreateCustomFood(e.ctx, e.q, e.user, catalog.CustomFood{Name: egg.Name, Per100g: egg.Per100g, Units: []catalog.Unit{{Name: "egg", Grams: 50}}})
	e.fatal(err)
	// An existing check-in keeps its body fat; a planned log on the 12th keeps its program.
	bf := 18.5
	_, err = e.s.Checkins.Create(e.ctx, e.user, logs.CheckinInput{Date: "2026-11-11", WeightKg: ptr(80.0), BodyFat: &bf})
	e.fatal(err)
	mealProg := e.program("Cut")
	_, err = e.s.Meals.Create(e.ctx, e.user, logs.MealLogInput{Date: "2026-11-12", ProgramID: &mealProg, Target: &model.Macros{Fiber: 30},
		Meals: []model.Meal{{Name: "Breakfast", Items: []model.Item{item(oats, 100, true)}}}})
	e.fatal(err)

	// Programs the golden names; the first one twice, the newest copy being the one records join.
	e.program("16-Week Strength")
	strength := e.program("16-Week Strength")
	e.program("Upper volume")
	readiness, _ := json.Marshal(logs.Readiness{Sleep: ptr(4)})
	_, err = e.q.UpsertDayLog(e.ctx, sqlc.UpsertDayLogParams{UserID: e.user, Date: date("2026-11-11"), Status: "planned", Readiness: readiness})
	e.fatal(err)
	text := golden(t)
	res, err := e.s.Import(e.ctx, e.user, export.KindDayLog, text)
	e.fatal(err)
	if res.Program != nil || strings.Join(res.Dates, ",") != "2026-11-11,2026-11-12" || len(res.Warnings) != 0 {
		t.Fatalf("result %+v", res)
	}

	recs, err := e.s.Logs.ListWorkoutRecords(e.ctx, e.user, "2026-11-11", "2026-11-12", uuid.Nil, "", 0)
	if err != nil || len(recs.Items) != 2 {
		t.Fatalf("records %+v, %v", recs, err)
	}
	r := recs.Items[0]
	if !r.Finalized || r.ProgramID == nil || *r.ProgramID != strength || r.Day.Week != 1 || r.Day.Day != 2 || r.Day.Name != "Tue Upper A bench" ||
		r.FinishedAt.Sub(*r.StartedAt) != 58*time.Minute || len(r.Entries) != 2 || len(r.Entries[0].Sets) != 7 || !r.Entries[0].Sets[0].Warmup ||
		r.Entries[0].Sets[5].Planned.Label != "Backoff" || r.Entries[1].Label != "heavy" || r.Entries[1].Sets[3].Reps != 5 {
		t.Fatalf("first record %+v", r)
	}
	if r = recs.Items[1]; r.Finalized || r.Day.Week != 2 || len(r.Entries) != 4 || !r.Entries[3].Sets[0].Planned.Ask || r.Entries[3].Sets[0].Weight != kg(30) {
		t.Fatalf("second record %+v", r)
	}
	d, err := e.q.GetDay(e.ctx, sqlc.GetDayParams{UserID: e.user, Date: date("2026-11-11")})
	var kept logs.Readiness
	if err != nil || d.Status != "done" || d.Notes != "slept badly" || json.Unmarshal(d.Readiness, &kept) != nil || kept.Sleep == nil || *kept.Sleep != 4 {
		t.Fatalf("day log %+v, %v", d, err)
	}
	cs, err := e.s.Checkins.List(e.ctx, e.user, "2026-11-11", "2026-11-12", "", 0, false)
	if err != nil || len(cs.Items) != 2 || *cs.Items[0].WeightKg != 82.4 || *cs.Items[0].BodyFat != 18.5 || *cs.Items[1].WeightKg != 82 || cs.Items[1].BodyFat != nil {
		t.Fatalf("check-ins %+v, %v", cs.Items, err)
	}
	ml, err := e.s.Meals.ByDate(e.ctx, e.user, "2026-11-12")
	if err != nil || ml.ProgramID == nil || *ml.ProgramID != mealProg || ml.Target.Fiber != 30 || ml.Target.Kcal != 2400 || len(ml.Meals) != 1 ||
		len(ml.Meals[0].Items) != 3 || ml.Meals[0].Items[1].Unit != "egg" || ml.Meals[0].Items[1].Macros.Kcal != 210 || ml.Meals[0].Items[2].Qty != 0 ||
		!ml.Meals[0].Items[2].Planned || ml.Totals.Kcal != 1890 || !strings.HasPrefix(ml.Meals[0].Items[1].Food.Ref, "custom:") || ml.Meals[0].Items[0].Food.Ref != "" {
		t.Fatalf("meal log %+v, %v", ml, err)
	}
	ml, err = e.s.Meals.ByDate(e.ctx, e.user, "2026-11-11")
	if err != nil || ml.Meals[1].Items[0].Planned || !ml.Meals[0].Items[0].Planned || ml.Totals.Kcal != 2510 {
		t.Fatalf("meal log %+v, %v", ml, err)
	}
	if got := e.exportText("2026-11-11", "2026-11-12"); got != text {
		t.Fatalf("round trip:\n%s", got)
	}
	// Importing again rewrites the 12th's record and leaves the finalized 11th alone.
	res, err = e.s.Import(e.ctx, e.user, export.KindDayLog, text)
	if err != nil || len(res.Warnings) != 1 || res.Warnings[0] != "line 2: the workout of 2026-11-11 is finalized and was not changed" {
		t.Fatalf("second import %+v, %v", res, err)
	}
	if recs, err = e.s.Logs.ListWorkoutRecords(e.ctx, e.user, "2026-11-11", "2026-11-12", uuid.Nil, "", 0); err != nil || len(recs.Items) != 2 {
		t.Fatalf("records after a second import %+v, %v", recs, err)
	}
	if got := e.exportText("2026-11-11", "2026-11-12"); got != text {
		t.Fatalf("round trip after a second import:\n%s", got)
	}

	for _, tc := range []struct{ text, want string }{
		{"2026-11-13 / status: done\nmeals: target 1kcal 1p 1c 1f / actual 1kcal 1p 1c 1f\n  Lunch / Dragon fruit 100g\n", "line 3: unknown food \"Dragon fruit\""},
		{"2026-11-13 / status: done\nmeals: target 1kcal 1p 1c 1f / actual 1kcal 1p 1c 1f\n  Lunch / Oats 2 cup\n", "line 3: Oats has no unit \"cup\""},
		{"2026-11-13 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat / 3x5 100kg -> 5,5\n", "line 3"},
		{"2026-11-13 / status: done\n  Squat / 3x5 100kg\n", "line 2"},
		{"2026-11-13 / status: nope\n", "line 1"},
		{"2026-11-13 / notes: \"x\"\n", "line 1"},
		{"hello\n", "line 1"},
		{"2026-11-13 / status: done\n2026-11-13 / status: rest\n", "line 2: 2026-11-13 again; it is already on line 1"},
		{"2026-11-13 / status: done / notes: \"\xff\"\n", "line 1: text is not valid UTF-8"},
		{manySets(6), "more than 50000 sets"},
		{"2026-11-13 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n" + strings.Repeat("  Squat / 1x1 1kg\n", 101), "line 103: more than 100 exercises"},
		{"2026-11-13 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat" + strings.Repeat(" / 50x1 1kg", 3) + "\n", "line 3: more than 100 sets for one exercise"},
		{"2026-11-13 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat / 1x5000 1kg\n", "line 3: reps must be at most 1000"},
		{"2026-11-13 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat / 1x5 1kg -> 5000\n", "line 3: reps must be at most 1000"},
		{"2026-11-13 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat / 1x12-8 1kg\n", "line 3: reps must be at most 1000, the range bottom first"},
	} {
		_, err := e.s.Import(e.ctx, e.user, export.KindDayLog, tc.text)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("import %q: err %v, want %q", tc.text, err, tc.want)
		}
	}
	if _, err := e.q.GetDay(e.ctx, sqlc.GetDayParams{UserID: e.user, Date: date("2026-11-13")}); err == nil {
		t.Fatal("a failed import wrote a day log")
	}
	var many strings.Builder
	for i := range 5001 {
		fmt.Fprintf(&many, "%s / status: rest\n", date("2000-01-01").AddDate(0, 0, i).Format(time.DateOnly))
	}
	if _, err := e.s.Import(e.ctx, e.user, export.KindDayLog, many.String()); err == nil || !strings.Contains(err.Error(), "line 5001: more than 5000 days") {
		t.Fatalf("too many days: %v", err)
	}

	// An AMRAP group keeps its reps, and an error after a written date names it.
	amrap := "2026-11-20 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat / 3x5+ 100kg -> 5,5,9\n\n2026-11-21 / status: done\nmeals: target 1kcal 1p 1c 1f / actual 1kcal 1p 1c 1f\n  Lunch / " + strings.Repeat("x", 201) + " 100g\n"
	e.s.Workouts = recordsVia(e)
	firstBlock, _, _ := strings.Cut(amrap, "\n\n")
	_, err = e.s.Import(e.ctx, e.user, export.KindDayLog, firstBlock+"\n")
	if err == nil || !strings.Contains(err.Error(), "line 2: logs: invalid: unknown program \"X\"") {
		t.Fatalf("unknown program: %v", err)
	}
	e.program("X") // an older finished copy; the active one wins
	active, err := e.s.Programs.Activate(e.ctx, e.user, e.create(programs.KindWorkout, strings.Replace(workoutScript, `"Base"`, `"X"`, 1)).ID, nil)
	e.fatal(err)
	_, err = e.s.Import(e.ctx, e.user, export.KindDayLog, amrap)
	if err == nil || !strings.Contains(err.Error(), "line 7") || strings.Contains(err.Error(), "after writing") {
		t.Fatalf("food error before any write: %v", err)
	}
	_, err = e.s.Import(e.ctx, e.user, export.KindDayLog, strings.Replace(amrap, "  Lunch / "+strings.Repeat("x", 201), "  "+strings.Repeat("L", 101)+" / Oats", 1))
	if err == nil || !strings.Contains(err.Error(), "after writing 2026-11-20: line 7") {
		t.Fatalf("error after a write: %v", err)
	}
	recs, err = e.s.Logs.ListWorkoutRecords(e.ctx, e.user, "2026-11-20", "2026-11-20", uuid.Nil, "", 0)
	if err != nil || len(recs.Items) != 1 || *recs.Items[0].ProgramID != active.ID || !recs.Items[0].Finalized {
		t.Fatalf("active program record %+v, %v", recs, err)
	}
	first := e.loadOverlay(active.ID)
	if len(first.Weights) == 0 { // the finalize ran the lp progression
		t.Fatalf("overlay after finalize %+v", first)
	}
	// A second import of the done day neither adds a record nor progresses again.
	block20, _, _ := strings.Cut(amrap, "\n\n")
	res, err = e.s.Import(e.ctx, e.user, export.KindDayLog, block20+"\n")
	if err != nil || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "finalized") {
		t.Fatalf("re-import of a done day %+v, %v", res, err)
	}
	if recs, err = e.s.Logs.ListWorkoutRecords(e.ctx, e.user, "2026-11-20", "2026-11-20", uuid.Nil, "", 0); err != nil || len(recs.Items) != 1 {
		t.Fatalf("records after re-import %+v, %v", recs, err)
	}
	if got, want := overlayText(t, e.loadOverlay(active.ID)), overlayText(t, first); got != want {
		t.Fatalf("progression ran again:\n%s\n%s", got, want)
	}
	if got := e.exportText("2026-11-20", "2026-11-20"); !strings.Contains(got, "3x5+ 100kg -> 5,5,9") {
		t.Fatalf("amrap round trip:\n%s", got)
	}

	// Without a workout importer the workout lines are skipped with a warning.
	e.s.Workouts = nil
	res, err = e.s.Import(e.ctx, e.user, export.KindDayLog, "2026-11-14 / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n  Squat / 3x5 100kg ✓✓✓\n")
	if err != nil || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "line 2") {
		t.Fatalf("no importer: %+v, %v", res, err)
	}
}

func ptr[T any](v T) *T { return &v }

// manySets is blocks days of 100 exercises with 100 sets each, within every per-line cap.
func manySets(blocks int) string {
	var b strings.Builder
	for i := range blocks {
		fmt.Fprintf(&b, "%s / status: done\nworkout: \"X\" week 1 day 1 \"A\"\n%s\n", date("2026-11-13").AddDate(0, 0, i).Format(time.DateOnly), strings.Repeat("  Squat / 100x1 1kg\n", 100))
	}
	return b.String()
}

// recordsVia is the real workout importer, as NewService wires it.
func recordsVia(e env) export.WorkoutImporter {
	return export.NewService(e.pool, e.s.Programs, e.s.Logs).Workouts
}

// TestAdHocExerciseRoundTrip exports an exercise the plan did not have and
// imports it back as an ad hoc entry.
func TestAdHocExerciseRoundTrip(t *testing.T) {
	e := setup(t)
	_, err := e.s.Programs.Activate(e.ctx, e.user, e.create(programs.KindWorkout, strings.Replace(workoutScript, `"Base"`, `"X"`, 1)).ID, nil)
	e.fatal(err)
	rec, err := e.s.Logs.CreateWorkoutRecord(e.ctx, e.user, logs.NewRecord{Date: "2026-11-20"})
	e.fatal(err)
	curl := model.RecordSet{Planned: planned(10, kg(20), ""), Completed: true, Reps: 10, Weight: kg(20), Edited: true}
	entries := slices.Concat(rec.Entries, []model.RecordEntry{{Name: "Curl", Sets: []model.RecordSet{curl, curl}}})
	for i := range entries[0].Sets {
		entries[0].Sets[i].Completed, entries[0].Sets[i].Reps, entries[0].Sets[i].Weight = true, 5, kg(100)
	}
	_, err = e.s.Logs.UpdateWorkoutRecord(e.ctx, e.user, rec.ID, logs.RecordInput{Entries: entries})
	e.fatal(err)
	text := "2026-11-20\nworkout: \"X\" week 1 day 1 \"Lower\"\n  Squat / 3x5 100kg ✓✓✓\n  Curl / 2x10 20kg ✓✓\n"
	if got := e.exportText("2026-11-20", "2026-11-20"); got != text {
		t.Fatalf("text export:\n%s\nwant:\n%s", got, text)
	}
	raw, err := e.s.ExportDays(e.ctx, e.user, date("2026-11-20"), date("2026-11-20"), export.FormatJSON)
	e.fatal(err)
	csvOut, err := e.s.ExportDays(e.ctx, e.user, date("2026-11-20"), date("2026-11-20"), export.FormatCSV)
	e.fatal(err)
	rows, err := csv.NewReader(bytes.NewReader(csvOut)).ReadAll()
	e.fatal(err)
	col := func(name string) int { return slices.Index(rows[0], name) }
	if !strings.Contains(string(raw), `"modified": true`) || !strings.Contains(string(raw), `"edited": true`) || len(rows) != 6 ||
		rows[1][col("modified")] != "true" || rows[1][col("edited")] != "false" || rows[4][col("exercise")] != "Curl" || rows[4][col("edited")] != "true" {
		t.Fatalf("json %s\ncsv %s", raw, csvOut)
	}
	e.fatal(e.s.Logs.DeleteWorkoutRecord(e.ctx, e.user, rec.ID))
	if _, err := e.s.Import(e.ctx, e.user, export.KindDayLog, text); err != nil {
		t.Fatal(err)
	}
	recs, err := e.s.Logs.ListWorkoutRecords(e.ctx, e.user, "2026-11-20", "2026-11-20", uuid.Nil, "", 0)
	if err != nil || len(recs.Items) != 1 || len(recs.Items[0].Entries) != 2 || recs.Items[0].Entries[1].Name != "Curl" ||
		len(recs.Items[0].Entries[1].Sets) != 2 || !recs.Items[0].Modified || logs.PlanEntries(&recs.Items[0].Day, recs.Items[0].Entries)[1] {
		t.Fatalf("imported %+v, %v", recs, err)
	}
	if got := e.exportText("2026-11-20", "2026-11-20"); got != text {
		t.Fatalf("re-export:\n%s", got)
	}
}
