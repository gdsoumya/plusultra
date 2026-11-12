package catalog

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// seedFormat is bumped whenever an importer's output changes, so an image with
// unchanged datasets still reseeds after that upgrade.
const seedFormat = 2

// SeedDir is where the image carries the datasets.
const SeedDir = "/seed"

// SEED_CATALOG modes: SeedAuto seeds when the stored catalog version differs,
// SeedOff never touches the catalog.
const (
	SeedAuto = "auto"
	SeedOff  = "off"
)

const catalogVersionKey = "catalog_version"

// Seed upserts the whole catalog from dir (VERSION, exercises/ as a
// free-exercise-db checkout, sr.zip) when "<format>-<VERSION>" differs from
// server_meta catalog_version, then stores it. Exercises are keyed by slug and
// foods by (source, source_id); the upserts skip user-owned rows. A missing dir
// is logged and skipped so a bare binary still serves.
func Seed(ctx context.Context, pool *pgxpool.Pool, mode, dir, imagesDir string) error {
	switch mode {
	case SeedOff:
		return nil
	case SeedAuto:
	default:
		return fmt.Errorf("SEED_CATALOG must be %q or %q, not %q", SeedAuto, SeedOff, mode)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "VERSION")) //nolint:gosec // operator-supplied path
	if errors.Is(err, fs.ErrNotExist) {
		slog.Info("no catalog seed in the image, skipping", "dir", dir)
		return nil
	}
	if err != nil {
		return err
	}
	version := fmt.Sprintf("%d-%s", seedFormat, strings.TrimSpace(string(raw)))
	q := sqlc.New(pool)
	stored, err := q.GetServerMeta(ctx, catalogVersionKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if stored == version {
		return nil
	}
	slog.Info("seeding catalog", "from", stored, "to", version, "images", imagesDir)

	// One transaction per seed: thousands of auto-committed rows each wait for an fsync, which on a
	// NAS disk turns a four-second seed into many minutes.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tq := q.WithTx(tx)
	exercises, err := ImportFreeExerciseDB(ctx, tq, filepath.Join(dir, "exercises"), imagesDir)
	if err != nil {
		return err
	}
	slog.Info("catalog seed: exercises done", "count", exercises)
	foods, err := ImportUSDA(ctx, tq, filepath.Join(dir, "sr.zip"))
	if err != nil {
		return err
	}
	ifct, err := ImportIFCT(ctx, tq, "")
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.Info("catalog seed: foods done", "count", foods+ifct)
	if _, err := ImportFoodConcepts(ctx, pool, ""); err != nil {
		return err
	}
	slog.Info("catalog seeded", "from", stored, "to", version, "exercises", exercises, "foods", foods+ifct)
	return q.SetServerMeta(ctx, sqlc.SetServerMetaParams{Key: catalogVersionKey, Value: version})
}
