// Package dbtest provides an isolated, migrated Postgres schema per test.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db"
)

const defaultURL = "postgres://plusultra:plusultra@localhost:5433/plusultra_test?sslmode=disable" //nolint:gosec // local dev database

// New returns a pool whose search_path is a fresh migrated schema, dropped on cleanup.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultURL
	}
	ensureDatabase(ctx, t, dsn)

	admin, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unreachable (run `make db-up`): %v", err)
	}
	t.Cleanup(admin.Close)
	// pg_trgm lives in public so every test schema can reach it through its search_path.
	// Parallel packages race to create it on a fresh database; the loser's unique violation is harmless.
	var pgErr *pgconn.PgError
	if _, err := admin.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`); err != nil &&
		(!errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation) {
		t.Fatalf("create extension: %v", err)
	}

	var b [6]byte
	_, _ = rand.Read(b[:])
	schema := "test_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// ensureDatabase creates the target database when it does not exist yet. It skips
// the test only when Postgres itself is unreachable.
func ensureDatabase(ctx context.Context, t *testing.T, dsn string) {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		t.Fatalf("TEST_DATABASE_URL has no database name: %q", dsn)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err == nil {
		_ = conn.Close(ctx)
		return
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.InvalidCatalogName {
		t.Skipf("postgres unreachable (run `make db-up`): %v", err)
	}

	u.Path = "/plusultra"
	maint, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("connect maintenance database: %v", err)
	}
	defer func() { _ = maint.Close(ctx) }()
	// A parallel package may create it first; that error is harmless.
	_, _ = maint.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize())
}
