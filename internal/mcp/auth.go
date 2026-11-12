package mcp

import (
	"context"

	"github.com/gdsoumya/protomcp/pkg/protomcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// bearerToMetadata forwards the MCP request's Authorization header as gRPC
// metadata, so the API's auth interceptor authenticates the tool call. Cookies
// are not forwarded: MCP callers hold API keys, never browser sessions.
func bearerToMetadata(next protomcp.ToolHandler) protomcp.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest, g *protomcp.GRPCData) (*sdk.CallToolResult, error) {
		if req.Extra != nil {
			if v := req.Extra.Header.Get("Authorization"); v != "" {
				g.SetMetadata("authorization", v)
			}
		}
		return next(ctx, req, g)
	}
}
