package api

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
)

func TestCreateListRevokeAPIKey(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	rw := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ, pb.ApiKeyScope_API_KEY_SCOPE_WRITE)
	if !strings.HasPrefix(rw.GetKey(), "lsk_"+rw.GetApiKey().GetPrefix()+"_") ||
		!slices.Equal(rw.GetApiKey().GetScopes(), []pb.ApiKeyScope{pb.ApiKeyScope_API_KEY_SCOPE_READ, pb.ApiKeyScope_API_KEY_SCOPE_WRITE}) {
		t.Fatalf("create = %v", rw)
	}
	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ)

	list, err := e.keys.ListApiKeys(ctx, as(tok, &pb.ListApiKeysRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if ks := list.Msg.GetApiKeys(); len(ks) != 2 || ks[0].GetId() != ro.GetApiKey().GetId() || ks[1].GetId() != rw.GetApiKey().GetId() {
		t.Fatalf("list = %v, want the read-only key first", ks)
	}
	if _, err := e.me(t, rw.GetKey()); err != nil {
		t.Fatalf("GetMe with key: %v", err)
	}

	revoke := &pb.RevokeApiKeyRequest{Id: rw.GetApiKey().GetId()}
	_, err = e.keys.RevokeApiKey(ctx, as(ro.GetKey(), revoke))
	assertCode(t, err, connect.CodePermissionDenied)
	res, err := e.keys.RevokeApiKey(ctx, as(tok, revoke))
	if err != nil || res.Msg.GetApiKey().GetRevokedAt() == nil || res.Msg.GetApiKey().GetId() != revoke.GetId() {
		t.Fatalf("revoke = %v, err %v", res, err)
	}
	_, err = e.me(t, rw.GetKey())
	assertCode(t, err, connect.CodeUnauthenticated)
	_, err = e.keys.RevokeApiKey(ctx, as(tok, revoke))
	assertCode(t, err, connect.CodeNotFound)
}
