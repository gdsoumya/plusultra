package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/gdsoumya/plusultra/internal/auth"
)

type callerKey struct{}

// logRequests logs every call and hides internal errors. It runs before the
// auth interceptor, so noteCaller hands it the user id through ctx. Headers and
// messages are never logged, since they carry tokens and passwords.
func logRequests(l *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			var userID string
			res, err := next(context.WithValue(ctx, callerKey{}, &userID), req)
			procedure := req.Spec().Procedure
			err = publicError(l, procedure, err)
			code := "ok"
			if err != nil {
				code = connect.CodeOf(err).String()
			}
			l.LogAttrs(ctx, slog.LevelInfo, "rpc",
				slog.String("procedure", procedure),
				slog.String("code", code),
				slog.Duration("duration", time.Since(start)),
				slog.String("user_id", userID),
			)
			return res, err
		}
	}
}

func noteCaller(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		p, ok := auth.FromContext(ctx)
		if id, slot := ctx.Value(callerKey{}).(*string); ok && slot {
			*id = p.UserID
		}
		return next(ctx, req)
	}
}

// validateRequests enforces the protovalidate rules in the protos and returns
// the violations as an error detail.
func validateRequests(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		msg, ok := req.Any().(proto.Message)
		if !ok {
			return next(ctx, req)
		}
		err := protovalidate.Validate(msg)
		var verr *protovalidate.ValidationError
		if errors.As(err, &verr) {
			cerr := connect.NewError(connect.CodeInvalidArgument, err)
			if d, derr := connect.NewErrorDetail(verr.ToProto()); derr == nil {
				cerr.AddDetail(d)
			}
			return nil, cerr
		}
		if err != nil {
			return nil, err // a broken rule; logRequests reports it as internal
		}
		return next(ctx, req)
	}
}
