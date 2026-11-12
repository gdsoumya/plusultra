// Package api serves the Connect, gRPC and HTTP endpoints.
package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/export"
	"github.com/gdsoumya/plusultra/internal/insights"
	"github.com/gdsoumya/plusultra/internal/llmdocs"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
	syncsvc "github.com/gdsoumya/plusultra/internal/sync"
)

// Deps are the services the handlers use.
type Deps struct {
	Pool    *pgxpool.Pool
	Queries *sqlc.Queries
	Auth    *auth.Service
	Logs    *logs.Service
	// Programs is the program service Logs plans with; a nil one is built from Pool.
	Programs *programs.Service
	// PublicURL is the address clients use; an http:// URL drops the Secure cookie flag for local dev.
	PublicURL string
	// AllowedOrigins are browser origins, besides PublicURL's, that may call the RPCs with credentials.
	AllowedOrigins []string
	// ImagesDir holds the exercise images the catalog import copied, one directory per slug.
	ImagesDir string
	Version   string
	Logger    *slog.Logger
	// Finalizer finalizes workout records pushed with finalized set; Logs in production.
	Finalizer syncsvc.Finalizer
}

// Prefix is the path the RPCs and the exercise images live under; /healthz
// and the LLM docs stay at the root beside the web app.
const Prefix = "/api/v1"

const (
	maxRequestBytes = 1 << 20
	maxScriptBytes  = 4 << 20 // ProgramService carries scripts, CheckinService 2 MB photos
)

// Protocols returns HTTP/1.1 plus unencrypted HTTP/2, which gRPC clients need
// on a plaintext listener. Set it on the http.Server that serves NewHandler.
func Protocols() *http.Protocols {
	var p http.Protocols
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	return &p
}

// NewHandler mounts every service under Prefix, plus /healthz and the LLM docs.
func NewHandler(d Deps) http.Handler {
	base := connect.WithHandlerOptions(
		// First, so it runs outermost and also covers panics in the interceptors below.
		connect.WithRecover(func(_ context.Context, spec connect.Spec, _ http.Header, p any) error {
			d.Logger.Error("panic", "procedure", spec.Procedure, "panic", p, "stack", string(debug.Stack()))
			return connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}),
		connect.WithInterceptors(
			logRequests(d.Logger),
			auth.NewInterceptor(d.Auth,
				plusultrav1connect.AuthServiceRegisterProcedure,
				plusultrav1connect.AuthServiceLoginProcedure,
				"/"+plusultrav1connect.ReferenceServiceName+"/",
			),
			connect.UnaryInterceptorFunc(noteCaller),
			connect.UnaryInterceptorFunc(validateRequests),
		),
	)
	limit := func(n int) connect.HandlerOption {
		return connect.WithHandlerOptions(base, connect.WithReadMaxBytes(n))
	}
	std, scripts := limit(maxRequestBytes), limit(maxScriptBytes)

	rpc := http.NewServeMux()
	mux := rpc
	mux.Handle(plusultrav1connect.NewAuthServiceHandler(&authServer{d}, std))
	mux.Handle(plusultrav1connect.NewApiKeyServiceHandler(&apiKeyServer{d}, std))
	docs := llmdocs.Build(d.PublicURL)
	mux.Handle(plusultrav1connect.NewReferenceServiceHandler(referenceServer{docs}, std))
	progs := cmp.Or(d.Programs, programs.NewService(d.Pool))
	mux.Handle(plusultrav1connect.NewProgramServiceHandler(&programServer{progs}, scripts))
	mux.Handle(plusultrav1connect.NewSyncServiceHandler(newSyncServer(d, progs), limit(maxSyncBytes)))
	mux.Handle(plusultrav1connect.NewCalendarServiceHandler(&calendarServer{d}, std))
	mux.Handle(plusultrav1connect.NewWorkoutServiceHandler(&workoutServer{d}, std))
	mux.Handle(plusultrav1connect.NewMealServiceHandler(&mealServer{logs.NewMealService(d.Queries, d.Pool), progs, d.Queries}, std))
	mux.Handle(plusultrav1connect.NewCheckinServiceHandler(&checkinServer{logs.NewCheckinService(d.Queries, d.Pool), d.Queries}, scripts))
	mux.Handle(plusultrav1connect.NewExerciseServiceHandler(&exerciseServer{d.Queries, d.Pool, progs, strings.TrimSuffix(d.PublicURL, "/")}, std))
	mux.Handle(plusultrav1connect.NewFoodServiceHandler(&foodServer{d.Queries}, std))
	mux.Handle(plusultrav1connect.NewInsightServiceHandler(&insightServer{insights.New(d.Pool), d.Queries}, std))
	exportOpts := connect.WithHandlerOptions(base, connect.WithReadMaxBytes(maxImportBytes), connect.WithSendMaxBytes(maxExportBytes))
	mux.Handle(plusultrav1connect.NewExportServiceHandler(&exportServer{export.NewService(d.Pool, progs, d.Logs)}, exportOpts))

	// The web app may run on another origin in dev; public reads stay open to every page.
	api := http.NewServeMux()
	api.Handle("/", browserCORS(append(originOf(d.PublicURL), d.AllowedOrigins...)).Handler(rpc))
	public := publicCORS()
	images := os.DirFS(d.ImagesDir)
	api.Handle("GET "+imagesPath+"{id}/{file}", public.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Join(r.PathValue("id"), r.PathValue("file"))
		if st, err := fs.Stat(images, name); err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, images, name) //nolint:gosec // two mux segments, checked by fs.Stat on the images dir
	})))

	mux = http.NewServeMux()
	mux.Handle(Prefix+"/", http.StripPrefix(Prefix, api))
	healthz := public.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			d.Logger.Error("healthz: ping database", "err", err)
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprintln(w, "ok", d.Version)
	}))
	// Clients that know only the API base URL probe health under the prefix.
	mux.Handle("GET /healthz", healthz)
	mux.Handle("GET "+Prefix+"/healthz", healthz)
	mux.Handle("GET /llms.txt", public.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeText(w, "text/plain", docs.LLMSTxt)
	})))
	mux.Handle("GET /llms/{name}", public.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, ok := docs.Page(r.PathValue("name"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeText(w, "text/markdown", page)
	})))
	return mux
}

func writeText(w http.ResponseWriter, contentType, body string) {
	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	_, _ = w.Write([]byte(body)) //nolint:gosec // embedded docs, not request data
}
