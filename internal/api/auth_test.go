package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
)

func TestRegisterLoginMe(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	reg := e.register(t, "alice", "")
	u := reg.GetUser()
	if reg.GetToken() == "" || u.GetUsername() != "alice" || !u.GetIsAdmin() || u.GetDisplayName() != "alice" ||
		u.GetUnits() != pb.Unit_UNIT_KG || u.GetTimezone() != "UTC" || u.GetMeasurementUnit() != pb.LengthUnit_LENGTH_UNIT_CM {
		t.Fatalf("register = %v", reg)
	}
	assertNear(t, "expires_at", reg.GetExpiresAt(), time.Now().Add(ninetyDay))

	me, err := e.me(t, reg.GetToken())
	if err != nil || me.GetUsername() != "alice" || me.GetId() != u.GetId() {
		t.Fatalf("GetMe = %v, err %v", me, err)
	}
	_, err = e.auth.GetMe(ctx, connect.NewRequest(&pb.GetMeRequest{}))
	assertCode(t, err, connect.CodeUnauthenticated)

	_, err = e.auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "alice", Password: "wrong password"}))
	assertCode(t, err, connect.CodeUnauthenticated)
	login, err := e.auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "alice", Password: password}))
	if err != nil || login.Msg.GetToken() == reg.GetToken() || login.Msg.GetUser().GetId() != u.GetId() {
		t.Fatalf("login = %v, err %v", login, err)
	}
	assertNear(t, "login expires_at", login.Msg.GetExpiresAt(), time.Now().Add(ninetyDay))

	if _, err := e.auth.Logout(ctx, as(login.Msg.GetToken(), &pb.LogoutRequest{})); err != nil {
		t.Fatal(err)
	}
	_, err = e.me(t, login.Msg.GetToken())
	assertCode(t, err, connect.CodeUnauthenticated)
	if _, err := e.me(t, reg.GetToken()); err != nil {
		t.Fatalf("other session ended by logout: %v", err)
	}

	logs := e.logs.String()
	for _, want := range []string{"procedure=/plusultra.v1.AuthService/GetMe", "user_id=" + u.GetId(), "code=unauthenticated", "duration="} {
		if !strings.Contains(logs, want) {
			t.Errorf("request log lacks %q:\n%s", want, logs)
		}
	}
	for _, secret := range []string{reg.GetToken(), login.Msg.GetToken(), password} {
		if strings.Contains(logs, secret) {
			t.Errorf("request log contains a secret:\n%s", logs)
		}
	}
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	other, err := e.auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "alice", Password: password}))
	if err != nil {
		t.Fatal(err)
	}
	const next = "a brand new password"

	_, err = e.auth.ChangePassword(ctx, as(tok, &pb.ChangePasswordRequest{CurrentPassword: "wrong password", NewPassword: next}))
	assertCode(t, err, connect.CodePermissionDenied)
	key := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ, pb.ApiKeyScope_API_KEY_SCOPE_WRITE).GetKey()
	_, err = e.auth.ChangePassword(ctx, as(key, &pb.ChangePasswordRequest{CurrentPassword: password, NewPassword: next}))
	assertCode(t, err, connect.CodePermissionDenied)

	if _, err := e.auth.ChangePassword(ctx, as(tok, &pb.ChangePasswordRequest{CurrentPassword: password, NewPassword: next})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.me(t, tok); err != nil {
		t.Fatalf("caller's session ended: %v", err)
	}
	_, err = e.me(t, other.Msg.GetToken())
	assertCode(t, err, connect.CodeUnauthenticated)
	_, err = e.auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "alice", Password: password}))
	assertCode(t, err, connect.CodeUnauthenticated)
	if _, err := e.auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "alice", Password: next})); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestInvites(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	admin := e.register(t, "alice", "").GetToken()
	first, err := e.auth.CreateInvite(ctx, as(admin, &pb.CreateInviteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "default expiry", first.Msg.GetInvite().GetExpiresAt(), time.Now().Add(7*24*time.Hour))
	bob := e.register(t, "bob", first.Msg.GetInvite().GetCode())
	if bob.GetUser().GetIsAdmin() {
		t.Fatal("invited user is admin")
	}

	at := time.Now().Add(48 * time.Hour)
	second, err := e.auth.CreateInvite(ctx, as(admin, &pb.CreateInviteRequest{ExpiresAt: timestamppb.New(at)}))
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "chosen expiry", second.Msg.GetInvite().GetExpiresAt(), at)

	_, err = e.auth.CreateInvite(ctx, as(bob.GetToken(), &pb.CreateInviteRequest{}))
	assertCode(t, err, connect.CodePermissionDenied)
	_, err = e.auth.ListInvites(ctx, as(bob.GetToken(), &pb.ListInvitesRequest{}))
	assertCode(t, err, connect.CodePermissionDenied)

	list, err := e.auth.ListInvites(ctx, as(admin, &pb.ListInvitesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	inv := list.Msg.GetInvites()
	if len(inv) != 2 || inv[0].GetCode() != second.Msg.GetInvite().GetCode() || inv[0].GetUsedBy() != "" ||
		inv[1].GetUsedBy() != "bob" || inv[1].GetUsedAt() == nil {
		t.Fatalf("invites = %v, want the unused one first and bob's second", inv)
	}
}

func TestCookieFlags(t *testing.T) {
	for _, tc := range []struct {
		publicURL string
		secure    bool
	}{{httpsURL, true}, {"", true}, {"http://localhost:8080", false}} {
		t.Run(tc.publicURL, func(t *testing.T) {
			e := newEnv(t, tc.publicURL)
			ctx := t.Context()
			reg, err := e.auth.Register(ctx, connect.NewRequest(&pb.RegisterRequest{Username: "alice", Password: password}))
			if err != nil {
				t.Fatal(err)
			}
			c := sessionCookie(t, reg.Header())
			if c.Value != reg.Msg.GetToken() || !c.HttpOnly || c.Secure != tc.secure || c.SameSite != http.SameSiteLaxMode ||
				c.Path != "/" || c.MaxAge != int(ninetyDay/time.Second) {
				t.Fatalf("register cookie = %+v", c)
			}
			login, err := e.auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "alice", Password: password}))
			if err != nil {
				t.Fatal(err)
			}
			if c := sessionCookie(t, login.Header()); c.Value != login.Msg.GetToken() || c.Secure != tc.secure {
				t.Fatalf("login cookie = %+v", c)
			}

			withCookie := func(req connect.AnyRequest) {
				req.Header().Set("Cookie", (&http.Cookie{Name: auth.SessionCookie, Value: reg.Msg.GetToken()}).String()) //nolint:gosec // request cookie
			}
			me := connect.NewRequest(&pb.GetMeRequest{})
			withCookie(me)
			if _, err := e.auth.GetMe(ctx, me); err != nil {
				t.Fatalf("GetMe with cookie: %v", err)
			}
			out := connect.NewRequest(&pb.LogoutRequest{})
			withCookie(out)
			lo, err := e.auth.Logout(ctx, out)
			if err != nil {
				t.Fatal(err)
			}
			if c := sessionCookie(t, lo.Header()); c.Value != "" || c.MaxAge >= 0 {
				t.Fatalf("logout cookie = %+v, want it cleared", c)
			}
			_, err = e.me(t, reg.Msg.GetToken())
			assertCode(t, err, connect.CodeUnauthenticated)
		})
	}
}

