package insights

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

type fixture struct {
	t    *testing.T
	ctx  context.Context
	q    *sqlc.Queries
	svc  *Service
	user uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := dbtest.New(t)
	q := sqlc.New(pool)
	u, err := q.CreateUser(t.Context(), sqlc.CreateUserParams{Username: "lifter", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, ctx: t.Context(), q: q, svc: New(pool), user: u.ID}
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) exercise(slug string, muscles ...string) {
	_, err := f.q.UpsertExercise(f.ctx, sqlc.UpsertExerciseParams{Slug: slug, Name: slug, PrimaryMuscles: muscles, Source: "curated"})
	must(f.t, err)
}

func (f *fixture) record(date string, finalized bool, entries ...model.RecordEntry) {
	b, err := json.Marshal(entries)
	must(f.t, err)
	_, err = f.q.UpsertWorkoutRecord(f.ctx, sqlc.UpsertWorkoutRecordParams{
		ID: uuid.New(), UserID: f.user, Date: day(date), ResolvedDay: []byte(`{}`), Entries: b, Finalized: finalized,
	})
	must(f.t, err)
}

func (f *fixture) dayLog(date, status string) {
	_, err := f.q.UpsertDayLog(f.ctx, sqlc.UpsertDayLogParams{UserID: f.user, Date: day(date), Status: status})
	must(f.t, err)
}

func (f *fixture) planned(dates ...string) {
	p, err := f.q.CreateProgram(f.ctx, sqlc.CreateProgramParams{UserID: f.user, Kind: "workout", Name: "p", Script: "", Structure: []byte(`{}`), Status: "active"})
	must(f.t, err)
	for i, d := range dates {
		dt := day(d)
		must(f.t, f.q.UpsertResolvedDay(f.ctx, sqlc.UpsertResolvedDayParams{ProgramID: p.ID, Week: 1, Day: int32(i + 1), Date: &dt, Payload: []byte(`{}`)}))
	}
}

func (f *fixture) weigh(date string, kg float64) {
	var n pgtype.Numeric
	must(f.t, n.Scan(strconv.FormatFloat(kg, 'g', -1, 64)))
	_, err := f.q.UpsertCheckin(f.ctx, sqlc.UpsertCheckinParams{ID: uuid.New(), UserID: f.user, Date: day(date), Weight: n, Measurements: []byte(`{}`)})
	must(f.t, err)
}

func (f *fixture) meals(date, target, meals string) {
	_, err := f.q.UpsertMealLog(f.ctx, sqlc.UpsertMealLogParams{ID: uuid.New(), UserID: f.user, Date: day(date), Target: []byte(target), Meals: []byte(meals)})
	must(f.t, err)
}

func set(reps int, w float64, unit model.Unit) model.RecordSet {
	return model.RecordSet{Completed: true, Reps: reps, Weight: model.RecordWeight{Value: w, Unit: unit}}
}

func entry(slug string, sets ...model.RecordSet) model.RecordEntry {
	return model.RecordEntry{Name: slug, Slug: slug, Sets: sets}
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestWeeklyAdherence(t *testing.T) {
	f := newFixture(t)
	f.planned("2026-11-02", "2026-11-03", "2026-11-04", "2026-11-06", "2026-11-09")
	f.dayLog("2026-11-02", "done")
	f.dayLog("2026-11-04", "skipped")
	f.dayLog("2026-11-05", "rest")
	f.dayLog("2026-11-07", "done") // unplanned session still counts as planned and done

	// Today is Nov 5: the unlogged Nov 3 is past and skipped, the unlogged Nov 6 is pending.
	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-11-05"))
	must(t, err)
	if got, want := s.Adherence, (Counts{Planned: 5, Done: 2, Skipped: 2, Pending: 1, Ratio: 0.5}); got != want {
		t.Errorf("adherence = %+v, want %+v", got, want)
	}
}

func TestE1RMEpley(t *testing.T) {
	f := newFixture(t)
	f.exercise("bench-press", "chest")
	warm := set(10, 200, model.UnitKg)
	warm.Warmup = true
	undone := set(5, 300, model.UnitKg)
	undone.Completed = false
	f.record("2026-11-03", true, entry("bench-press", set(5, 100, model.UnitKg), set(3, 110, model.UnitKg), warm, undone))
	f.record("2026-11-05", true, entry("bench-press", set(5, 225, model.UnitLb)))

	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	// 100x5 = 116.67, 110x3 = 121, 225lb x5 = 102.06x(1+5/30) = 119.07
	near(t, "e1rm", s.E1RM["bench-press"].Value, 121)
	if s.E1RM["bench-press"].Unit != model.UnitKg {
		t.Errorf("unit = %q, want kg", s.E1RM["bench-press"].Unit)
	}

	h, err := f.svc.LiftHistory(f.ctx, f.user, "bench-press", time.Time{}, time.Time{})
	must(t, err)
	if len(h) != 2 {
		t.Fatalf("points = %d, want 2", len(h))
	}
	near(t, "day1 e1rm", h[0].E1RM.Value, 121)
	if h[0].BestSet.Reps != 3 || h[0].BestSet.Weight.Value != 110 {
		t.Errorf("best set = %+v", h[0].BestSet)
	}
	near(t, "day2 e1rm", h[1].E1RM.Value, 225*0.45359237*(1+5.0/30))
	near(t, "day2 best weight", h[1].BestSet.Weight.Value, 225*0.45359237)

	h, err = f.svc.LiftHistory(f.ctx, f.user, "bench-press", day("2026-11-04"), day("2026-11-30"))
	must(t, err)
	if len(h) != 1 {
		t.Errorf("windowed points = %d, want 1", len(h))
	}
}

func TestSetsPerMuscleUsesCatalog(t *testing.T) {
	f := newFixture(t)
	f.exercise("squat", "quads", "glutes")
	f.exercise("curl", "biceps")
	warm := set(5, 20, model.UnitKg)
	warm.Warmup = true
	undone := set(5, 100, model.UnitKg)
	undone.Completed = false
	f.record("2026-11-03", true,
		entry("squat", warm, set(5, 100, model.UnitKg), set(5, 100, model.UnitKg), undone),
		entry("curl", set(10, 20, model.UnitKg)),
		entry("mystery", set(10, 20, model.UnitKg)),
	)
	f.record("2026-11-10", true, entry("curl", set(10, 20, model.UnitKg))) // next week

	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	want := map[string]int{"quadriceps": 2, "glutes": 2, "biceps": 1, "unknown": 1} // "quads" normalises
	if len(s.SetsPerMuscle) != len(want) {
		t.Errorf("sets per muscle = %v, want %v", s.SetsPerMuscle, want)
	}
	for m, n := range want {
		if s.SetsPerMuscle[m] != n {
			t.Errorf("%s = %d, want %d", m, s.SetsPerMuscle[m], n)
		}
	}
}

// A region counts once for its group in SetsPerMuscle and by itself in SetsPerRegion (R83).
func TestSetsPerRegionRollsUp(t *testing.T) {
	f := newFixture(t)
	_, err := f.q.UpsertExercise(f.ctx, sqlc.UpsertExerciseParams{Slug: "custom-raise", Name: "Plate Raise",
		PrimaryMuscles: []string{"front delts", "side delts", "shoulders"}, Source: "custom", UserID: &f.user})
	must(t, err)
	f.exercise("press", "shoulders")
	f.record("2026-11-03", true, entry("custom-raise", set(10, 5, model.UnitKg), set(10, 5, model.UnitKg)), entry("press", set(5, 60, model.UnitKg)))

	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	if len(s.SetsPerMuscle) != 1 || s.SetsPerMuscle["shoulders"] != 3 {
		t.Errorf("sets per muscle = %v, want shoulders: 3", s.SetsPerMuscle)
	}
	if len(s.SetsPerRegion) != 2 || s.SetsPerRegion["front delts"] != 2 || s.SetsPerRegion["side delts"] != 2 {
		t.Errorf("sets per region = %v, want front delts: 2, side delts: 2", s.SetsPerRegion)
	}
}

// An entry without a slug is attributed by its name, then by any alternate, before "unknown" (R80).
func TestSetsPerMuscleFallsBackToNames(t *testing.T) {
	f := newFixture(t)
	_, err := f.q.UpsertExercise(f.ctx, sqlc.UpsertExerciseParams{Slug: "back-extension-bodyweight", Name: "Back Extension",
		Aliases: []string{"Hyperextensions (Back Extensions)"}, PrimaryMuscles: []string{"lower back"}, Source: "curated"})
	must(t, err)
	f.exercise("dead-hang-bodyweight", "forearms")
	byName := model.RecordEntry{Name: "back extension", Sets: []model.RecordSet{set(10, 0, model.UnitKg)}}
	byAlias := model.RecordEntry{Name: "Hyperextensions (Back Extensions)", Sets: []model.RecordSet{set(10, 0, model.UnitKg)}}
	byAlternate := model.RecordEntry{Name: "Hang", Alternates: []string{"Hang", "dead-hang-bodyweight"}, Sets: []model.RecordSet{set(10, 0, model.UnitKg)}}
	f.record("2026-11-03", true, byName, byAlias, byAlternate, entry("mystery", set(10, 20, model.UnitKg)))

	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	want := map[string]int{"lower back": 2, "forearms": 1, "unknown": 1}
	if len(s.SetsPerMuscle) != len(want) {
		t.Errorf("sets per muscle = %v, want %v", s.SetsPerMuscle, want)
	}
	for m, n := range want {
		if s.SetsPerMuscle[m] != n {
			t.Errorf("%s = %d, want %d", m, s.SetsPerMuscle[m], n)
		}
	}
}

func TestWeightTrendMovingAverage(t *testing.T) {
	f := newFixture(t)
	f.weigh("2026-11-01", 80)
	f.weigh("2026-11-02", 79)
	f.weigh("2026-11-04", 78)
	f.weigh("2026-11-10", 77)

	tr, err := f.svc.WeightTrend(f.ctx, f.user, day("2026-11-01"), day("2026-11-30"))
	must(t, err)
	if len(tr.Points) != 4 {
		t.Fatalf("points = %d, want 4", len(tr.Points))
	}
	// Trailing 7 days inclusive: Nov 10 covers Nov 4..10 = 78 and 77.
	for i, want := range []float64{80, 79.5, 79, 77.5} {
		near(t, tr.Points[i].Date.Format(time.DateOnly), tr.Points[i].MovingAvg7.Value, want)
	}
	// Least squares of the raw weights over days 0, 1, 3, 9.
	near(t, "weekly rate", tr.WeeklyRate.Value, -14.5*7/48.75)
}

func TestAdherenceRange(t *testing.T) {
	f := newFixture(t)
	f.planned("2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05")
	f.dayLog("2026-11-02", "done")
	f.dayLog("2026-11-03", "partial")
	f.dayLog("2026-11-04", "skipped")
	f.meals("2026-11-02", `{"kcal":2000}`, `[{"items":[{"macros":{"kcal":500},"planned":false}]}]`)
	f.meals("2026-11-03", `{"kcal":2000}`, `[{"items":[{"macros":{"kcal":500},"planned":true}]}]`)
	f.meals("2026-11-06", `{"kcal":2000}`, `[]`)

	// Today is Nov 5: unlogged Nov 5 and Nov 6 are pending, unlogged Nov 3 meals are skipped.
	a, err := f.svc.Adherence(f.ctx, f.user, day("2026-11-01"), day("2026-11-07"), day("2026-11-05"))
	must(t, err)
	if got, want := a.Workouts, (Counts{Planned: 4, Done: 1, Partial: 1, Skipped: 1, Pending: 1, Ratio: 1.0 / 3}); got != want {
		t.Errorf("workouts = %+v, want %+v", got, want)
	}
	if got, want := a.Meals, (Counts{Planned: 3, Done: 1, Skipped: 1, Pending: 1, Ratio: 0.5}); got != want {
		t.Errorf("meals = %+v, want %+v", got, want)
	}
}

func TestWeeklyMacrosAndWeight(t *testing.T) {
	f := newFixture(t)
	f.meals("2026-11-02", `{"kcal":2000,"protein":150}`, `[{"items":[{"macros":{"kcal":600,"protein":40}},{"macros":{"kcal":900,"protein":60},"planned":true}]}]`)
	f.meals("2026-11-03", `{"kcal":1000}`, `[]`)                                      // target, nothing eaten
	f.meals("2026-11-04", `{}`, `[{"items":[{"macros":{"kcal":700,"protein":50}}]}]`) // eaten, no target
	f.weigh("2026-10-30", 82)
	f.weigh("2026-11-02", 81)
	f.weigh("2026-11-08", 79)

	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	if s.Macros.TargetDays != 2 || s.Macros.ActualDays != 2 {
		t.Errorf("days = %d/%d, want 2/2", s.Macros.TargetDays, s.Macros.ActualDays)
	}
	near(t, "target kcal", s.Macros.TargetAvg.Kcal, 1500)
	near(t, "target protein", s.Macros.TargetAvg.Protein, 75)
	near(t, "actual kcal", s.Macros.ActualAvg.Kcal, 650)
	near(t, "actual protein", s.Macros.ActualAvg.Protein, 45)
	if s.Weight.Start == nil || s.Weight.End == nil || s.Weight.Avg7 == nil {
		t.Fatalf("weight = %+v", s.Weight)
	}
	near(t, "start", s.Weight.Start.Value, 81)
	near(t, "end", s.Weight.End.Value, 79)
	near(t, "avg7", s.Weight.Avg7.Value, 80) // Oct 30 is outside the 7 days ending Nov 8
}

func TestWeeklySummaryNeedsMonday(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-03"), day("2026-11-05"))
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestUnfinalizedRecordsAreExcluded(t *testing.T) {
	f := newFixture(t)
	f.exercise("squat", "quadriceps")
	f.record("2026-11-03", false, entry("squat", set(5, 200, model.UnitKg)))
	f.record("2026-11-04", true, entry("squat", set(5, 100, model.UnitKg)))

	h, err := f.svc.LiftHistory(f.ctx, f.user, "squat", time.Time{}, time.Time{})
	must(t, err)
	if len(h) != 1 || h[0].Date != day("2026-11-04") {
		t.Errorf("history = %+v, want only Nov 4", h)
	}
	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	near(t, "e1rm", s.E1RM["squat"].Value, 100*(1+5.0/30))
	if s.SetsPerMuscle["quadriceps"] != 1 {
		t.Errorf("sets = %v, want 1 quadriceps", s.SetsPerMuscle)
	}
}

func TestWeightTrendSinglePoint(t *testing.T) {
	f := newFixture(t)
	f.weigh("2026-11-03", 80)
	tr, err := f.svc.WeightTrend(f.ctx, f.user, day("2026-11-01"), day("2026-11-30"))
	must(t, err)
	if len(tr.Points) != 1 || tr.WeeklyRate.Value != 0 {
		t.Errorf("trend = %+v, want one point and rate 0", tr)
	}
	near(t, "avg", tr.Points[0].MovingAvg7.Value, 80)
}

func TestWeightTrendWindowEdges(t *testing.T) {
	f := newFixture(t)
	f.weigh("2026-10-29", 100) // before the lookback
	f.weigh("2026-11-03", 80)  // inside the lookback, before from
	f.weigh("2026-11-06", 78)
	f.weigh("2026-11-13", 76) // a full week after Nov 6: Nov 6 leaves the window

	tr, err := f.svc.WeightTrend(f.ctx, f.user, day("2026-11-05"), day("2026-11-30"))
	must(t, err)
	if len(tr.Points) != 2 {
		t.Fatalf("points = %d, want 2", len(tr.Points))
	}
	near(t, "Nov 6", tr.Points[0].MovingAvg7.Value, 79)
	near(t, "Nov 13", tr.Points[1].MovingAvg7.Value, 76)
	near(t, "rate", tr.WeeklyRate.Value, -2)
}

func TestEmptyRangesAreZero(t *testing.T) {
	f := newFixture(t)
	s, err := f.svc.WeeklySummary(f.ctx, f.user, day("2026-11-02"), day("2026-12-01"))
	must(t, err)
	if s.Adherence != (Counts{}) || len(s.SetsPerMuscle) != 0 || len(s.E1RM) != 0 || s.Macros != (MacroSummary{}) ||
		s.Weight.Start != nil || s.Weight.End != nil || s.Weight.Avg7 != nil {
		t.Errorf("summary not zero: %+v", s)
	}
	h, err := f.svc.LiftHistory(f.ctx, f.user, "squat", time.Time{}, time.Time{})
	must(t, err)
	if len(h) != 0 {
		t.Errorf("history = %v", h)
	}
	tr, err := f.svc.WeightTrend(f.ctx, f.user, day("2026-11-01"), day("2026-11-30"))
	must(t, err)
	if len(tr.Points) != 0 || tr.WeeklyRate.Value != 0 {
		t.Errorf("trend = %+v", tr)
	}
	a, err := f.svc.Adherence(f.ctx, f.user, day("2026-11-01"), day("2026-11-30"), day("2026-11-15"))
	must(t, err)
	if a != (Report{}) {
		t.Errorf("adherence = %+v", a)
	}
}
