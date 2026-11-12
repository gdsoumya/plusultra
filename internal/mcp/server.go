// Package mcp serves the annotated RPCs as MCP tools over streamable HTTP.
package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gdsoumya/protomcp/pkg/protomcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/api"
)

// maxBodyBytes matches the ProgramService read limit, since tool calls carry scripts.
const maxBodyBytes = 4 << 20

const instructions = `Before writing a program, read reference_get_program_language and reference_get_program_examples. ` +
	`Check the script with program_validate, preview it with program_simulate, ` +
	`then program_create and program_activate. Full docs: %[1]s/llms.txt, which links the four reference pages under %[1]s/llms/. ` +
	`Dates are YYYY-MM-DD in the user's time zone from auth_get_me. ` +
	`Destructive tools (delete, revoke, finish, finalize) ask the user first.`

// NewHandler serves every annotated RPC as an MCP tool. Tool calls reach the
// API handler over an in-process gRPC connection, so its interceptors
// authenticate and validate them unchanged.
func NewHandler(d api.Deps) (http.Handler, error) {
	conn, err := dialInProcess(prefixed(api.NewHandler(d)))
	if err != nil {
		return nil, err
	}
	srv := protomcp.New("plusultra", d.Version,
		protomcp.WithToolMiddleware(bearerToMetadata),
		protomcp.WithSDKOptions(&sdk.ServerOptions{Instructions: fmt.Sprintf(instructions, strings.TrimRight(d.PublicURL, "/")), Logger: d.Logger}),
		protomcp.WithHTTPOptions(&sdk.StreamableHTTPOptions{
			Stateless:           true,
			MaxRequestBodyBytes: maxBodyBytes,
			// Self-hosted servers often sit behind a reverse proxy on loopback, which this check would block.
			DisableLocalhostProtection: true,
		}),
	)
	pb.RegisterAuthServiceMCPTools(srv, pb.NewAuthServiceClient(conn))
	pb.RegisterApiKeyServiceMCPTools(srv, pb.NewApiKeyServiceClient(conn))
	pb.RegisterProgramServiceMCPTools(srv, pb.NewProgramServiceClient(conn))
	pb.RegisterCalendarServiceMCPTools(srv, pb.NewCalendarServiceClient(conn))
	pb.RegisterWorkoutServiceMCPTools(srv, pb.NewWorkoutServiceClient(conn))
	pb.RegisterMealServiceMCPTools(srv, pb.NewMealServiceClient(conn))
	pb.RegisterCheckinServiceMCPTools(srv, pb.NewCheckinServiceClient(conn))
	pb.RegisterExerciseServiceMCPTools(srv, pb.NewExerciseServiceClient(conn))
	pb.RegisterFoodServiceMCPTools(srv, pb.NewFoodServiceClient(conn))
	pb.RegisterInsightServiceMCPTools(srv, pb.NewInsightServiceClient(conn))
	pb.RegisterExportServiceMCPTools(srv, pb.NewExportServiceClient(conn))
	pb.RegisterReferenceServiceMCPTools(srv, pb.NewReferenceServiceClient(conn))
	return srv, nil
}

// prefixed moves requests under api.Prefix: the generated gRPC clients dial
// bare procedure paths.
func prefixed(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.Clone(r.Context())
		r.URL.Path = api.Prefix + r.URL.Path
		h.ServeHTTP(w, r)
	})
}

// dialInProcess serves h over h2c on an in-memory listener and returns a gRPC
// connection to it.
func dialInProcess(h http.Handler) (*grpc.ClientConn, error) {
	lis := bufconn.Listen(1 << 20)
	go func() {
		_ = (&http.Server{Handler: h, Protocols: api.Protocols(), ReadHeaderTimeout: 10 * time.Second}).Serve(lis)
	}()
	return grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}
