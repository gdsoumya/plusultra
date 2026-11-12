package auth

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const inviteTTL = 7 * 24 * time.Hour

// CreateInvite returns a single-use registration code that expires at
// expiresAt, or in 7 days when it is zero. p must be an admin: an admin's
// session, or an admin's key with the admin scope.
func (s *Service) CreateInvite(ctx context.Context, p Principal, expiresAt time.Time) (sqlc.Invite, error) {
	if !p.Admin {
		return sqlc.Invite{}, permissionDenied("only admins can create invites")
	}
	id, err := parseID(p.UserID)
	if err != nil {
		return sqlc.Invite{}, err
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(inviteTTL)
	}
	inv, err := s.Q.CreateInvite(ctx, sqlc.CreateInviteParams{Code: rand.Text(), CreatedBy: id, ExpiresAt: expiresAt})
	if err != nil {
		return sqlc.Invite{}, s.publicError(err)
	}
	return inv, nil
}
