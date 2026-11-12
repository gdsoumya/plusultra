package api

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/insights"
)

// publicError passes Connect errors through and hides the rest, since database
// errors can name hosts, users and schemas.
func publicError(l *slog.Logger, procedure string, err error) error {
	err = logsError(err)
	var cerr *connect.Error
	switch {
	case err == nil || errors.As(err, &cerr):
		return err
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, catalog.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("not found"))
	case errors.Is(err, catalog.ErrInvalid), errors.Is(err, insights.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, catalog.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, catalog.ErrUpstream):
		return connect.NewError(connect.CodeUnavailable, errors.New("food database unavailable"))
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded)
	}
	l.Error("internal error", "procedure", procedure, "err", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// caller returns the principal, which the auth interceptor guarantees on every
// procedure that is not public.
func caller(ctx context.Context) auth.Principal {
	p, _ := auth.FromContext(ctx)
	return p
}

func callerID(ctx context.Context) (uuid.UUID, error) {
	return uuid.Parse(caller(ctx).UserID)
}

// Database spellings, indexed by enum value.
var (
	unitNames   = [...]string{pb.Unit_UNIT_KG: "kg", pb.Unit_UNIT_LB: "lb"}
	lengthNames = [...]string{pb.LengthUnit_LENGTH_UNIT_CM: "cm", pb.LengthUnit_LENGTH_UNIT_IN: "in"}
	scopeNames  = [...]string{
		pb.ApiKeyScope_API_KEY_SCOPE_READ:  auth.ScopeRead,
		pb.ApiKeyScope_API_KEY_SCOPE_WRITE: auth.ScopeWrite,
		pb.ApiKeyScope_API_KEY_SCOPE_ADMIN: auth.ScopeAdmin,
	}
)

func userPB(u sqlc.User) *pb.User {
	return &pb.User{
		Id:          u.ID.String(),
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Units:       pb.Unit(slices.Index(unitNames[:], u.Units)), //nolint:gosec // index into a 3-element array
		Timezone:    u.Timezone,
		IsAdmin:     u.IsAdmin,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		//nolint:gosec // index into a 3-element array
		MeasurementUnit: pb.LengthUnit(slices.Index(lengthNames[:], u.MeasurementUnit)),
	}
}

func apiKeyPB(k sqlc.ApiKey) *pb.ApiKey {
	scopes := make([]pb.ApiKeyScope, len(k.Scopes))
	for i, s := range k.Scopes {
		scopes[i] = pb.ApiKeyScope(slices.Index(scopeNames[:], s)) //nolint:gosec // index into a 4-element array
	}
	return &pb.ApiKey{
		Id:         k.ID.String(),
		Name:       k.Name,
		Prefix:     k.Prefix,
		Scopes:     scopes,
		ExpiresAt:  optTime(k.ExpiresAt),
		RevokedAt:  optTime(k.RevokedAt),
		LastUsedAt: optTime(k.LastUsedAt),
		CreatedAt:  timestamppb.New(k.CreatedAt),
	}
}

func scopesDB(scopes []pb.ApiKeyScope) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = scopeNames[s] // protovalidate allows only defined, non-zero scopes
	}
	return out
}

func invitePB(inv sqlc.Invite, usedBy string) *pb.Invite {
	return &pb.Invite{
		Code:      inv.Code,
		ExpiresAt: timestamppb.New(inv.ExpiresAt),
		UsedAt:    optTime(inv.UsedAt),
		UsedBy:    usedBy,
		CreatedAt: timestamppb.New(inv.CreatedAt),
	}
}

func optTime(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
