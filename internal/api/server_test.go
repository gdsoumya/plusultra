package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

const (
	password  = "correct horse battery"
	httpsURL  = "https://plusultra.test"
	ninetyDay = 90 * 24 * time.Hour
)

type testEnv struct {
	root     string // the server; url is its API base
	url      string
	images   string
	pool     *pgxpool.Pool
	logs     *syncBuffer
	programs *programs.Service
	auth     plusultrav1connect.AuthServiceClient
	keys     plusultrav1connect.ApiKeyServiceClient
	ref      plusultrav1connect.ReferenceServiceClient
}

// newEnv serves NewHandler in invite mode, with HTTP/1.1 and h2c like serve does.
// Each opt adjusts the Deps first.
func newEnv(t *testing.T, publicURL string, opts ...func(*Deps)) *testEnv {
	t.Helper()
	pool := dbtest.New(t)
	logBuf := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, nil))
	svc := auth.NewService(pool, auth.Config{Registration: auth.RegistrationInvite})
	svc.Logger = logger
	progs := programs.NewService(pool)
	logSvc := logs.NewService(pool, progs)
	images := t.TempDir()
	d := Deps{Pool: pool, Queries: sqlc.New(pool), Auth: svc, Logs: logSvc, Programs: progs, Finalizer: logSvc,
		PublicURL: publicURL, ImagesDir: images, Version: "test", Logger: logger}
	for _, opt := range opts {
		opt(&d)
	}
	ts := httptest.NewUnstartedServer(NewHandler(d))
	ts.Config.Protocols = Protocols()
	ts.Start()
	t.Cleanup(ts.Close)
	c := http.DefaultClient
	return &testEnv{
		root: ts.URL, url: ts.URL + Prefix, images: images, pool: pool, logs: logBuf, programs: progs,
		auth: plusultrav1connect.NewAuthServiceClient(c, ts.URL+Prefix),
		keys: plusultrav1connect.NewApiKeyServiceClient(c, ts.URL+Prefix),
		ref:  plusultrav1connect.NewReferenceServiceClient(c, ts.URL+Prefix),
	}
}

func (e *testEnv) register(t *testing.T, username, invite string) *pb.RegisterResponse {
	t.Helper()
	res, err := e.auth.Register(t.Context(), connect.NewRequest(&pb.RegisterRequest{
		Username: username, Password: password, InviteCode: invite,
	}))
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	return res.Msg
}

func (e *testEnv) key(t *testing.T, token string, scopes ...pb.ApiKeyScope) *pb.CreateApiKeyResponse {
	t.Helper()
	res, err := e.keys.CreateApiKey(t.Context(), as(token, &pb.CreateApiKeyRequest{
		Name: "script", Scopes: scopes, ExpiresAt: timestamppb.New(time.Now().Add(30 * 24 * time.Hour)),
	}))
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return res.Msg
}

