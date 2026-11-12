package auth

import (
	"bytes"
	"crypto/sha256"
	"regexp"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
)

var keyFormat = regexp.MustCompile(`^lsk_[0-9a-f]{8}_[0-9a-f]{32}$`)

func TestAPIKeyFormatAndHash(t *testing.T) {
	s := newService(t, RegistrationOpen)
	ctx := t.Context()
	mustRegister(t, s, "alice")
	_, p := mustSession(t, s, "alice")
	plain, key := mustKey(t, s, p)

	if !keyFormat.MatchString(plain) {
		t.Fatalf("key %q does not match %v", plain, keyFormat)
	}
	if key.Prefix != plain[4:12] {
		t.Fatalf("prefix = %q, want %q", key.Prefix, plain[4:12])
	}
	sum := sha256.Sum256([]byte(plain))
	if !bytes.Equal(key.KeyHash, sum[:]) {
		t.Fatal("key_hash is not the SHA-256 of the key")
	}
	if !slices.Equal(key.Scopes, []string{ScopeRead, ScopeWrite}) {
		t.Fatalf("default scopes = %v", key.Scopes)
	}
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM api_keys k WHERE strpos(k::text, $1) > 0`, plain[13:]).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows containing the secret = %d, err %v; want 0", n, err)
	}
	keys, err := s.ListAPIKeys(ctx, p.UserID)
	if err != nil || len(keys) != 1 || keys[0].ID != key.ID {
		t.Fatalf("list = %v, err %v", keys, err)
	}
}

func TestAPIKeyExpiryRequiredMax365(t *testing.T) {
	s := newService(t, RegistrationOpen)
	mustRegister(t, s, "alice")
	_, p := mustSession(t, s, "alice")
	now := time.Now()
	for name, exp := range map[string]time.Time{
		"missing":  {},
		"past":     now.Add(-time.Minute),
		"366 days": now.AddDate(0, 0, 366),
	} {
		_, _, err := s.CreateAPIKey(t.Context(), p, "k", exp, nil)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s: err = %v, want invalid argument", name, err)
		}
	}
	_, key, err := s.CreateAPIKey(t.Context(), p, "k", now.Add(365*24*time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	if key.ExpiresAt == nil {
		t.Fatal("expires_at not stored")
	}
}

func TestAPIKeyScopes(t *testing.T) {
	s := newService(t, RegistrationOpen)
	ctx := t.Context()
	mustRegister(t, s, "alice")
	mustRegister(t, s, "bob")
	_, alice := mustSession(t, s, "alice")
	_, bob := mustSession(t, s, "bob")
	exp := time.Now().Add(time.Hour)

	for _, scopes := range [][]string{{ScopeRead, "delete"}, {ScopeRead, ScopeRead}} {
		_, _, err := s.CreateAPIKey(ctx, alice, "k", exp, scopes)
		assertCode(t, err, connect.CodeInvalidArgument)
	}
	_, _, err := s.CreateAPIKey(ctx, bob, "k", exp, []string{ScopeAdmin})
	assertCode(t, err, connect.CodePermissionDenied)

	plain, key, err := s.CreateAPIKey(ctx, alice, "k", exp, []string{ScopeRead, ScopeAdmin})
	if err != nil || !slices.Equal(key.Scopes, []string{ScopeRead, ScopeAdmin}) {
		t.Fatalf("scopes = %v, err %v", key.Scopes, err)
	}

	// A key cannot mint keys, even an admin's key with every scope.
	keyP, err := s.principal(ctx, bearer(plain))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.CreateAPIKey(ctx, keyP, "k", exp, []string{ScopeRead})
	assertCode(t, err, connect.CodePermissionDenied)
}

func TestRevokedKeyRejected(t *testing.T) {
	s, ts := newTestServer(t)
	ctx := t.Context()
	mustRegister(t, s, "alice")
	bob := mustRegister(t, s, "bob")
	_, alice := mustSession(t, s, "alice")
	plain, key := mustKey(t, s, alice)
	if _, err := ts.call(t, readProc, bearer(plain)); err != nil {
		t.Fatal(err)
	}

	_, err := s.RevokeAPIKey(ctx, bob.ID.String(), key.ID.String())
	assertCode(t, err, connect.CodeNotFound)
	if _, err := s.RevokeAPIKey(ctx, alice.UserID, key.ID.String()); err != nil {
		t.Fatal(err)
	}
	_, err = ts.call(t, readProc, bearer(plain))
	assertUnauthenticated(t, err)

	keys, err := s.ListAPIKeys(ctx, alice.UserID)
	if err != nil || len(keys) != 1 || keys[0].RevokedAt == nil {
		t.Fatalf("keys = %v, err %v; want revoked_at set", keys, err)
	}
}

func TestExpiredKeyRejected(t *testing.T) {
	s, ts := newTestServer(t)
	mustRegister(t, s, "alice")
	_, alice := mustSession(t, s, "alice")
	plain, key := mustKey(t, s, alice)
	if _, err := ts.call(t, readProc, bearer(plain)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(t.Context(), `UPDATE api_keys SET expires_at = now() - interval '1 second' WHERE id = $1`, key.ID); err != nil {
		t.Fatal(err)
	}
	_, err := ts.call(t, readProc, bearer(plain))
	assertUnauthenticated(t, err)
}

func TestAPIKeyLastUsedThrottled(t *testing.T) {
	s, ts := newTestServer(t)
	ctx := t.Context()
	mustRegister(t, s, "alice")
	_, alice := mustSession(t, s, "alice")
	plain, key := mustKey(t, s, alice)
	lastUsed := func() time.Time {
		t.Helper()
		var at *time.Time
		if err := s.pool.QueryRow(ctx, `SELECT last_used_at FROM api_keys WHERE id = $1`, key.ID).Scan(&at); err != nil || at == nil {
			t.Fatalf("last_used_at = %v, err %v", at, err)
		}
		return *at
	}
	use := func() {
		t.Helper()
		if _, err := ts.call(t, readProc, bearer(plain)); err != nil {
			t.Fatal(err)
		}
	}

	use()
	first := lastUsed()
	use()
	if !lastUsed().Equal(first) {
		t.Fatal("last_used_at rewritten within a minute")
	}
	if _, err := s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() - interval '2 minutes' WHERE id = $1`, key.ID); err != nil {
		t.Fatal(err)
	}
	stale := lastUsed()
	use()
	if !lastUsed().After(stale) {
		t.Fatal("stale last_used_at not refreshed")
	}
}
