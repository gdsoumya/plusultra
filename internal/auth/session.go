package auth

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// SessionCookie names the cookie that carries a session token for the web app.
const SessionCookie = "session"

// touchInterval limits last-seen writes to one per credential per minute.
const touchInterval = time.Minute

var sessionFormat = regexp.MustCompile(`^[0-9a-f]{64}$`)

// dummyHash makes a login for an unknown username cost as much as a wrong password.
var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword(context.Background(), "not a real password")
	return h
})

// Session is a new login session. Token is its only copy; the database keeps a hash.
type Session struct {
	Token     string
	User      sqlc.User
	ExpiresAt time.Time
}

// Login checks the password and starts a session valid for 90 days after its last use.
func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	if len(password) > maxPasswordLen {
		return Session{}, errUnauthenticated()
	}
	user, err := s.Q.GetUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		VerifyPassword(ctx, dummyHash(), password)
		return Session{}, errUnauthenticated()
	}
	if err != nil {
		return Session{}, s.publicError(err)
	}
	if !VerifyPassword(ctx, user.PasswordHash, password) {
		return Session{}, errUnauthenticated()
	}
	return s.StartSession(ctx, user)
}

// StartSession signs user in without a password check, as right after Register.
func (s *Service) StartSession(ctx context.Context, user sqlc.User) (Session, error) {
	token := randomHex(32)
	sess, err := s.Q.CreateSession(ctx, sqlc.CreateSessionParams{UserID: user.ID, TokenHash: hashToken(token)})
	if err != nil {
		return Session{}, s.publicError(err)
	}
	return Session{Token: token, User: user, ExpiresAt: sess.ExpiresAt}, nil
}

// Logout revokes the session for token. Unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, token string) error {
	if !sessionFormat.MatchString(token) {
		return nil
	}
	sess, err := s.Q.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return s.publicError(err)
	}
	return s.publicError(s.Q.DeleteSession(ctx, sess.ID))
}

func (s *Service) sessionPrincipal(ctx context.Context, token string) (Principal, error) {
	sess, err := s.Q.GetSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		return Principal{}, err
	}
	if time.Since(sess.LastSeenAt) > touchInterval {
		if err := s.Q.TouchSession(ctx, sess.ID); err != nil {
			s.Logger.Warn("auth: extend session", "err", err)
		}
	}
	user, err := s.Q.GetUser(ctx, sess.UserID)
	if err != nil {
		return Principal{}, err
	}
	scopes := []string{ScopeRead, ScopeWrite}
	if user.IsAdmin {
		scopes = append(scopes, ScopeAdmin)
	}
	return Principal{UserID: user.ID.String(), Admin: user.IsAdmin, Scopes: scopes, Via: ViaSession}, nil
}
