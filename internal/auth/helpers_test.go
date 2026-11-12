package auth

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const testPassword = "correct horse battery"

const (
	readProc   = "/test.v1.TestService/Read"
	writeProc  = "/test.v1.TestService/Write"
	adminProc  = "/test.v1.TestService/Admin"
	healthProc = "/test.v1.TestService/Health"
	publicProc = "/test.v1.PublicService/Ping"
)

func newService(t *testing.T, registration string) *Service {
	t.Helper()
	return NewService(dbtest.New(t), Config{Registration: registration})
}

func mustRegister(t *testing.T, s *Service, username string) sqlc.User {
	t.Helper()
	u, err := s.Register(t.Context(), NewUser{Username: username, Password: testPassword})
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	return u
}

func mustLogin(t *testing.T, s *Service, username string) string {
	t.Helper()
	sess, err := s.Login(t.Context(), username, testPassword)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return sess.Token
}

// mustSession logs in and returns the token and the principal it resolves to.
func mustSession(t *testing.T, s *Service, username string) (string, Principal) {
	t.Helper()
	tok := mustLogin(t, s, username)
	p, err := s.principal(t.Context(), bearer(tok))
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	return tok, p
}

func mustKey(t *testing.T, s *Service, p Principal, scopes ...string) (string, sqlc.ApiKey) {
	t.Helper()
	plain, key, err := s.CreateAPIKey(t.Context(), p, "test", time.Now().Add(time.Hour), scopes)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return plain, key
}

func assertCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != want {
		t.Fatalf("err = %v (code %v), want code %v", err, got, want)
	}
}

// assertUnauthenticated also checks the message, which must not reveal why.
func assertUnauthenticated(t *testing.T, err error) {
	t.Helper()
	assertCode(t, err, connect.CodeUnauthenticated)
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Message() != msgUnauthenticated {
		t.Fatalf("message = %q, want %q", ce.Message(), msgUnauthenticated)
	}
}

func assertInternal(t *testing.T, err error) {
	t.Helper()
	assertCode(t, err, connect.CodeInternal)
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Message() != "internal error" {
		t.Fatalf("message = %q, want %q", ce.Message(), "internal error")
	}
}

type testServer struct{ url string }

// newTestServer serves dummy procedures behind the interceptor. Each returns the
// principal it saw as JSON; empty when FromContext reports none.
func newTestServer(t *testing.T) (*Service, *testServer) {
	t.Helper()
	s := newService(t, RegistrationOpen)
	opt := connect.WithInterceptors(NewInterceptor(s, "/test.v1.PublicService/", healthProc))
	checks := map[string]func(context.Context) error{
		readProc: nil, writeProc: RequireWrite, adminProc: RequireAdmin, healthProc: nil, publicProc: nil,
	}
	mux := http.NewServeMux()
	for proc, check := range checks {
		mux.Handle(proc, connect.NewUnaryHandler(proc, func(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[wrapperspb.StringValue], error) {
			if check != nil {
				if err := check(ctx); err != nil {
					return nil, err
				}
			}
			p, ok := FromContext(ctx)
			if !ok {
				return connect.NewResponse(wrapperspb.String("")), nil
			}
			b, err := json.Marshal(p)
			if err != nil {
				return nil, err
			}
			return connect.NewResponse(wrapperspb.String(string(b))), nil
		}, opt))
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return s, &testServer{url: srv.URL}
}

// call returns the principal the handler saw, or nil when it saw none.
func (ts *testServer) call(t *testing.T, proc string, h http.Header) (*Principal, error) {
	t.Helper()
	c := connect.NewClient[emptypb.Empty, wrapperspb.StringValue](http.DefaultClient, ts.url+proc)
	req := connect.NewRequest(&emptypb.Empty{})
	maps.Copy(req.Header(), h)
	res, err := c.CallUnary(t.Context(), req)
	if err != nil {
		return nil, err
	}
	if res.Msg.GetValue() == "" {
		return nil, nil
	}
	var p Principal
	if err := json.Unmarshal([]byte(res.Msg.GetValue()), &p); err != nil {
		t.Fatal(err)
	}
	return &p, nil
}

func bearer(token string) http.Header {
	return http.Header{"Authorization": {"Bearer " + token}}
}
