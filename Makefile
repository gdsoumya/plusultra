GO_BIN := $(shell go env GOPATH)/bin
# The app's node_modules may hold Go files; keep go tooling to our packages.
GO_PKGS := $(shell go list ./... | grep -v '/app/')
export PATH := $(GO_BIN):$(PATH)

VERSION ?= dev
LDFLAGS := -X github.com/gdsoumya/plusultra/internal/version.Version=$(VERSION)
IMAGE := gdsoumya/plusultra
TAGS := -t $(IMAGE):latest -t $(IMAGE):$(shell git rev-parse --short HEAD)

.PHONY: tools generate lint test check e2e db-up db-down run web build docker docker-multiarch

tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/gdsoumya/protomcp/cmd/protoc-gen-mcp@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

# Retried because the BSR dependency fetch rate-limits anonymous clients; set BUF_TOKEN to avoid it.
generate:
	for t in buf.gen.yaml buf.gen.ts.yaml; do \
		for i in 1 2 3; do \
			buf generate --template $$t && break || { [ $$i -lt 3 ] && sleep 5 || exit 1; }; \
		done; \
	done
	@if [ -f sqlc.yaml ]; then sqlc generate; fi

lint:
	buf format --diff --exit-code
	buf lint
	golangci-lint run ./...
	go vet $(GO_PKGS)

test:
	bash -c 'ulimit -v 8388608 && go test $(GO_PKGS)'

check: lint test

# Builds the compose stack, drives it over HTTP and MCP, then tears it down.
e2e:
	bash -c 'ulimit -v 8388608 && go test -tags e2e -count=1 -timeout 20m ./e2e/'

db-up:
	docker compose up -d --wait postgres

db-down:
	docker compose down

run:
	go run ./cmd/plusultra serve

# Exports the web app into internal/web/dist, which the server embeds; .gitkeep stays.
web:
	cd app && CI=1 npx expo export --platform web
	rm -rf internal/web/dist/* && cp -R app/dist/. internal/web/dist/

build: web
	go build -ldflags "$(LDFLAGS)" -o bin/plusultra ./cmd/plusultra

docker:
	docker build --build-arg VERSION=$(VERSION) $(TAGS) .

# PUSH=1 publishes both tags; without it the result loads into the local store only with containerd image storage.
docker-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 --build-arg VERSION=$(VERSION) $(TAGS) $(if $(PUSH),--push) .

.PHONY: app-install app-check app-start app-android app-web app-e2e-web app-e2e-android

app-install:
	cd app && npm install

app-check:
	cd app && npm run check

app-start:
	cd app && npx expo start

app-android:
	cd app && npx expo start --android

app-web:
	cd app && npx expo start --web

app-e2e-web:
	cd app && npx playwright install chromium && npx playwright test

# With E2E_SERVER_URL set, both drive the app against that server instead of the demo.
app-e2e-android:
	cd app && bash e2e/android/$(if $(E2E_SERVER_URL),server,smoke).sh
