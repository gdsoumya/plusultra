package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdsoumya/plusultra/internal/api"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/mcp"
	"github.com/gdsoumya/plusultra/internal/programs"
	"github.com/gdsoumya/plusultra/internal/version"
	"github.com/gdsoumya/plusultra/internal/web"
)

// serve migrates the database and serves the API until SIGINT or SIGTERM.
// Each flag defaults to its environment variable.
func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dbURL := fs.String("database-url", os.Getenv("DATABASE_URL"), "Postgres URL (DATABASE_URL)")
	listen := fs.String("listen", cmp.Or(os.Getenv("LISTEN"), ":8080"), "listen address (LISTEN)")
	registration := fs.String("registration", cmp.Or(os.Getenv("REGISTRATION"), auth.RegistrationInvite), "open or invite (REGISTRATION)")
	publicURL := fs.String("public-url", os.Getenv("PUBLIC_URL"), "address clients use (PUBLIC_URL)")
	origins := fs.String("allowed-origins", os.Getenv("ALLOWED_ORIGINS"), "comma-separated browser origins besides PUBLIC_URL (ALLOWED_ORIGINS)")
	imagesDir := fs.String("images-dir", cmp.Or(os.Getenv("IMAGES_DIR"), "/data/exercises"), "exercise images directory (IMAGES_DIR)")
	seed := fs.String("seed-catalog", cmp.Or(os.Getenv("SEED_CATALOG"), catalog.SeedAuto), "auto or off (SEED_CATALOG)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case *dbURL == "":
		return errors.New("DATABASE_URL is not set")
	case *registration != auth.RegistrationOpen && *registration != auth.RegistrationInvite:
		return fmt.Errorf("REGISTRATION must be %q or %q, not %q", auth.RegistrationOpen, auth.RegistrationInvite, *registration)
	case *seed != catalog.SeedAuto && *seed != catalog.SeedOff:
		return fmt.Errorf("SEED_CATALOG must be %q or %q, not %q", catalog.SeedAuto, catalog.SeedOff, *seed)
	}
	allowed, err := api.ParseOrigins(*origins)
	if err != nil {
		return fmt.Errorf("ALLOWED_ORIGINS: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := db.Connect(ctx, *dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	progs := programs.NewService(pool)
	logger := slog.Default()
	svc := auth.NewService(pool, auth.Config{Registration: *registration})
	svc.Logger = logger
	logSvc := logs.NewService(pool, progs)
	deps := api.Deps{
		Pool: pool, Queries: sqlc.New(pool), Auth: svc, Logs: logSvc, Programs: progs, Finalizer: logSvc,
		PublicURL: *publicURL, AllowedOrigins: allowed, ImagesDir: *imagesDir, Version: version.Version, Logger: logger,
	}
	mcpHandler, err := mcp.NewHandler(deps)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	apiHandler := api.NewHandler(deps)
	for _, p := range []string{"/api/", "GET /healthz", "GET /llms.txt", "GET /llms/"} {
		mux.Handle(p, apiHandler)
	}
	mux.Handle("/", web.Handler())
	srv := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		Protocols:         api.Protocols(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("serving", "addr", *listen, "version", version.Version)
	// Seeding copies the catalog images onto a volume that may be slow on a NAS, so it runs after
	// the listener is up: /healthz answers at once and the first start is visible in the logs.
	go func() {
		// A failed seed leaves the catalog as it was; the server still serves.
		if err := catalog.Seed(ctx, pool, *seed, catalog.SeedDir, *imagesDir); err != nil {
			logger.Error("catalog seed", "err", err)
		}
		// A dev build has no stable version, so it re-resolves on every start.
		build := version.Version
		if build == "dev" {
			build = "dev-" + time.Now().Format(time.RFC3339Nano)
		}
		if err := progs.ReresolveAllIfUpgraded(ctx, build); err != nil {
			logger.Error("re-resolve after upgrade", "err", err)
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		stop() // a second signal during the drain kills the process
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
