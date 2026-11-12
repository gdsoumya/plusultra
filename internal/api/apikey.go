package api

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
)

type apiKeyServer struct{ Deps }

//nolint:revive // names come from the generated ApiKeyServiceHandler
func (s *apiKeyServer) CreateApiKey(ctx context.Context, req *connect.Request[pb.CreateApiKeyRequest]) (*connect.Response[pb.CreateApiKeyResponse], error) {
	m := req.Msg
	plain, key, err := s.Auth.CreateAPIKey(ctx, caller(ctx), m.GetName(), m.GetExpiresAt().AsTime(), scopesDB(m.GetScopes()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateApiKeyResponse{Key: plain, ApiKey: apiKeyPB(key)}), nil
}

//nolint:revive // generated name
func (s *apiKeyServer) ListApiKeys(ctx context.Context, _ *connect.Request[pb.ListApiKeysRequest]) (*connect.Response[pb.ListApiKeysResponse], error) {
	keys, err := s.Auth.ListAPIKeys(ctx, caller(ctx).UserID)
	if err != nil {
		return nil, err
	}
	res := &pb.ListApiKeysResponse{ApiKeys: make([]*pb.ApiKey, len(keys))}
	for i, k := range keys {
		res.ApiKeys[i] = apiKeyPB(k)
	}
	return connect.NewResponse(res), nil
}

//nolint:revive // generated name
func (s *apiKeyServer) RevokeApiKey(ctx context.Context, req *connect.Request[pb.RevokeApiKeyRequest]) (*connect.Response[pb.RevokeApiKeyResponse], error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return nil, err
	}
	key, err := s.Auth.RevokeAPIKey(ctx, caller(ctx).UserID, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RevokeApiKeyResponse{ApiKey: apiKeyPB(key)}), nil
}