func sessionCookie(t *testing.T, h http.Header) *http.Cookie {
	t.Helper()
	c, err := http.ParseSetCookie(h.Get("Set-Cookie"))
	if err != nil || c.Name != auth.SessionCookie {
		t.Fatalf("Set-Cookie %q: %v", h.Get("Set-Cookie"), err)
	}
	return c
}

func TestRegisterSettings(t *testing.T) {
	e := newEnv(t, httpsURL)
	_, err := e.auth.Register(t.Context(), connect.NewRequest(&pb.RegisterRequest{
		Username: "alice", Password: password, Timezone: "Mars/Olympus_Mons",
	}))
	assertCode(t, err, connect.CodeInvalidArgument)
	res, err := e.auth.Register(t.Context(), connect.NewRequest(&pb.RegisterRequest{
		Username: "alice", Password: password, DisplayName: "Alice A", Units: pb.Unit_UNIT_LB, Timezone: "Asia/Kolkata",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if u := res.Msg.GetUser(); u.GetDisplayName() != "Alice A" || u.GetUnits() != pb.Unit_UNIT_LB || u.GetTimezone() != "Asia/Kolkata" {
		t.Fatalf("register = %v", u)
	}
}

func TestUpdateMe(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()

	res, err := e.auth.UpdateMe(ctx, as(tok, &pb.UpdateMeRequest{Units: pb.Unit_UNIT_LB.Enum(), Timezone: proto.String("Asia/Kolkata")}))
	if err != nil {
		t.Fatal(err)
	}
	if u := res.Msg.GetUser(); u.GetUnits() != pb.Unit_UNIT_LB || u.GetTimezone() != "Asia/Kolkata" || u.GetDisplayName() != "alice" {
		t.Fatalf("UpdateMe = %v", u)
	}
	if _, err := e.auth.UpdateMe(ctx, as(tok, &pb.UpdateMeRequest{
		DisplayName: proto.String("Alice A"), MeasurementUnit: pb.LengthUnit_LENGTH_UNIT_IN.Enum(),
	})); err != nil {
		t.Fatal(err)
	}
	if u, err := e.me(t, tok); err != nil || u.GetDisplayName() != "Alice A" || u.GetUnits() != pb.Unit_UNIT_LB ||
		u.GetTimezone() != "Asia/Kolkata" || u.GetMeasurementUnit() != pb.LengthUnit_LENGTH_UNIT_IN {
		t.Fatalf("GetMe = %v, err %v", u, err)
	}

	for _, bad := range []*pb.UpdateMeRequest{
		{Timezone: proto.String("Mars/Olympus_Mons")},
		{Timezone: proto.String("Local")},
		{Units: pb.Unit_UNIT_UNSPECIFIED.Enum()},
		{MeasurementUnit: pb.LengthUnit_LENGTH_UNIT_UNSPECIFIED.Enum()},
		{DisplayName: proto.String("")},
	} {
		_, err := e.auth.UpdateMe(ctx, as(tok, bad))
		assertCode(t, err, connect.CodeInvalidArgument)
	}
	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = e.auth.UpdateMe(ctx, as(ro, &pb.UpdateMeRequest{Units: pb.Unit_UNIT_KG.Enum()}))
	assertCode(t, err, connect.CodePermissionDenied)
}
