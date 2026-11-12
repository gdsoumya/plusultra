package api

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	protomcpv1 "github.com/gdsoumya/protomcp/pkg/api/gen/protomcp/v1"
	"github.com/gdsoumya/protomcp/pkg/protomcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
)

// Sessions, secrets, invites, sync and the full-account zip never reach the LLM.
var privateRPCs = map[string]bool{
	"AuthService.Register": true, "AuthService.Login": true, "AuthService.Logout": true,
	"AuthService.ChangePassword": true, "AuthService.CreateInvite": true, "AuthService.ListInvites": true,
	"ApiKeyService.CreateApiKey": true, "SyncService.Pull": true, "SyncService.Push": true,
	"ExportService.ExportAll": true,
}

var toolPrefixes = map[string]string{
	"AuthService": "auth_", "ApiKeyService": "keys_", "ProgramService": "program_", "CalendarService": "calendar_",
	"WorkoutService": "workout_", "MealService": "meal_", "CheckinService": "checkin_", "ExerciseService": "exercise_",
	"FoodService": "food_", "InsightService": "insight_", "ExportService": "export_", "ReferenceService": "reference_",
}

type hints struct{ readOnly, idempotent, destructive bool }

// First match wins; an RPC matching no rule fails the test so new RPCs get classified.
var hintRules = []struct {
	rpc  *regexp.Regexp
	want hints
}{
	{regexp.MustCompile(`^(Get|List|Search|Lookup|Validate|Simulate|Export)|^(WeeklySummary|LiftHistory|WeightTrend|Adherence)$`), hints{readOnly: true, idempotent: true}},
	{regexp.MustCompile(`^(Delete|Revoke)`), hints{idempotent: true, destructive: true}},
	{regexp.MustCompile(`^(Finish|Finalize)`), hints{destructive: true}},
	{regexp.MustCompile(`^Set`), hints{idempotent: true}},
	{regexp.MustCompile(`^(Create|Update|Activate|QuickFill|Import)`), hints{}},
}

var toolName = regexp.MustCompile(`^[a-z]+(_[a-z]+)+$`)

// toolMention finds tool names cited in descriptions, so a rename cannot leave a dangling reference.
var toolMention = regexp.MustCompile(`\b(?:` + strings.Join(slices.Collect(maps.Values(toolPrefixes)), "|") + `)[a-z_]*[a-z]`)

func TestMCPToolAnnotations(t *testing.T) {
	tools := map[string]string{} // tool name -> RPC
	rpcs := map[string]bool{}
	var descriptions []string
	protoregistry.GlobalFiles.RangeFilesByPackage("plusultra.v1", func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Services().Len() {
			sd := fd.Services().Get(i)
			svc := string(sd.Name())
			prefix := proto.GetExtension(sd.Options(), protomcpv1.E_Service).(*protomcpv1.ServiceOptions).GetToolPrefix()
			if want, exposed := toolPrefixes[svc]; exposed && prefix != want {
				t.Errorf("%s: tool_prefix %q, want %q", svc, prefix, want)
			} else if !exposed && proto.HasExtension(sd.Options(), protomcpv1.E_Service) {
				t.Errorf("%s: private service has a protomcp.v1.service option", svc)
			}
			for j := range sd.Methods().Len() {
				md := sd.Methods().Get(j)
				rpc := svc + "." + string(md.Name())
				rpcs[rpc] = true
				annotated := proto.HasExtension(md.Options(), protomcpv1.E_Tool)
				if privateRPCs[rpc] || toolPrefixes[svc] == "" {
					if annotated {
						t.Errorf("%s: private RPC is annotated as a tool", rpc)
					}
					continue
				}
				if !annotated {
					t.Errorf("%s: missing protomcp.v1.tool", rpc)
					continue
				}
				tool := proto.GetExtension(md.Options(), protomcpv1.E_Tool).(*protomcpv1.ToolOptions)
				checkTool(t, rpc, string(md.Name()), prefix, tool, tools)
				descriptions = append(descriptions, tool.GetDescription())
			}
		}
		return true
	})
	if len(tools) == 0 {
		t.Fatal("no tools found")
	}
	for rpc := range privateRPCs {
		if !rpcs[rpc] {
			t.Errorf("privateRPCs names %s, which does not exist", rpc)
		}
	}
	for _, d := range descriptions {
		for _, name := range toolMention.FindAllString(d, -1) {
			if _, ok := tools[name]; !ok {
				t.Errorf("description mentions unknown tool %s: %q", name, d)
			}
		}
	}

	if got, want := listRegisteredTools(t), slices.Sorted(maps.Keys(tools)); !slices.Equal(got, want) {
		t.Errorf("registered tools differ from annotations:\ngot  %v\nwant %v", got, want)
	}
}

func checkTool(t *testing.T, rpc, method, prefix string, tool *protomcpv1.ToolOptions, seen map[string]string) {
	t.Helper()
	name := prefix + tool.GetName()
	if tool.GetName() == "" || !toolName.MatchString(name) {
		t.Errorf("%s: tool name %q does not match %s", rpc, name, toolName)
	}
	if prev, dup := seen[name]; dup {
		t.Errorf("%s: tool name %q already used by %s", rpc, name, prev)
	}
	seen[name] = rpc
	if n := len(strings.Fields(tool.GetTitle())); n < 2 || n > 4 {
		t.Errorf("%s: title %q has %d words, want 2 to 4", rpc, tool.GetTitle(), n)
	}
	if n := len(strings.Fields(tool.GetDescription())); n == 0 || n > 60 {
		t.Errorf("%s: description has %d words, want 1 to 60", rpc, n)
	}
	got := hints{tool.GetReadOnly(), tool.GetIdempotent(), tool.GetDestructive()}
	for _, r := range hintRules {
		if r.rpc.MatchString(method) {
			if got != r.want {
				t.Errorf("%s: hints %+v, want %+v", rpc, got, r.want)
			}
			return
		}
	}
	t.Errorf("%s: no hint rule matches; add one to hintRules", rpc)
}

// listRegisteredTools runs every generated registrar and lists the tools over MCP.
// Clients are nil because no tool is called.
func listRegisteredTools(t *testing.T) []string {
	t.Helper()
	srv := protomcp.New("plusultra-test", "0")
	pb.RegisterAuthServiceMCPTools(srv, nil)
	pb.RegisterApiKeyServiceMCPTools(srv, nil)
	pb.RegisterProgramServiceMCPTools(srv, nil)
	pb.RegisterCalendarServiceMCPTools(srv, nil)
	pb.RegisterWorkoutServiceMCPTools(srv, nil)
	pb.RegisterMealServiceMCPTools(srv, nil)
	pb.RegisterCheckinServiceMCPTools(srv, nil)
	pb.RegisterExerciseServiceMCPTools(srv, nil)
	pb.RegisterFoodServiceMCPTools(srv, nil)
	pb.RegisterInsightServiceMCPTools(srv, nil)
	pb.RegisterExportServiceMCPTools(srv, nil)
	pb.RegisterReferenceServiceMCPTools(srv, nil)

	ctx := t.Context()
	clientT, serverT := mcp.NewInMemoryTransports()
	if _, err := srv.SDK().Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	var names []string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}
