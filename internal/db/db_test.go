package db_test

import (
	"bytes"
	"errors"
	"slices"

	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"

	"github.com/gdsoumya/plusultra/internal/db"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

func TestMigrateAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	q := sqlc.New(pool)

	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "ann", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := q.CreateProgram(ctx, sqlc.CreateProgramParams{
		UserID: u.ID, Kind: "workout", Name: "p", Script: "s", Structure: []byte(`{}`), Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertResolvedDay(ctx, sqlc.UpsertResolvedDayParams{
		ProgramID: p.ID, Week: 1, Day: 1, Kind: "training", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	days, err := q.ListResolvedDays(ctx, p.ID)
	if err != nil || len(days) != 1 {
		t.Fatalf("days = %d, err = %v", len(days), err)
	}

	date := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	const insert = `INSERT INTO day_logs (user_id, date, status) VALUES ($1, $2, 'done')`
	if _, err := pool.Exec(ctx, insert, u.ID, date); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, insert, u.ID, date); err == nil {
		t.Fatal("duplicate (user_id, date) day_log accepted")
	}
}

func TestMigrateIdempotent(t *testing.T) {
	pool := dbtest.New(t)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
}

func TestSearchAndChangedSince(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	q := sqlc.New(pool)
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: "bob", PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{Slug: "bench-press", Name: "Bench Press"}); err != nil {
		t.Fatal(err)
	}
	ex, err := q.SearchCatalog(ctx, sqlc.SearchCatalogParams{Q: "bench pres", UserID: &u.ID, MaxRows: 5})
	if err != nil || len(ex) != 1 {
		t.Fatalf("exercises = %d, err = %v", len(ex), err)
	}

	if _, err := q.UpsertFood(ctx, sqlc.UpsertFoodParams{Source: "t", Name: "Rolled Oats", Aliases: []string{"porridge"}, Per100g: []byte(`{}`), Units: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	foods, err := q.SearchFoods(ctx, sqlc.SearchFoodsParams{Tsq: "porridge:*", Lq: "porridge", UserID: &u.ID, MaxRows: 5})
	if err != nil || len(foods) != 1 {
		t.Fatalf("foods = %d, err = %v", len(foods), err)
	}

	d, err := q.UpsertDayLog(ctx, sqlc.UpsertDayLogParams{UserID: u.ID, Date: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := q.DeleteDayLog(ctx, sqlc.DeleteDayLogParams{UserID: u.ID, Date: d.Date}); err != nil || n != 1 {
		t.Fatalf("delete = %d, err = %v", n, err)
	}
	if changed, err := q.ListDaysChangedSince(ctx, sqlc.ListDaysChangedSinceParams{UserID: u.ID, MaxRows: 10}); err != nil || len(changed) != 0 {
		t.Fatalf("a row changed just now is listed: %v, err = %v", changed, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE day_logs SET updated_at = updated_at - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	changed, err := q.ListDaysChangedSince(ctx, sqlc.ListDaysChangedSinceParams{UserID: u.ID, MaxRows: 10})
	if err != nil || len(changed) != 1 || changed[0].DeletedAt == nil {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	cursor, err := q.ListDaysChangedSince(ctx, sqlc.ListDaysChangedSinceParams{UserID: u.ID, Since: changed[0].UpdatedAt, SinceID: changed[0].ID, MaxRows: 10})
	if err != nil || len(cursor) != 0 {
		t.Fatalf("after cursor = %d, err = %v", len(cursor), err)
	}
}

func newUser(t *testing.T, q *sqlc.Queries, name string) sqlc.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{Username: name, PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCustomCatalogIsolation(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	a, b := newUser(t, q, "a"), newUser(t, q, "b")

	if _, err := q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{Slug: "bench-press", Name: "Bench Press"}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{Slug: "bench-press", Name: "Hijacked", UserID: &a.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("custom upsert over catalog slug: err = %v, want ErrNoRows", err)
	}
	got, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "bench-press", UserID: &a.ID})
	if err != nil || got.Name != "Bench Press" {
		t.Fatalf("catalog row = %+v, err = %v", got, err)
	}

	if _, err := q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{Slug: "my-lift", Name: "My Lift", UserID: &a.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{Slug: "my-lift", Name: "Stolen", UserID: &b.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-user upsert: err = %v, want ErrNoRows", err)
	}
	if _, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "my-lift", UserID: &b.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("other user's exercise visible: err = %v", err)
	}

	barcode := "123"
	f, err := q.UpsertFood(ctx, sqlc.UpsertFoodParams{Source: "custom", Name: "Mine", Barcode: &barcode, Per100g: []byte(`{}`), Units: []byte(`{}`), UserID: &a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetFood(ctx, sqlc.GetFoodParams{ID: f.ID, UserID: &b.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("other user's food visible: err = %v", err)
	}
	if _, err := q.GetFoodByBarcode(ctx, sqlc.GetFoodByBarcodeParams{Barcode: &barcode, UserID: &b.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("other user's barcode visible: err = %v", err)
	}
	if _, err := q.GetFoodByBarcode(ctx, sqlc.GetFoodByBarcodeParams{Barcode: &barcode, UserID: &a.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestInviteStaysUsedAfterUserDeleted(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	admin, joiner := newUser(t, q, "admin"), newUser(t, q, "joiner")
	if _, err := q.CreateInvite(ctx, sqlc.CreateInviteParams{Code: "c", CreatedBy: admin.ID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	consume := func() int64 {
		n, err := q.ConsumeInvite(ctx, sqlc.ConsumeInviteParams{Code: "c", UsedBy: &joiner.ID})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if consume() != 1 {
		t.Fatal("first consume failed")
	}
	if err := q.DeleteUser(ctx, joiner.ID); err != nil {
		t.Fatal(err)
	}
	if consume() != 0 {
		t.Fatal("invite reusable after consuming user was deleted")
	}
}

func TestChangedSinceOrderingLimitAndVersion(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	q := sqlc.New(pool)
	u := newUser(t, q, "c")
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	var ids []uuid.UUID
	for i := range 3 {
		d, err := q.UpsertDayLog(ctx, sqlc.UpsertDayLogParams{UserID: u.ID, Date: base.AddDate(0, 0, i), Status: "done"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.ID)
	}
	// Force a tie on updated_at so the id tiebreaker decides the order.
	if _, err := pool.Exec(ctx, `UPDATE day_logs SET updated_at = $2 WHERE user_id = $1`, u.ID, base); err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(ids, func(x, y uuid.UUID) int { return bytes.Compare(x[:], y[:]) })

	var got []uuid.UUID
	p := sqlc.ListDaysChangedSinceParams{UserID: u.ID, MaxRows: 2}
	for {
		page, err := q.ListDaysChangedSince(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) > 2 {
			t.Fatalf("page size %d exceeds limit", len(page))
		}
		if len(page) == 0 {
			break
		}
		for _, d := range page {
			got = append(got, d.ID)
		}
		last := page[len(page)-1]
		p.Since, p.SinceID = last.UpdatedAt, last.ID
	}
	if !slices.Equal(got, ids) {
		t.Fatalf("paged ids = %v, want %v", got, ids)
	}

	d, err := q.UpsertDayLog(ctx, sqlc.UpsertDayLogParams{UserID: u.ID, Date: base, Status: "partial"})
	if err != nil || d.Version != 2 {
		t.Fatalf("version = %d, err = %v", d.Version, err)
	}
}
