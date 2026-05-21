package config

import (
	"testing"

	"google.golang.org/grpc/codes"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// seedFixture creates: an agent, a skill, two connectors (one
// static-token with a per-user token; one OAuth without one). Returns
// the worker context bound to a run owned by user.
func seedFixture(t *testing.T, svc *Service, sec *fakeSecrets, runOwners map[string]string, user, runID string) {
	t.Helper()
	owner := ctxAs(verifiers.SchemeUser, user)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")

	if _, err := svc.CreateAgent(owner, &nsv1.CreateAgentRequest{
		UserId: user, Name: "summarizer", Prompt: "summarize",
	}); err != nil {
		t.Fatalf("agent: %v", err)
	}
	if _, err := svc.CreateSkill(owner, &nsv1.CreateSkillRequest{
		UserId: user, Name: "skill", Content: "skill body",
	}); err != nil {
		t.Fatalf("skill: %v", err)
	}
	if _, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:            "notion",
		McpUrl:          "https://mcp.notion.com/sse",
		AuthType:        nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
		McpAllowedTools: []string{"pages.*"},
	}); err != nil {
		t.Fatalf("notion: %v", err)
	}
	if _, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:            "github",
		McpUrl:          "https://api.githubcopilot.com/mcp",
		AuthType:        nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
		McpAllowedTools: []string{"repos.*"},
	}); err != nil {
		t.Fatalf("github: %v", err)
	}
	if _, err := svc.SetConnectorStaticToken(owner, &nsv1.SetConnectorStaticTokenRequest{
		UserId: user, ConnectorName: "notion", Token: "ntn_xxx",
	}); err != nil {
		t.Fatalf("token: %v", err)
	}
	runOwners[runID] = user
}

