package auth

import (
	"context"
	"slices"
)

// API key scopes. Sessions carry read and write, plus admin for admin users.
const (
	ScopeRead  = "read"
	ScopeWrite = "write"
	ScopeAdmin = "admin"
)

// Values of Principal.Via.
const (
	ViaSession = "session"
	ViaAPIKey  = "apikey"
)

// Principal is the authenticated caller of a request.
type Principal struct {
	UserID string
	// Admin is true for an admin's session, or an admin's key with the admin scope.
	Admin  bool
	Scopes []string
	Via    string
}

type principalKey struct{}

// FromContext returns the principal the interceptor attached to ctx.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func withPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// RequireWrite fails unless the caller may modify data.
func RequireWrite(ctx context.Context) error {
	return require(ctx, "write", func(p Principal) bool { return slices.Contains(p.Scopes, ScopeWrite) })
}

// RequireAdmin fails unless the caller acts with admin rights.
func RequireAdmin(ctx context.Context) error {
	return require(ctx, "admin", func(p Principal) bool { return p.Admin })
}

func require(ctx context.Context, what string, allowed func(Principal) bool) error {
	p, ok := FromContext(ctx)
	if !ok {
		return errUnauthenticated()
	}
	if !allowed(p) {
		return permissionDenied("requires " + what + " access")
	}
	return nil
}
