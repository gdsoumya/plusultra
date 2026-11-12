package catalog

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

func datasetDir(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "exercises.json"), []byte(json), 0o600); err != nil { //nolint:gosec // test temp dir
		t.Fatal(err)
	}
	return dir
}

func slugs(ctx context.Context, t *testing.T, q *sqlc.Queries, user uuid.UUID) []string {
	t.Helper()
	all, err := q.ListExercises(ctx, &user)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(all))
	for i, e := range all {
		out[i] = e.Slug
	}
	return out
}

func TestImportPrunesRenamedRows(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	dir := datasetDir(t, `[]`)
	rows := []Row{{Name: "Bench Press", Equipment: "barbell", Default: true}, {Name: "Row", Equipment: "cable", Default: true}}
	if _, err := importRows(ctx, q, rows, dir, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertUserExerciseData(ctx, sqlc.UpsertUserExerciseDataParams{UserID: u.ID, ExerciseSlug: "row-cable"}); err != nil {
		t.Fatal(err)
	}
	custom, err := CreateCustomExercise(ctx, q, u.ID, Custom{Name: "Mine"})
	if err != nil {
		t.Fatal(err)
	}

	renamed := []Row{{Name: "Flat Bench Press", Equipment: "barbell", Default: true}, {Name: "Seated Row", Equipment: "cable", Default: true}}
	n, err := importRows(ctx, q, renamed, dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got := slugs(ctx, t, q, u.ID)
	slices.Sort(got)
	// bench-press-barbell is pruned; row-cable stays because a user has data on it; the custom row is untouched.
	want := []string{"flat-bench-press-barbell", "row-cable", "seated-row-cable", custom.Slug}
	slices.Sort(want)
	if n != 2 || !slices.Equal(got, want) {
		t.Fatalf("n=%d slugs=%v want %v", n, got, want)
	}
}

func TestUnmatchedEntryNeverTakesBareName(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	dir := datasetDir(t, `[
	 {"id":"Odd_Press","name":"Bench  Press","equipment":"kettlebells","category":"strength","primaryMuscles":["Chest"],"secondaryMuscles":["Triceps"],"images":[]},
	 {"id":"Real_Bench","name":"Barbell Bench Press","equipment":"barbell","category":"strength","primaryMuscles":["chest"],"secondaryMuscles":[],"images":[]}]`)
	rows := []Row{{Name: "Bench Press", Equipment: "barbell", Default: true, SourceID: "Real_Bench"}}
	for range 2 { // the order of rows must not matter, so import twice
		if _, err := importRows(ctx, q, rows, dir, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	odd, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "bench-press-kettlebell", UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if odd.Name != "Bench Press" || odd.Source != sourceFreeDB || !slices.Equal(odd.PrimaryMuscles, []string{"chest"}) {
		t.Fatalf("unmatched entry = %+v", odd)
	}
	if got, err := ResolveName(ctx, q, u.ID, "bench press"); err != nil || got != "bench-press-barbell" {
		t.Fatalf("bare name = %q, %v", got, err)
	}
}

func TestImportTruncatesLongSteps(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	long := strings.Repeat("é", maxStepLen+5)
	dir := datasetDir(t, `[{"id":"Real_Bench","name":"Barbell Bench Press","equipment":"barbell","category":"strength","primaryMuscles":["chest"],"secondaryMuscles":[],"images":[],"instructions":["Lie down.","`+long+`"]}]`)
	rows := []Row{{Name: "Bench Press", Equipment: "barbell", Default: true, SourceID: "Real_Bench"}}
	if _, err := importRows(ctx, q, rows, dir, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	ex, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "bench-press-barbell"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Instructions) != 2 || ex.Instructions[0] != "Lie down." ||
		utf8.RuneCountInString(ex.Instructions[1]) != maxStepLen || !strings.HasSuffix(ex.Instructions[1], "…") {
		t.Fatalf("instructions = %d steps, last %d runes", len(ex.Instructions), utf8.RuneCountInString(ex.Instructions[1]))
	}
}

func TestStubTakesSiblingMuscles(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	dir := datasetDir(t, `[{"id":"Real_Bench","name":"Barbell Bench Press","equipment":"barbell","category":"strength","primaryMuscles":["Chest"],"secondaryMuscles":["Triceps"],"images":[]}]`)
	rows := []Row{
		{Name: "Bench Press", Equipment: "band", Muscles: []string{"chest"}, Secondary: []string{"shoulders"}},
		{Name: "Bench Press", Equipment: "barbell", Default: true, SourceID: "Real_Bench"},
		{Name: "Bench Press", Equipment: "dumbbell"},
		{Name: "Dead Hang", Equipment: "bodyweight", Default: true},
	}
	if _, err := importRows(ctx, q, rows, dir, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2][]string{
		"bench-press-band":     {{"chest"}, {"shoulders"}}, // its own column
		"bench-press-dumbbell": {{"chest"}, {"shoulders"}}, // copied from the first variant with muscles
		"dead-hang-bodyweight": {nil, nil},                 // nothing to copy from
	}
	for slug, w := range want {
		ex, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: slug, UserID: &u.ID})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(ex.PrimaryMuscles, w[0]) || !slices.Equal(ex.SecondaryMuscles, w[1]) {
			t.Errorf("%s muscles = %v / %v, want %v / %v", slug, ex.PrimaryMuscles, ex.SecondaryMuscles, w[0], w[1])
		}
	}
}