func TestResolver_HappyPath(t *testing.T) {
	svc, sec, runOwners := newTestService(t)
	seedFixture(t, svc, sec, runOwners, "alice", "run-1")

	worker := verifiers.WithPrincipal(t.Context(), &verifiers.Principal{
		Scheme: verifiers.SchemeWorker, ID: "run-1", RunID: "run-1",
	})
	resp, err := svc.GetUserConfig(worker, &nsv1.GetUserConfigRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	cfg := resp.GetConfig()
	if got := len(cfg.GetAgents()); got != 1 {
		t.Fatalf("agents=%d", got)
	}
	if got := len(cfg.GetSkills()); got != 1 {
		t.Fatalf("skills=%d", got)
	}
	// notion is connected; github (OAuth) is skipped.
	if _, ok := cfg.GetMcpServers()["notion"]; !ok {
		t.Fatalf("expected notion in mcp_servers, got %v", cfg.GetMcpServers())
	}
	if _, ok := cfg.GetMcpServers()["github"]; ok {
		t.Fatalf("OAuth connector should be skipped without credential")
	}
	notion := cfg.GetMcpServers()["notion"]
	if notion.GetTransport() != nsv1.McpTransport_MCP_TRANSPORT_SSE {
		t.Fatalf("transport=%v", notion.GetTransport())
	}
	if got := notion.GetHeaders()["Authorization"]; got != "Bearer ntn_xxx" {
		t.Fatalf("auth header=%q", got)
	}
	if len(cfg.GetAllowedMcpTools()) != 1 || cfg.GetAllowedMcpTools()[0] != "pages.*" {
		t.Fatalf("allowed_mcp_tools=%v", cfg.GetAllowedMcpTools())
	}
}

func TestResolver_RequiresWorkerScheme(t *testing.T) {
	svc, _, _ := newTestService(t)
	user := ctxAs(verifiers.SchemeUser, "alice")
	_, err := svc.GetUserConfig(user, &nsv1.GetUserConfigRequest{UserId: "alice"})
	mustCode(t, err, codes.PermissionDenied)
}

func TestResolver_RunOwnerMismatch(t *testing.T) {
	svc, sec, runOwners := newTestService(t)
	seedFixture(t, svc, sec, runOwners, "alice", "run-1")
	worker := verifiers.WithPrincipal(t.Context(), &verifiers.Principal{
		Scheme: verifiers.SchemeWorker, ID: "run-1", RunID: "run-1",
	})
	_, err := svc.GetUserConfig(worker, &nsv1.GetUserConfigRequest{UserId: "bob"})
	mustCode(t, err, codes.PermissionDenied)
}

func TestResolver_OAuthConnectorWithToken(t *testing.T) {
	svc, _, runOwners, disp := newTestServiceWithOAuth(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	if _, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:            "github",
		McpUrl:          "https://api.githubcopilot.com/mcp",
		AuthType:        nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
		McpAllowedTools: []string{"repos.*"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Seed the dispenser with a credential as if the OAuth flow had
	// completed for alice. Bypasses Start/Complete since those are
	// covered separately.
	disp.creds[oauthCredName("alice", "github")] = "ghp_xxx"
	runOwners["run-1"] = "alice"
	worker := verifiers.WithPrincipal(t.Context(), &verifiers.Principal{
		Scheme: verifiers.SchemeWorker, ID: "run-1", RunID: "run-1",
	})
	resp, err := svc.GetUserConfig(worker, &nsv1.GetUserConfigRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	g, ok := resp.GetConfig().GetMcpServers()["github"]
	if !ok {
		t.Fatalf("github missing: %v", resp.GetConfig().GetMcpServers())
	}
	if got := g.GetHeaders()["Authorization"]; got != "Bearer ghp_xxx" {
		t.Fatalf("auth header=%q", got)
	}
}

// TestResolver_DisallowedMcpToolsUnion pins that
// `mcp_disallowed_tools` flows through GetUserConfig, gets dedup'd
// across connectors, and only contributes when the connector has a
// stored credential — same posture as `mcp_allowed_tools` (the
// resolver should not advertise a deny for a connector the user
// hasn't actually connected).
func TestResolver_DisallowedMcpToolsUnion(t *testing.T) {
	svc, sec, runOwners := newTestService(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	owner := ctxAs(verifiers.SchemeUser, "alice")
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:               "dropbox",
		McpUrl:             "https://mcp.dropbox.com/mcp",
		AuthType:           nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
		McpDisallowedTools: []string{"mcp__dropbox__list_folder", "shared_tool"},
	})
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:               "notion",
		McpUrl:             "https://mcp.notion.com/sse",
		AuthType:           nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
		McpDisallowedTools: []string{"shared_tool", "mcp__notion__bad_tool"},
	})
	// Connector that the user never connected — its denies must NOT
	// surface (parity with allowed_tools handling).
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:               "github",
		McpUrl:             "https://api.githubcopilot.com/mcp",
		AuthType:           nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
		McpDisallowedTools: []string{"mcp__github__never"},
	})
	if _, err := svc.SetConnectorStaticToken(owner, &nsv1.SetConnectorStaticTokenRequest{
		UserId: "alice", ConnectorName: "dropbox", Token: "tok-d",
	}); err != nil {
		t.Fatalf("dropbox token: %v", err)
	}
	if _, err := svc.SetConnectorStaticToken(owner, &nsv1.SetConnectorStaticTokenRequest{
		UserId: "alice", ConnectorName: "notion", Token: "tok-n",
	}); err != nil {
		t.Fatalf("notion token: %v", err)
	}
	runOwners["run-1"] = "alice"
	_ = sec

	worker := verifiers.WithPrincipal(t.Context(), &verifiers.Principal{
		Scheme: verifiers.SchemeWorker, ID: "run-1", RunID: "run-1",
	})
	resp, err := svc.GetUserConfig(worker, &nsv1.GetUserConfigRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	got := resp.GetConfig().GetDisallowedMcpTools()

	want := map[string]bool{
		"mcp__dropbox__list_folder": true,
		"shared_tool":               true,
		"mcp__notion__bad_tool":     true,
	}
	if len(got) != len(want) {
		t.Fatalf("disallowed=%v want=%v (size mismatch)", got, want)
	}
	for _, t2 := range got {
		if !want[t2] {
			t.Fatalf("unexpected disallowed entry %q (got=%v)", t2, got)
		}
	}
	for _, t2 := range got {
		if t2 == "mcp__github__never" {
			t.Fatalf("github deny leaked despite no credential: %v", got)
		}
	}
}

func TestResolver_OAuthConnectorWithoutTokenSkipped(t *testing.T) {
	svc, _, runOwners, _ := newTestServiceWithOAuth(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name: "github", McpUrl: "https://api.githubcopilot.com/mcp",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
	})
	runOwners["run-1"] = "alice"
	worker := verifiers.WithPrincipal(t.Context(), &verifiers.Principal{
		Scheme: verifiers.SchemeWorker, ID: "run-1", RunID: "run-1",
	})
	resp, err := svc.GetUserConfig(worker, &nsv1.GetUserConfigRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if _, ok := resp.GetConfig().GetMcpServers()["github"]; ok {
		t.Fatalf("expected github skipped, got %v", resp.GetConfig().GetMcpServers())
	}
}

func TestTransportFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want nsv1.McpTransport
	}{
		{"https://x/sse", nsv1.McpTransport_MCP_TRANSPORT_SSE},
		{"https://x/sse/", nsv1.McpTransport_MCP_TRANSPORT_SSE},
		{"https://x/mcp", nsv1.McpTransport_MCP_TRANSPORT_HTTP},
		{"https://x", nsv1.McpTransport_MCP_TRANSPORT_HTTP},
	}
	for _, tc := range cases {
		if got := transportFromURL(tc.url); got != tc.want {
			t.Errorf("%s: got=%v want=%v", tc.url, got, tc.want)
		}
	}
}