func (e *testEnv) me(t *testing.T, token string) (*pb.User, error) {
	t.Helper()
	res, err := e.auth.GetMe(t.Context(), as(token, &pb.GetMeRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetUser(), nil
}

func (e *testEnv) get(t *testing.T, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, e.root+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(b)
}

// as builds a request that authenticates with a bearer token or API key.
func as[T any](token string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}

func assertCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != want {
		t.Fatalf("err = %v (code %v), want code %v", err, got, want)
	}
}

func assertNear(t *testing.T, what string, got *timestamppb.Timestamp, want time.Time) {
	t.Helper()
	if d := got.AsTime().Sub(want); d > time.Minute || d < -time.Minute {
		t.Fatalf("%s = %v, want about %v", what, got.AsTime(), want)
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestReferenceIsPublic(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	lang, err := e.ref.GetProgramLanguageReference(ctx, connect.NewRequest(&pb.GetProgramLanguageReferenceRequest{}))
	if err != nil || lang.Msg.GetMarkdown() == "" {
		t.Fatalf("program language = %v, err %v", lang, err)
	}
	meal, err := e.ref.GetMealLanguageReference(ctx, connect.NewRequest(&pb.GetMealLanguageReferenceRequest{}))
	if err != nil || meal.Msg.GetMarkdown() == "" {
		t.Fatalf("meal language = %v, err %v", meal, err)
	}
	ex, err := e.ref.GetProgramExamples(ctx, connect.NewRequest(&pb.GetProgramExamplesRequest{}))
	if err != nil || ex.Msg.GetMarkdown() == "" {
		t.Fatalf("examples = %v, err %v", ex, err)
	}
	logf, err := e.ref.GetLogFormatReference(ctx, connect.NewRequest(&pb.GetLogFormatReferenceRequest{}))
	if err != nil || logf.Msg.GetMarkdown() == "" {
		t.Fatalf("log format = %v, err %v", logf, err)
	}

	code, index := e.get(t, "/llms.txt")
	if code != http.StatusOK || !strings.Contains(index, "/llms/program-language.md") {
		t.Fatalf("/llms.txt = %d %q", code, index)
	}
	if code, body := e.get(t, "/llms/program-language.md"); code != http.StatusOK || body != lang.Msg.GetMarkdown() {
		t.Fatalf("/llms/program-language.md = %d %q", code, body)
	}
	if code, _ := e.get(t, "/llms/nope.md"); code != http.StatusNotFound {
		t.Fatalf("/llms/nope.md = %d, want 404", code)
	}
}

// RPCs live under Prefix only; /healthz (also aliased under Prefix) and the docs stay at the root.
func TestAPIPrefix(t *testing.T) {
	e := newEnv(t, httpsURL)
	for path, want := range map[string]int{
		"/plusultra.v1.ReferenceService/GetProgramExamples":          http.StatusNotFound,
		Prefix + "/plusultra.v1.ReferenceService/GetProgramExamples": http.StatusOK,
	} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, e.root+path, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("POST %s = %d, want %d", path, res.StatusCode, want)
		}
	}
	// Clients that only know the API base URL probe health under the prefix.
	if code, body := e.get(t, Prefix+"/healthz"); code != http.StatusOK || !strings.HasPrefix(body, "ok") {
		t.Fatalf("%s/healthz = %d %q", Prefix, code, body)
	}
}

func TestValidateInterceptor(t *testing.T) {
	e := newEnv(t, httpsURL)
	_, err := e.auth.Register(t.Context(), connect.NewRequest(&pb.RegisterRequest{Username: "Bad Name", Password: password}))
	assertCode(t, err, connect.CodeInvalidArgument)
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	for _, d := range ce.Details() {
		v, err := d.Value()
		if vs, ok := v.(*validate.Violations); err == nil && ok {
			got := vs.GetViolations()[0]
			if got.GetRuleId() != "string.pattern" || got.GetField().GetElements()[0].GetFieldName() != "username" {
				t.Fatalf("violation = %v", got)
			}
			return
		}
	}
	t.Fatalf("no buf.validate.Violations detail in %v", ce.Details())
}

func TestHealthz(t *testing.T) {
	e := newEnv(t, httpsURL)
	if code, body := e.get(t, "/healthz"); code != http.StatusOK || !strings.HasPrefix(body, "ok") {
		t.Fatalf("/healthz = %d %q", code, body)
	}
	e.pool.Close()
	if code, _ := e.get(t, "/healthz"); code != http.StatusServiceUnavailable {
		t.Fatalf("/healthz with the database down = %d, want 503", code)
	}
}

func TestPublicError(t *testing.T) {
	logs := &syncBuffer{}
	l := slog.New(slog.NewTextHandler(logs, nil))
	notFound := connect.NewError(connect.CodeNotFound, errors.New("program not found"))
	for _, tc := range []struct {
		in   error
		want connect.Code
	}{
		{notFound, connect.CodeNotFound},
		{fmt.Errorf("get: %w", pgx.ErrNoRows), connect.CodeNotFound},
		{fmt.Errorf("query: %w", context.Canceled), connect.CodeCanceled},
		{errors.New("dial db.internal:5432"), connect.CodeInternal},
	} {
		if got := publicError(l, "/p", tc.in); connect.CodeOf(got) != tc.want {
			t.Errorf("publicError(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if got := publicError(l, "/p", notFound); got != notFound {
		t.Errorf("connect error not passed through: %v", got)
	}
	var ce *connect.Error
	if got := publicError(l, "/p", errors.New("dial db.internal:5432")); !errors.As(got, &ce) || strings.Contains(ce.Message(), "db.internal") {
		t.Errorf("internal detail leaked: %v", got)
	}
	if !strings.Contains(logs.String(), "db.internal") {
		t.Errorf("internal error not logged: %s", logs.String())
	}
	if publicError(l, "/p", nil) != nil {
		t.Error("nil error mapped")
	}
}

// A gRPC client over plaintext HTTP/2 reaches the same handlers as Connect does.
func TestGRPCOverH2C(t *testing.T) {
	e := newEnv(t, httpsURL)
	tok := e.register(t, "alice", "").GetToken()
	var h2c http.Protocols
	h2c.SetUnencryptedHTTP2(true)
	grpcClient := &http.Client{Transport: &http.Transport{Protocols: &h2c}}
	c := plusultrav1connect.NewAuthServiceClient(grpcClient, e.url, connect.WithGRPC())
	res, err := c.GetMe(t.Context(), as(tok, &pb.GetMeRequest{}))
	if err != nil || res.Msg.GetUser().GetUsername() != "alice" {
		t.Fatalf("GetMe over gRPC = %v, err %v", res, err)
	}
}

// A panic inside the interceptor chain (here a nil auth service) still reaches
// the client as Internal because recover runs outermost.
func TestInterceptorPanicIsInternal(t *testing.T) {
	e := newEnv(t, httpsURL, func(d *Deps) { d.Auth = nil })
	_, err := e.auth.GetMe(t.Context(), as(strings.Repeat("ab", 32), &pb.GetMeRequest{}))
	assertCode(t, err, connect.CodeInternal)
	if !strings.Contains(e.logs.String(), "panic") {
		t.Fatalf("panic not logged:\n%s", e.logs.String())
	}
}
