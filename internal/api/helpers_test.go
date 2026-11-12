package api

import (
	"testing"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
)

// result holds a client call's outcome so a test can write call(c.Rpc(...)).ok(t).
type result[T any] struct {
	res *connect.Response[T]
	err error
}

func call[T any](res *connect.Response[T], err error) result[T] { return result[T]{res, err} }

// ok fails the test on an error and returns the response message.
func (r result[T]) ok(t *testing.T) *T {
	t.Helper()
	if r.err != nil {
		t.Fatal(r.err)
	}
	return r.res.Msg
}

// invitee registers a second, non-admin user with an invite from admin.
func (e *testEnv) invitee(t *testing.T, admin, name string) *pb.RegisterResponse {
	t.Helper()
	inv, err := e.auth.CreateInvite(t.Context(), as(admin, &pb.CreateInviteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	return e.register(t, name, inv.Msg.GetInvite().GetCode())
}

func (e *testEnv) setUnits(t *testing.T, tok string, u pb.Unit) {
	t.Helper()
	if _, err := e.auth.UpdateMe(t.Context(), as(tok, &pb.UpdateMeRequest{Units: u.Enum()})); err != nil {
		t.Fatal(err)
	}
}
