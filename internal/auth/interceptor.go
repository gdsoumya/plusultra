package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
)

// NewInterceptor authenticates every handler call except public procedures. A
// public entry is a full procedure ("/pkg.Service/Method") or, ending in "/", a
// whole service ("/pkg.Service/"). Credentials come from "Authorization: Bearer
// <session token or lsk_ key>" or the session cookie.
func NewInterceptor(s *Service, public ...string) connect.Interceptor {
	return &interceptor{s: s, public: public}
}

type interceptor struct {
	s      *Service
	public []string
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			return next(ctx, req)
		}
		ctx, err := i.authenticate(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *interceptor) authenticate(ctx context.Context, procedure string, h http.Header) (context.Context, error) {
	if slices.ContainsFunc(i.public, func(p string) bool {
		return p == procedure || strings.HasSuffix(p, "/") && strings.HasPrefix(procedure, p)
	}) {
		return ctx, nil
	}
	p, err := i.s.principal(ctx, h)
	if err != nil {
		return ctx, err
	}
	return withPrincipal(ctx, p), nil
}

// ParseCredential returns the raw token from "Authorization: Bearer", falling
// back to the session cookie, and reports whether it came from the cookie.
func ParseCredential(h http.Header) (token string, fromCookie bool) {
	if scheme, token, ok := strings.Cut(h.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
		return strings.TrimSpace(token), false
	}
	if c, err := (&http.Request{Header: h}).Cookie(SessionCookie); err == nil {
		return c.Value, true
	}
	return "", false
}

func (s *Service) principal(ctx context.Context, h http.Header) (Principal, error) {
	token, fromCookie := ParseCredential(h)
	var p Principal
	var err error
	switch {
	case apiKeyFormat.MatchString(token) && !fromCookie: // keys are for programs, never browser cookies
		p, err = s.apiKeyPrincipal(ctx, token)
	case sessionFormat.MatchString(token):
		p, err = s.sessionPrincipal(ctx, token)
	default:
		return Principal{}, errUnauthenticated()
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, errUnauthenticated()
	}
	if err != nil {
		return Principal{}, s.publicError(err)
	}
	return p, nil
}
