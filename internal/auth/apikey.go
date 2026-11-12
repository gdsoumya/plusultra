package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const (
	apiKeyPrefix = "lsk_"
	maxKeyTTL    = 365 * 24 * time.Hour
)

var (
	apiKeyFormat = regexp.MustCompile(`^lsk_[0-9a-f]{8}_[0-9a-f]{32}$`)
	validScopes  = []string{ScopeRead, ScopeWrite, ScopeAdmin}
)

// CreateAPIKey issues a key for the session principal p and returns its
// plaintext, which is never stored. Keys are lsk_<8 hex prefix>_<32 hex secret>;
// only the prefix and the SHA-256 of the whole key are kept. Empty scopes mean
// read and write. Keys cannot create keys, and a key cannot exceed p's scopes.
func (s *Service) CreateAPIKey(ctx context.Context, p Principal, name string, expiresAt time.Time, scopes []string) (string, sqlc.ApiKey, error) {
	if p.Via != ViaSession {
		return "", sqlc.ApiKey{}, permissionDenied("api keys can only be created from a login session")
	}
	uid, err := parseID(p.UserID)
	if err != nil {
		return "", sqlc.ApiKey{}, err
	}
	now := time.Now()
	if !expiresAt.After(now) || expiresAt.After(now.Add(maxKeyTTL)) {
		return "", sqlc.ApiKey{}, invalidArgument("expiry is required and must be within 365 days")
	}
	if len(scopes) == 0 {
		scopes = []string{ScopeRead, ScopeWrite}
	}
	if err := checkScopes(p, scopes); err != nil {
		return "", sqlc.ApiKey{}, err
	}
	prefix := randomHex(4)
	plain := apiKeyPrefix + prefix + "_" + randomHex(16)
	key, err := s.Q.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		UserID: uid, Name: name, Prefix: prefix, KeyHash: hashToken(plain), Scopes: scopes, ExpiresAt: &expiresAt,
	})
	if err != nil {
		return "", sqlc.ApiKey{}, s.publicError(err)
	}
	return plain, key, nil
}

func checkScopes(p Principal, scopes []string) error {
	for i, sc := range scopes {
		switch {
		case !slices.Contains(validScopes, sc):
			return invalidArgument(fmt.Sprintf("unknown scope %q", sc))
		case slices.Contains(scopes[:i], sc):
			return invalidArgument(fmt.Sprintf("duplicate scope %q", sc))
		case !slices.Contains(p.Scopes, sc):
			return permissionDenied(fmt.Sprintf("scope %q exceeds your own", sc))
		}
	}
	return nil
}

// ListAPIKeys returns the user's keys, including revoked and expired ones.
func (s *Service) ListAPIKeys(ctx context.Context, userID string) ([]sqlc.ApiKey, error) {
	uid, err := parseID(userID)
	if err != nil {
		return nil, err
	}
	keys, err := s.Q.ListAPIKeys(ctx, uid)
	return keys, s.publicError(err)
}

// RevokeAPIKey revokes one of the user's active keys and returns it.
func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID string) (sqlc.ApiKey, error) {
	uid, err := parseID(userID)
	if err != nil {
		return sqlc.ApiKey{}, err
	}
	kid, err := parseID(keyID)
	if err != nil {
		return sqlc.ApiKey{}, err
	}
	key, err := s.Q.RevokeAPIKey(ctx, sqlc.RevokeAPIKeyParams{ID: kid, UserID: uid})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ApiKey{}, connect.NewError(connect.CodeNotFound, fmt.Errorf("api key %s not found", keyID))
	}
	if err != nil {
		return sqlc.ApiKey{}, s.publicError(err)
	}
	return key, nil
}

func (s *Service) apiKeyPrincipal(ctx context.Context, token string) (Principal, error) {
	key, err := s.Q.GetAPIKeyByHash(ctx, hashToken(token))
	if err != nil {
		return Principal{}, err
	}
	if key.LastUsedAt == nil || time.Since(*key.LastUsedAt) > touchInterval {
		if err := s.Q.TouchAPIKey(ctx, key.ID); err != nil {
			s.Logger.Warn("auth: record api key use", "err", err)
		}
	}
	user, err := s.Q.GetUser(ctx, key.UserID)
	if err != nil {
		return Principal{}, err
	}
	return Principal{
		UserID: user.ID.String(),
		Admin:  user.IsAdmin && slices.Contains(key.Scopes, ScopeAdmin),
		Scopes: key.Scopes,
		Via:    ViaAPIKey,
	}, nil
}
