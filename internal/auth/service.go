// Package auth handles passwords, sessions, API keys, invites and the Connect
// interceptor that turns credentials into a Principal.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// Registration modes for Config.Registration.
const (
	RegistrationOpen   = "open"
	RegistrationInvite = "invite"
)

// Config holds server-wide auth settings.
type Config struct {
	// Registration is RegistrationOpen or RegistrationInvite. Anything else,
	// including empty, means invite.
	Registration string
}

// Service implements registration, login, API keys and invites. Build it with NewService.
type Service struct {
	Q      *sqlc.Queries
	Logger *slog.Logger
	pool   *pgxpool.Pool
	cfg    Config
}

// NewService returns a Service backed by pool that logs to slog.Default().
func NewService(pool *pgxpool.Pool, cfg Config) *Service {
	return &Service{Q: sqlc.New(pool), Logger: slog.Default(), pool: pool, cfg: cfg}
}

const msgUnauthenticated = "invalid or missing credentials"

// errUnauthenticated is the one error for every credential failure, so a response
// never reveals whether a user, session or key exists.
func errUnauthenticated() error {
	return connect.NewError(connect.CodeUnauthenticated, errors.New(msgUnauthenticated))
}

func invalidArgument(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

func permissionDenied(msg string) error {
	return connect.NewError(connect.CodePermissionDenied, errors.New(msg))
}

// publicError passes Connect errors through and logs and hides anything else,
// since pgx errors can name the database host, user and schema.
func (s *Service) publicError(err error) error {
	var ce *connect.Error
	switch {
	case err == nil || errors.As(err, &ce):
		return err
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded)
	}
	s.Logger.Error("auth: internal error", "err", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

func parseID(id string) (uuid.UUID, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, invalidArgument("invalid id")
	}
	return u, nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return hex.EncodeToString(b)
}
