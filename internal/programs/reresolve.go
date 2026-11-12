package programs

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const engineVersionKey = "engine_version"

// ReresolveAllIfUpgraded re-resolves every active program when build differs
// from the build that last resolved them, so stored days follow engine
// changes. A program that fails is logged and skipped.
func (s *Service) ReresolveAllIfUpgraded(ctx context.Context, build string) error {
	stored, err := s.Q.GetServerMeta(ctx, engineVersionKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if stored == build {
		return nil
	}
	ps, err := s.Q.ListActivePrograms(ctx)
	if err != nil {
		return err
	}
	n := 0
	for _, p := range ps {
		if err := s.Resolve(ctx, p.UserID, p.ID); err != nil {
			slog.Error("re-resolve after upgrade", "program", p.ID, "err", err)
			continue
		}
		n++
	}
	slog.Info("re-resolved active programs after upgrade", "from", stored, "to", build, "programs", n)
	return s.Q.SetServerMeta(ctx, sqlc.SetServerMetaParams{Key: engineVersionKey, Value: build})
}
