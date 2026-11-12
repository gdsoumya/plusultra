package auth

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// ChangePassword replaces p's password after checking the current one and ends
// every session of the user except keepToken's. Only a login session may do
// this, so a leaked API key cannot take over the account.
func (s *Service) ChangePassword(ctx context.Context, p Principal, keepToken, current, next string) error {
	if p.Via != ViaSession {
		return permissionDenied("the password can only be changed from a login session")
	}
	if err := checkPassword(next); err != nil {
		return err
	}
	uid, err := parseID(p.UserID)
	if err != nil {
		return err
	}
	user, err := s.Q.GetUser(ctx, uid)
	if err != nil {
		return s.publicError(err)
	}
	if !VerifyPassword(ctx, user.PasswordHash, current) {
		return permissionDenied("current password is wrong")
	}
	hash, err := HashPassword(ctx, next)
	if err != nil {
		return s.publicError(err)
	}
	return s.publicError(pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.Q.WithTx(tx)
		if err := q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: uid, PasswordHash: hash}); err != nil {
			return err
		}
		return q.DeleteOtherSessions(ctx, sqlc.DeleteOtherSessionsParams{UserID: uid, KeepHash: hashToken(keepToken)})
	}))
}
