# plusultra

Self-hosted Go server (Connect RPC + MCP over Postgres). Module `github.com/gdsoumya/plusultra`.

## Commands

- `make tools`: install protoc plugins, sqlc, goose, govulncheck
- `make generate`: `buf generate` (Go into `gen/go`), `buf generate --template buf.gen.ts.yaml`
  (TypeScript into `app/src/gen`), `sqlc generate`
- `make check`: buf format check, buf lint, golangci-lint, go vet, go test ./...
- `make e2e`: builds the compose stack under project `plusultra-e2e` (ports 18080 and 15433),
  drives it over Connect JSON and MCP (`e2e/smoke_test.go`, build tag `e2e`), then `down -v`
- `make db-up` / `make db-down`: the stack's Postgres on port 5433 (from `docker-compose.override.yaml`)
- `make web`: `npx expo export --platform web` into `internal/web/dist` (gitignored but `.gitkeep`), which the
  server embeds and serves at `/`; without it the binary serves the placeholder page
- `make run` / `make build`: run `plusultra serve` or build `cmd/plusultra` (`build` runs `web` first). `serve` needs `DATABASE_URL`;
  optional `LISTEN` (`:8080`), `REGISTRATION` (`invite`|`open`), `PUBLIC_URL`, `ALLOWED_ORIGINS`
  (extra browser origins for CORS), `IMAGES_DIR` (`/data/exercises`), `SEED_CATALOG` (`auto`|`off`: apply the
  catalog shipped in `/seed` when `/seed/VERSION` differs from server_meta `catalog_version`). No session secret.
- `make docker` builds the local image (a Node 22 stage exports the web app first; a `seed` stage downloads the
  pinned catalog datasets and shrinks the exercise images with `cmd/seedprep`, so building needs network);
  both tag `gdsoumya/plusultra:latest` and `:<short commit>`; `make docker-multiarch` builds `linux/amd64,linux/arm64`
  with buildx and pushes with `PUSH=1`
- Routes: RPCs and exercise images under `/api/v1`; `/mcp`, `/healthz` (also at `/api/v1/healthz`), `/llms.txt`, `/llms/{name}` at the root;
  everything else is the web app with an `index.html` fallback
- Compose: `docker-compose.yaml` is the deployment (image `${PLUSULTRA_IMAGE}`, `postgres:17-alpine` without a host
  port, host paths under `DATA_ROOT`, env `POSTGRES_PASSWORD`, `LISTEN_PORT`, `PUID`/`PGID`, `PUBLIC_URL`,
  `REGISTRATION`, `SEED_CATALOG`; `.env.example` lists them); `docker-compose.override.yaml` adds build-from-source,
  named volumes and Postgres on `PG_PORT` for development.
- CLI: `plusultra serve | migrate | seed-catalog | invite [-expires-in 168h] | version | import-exercises | import-foods`;
  everything but `serve` and `version` reads `DATABASE_URL` and migrates first.

## Layout

```
cmd/plusultra/     entrypoint and subcommands
e2e/               compose smoke test (make e2e)
internal/version/  build version
proto/plusultra/v1/ API definitions (buf module root is proto/)
gen/go/            generated Go messages and connect handlers; never edit
app/src/gen/       generated TypeScript (protoc-gen-es); never edit
internal/api/      connect server under /api/v1: handlers, interceptors, /healthz, /llms routes
internal/llmdocs/  docs for LLMs (/llms.txt, ReferenceService)
internal/mcp/      MCP endpoint (/mcp) over the generated protomcp registrars
internal/web/      embedded web export (dist/, from make web) served at /, placeholder without it
docs/              specs and plans
```

Other packages under `internal/`: engine, programs, logs, catalog, sync, insights, auth, db (sqlc, migrations).

## Rules

- TDD: write the failing test first. Tests live beside the code as `*_test.go`.
- DB tests need `make db-up` and read `TEST_DATABASE_URL`, default
  `postgres://plusultra:plusultra@localhost:5433/plusultra_test?sslmode=disable`.
- Keep files small, one responsibility each.
- Comment only the non-obvious why.
- Proto files are the single source of API truth.
- Scripts never run on clients.
- Use latest dependency versions. Run `govulncheck ./...` before committing dependency changes.
- Run `make check` before every commit.
- Run tests only through `make test` or `make check` (8 GB memory cap). Never run a bare `go test` on code whose allocations scale with user input.
- Conventional Commits, signed off: `git commit -sm "type(scope): summary"`.
- No AI attribution in commits or PRs.

## App

Expo SDK 58 / React Native 0.88 app in `app/` (TypeScript strict, Expo Router, Jest + RNTL, Playwright for web).

### Commands

- `make app-install`: npm install
- `make app-check`: typecheck, eslint, prettier, jest
- `make app-start` / `make app-android` / `make app-web`: Metro dev server
- `make app-e2e-web`: export web build, serve it, run Playwright
- `make app-e2e-android`: `app/e2e/android/smoke.sh` (boots emulator, builds, starts Metro, drives with agent-device)

### Layout

```
app/app/          Expo Router routes (thin, re-export from src)
app/src/ui/       shared components and theme
app/src/domain/   pure logic
app/src/data/     local storage and sync
app/src/api/      server client
app/src/features/ screens and feature code
app/e2e/          android/ (agent-device) and web/ (Playwright)
```

### Rules

- Tests live beside the code. Never put test files under `app/app/` (the router treats them as routes).
- Run `make app-check` before every app commit.
- Never start Metro with `CI=1` (it disables file watching). See `docs/dev/android-emulator.md`.
- Native config lives in `app.json` (expo-build-properties, exact alarms, `app/plugins/*`): after changing it run
  `npx expo prebuild --platform android --clean` before a native build; `expo run:android` does not re-run prebuild.
- Run Android e2e through `app/e2e/android/smoke.sh`, not ad hoc.
