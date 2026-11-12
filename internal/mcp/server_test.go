package mcp

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/api"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// JSON-RPC codes protomcp assigns to gRPC Unauthenticated and PermissionDenied.
const (
	codeUnauthenticated  = -32001
	codePermissionDenied = -32002
)

type env struct {
	url  string
	auth *auth.Service
	user sqlc.User
}

// newEnv serves NewHandler at /mcp over real HTTP with one registered user.
func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := auth.NewService(pool, auth.Config{Registration: auth.RegistrationOpen})
	svc.Logger = logger
	progs := programs.NewService(pool)
	h, err := NewHandler(api.Deps{
		Pool: pool, Queries: sqlc.New(pool), Auth: svc, Logs: logs.NewService(pool, progs), Programs: progs,
		PublicURL: "https://plusultra.test", Version: "test", Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	user, err := svc.Register(t.Context(), auth.NewUser{Username: "alice", Password: "correct horse battery"})
	if err != nil {
		t.Fatal(err)
	}
	return &env{url: ts.URL + "/mcp", auth: svc, user: user}
}

func (e *env) key(t *testing.T, scopes ...string) (string, sqlc.ApiKey) {
	t.Helper()
	session := auth.Principal{UserID: e.user.ID.String(), Scopes: []string{auth.ScopeRead, auth.ScopeWrite}, Via: auth.ViaSession}
	plain, key, err := e.auth.CreateAPIKey(t.Context(), session, "mcp", time.Now().Add(24*time.Hour), scopes)
	if err != nil {
		t.Fatal(err)
	}
	return plain, key
}

// connect opens an MCP session; a non-empty token is sent as a bearer on every request.
func (e *env) connect(t *testing.T, token string) *sdk.ClientSession {
	t.Helper()
	hc := &http.Client{Transport: bearer{token: token, next: http.DefaultTransport}}
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	s, err := c.Connect(t.Context(), &sdk.StreamableClientTransport{Endpoint: e.url, HTTPClient: hc}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	if b.token != "" {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return b.next.RoundTrip(r)
}

// call runs a tool and decodes its text content into out.
func call(t *testing.T, s *sdk.ClientSession, name string, args map[string]any, out proto.Message) {
	t.Helper()
	if args == nil {
		args = map[string]any{} // a nil map would be sent as JSON null
	}
	res, err := s.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("%s: content %T, want text", name, res.Content[0])
	}
	if res.IsError {
		t.Fatalf("%s: tool error %s", name, text.Text)
	}
	if err := protojson.Unmarshal([]byte(text.Text), out); err != nil {
		t.Fatalf("%s: decode %q: %v", name, text.Text, err)
	}
}

func assertRPCError(t *testing.T, err error, code int64) {
	t.Helper()
	var jerr *jsonrpc.Error
	if !errors.As(err, &jerr) || jerr.Code != code {
		t.Fatalf("err = %v, want JSON-RPC code %d", err, code)
	}
}

func TestInitializeAdvertisesInstructions(t *testing.T) {
	s := newEnv(t).connect(t, "")
	got := s.InitializeResult().Instructions
	for _, want := range []string{"https://plusultra.test/llms.txt", "program_validate", "program_simulate", "YYYY-MM-DD"} {
		if !strings.Contains(got, want) {
			t.Errorf("instructions lack %q:\n%s", want, got)
		}
	}
	if n := len(strings.Fields(got)); n > 120 {
		t.Errorf("instructions have %d words, want at most 120", n)
	}
}

func TestToolsListContainsExpected(t *testing.T) {
	s := newEnv(t).connect(t, "")
	var names []string
	for tool, err := range s.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	if len(names) != 62 {
		t.Errorf("%d tools, want 62: %v", len(names), names)
	}
	for _, want := range []string{"program_create", "reference_get_program_language", "auth_update_me"} {
		if !slices.Contains(names, want) {
			t.Errorf("tools lack %s", want)
		}
	}
	for _, n := range names {
		if strings.HasPrefix(n, "sync_") || n == "export_all" {
			t.Errorf("private tool %s is listed", n)
		}
	}
}

func TestReferenceToolWorksWithoutAuth(t *testing.T) {
	s := newEnv(t).connect(t, "")
	var out pb.GetProgramLanguageReferenceResponse
	call(t, s, "reference_get_program_language", nil, &out)
	if out.GetMarkdown() == "" {
		t.Fatal("empty program language reference")
	}
}

func TestAccountToolRequiresKey(t *testing.T) {
	e := newEnv(t)
	_, err := e.connect(t, "").CallTool(t.Context(), &sdk.CallToolParams{Name: "auth_get_me"})
	assertRPCError(t, err, codeUnauthenticated)

	key, _ := e.key(t, auth.ScopeRead, auth.ScopeWrite)
	var out pb.GetMeResponse
	call(t, e.connect(t, key), "auth_get_me", nil, &out)
	if out.GetUser().GetUsername() != "alice" {
		t.Fatalf("GetMe = %v", out.GetUser())
	}
}

func TestRevokedKeyRejectedOnMCP(t *testing.T) {
	e := newEnv(t)
	plain, key := e.key(t, auth.ScopeRead, auth.ScopeWrite)
	if _, err := e.auth.RevokeAPIKey(t.Context(), e.user.ID.String(), key.ID.String()); err != nil {
		t.Fatal(err)
	}
	_, err := e.connect(t, plain).CallTool(t.Context(), &sdk.CallToolParams{Name: "auth_get_me"})
	assertRPCError(t, err, codeUnauthenticated)
}

const workoutScript = `program("Strength", weeks=1)
day(1, 1, "Upper")
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(60))])
`

func TestCreateProgramViaMCP(t *testing.T) {
	e := newEnv(t)
	if _, err := catalog.CreateCustomExercise(t.Context(), e.auth.Q, e.user.ID, catalog.Custom{Name: "Bench Press"}); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"kind": "PROGRAM_KIND_WORKOUT", "script": workoutScript}

	ro, _ := e.key(t, auth.ScopeRead)
	_, err := e.connect(t, ro).CallTool(t.Context(), &sdk.CallToolParams{Name: "program_create", Arguments: args})
	assertRPCError(t, err, codePermissionDenied)

	rw, _ := e.key(t, auth.ScopeRead, auth.ScopeWrite)
	s := e.connect(t, rw)
	var created pb.CreateProgramResponse
	call(t, s, "program_create", args, &created)
	var got pb.GetProgramResponse
	call(t, s, "program_get", map[string]any{"id": created.GetProgram().GetId()}, &got)
	if p := got.GetProgram(); p.GetName() != "Strength" || p.GetScript() != workoutScript || p.GetStatus() != pb.ProgramStatus_PROGRAM_STATUS_DRAFT {
		t.Fatalf("program = %v", p)
	}
}

// A read-scope key cannot call write tools.
func TestReadScopeCannotWrite(t *testing.T) {
	e := newEnv(t)
	key, _ := e.key(t, auth.ScopeRead)
	_, err := e.connect(t, key).CallTool(t.Context(), &sdk.CallToolParams{
		Name: "auth_update_me", Arguments: map[string]any{"timezone": "Asia/Kolkata"},
	})
	assertRPCError(t, err, codePermissionDenied)
}

func TestWriteToolViaMCP(t *testing.T) {
	e := newEnv(t)
	key, _ := e.key(t, auth.ScopeRead, auth.ScopeWrite)
	s := e.connect(t, key)

	var updated pb.UpdateMeResponse
	call(t, s, "auth_update_me", map[string]any{"timezone": "Asia/Kolkata"}, &updated)
	var me pb.GetMeResponse
	call(t, s, "auth_get_me", nil, &me)
	if updated.GetUser().GetTimezone() != "Asia/Kolkata" || me.GetUser().GetTimezone() != "Asia/Kolkata" {
		t.Fatalf("timezone after update = %q, then %q", updated.GetUser().GetTimezone(), me.GetUser().GetTimezone())
	}

	// Validation failures reach the model as tool errors, not protocol errors.
	res, err := s.CallTool(t.Context(), &sdk.CallToolParams{
		Name: "auth_update_me", Arguments: map[string]any{"timezone": "Mars/Olympus_Mons"},
	})
	if err != nil || !res.IsError {
		t.Fatalf("invalid timezone: err %v, result %+v", err, res)
	}
}

// protomcp validates tool output against the schema it derives from the
// validate rules, so data the server legitimately holds must fit them.
func TestOutputValidationAcceptsCatalogData(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	step := strings.Repeat("x", 1500)
	if _, err := e.auth.Q.UpsertExercise(ctx, sqlc.UpsertExerciseParams{
		Slug: "squat-barbell", Name: "Squat, Barbell", Equipment: "barbell", Category: "strength",
		PrimaryMuscles: []string{"quads"}, Instructions: []string{step}, Source: "curated",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.CreateCustomFood(ctx, e.auth.Q, e.user.ID, catalog.CustomFood{Name: "Oats"}); err != nil {
		t.Fatal(err)
	}
	key, _ := e.key(t, auth.ScopeRead, auth.ScopeWrite)
	s := e.connect(t, key)

	var ex pb.GetExerciseResponse
	call(t, s, "exercise_get", map[string]any{"slug": "squat-barbell"}, &ex)
	if got := ex.GetExercise().GetInstructions(); len(got) != 1 || got[0] != step {
		t.Fatalf("instructions = %v", got)
	}

	var created pb.CreateCheckinResponse
	call(t, s, "checkin_create", map[string]any{"checkin": map[string]any{
		"id": "6f1d2c3e-0000-4000-8000-000000000001", "date": "2026-11-11", "weight": map[string]any{"value": 80, "unit": "UNIT_KG"},
	}}, &created)
	var day pb.GetDayResponse
	call(t, s, "calendar_get_day", map[string]any{"date": "2026-11-11"}, &day)
	if day.GetDay().GetCheckin().GetPhotoContentType() != "" || day.GetDay().GetCheckin().GetWeight().GetValue() != 80 {
		t.Fatalf("checkin = %v", day.GetDay().GetCheckin())
	}

	var foods pb.SearchFoodsResponse
	call(t, s, "food_search", map[string]any{"query": "oats"}, &foods)
	if len(foods.GetFoods()) != 1 || foods.GetFoods()[0].GetBarcode() != "" {
		t.Fatalf("foods = %v", foods.GetFoods())
	}
}
