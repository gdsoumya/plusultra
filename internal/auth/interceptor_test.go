package auth

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestInterceptorSetsPrincipal(t *testing.T) {
	s, ts := newTestServer(t)
	alice := mustRegister(t, s, "alice")
	tok, aliceP := mustSession(t, s, "alice")
	key, _ := mustKey(t, s, aliceP, ScopeRead)

	for name, h := range map[string]http.Header{
		"none":              nil,
		"garbage":           bearer("nope"),
		"unknown key":       bearer("lsk_00000000_00000000000000000000000000000000"),
		"malformed key":     bearer(key + "0"),
		"uppercase session": bearer(strings.ToUpper(tok)),
		"basic scheme":      {"Authorization": {"Basic " + tok}},
		"key in cookie":     {"Cookie": {SessionCookie + "=" + key}},
	} {
		_, err := ts.call(t, readProc, h)
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("%s: err = %v, want unauthenticated", name, err)
		}
		assertUnauthenticated(t, err)
	}

	for _, proc := range []string{publicProc, healthProc} {
		for _, h := range []http.Header{nil, bearer("nope")} {
			if p, err := ts.call(t, proc, h); err != nil || p != nil {
				t.Fatalf("%s with %v: principal %v, err %v", proc, h, p, err)
			}
		}
	}

	for name, h := range map[string]http.Header{
		"bearer":           bearer(tok),
		"lowercase scheme": {"Authorization": {"bearer " + tok}},
		"cookie":           {"Cookie": {SessionCookie + "=" + tok}},
	} {
		p, err := ts.call(t, readProc, h)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := Principal{UserID: alice.ID.String(), Admin: true, Scopes: []string{ScopeRead, ScopeWrite, ScopeAdmin}, Via: ViaSession}
		if p == nil || p.UserID != want.UserID || !p.Admin || p.Via != want.Via || !slices.Equal(p.Scopes, want.Scopes) {
			t.Fatalf("%s: principal %+v, want %+v", name, p, want)
		}
	}

	p, err := ts.call(t, readProc, bearer(key))
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.UserID != alice.ID.String() || p.Admin || p.Via != ViaAPIKey || !slices.Equal(p.Scopes, []string{ScopeRead}) {
		t.Fatalf("api key principal %+v, want alice's non-admin read key", p)
	}
	keys, err := s.ListAPIKeys(t.Context(), alice.ID.String())
	if err != nil || keys[0].LastUsedAt == nil {
		t.Fatalf("last_used_at not recorded: %v, err %v", keys, err)
	}
}

func TestParseCredential(t *testing.T) {
	for _, c := range []struct {
		h          http.Header
		token      string
		fromCookie bool
	}{
		{nil, "", false},
		{bearer("abc"), "abc", false},
		{http.Header{"Authorization": {"BEARER  abc "}}, "abc", false},
		{http.Header{"Cookie": {"a=b; session=abc"}}, "abc", true},
		{http.Header{"Authorization": {"Bearer abc"}, "Cookie": {"session=def"}}, "abc", false},
		{http.Header{"Authorization": {"Basic abc"}}, "", false},
	} {
		token, fromCookie := ParseCredential(c.h)
		if token != c.token || fromCookie != c.fromCookie {
			t.Fatalf("ParseCredential(%v) = %q, %v; want %q, %v", c.h, token, fromCookie, c.token, c.fromCookie)
		}
	}
}

func TestReadScopeCannotWrite(t *testing.T) {
	s, ts := newTestServer(t)
	mustRegister(t, s, "alice")
	tok, alice := mustSession(t, s, "alice")
	readKey, _ := mustKey(t, s, alice, ScopeRead)
	rwKey, _ := mustKey(t, s, alice)

	_, err := ts.call(t, writeProc, bearer(readKey))
	assertCode(t, err, connect.CodePermissionDenied)
	for _, tok := range []string{rwKey, tok} {
		if _, err := ts.call(t, writeProc, bearer(tok)); err != nil {
			t.Fatalf("write denied: %v", err)
		}
	}
}

func TestRequireAdmin(t *testing.T) {
	s, ts := newTestServer(t)
	ctx := t.Context()
	mustRegister(t, s, "alice")
	mustRegister(t, s, "bob")
	aliceTok, alice := mustSession(t, s, "alice")
	bobTok, _ := mustSession(t, s, "bob")
	readKey, _ := mustKey(t, s, alice, ScopeRead, ScopeWrite)
	adminKey, _ := mustKey(t, s, alice, ScopeRead, ScopeAdmin)

	codeOf := func(err error) connect.Code { // 0 means success
		if err == nil {
			return 0
		}
		return connect.CodeOf(err)
	}
	check := func(tok string, want connect.Code) {
		t.Helper()
		_, err := ts.call(t, adminProc, bearer(tok))
		if codeOf(err) != want {
			t.Fatalf("RequireAdmin: err = %v, want code %v", err, want)
		}
		p, err := s.principal(ctx, bearer(tok))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateInvite(ctx, p, time.Time{}); codeOf(err) != want {
			t.Fatalf("CreateInvite: err = %v, want code %v", err, want)
		}
	}
	check(bobTok, connect.CodePermissionDenied)
	check(readKey, connect.CodePermissionDenied) // an admin's key without the admin scope
	check(aliceTok, 0)
	check(adminKey, 0)

	// Demotion takes effect on the next request, even for an admin-scoped key.
	if _, err := s.pool.Exec(ctx, `UPDATE users SET is_admin = false WHERE id = $1`, alice.UserID); err != nil {
		t.Fatal(err)
	}
	check(adminKey, connect.CodePermissionDenied)
}
