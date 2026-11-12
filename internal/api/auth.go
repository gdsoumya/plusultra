package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// sessionMaxAge matches the 90-day session lifetime in the database, in seconds.
const sessionMaxAge = 90 * 24 * 60 * 60

type authServer struct{ Deps }

// cookie returns the session cookie for the web app; maxAge < 0 deletes it.
func (s *authServer) cookie(token string, maxAge int) string {
	return (&http.Cookie{ //nolint:gosec // Secure is dropped only for an http:// PUBLIC_URL, i.e. local dev
		Name:     auth.SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !strings.HasPrefix(s.PublicURL, "http://"),
		SameSite: http.SameSiteLaxMode,
	}).String()
}

func withCookie[T any](msg *T, cookie string) *connect.Response[T] {
	res := connect.NewResponse(msg)
	res.Header().Add("Set-Cookie", cookie)
	return res
}

func (s *authServer) Register(ctx context.Context, req *connect.Request[pb.RegisterRequest]) (*connect.Response[pb.RegisterResponse], error) {
	m := req.Msg
	user, err := s.Auth.Register(ctx, auth.NewUser{
		Username: m.GetUsername(), Password: m.GetPassword(), InviteCode: m.GetInviteCode(),
		DisplayName: m.GetDisplayName(), Units: unitNames[m.GetUnits()], Timezone: m.GetTimezone(),
	})
	if err != nil {
		return nil, err
	}
	sess, err := s.Auth.StartSession(ctx, user)
	if err != nil {
		return nil, err
	}
	return withCookie(&pb.RegisterResponse{
		User: userPB(sess.User), Token: sess.Token, ExpiresAt: timestamppb.New(sess.ExpiresAt),
	}, s.cookie(sess.Token, sessionMaxAge)), nil
}

func (s *authServer) Login(ctx context.Context, req *connect.Request[pb.LoginRequest]) (*connect.Response[pb.LoginResponse], error) {
	sess, err := s.Auth.Login(ctx, req.Msg.GetUsername(), req.Msg.GetPassword())
	if err != nil {
		return nil, err
	}
	return withCookie(&pb.LoginResponse{
		User: userPB(sess.User), Token: sess.Token, ExpiresAt: timestamppb.New(sess.ExpiresAt),
	}, s.cookie(sess.Token, sessionMaxAge)), nil
}

func (s *authServer) Logout(ctx context.Context, req *connect.Request[pb.LogoutRequest]) (*connect.Response[pb.LogoutResponse], error) {
	token, _ := auth.ParseCredential(req.Header())
	if err := s.Auth.Logout(ctx, token); err != nil {
		return nil, err
	}
	return withCookie(&pb.LogoutResponse{}, s.cookie("", -1)), nil
}

func (s *authServer) ChangePassword(ctx context.Context, req *connect.Request[pb.ChangePasswordRequest]) (*connect.Response[pb.ChangePasswordResponse], error) {
	token, _ := auth.ParseCredential(req.Header())
	if err := s.Auth.ChangePassword(ctx, caller(ctx), token, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ChangePasswordResponse{}), nil
}

func (s *authServer) GetMe(ctx context.Context, _ *connect.Request[pb.GetMeRequest]) (*connect.Response[pb.GetMeResponse], error) {
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.Queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetMeResponse{User: userPB(user)}), nil
}

func (s *authServer) UpdateMe(ctx context.Context, req *connect.Request[pb.UpdateMeRequest]) (*connect.Response[pb.UpdateMeResponse], error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return nil, err
	}
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if m.Timezone != nil {
		if err := auth.CheckTimezone(m.GetTimezone()); err != nil {
			return nil, err
		}
	}
	arg := sqlc.UpdateUserSettingsParams{ID: id, DisplayName: m.DisplayName, Timezone: m.Timezone}
	if m.Units != nil {
		arg.Units = &unitNames[m.GetUnits()]
	}
	if m.MeasurementUnit != nil {
		arg.MeasurementUnit = &lengthNames[m.GetMeasurementUnit()]
	}
	user, err := s.Queries.UpdateUserSettings(ctx, arg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateMeResponse{User: userPB(user)}), nil
}

func (s *authServer) CreateInvite(ctx context.Context, req *connect.Request[pb.CreateInviteRequest]) (*connect.Response[pb.CreateInviteResponse], error) {
	var expiresAt time.Time
	if req.Msg.ExpiresAt != nil {
		expiresAt = req.Msg.ExpiresAt.AsTime()
	}
	inv, err := s.Auth.CreateInvite(ctx, caller(ctx), expiresAt)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateInviteResponse{Invite: invitePB(inv, "")}), nil
}

func (s *authServer) ListInvites(ctx context.Context, _ *connect.Request[pb.ListInvitesRequest]) (*connect.Response[pb.ListInvitesResponse], error) {
	if err := auth.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Queries.ListInvites(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.ListInvitesResponse{Invites: make([]*pb.Invite, len(rows))}
	for i, r := range rows {
		var usedBy string
		if r.UsedByUsername != nil {
			usedBy = *r.UsedByUsername
		}
		res.Invites[i] = invitePB(r.Invite, usedBy)
	}
	return connect.NewResponse(res), nil
}
