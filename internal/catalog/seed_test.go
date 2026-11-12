package catalog_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// seedDir lays out /seed as the image ships it: VERSION, the exercise dataset and the USDA zip.
func seedDir(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(version+"\n"), 0o600); err != nil { //nolint:gosec // test temp dir
		t.Fatal(err)
	}
	if err := os.Rename(sampleDir(t), filepath.Join(dir, "exercises")); err != nil {
		t.Fatal(err)
	}
	zipData, err := os.ReadFile("testdata/usda-sample.zip")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sr.zip"), zipData, 0o600); err != nil { //nolint:gosec // test temp dir
		t.Fatal(err)
	}
	return dir
}

func counts(ctx context.Context, t *testing.T, pool *pgxpool.Pool) (exercises, foods int) {
	t.Helper()
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM exercises), (SELECT count(*) FROM foods)`).Scan(&exercises, &foods); err != nil {
		t.Fatal(err)
	}
	return exercises, foods
}

func TestSeedVersionsTheCatalog(t *testing.T) {
	ctx, q, pool := foodPool(t)
	images := t.TempDir()

	if err := catalog.Seed(ctx, pool, "off", seedDir(t, "aaa"), images); err != nil {
		t.Fatal(err)
	}
	if e, f := counts(ctx, t, pool); e != 0 || f != 0 {
		t.Fatalf("off seeded %d exercises and %d foods", e, f)
	}
	if err := catalog.Seed(ctx, pool, "auto", t.TempDir(), images); err != nil {
		t.Fatalf("missing seed dir: %v", err)
	}
	if _, err := q.GetServerMeta(ctx, "catalog_version"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing seed dir stored a version: %v", err)
	}

	dir := seedDir(t, "aaa")
	if err := catalog.Seed(ctx, pool, "auto", dir, images); err != nil {
		t.Fatal(err)
	}
	e, f := counts(ctx, t, pool)
	if e == 0 || f == 0 {
		t.Fatalf("fresh seed left %d exercises and %d foods", e, f)
	}
	if v, err := q.GetServerMeta(ctx, "catalog_version"); err != nil || v != "2-aaa" {
		t.Fatalf("catalog_version = %q, %v", v, err)
	}
	if _, err := os.Stat(filepath.Join(images, "squat-barbell", "0.jpg")); err != nil {
		t.Fatal(err)
	}

	// A user's rows and a hand edit of a catalog row, to tell a no-op from a reseed.
	user := newUser(ctx, t, q, "ann")
	custom, err := catalog.CreateCustomFood(ctx, q, user, catalog.CustomFood{Name: "Mine", Per100g: model.Macros{Kcal: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertUserExerciseData(ctx, sqlc.UpsertUserExerciseDataParams{UserID: user, ExerciseSlug: "squat-barbell", Notes: "keep"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE exercises SET name = 'Edited' WHERE slug = 'squat-barbell'`); err != nil {
		t.Fatal(err)
	}

	if err := catalog.Seed(ctx, pool, "auto", dir, images); err != nil {
		t.Fatal(err)
	}
	if ex, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "squat-barbell", UserID: &user}); err != nil || ex.Name != "Edited" {
		t.Fatalf("same version reimported: %q, %v", ex.Name, err)
	}

	if err := catalog.Seed(ctx, pool, "auto", seedDir(t, "bbb"), images); err != nil {
		t.Fatal(err)
	}
	if ex, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "squat-barbell", UserID: &user}); err != nil || ex.Name != "Squat" {
		t.Fatalf("new version did not reimport: %q, %v", ex.Name, err)
	}
	if v, _ := q.GetServerMeta(ctx, "catalog_version"); v != "2-bbb" {
		t.Fatalf("catalog_version = %q", v)
	}
	if e2, f2 := counts(ctx, t, pool); e2 != e || f2 != f+1 {
		t.Fatalf("reseed changed counts: %d/%d -> %d/%d", e, f, e2, f2)
	}
	if got, err := q.GetFood(ctx, sqlc.GetFoodParams{ID: custom.ID, UserID: &user}); err != nil || got.Name != "Mine" {
		t.Fatalf("custom food after reseed: %+v, %v", got, err)
	}
	if d, err := q.GetUserExerciseData(ctx, sqlc.GetUserExerciseDataParams{UserID: user, ExerciseSlug: "squat-barbell"}); err != nil || d.Notes != "keep" {
		t.Fatalf("exercise data after reseed: %+v, %v", d, err)
	}
}
