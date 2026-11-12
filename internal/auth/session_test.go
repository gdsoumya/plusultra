package auth

import (
	"bytes"
	"crypto/sha256"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLoginBadPassword(t *testing.T) {
	s := newService(t, RegistrationOpen)
	ctx := t.Context()
	mustRegister(t, s, "alice")

	_, err := s.Login(ctx, "alice", "wrong password")
	assertUnauthenticated(t, err)
	_, err = s.Login(ctx, "nobody", testPassword)
	assertUnauthenticated(t, err)
	_, err = s.Login(ctx, "alice", strings.Repeat("a", 1025))
	assertUnauthenticated(t, err)

	tok := mustLogin(t, s, "alice")
	if len(tok) != 64 {
		t.Fatalf("token %q: want 32 random bytes as hex", tok)
	}
	sum := sha256.Sum256([]byte(tok))
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE token_hash = $1`, sum[:]).Scan(&n); err != nil || n != 1 {
		t.Fatalf("sessions with token hash = %d, err %v; want 1", n, err)
	}
}

func TestSessionExpires(t *testing.T) {
	s, ts := newTestServer(t)
	mustRegister(t, s, "alice")
	tok := mustLogin(t, s, "alice")
	if p, err := ts.call(t, readProc, bearer(tok)); err != nil || p == nil {
		t.Fatalf("fresh session: principal %v, err %v", p, err)
	}

	if _, err := s.pool.Exec(t.Context(), `UPDATE sessions SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	p, err := ts.call(t, readProc, bearer(tok))
	assertUnauthenticated(t, err)
	if p != nil {
		t.Fatalf("expired session produced principal %v", p)
	}
}

func TestSessionSlidingExpiry(t *testing.T) {
	s, ts := newTestServer(t)
	mustRegister(t, s, "alice")
	tok := mustLogin(t, s, "alice")
	ctx := t.Context()
	if _, err := s.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() - interval '2 minutes', expires_at = now() + interval '1 day'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.call(t, readProc, bearer(tok)); err != nil {
		t.Fatal(err)
	}
	var exp time.Time
	if err := s.pool.QueryRow(ctx, `SELECT expires_at FROM sessions`).Scan(&exp); err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 89*24*time.Hour {
		t.Fatalf("expires_at %v not extended to 90 days", exp)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	s, ts := newTestServer(t)
	mustRegister(t, s, "alice")
	tok := mustLogin(t, s, "alice")
	for range 2 { // logout is idempotent
		if err := s.Logout(t.Context(), tok); err != nil {
			t.Fatal(err)
		}
	}
	_, err := ts.call(t, readProc, bearer(tok))
	assertUnauthenticated(t, err)
}

func TestInternalErrorsHidden(t *testing.T) {
	s := newService(t, RegistrationOpen)
	ctx := t.Context()
	mustRegister(t, s, "alice")
	tok, p := mustSession(t, s, "alice")
	var logs bytes.Buffer
	s.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	s.pool.Close()

	_, err := s.Register(ctx, NewUser{Username: "bob", Password: testPassword})
	assertInternal(t, err)
	_, err = s.Login(ctx, "alice", testPassword)
	assertInternal(t, err)
	assertInternal(t, s.Logout(ctx, tok))
	_, err = s.principal(ctx, bearer(tok))
	assertInternal(t, err)
	_, _, err = s.CreateAPIKey(ctx, p, "k", time.Now().Add(time.Hour), nil)
	assertInternal(t, err)
	_, err = s.ListAPIKeys(ctx, p.UserID)
	assertInternal(t, err)
	_, err = s.RevokeAPIKey(ctx, p.UserID, uuid.NewString())
	assertInternal(t, err)
	_, err = s.CreateInvite(ctx, p, time.Time{})
	assertInternal(t, err)

	if !strings.Contains(logs.String(), "closed pool") {
		t.Fatalf("original error not logged; logs:\n%s", logs.String())
	}
}
