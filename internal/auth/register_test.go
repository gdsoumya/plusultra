package auth

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestFirstUserIsAdmin(t *testing.T) {
	s := newService(t, RegistrationOpen)
	first := mustRegister(t, s, "alice")
	if !first.IsAdmin {
		t.Fatal("first user is not admin")
	}
	if !VerifyPassword(t.Context(), first.PasswordHash, testPassword) {
		t.Fatal("stored password hash does not verify")
	}
	if second := mustRegister(t, s, "bob"); second.IsAdmin {
		t.Fatal("second user is admin")
	}
	_, err := s.Register(t.Context(), NewUser{Username: "alice", Password: testPassword})
	assertCode(t, err, connect.CodeAlreadyExists)
}

func TestInviteRequiredForSecondUser(t *testing.T) {
	s := newService(t, "") // invite is the default
	ctx := t.Context()
	mustRegister(t, s, "alice")
	_, admin := mustSession(t, s, "alice")

	for _, code := range []string{"", "nope"} {
		_, err := s.Register(ctx, NewUser{Username: "bob", Password: testPassword, InviteCode: code})
		assertCode(t, err, connect.CodePermissionDenied)
	}
	// Without a valid invite, a taken username is not revealed.
	_, err := s.Register(ctx, NewUser{Username: "alice", Password: testPassword, InviteCode: "nope"})
	assertCode(t, err, connect.CodePermissionDenied)

	inv, err := s.CreateInvite(ctx, admin, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	code := inv.Code
	invites, err := s.Q.ListInvites(ctx)
	if err != nil || len(invites) != 1 {
		t.Fatalf("invites = %v, err %v", invites, err)
	}
	if d := time.Until(invites[0].Invite.ExpiresAt) - 7*24*time.Hour; d > time.Minute || d < -time.Minute {
		t.Fatalf("invite expires at %v, want 7 days from now", invites[0].Invite.ExpiresAt)
	}
	_, err = s.Register(ctx, NewUser{Username: "alice", Password: testPassword, InviteCode: code})
	assertCode(t, err, connect.CodeAlreadyExists)

	// None of the failures above consumed the invite or left a user behind.
	bob, err := s.Register(ctx, NewUser{Username: "bob", Password: testPassword, InviteCode: code})
	if err != nil {
		t.Fatal(err)
	}
	if bob.IsAdmin {
		t.Fatal("invited user is admin")
	}
	_, err = s.Register(ctx, NewUser{Username: "carol", Password: testPassword, InviteCode: code})
	assertCode(t, err, connect.CodePermissionDenied)

	_, bobP := mustSession(t, s, "bob")
	_, err = s.CreateInvite(ctx, bobP, time.Time{})
	assertCode(t, err, connect.CodePermissionDenied)

	expired, err := s.CreateInvite(ctx, admin, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE invites SET expires_at = now() - interval '1 second' WHERE code = $1`, expired.Code); err != nil {
		t.Fatal(err)
	}
	_, err = s.Register(ctx, NewUser{Username: "carol", Password: testPassword, InviteCode: expired.Code})
	assertCode(t, err, connect.CodePermissionDenied)
}

func TestRegisterValidatesInput(t *testing.T) {
	s := newService(t, RegistrationOpen)
	for _, c := range []struct{ username, password string }{
		{"ab", testPassword},
		{strings.Repeat("a", 33), testPassword},
		{"Alice", testPassword},
		{"al ice", testPassword},
		{"alice", "short"},
		{"alice", "123456789"},
		{"alice", strings.Repeat("a", 1025)},
	} {
		_, err := s.Register(t.Context(), NewUser{Username: c.username, Password: c.password})
		assertCode(t, err, connect.CodeInvalidArgument)
	}
	if _, err := s.Register(t.Context(), NewUser{Username: "a_1", Password: "1234567890"}); err != nil {
		t.Fatalf("minimal valid input rejected: %v", err)
	}
}
