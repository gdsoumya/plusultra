//go:build e2e

// Package e2e drives the compose stack end to end. Run it with make e2e.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Ports that do not collide with make db-up or a running server.
const (
	serverPort = "18080"
	pgPort     = "15433"
	base       = "http://localhost:" + serverPort
)

var inviteCode = regexp.MustCompile(`^[A-Z0-9]{26}$`)

type stack struct {
	t    *testing.T
	root string
}

// compose runs docker compose for the e2e project.
func (s *stack) compose(args ...string) string {
	s.t.Helper()
	cmd := exec.Command("docker", append([]string{"compose", "-p", "plusultra-e2e"}, args...)...) //nolint:gosec // test-controlled args
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "LISTEN_PORT="+serverPort, "PG_PORT="+pgPort)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr // compose prints progress on stderr
	if err := cmd.Run(); err != nil {
		s.t.Fatalf("docker compose %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String()
}

// run starts the server image with a different command and returns its stdout.
func (s *stack) run(args ...string) string {
	s.t.Helper()
	return s.compose(append([]string{"run", "--rm", "--no-deps", "-T"}, args...)...)
}

func (s *stack) connectJSON(token, procedure string, in, out any) {
	s.t.Helper()
	body, err := json.Marshal(in)
	if err != nil {
		s.t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(s.t.Context(), http.MethodPost, base+"/api/v1/plusultra.v1."+procedure, bytes.NewReader(body))
	if err != nil {
		s.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatalf("%s: %v", procedure, err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		s.t.Fatalf("%s: %d %s", procedure, res.StatusCode, b)
	}
	if err := json.Unmarshal(b, out); err != nil {
		s.t.Fatalf("%s: decode %s: %v", procedure, b, err)
	}
}

func get(t *testing.T, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

type bearer struct{ key string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.key)
	return http.DefaultTransport.RoundTrip(r)
}

// tool calls an MCP tool and decodes its text result into out.
func tool(t *testing.T, s *sdk.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := s.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	text, ok := res.Content[0].(*sdk.TextContent)
	if !ok || res.IsError {
		t.Fatalf("%s: error %v, content %+v", name, res.IsError, res.Content[0])
	}
	if err := json.Unmarshal([]byte(text.Text), out); err != nil {
		t.Fatalf("%s: decode %s: %v", name, text.Text, err)
	}
}

func nextMonday() time.Time {
	d := time.Now().UTC().Truncate(24 * time.Hour)
	for d.Weekday() != time.Monday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

type named struct {
	Name string `json:"name"`
}

func TestSmoke(t *testing.T) {
	start := time.Now()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	s := &stack{t: t, root: root}
	t.Cleanup(func() {
		defer s.compose("down", "-v") // also when the logs call below fails the test
		t.Logf("wall time %s", time.Since(start).Round(time.Second))
		if t.Failed() {
			t.Log(s.compose("logs", "server"))
		}
	})
	s.compose("up", "-d", "--build")
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if code, body := get(t, "/healthz"); code == http.StatusOK && strings.HasPrefix(body, "ok") {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("/healthz: %d %s", code, body)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// The CLI shares the image: migrate is a no-op after serve.
	if out := s.run("server", "migrate"); !strings.Contains(out, "migrations applied") {
		t.Fatalf("migrate: %s", out)
	}

	var reg struct {
		Token string `json:"token"`
		User  struct {
			IsAdmin bool `json:"isAdmin"`
		} `json:"user"`
	}
	s.connectJSON("", "AuthService/Register", map[string]any{"username": "alice", "password": "correct horse battery"}, &reg)
	if reg.Token == "" || !reg.User.IsAdmin {
		t.Fatalf("register = %+v", reg)
	}

	// The image ships the catalog and serve seeded it before listening.
	var found struct {
		Exercises []named `json:"exercises"`
		Foods     []named `json:"foods"`
	}
	s.connectJSON(reg.Token, "ExerciseService/SearchExercises", map[string]any{"query": "squat"}, &found)
	if !slices.ContainsFunc(found.Exercises, func(n named) bool { return n.Name == "Squat" }) {
		t.Fatalf("SearchExercises(squat) = %+v", found.Exercises)
	}
	s.connectJSON(reg.Token, "FoodService/SearchFoods", map[string]any{"query": "paneer"}, &found)
	if len(found.Foods) == 0 || !strings.Contains(strings.ToLower(found.Foods[0].Name), "paneer") {
		t.Fatalf("SearchFoods(paneer) = %+v", found.Foods)
	}

	code := strings.TrimSpace(s.run("server", "invite"))
	if !inviteCode.MatchString(code) {
		t.Fatalf("invite printed %q", code)
	}
	var bob struct {
		Token string `json:"token"`
	}
	s.connectJSON("", "AuthService/Register", map[string]any{"username": "bob", "password": "correct horse battery", "inviteCode": code}, &bob)
	if bob.Token == "" {
		t.Fatal("register with invite returned no token")
	}

	var key struct {
		Key string `json:"key"`
	}
	s.connectJSON(reg.Token, "ApiKeyService/CreateApiKey", map[string]any{
		"name": "e2e", "scopes": []string{"API_KEY_SCOPE_READ", "API_KEY_SCOPE_WRITE"},
		"expiresAt": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	}, &key)
	if !strings.HasPrefix(key.Key, "lsk_") {
		t.Fatalf("api key = %q", key.Key)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "e2e", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint: base + "/mcp", HTTPClient: &http.Client{Transport: bearer{key.Key}},
	}, nil)
	if err != nil {
		t.Fatalf("mcp initialize: %v", err)
	}
	defer func() { _ = session.Close() }()
	if !strings.Contains(session.InitializeResult().Instructions, "/llms.txt") {
		t.Fatalf("instructions = %q", session.InitializeResult().Instructions)
	}
	tools := map[string]bool{}
	for tl, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		tools[tl.Name] = true
	}
	if !tools["program_create"] || !tools["program_activate"] || !tools["calendar_get_day"] {
		t.Fatalf("tools/list lacks the program tools: %v", tools)
	}

	script, err := os.ReadFile(filepath.Join(root, "internal/engine/testdata/examples/strength16.star")) //nolint:gosec // repo testdata
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Program struct {
			ID string `json:"id"`
		} `json:"program"`
	}
	tool(t, session, "program_create", map[string]any{"kind": "PROGRAM_KIND_WORKOUT", "script": string(script)}, &created)
	monday := nextMonday()
	var activated struct {
		Program struct {
			Status string `json:"status"`
		} `json:"program"`
	}
	tool(t, session, "program_activate", map[string]any{"id": created.Program.ID, "startDate": monday.Format(time.DateOnly)}, &activated)
	if activated.Program.Status != "PROGRAM_STATUS_ACTIVE" {
		t.Fatalf("activate = %+v", activated)
	}

	// Day 1 is the Monday conditioning day; bench is day 2 of week 1: 4x5 at 75 kg.
	var day struct {
		Day struct {
			Plan struct {
				Exercises []struct {
					Name string `json:"name"`
					Sets []struct {
						Reps   int    `json:"reps"`
						Label  string `json:"label"`
						Warmup bool   `json:"warmup"`
						Weight struct {
							Value float64 `json:"value"`
							Unit  string  `json:"unit"`
						} `json:"weight"`
					} `json:"sets"`
				} `json:"exercises"`
			} `json:"plan"`
		} `json:"day"`
	}
	s.connectJSON(key.Key, "CalendarService/GetDay", map[string]any{"date": monday.AddDate(0, 0, 1).Format(time.DateOnly)}, &day)
	var work []string
	for _, ex := range day.Day.Plan.Exercises {
		if ex.Name != "Bench Press" {
			continue
		}
		for _, st := range ex.Sets {
			if !st.Warmup && st.Label == "Work" {
				work = append(work, fmt.Sprintf("%dx%g%s", st.Reps, st.Weight.Value, st.Weight.Unit))
			}
		}
	}
	if len(work) != 4 || strings.Join(work, ",") != strings.Repeat("5x75UNIT_KG,", 3)+"5x75UNIT_KG" {
		t.Fatalf("bench work sets = %v, want 4 sets of 5 at 75 kg", work)
	}

	if code, body := get(t, "/llms.txt"); code != http.StatusOK || !strings.Contains(body, "/llms/") {
		t.Fatalf("/llms.txt = %d %q", code, body)
	}
	// The image embeds the web export: the real index, also for client-side routes.
	for _, p := range []string{"/", "/settings"} {
		if code, body := get(t, p); code != http.StatusOK || !strings.Contains(body, "_expo/static/js/web/entry-") {
			t.Fatalf("%s = %d %q", p, code, body)
		}
	}
	if code, _ := get(t, "/api/v1/plusultra.v1.AuthService/Login"); code != http.StatusMethodNotAllowed {
		t.Fatalf("GET on an RPC = %d, want 405, not the web app", code)
	}
}
