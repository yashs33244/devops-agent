package config

import (
	"testing"

	"google.golang.org/grpc/codes"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

func TestConnector_AdminGate(t *testing.T) {
	svc, _, _ := newTestService(t)

	// Non-admin user can't create.
	user := ctxAs(verifiers.SchemeUser, "alice")
	_, err := svc.CreateConnector(user, &nsv1.CreateConnectorRequest{
		Name:     "notion",
		McpUrl:   "https://mcp.notion.com/sse",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
	})
	mustCode(t, err, codes.PermissionDenied)

	// Admin via group works.
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	cr, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:     "notion",
		McpUrl:   "https://mcp.notion.com/sse",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if cr.GetConnector().GetId() == "" {
		t.Fatalf("id empty")
	}

	// Admin via static-token name works.
	cliAdmin := verifiers.WithPrincipal(t.Context(), &verifiers.Principal{
		Scheme: verifiers.SchemeService, ID: "cli-admin",
	})
	if _, err := svc.CreateConnector(cliAdmin, &nsv1.CreateConnectorRequest{
		Name:     "github",
		McpUrl:   "https://api.githubcopilot.com/mcp",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
	}); err != nil {
		t.Fatalf("cli-admin create: %v", err)
	}
}

func TestConnector_StaticTokenSetGetDisconnect(t *testing.T) {
	svc, sec, _ := newTestService(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	if _, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:     "notion",
		McpUrl:   "https://mcp.notion.com/sse",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	alice := ctxAs(verifiers.SchemeUser, "alice")
	if _, err := svc.SetConnectorStaticToken(alice, &nsv1.SetConnectorStaticTokenRequest{
		UserId: "alice", ConnectorName: "notion", Token: "ntn_xxx",
	}); err != nil {
		t.Fatalf("set token: %v", err)
	}
	kv, err := sec.Get(t.Context(), tokenPath("alice", "notion"))
	if err != nil {
		t.Fatalf("secrets get: %v", err)
	}
	if kv["access_token"] != "ntn_xxx" {
		t.Fatalf("kv=%v", kv)
	}

	// Bob can't see Alice's connect — and can't set tokens for Alice.
	bob := ctxAs(verifiers.SchemeUser, "bob")
	_, err = svc.SetConnectorStaticToken(bob, &nsv1.SetConnectorStaticTokenRequest{
		UserId: "alice", ConnectorName: "notion", Token: "x",
	})
	mustCode(t, err, codes.PermissionDenied)

	// ListConnectors with user_id reflects connected state.
	lr, err := svc.ListConnectors(alice, &nsv1.ListConnectorsRequest{UserId: "alice"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lr.GetEntries()) != 1 || !lr.GetEntries()[0].GetConnected() {
		t.Fatalf("entries=%v", lr.GetEntries())
	}

	// Disconnect drops the credential.
	if _, err := svc.DisconnectConnector(alice, &nsv1.DisconnectConnectorRequest{
		UserId: "alice", ConnectorName: "notion",
	}); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if _, err := sec.Get(t.Context(), tokenPath("alice", "notion")); err == nil {
		t.Fatalf("expected ErrNotFound after disconnect")
	}
}

func TestConnector_OAuthDisabledWhenNoDispenser(t *testing.T) {
	svc, _, _ := newTestService(t)
	alice := ctxAs(verifiers.SchemeUser, "alice")
	_, err := svc.StartConnectorOAuthFlow(alice, &nsv1.StartConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "github", RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.Unimplemented)
	_, err = svc.CompleteConnectorOAuthFlow(alice, &nsv1.CompleteConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "github", Code: "x", State: "y", RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.Unimplemented)
}

func TestConnector_OAuthFlowHappyPath(t *testing.T) {
	svc, _, _, disp := newTestServiceWithOAuth(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	if _, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:        "github",
		McpUrl:      "https://api.githubcopilot.com/mcp",
		AuthType:    nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
		OauthScopes: "repo,read:user",
	}); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	alice := ctxAs(verifiers.SchemeUser, "alice")
	startResp, err := svc.StartConnectorOAuthFlow(alice, &nsv1.StartConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "github", RedirectUrl: "http://app/cb",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if startResp.GetAuthorizeUrl() == "" || startResp.GetState() == "" {
		t.Fatalf("missing url/state: %+v", startResp)
	}

	// Complete with that state. Dispenser should record an exchange.
	if _, err := svc.CompleteConnectorOAuthFlow(alice, &nsv1.CompleteConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "github",
		Code: "auth-code-X", State: startResp.GetState(),
		RedirectUrl: "http://app/cb",
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if disp.creds["alice-github"] == "" {
		t.Fatalf("expected stored credential, got %v", disp.creds)
	}
}

func TestConnector_OAuthCompleteRejectsTamperedState(t *testing.T) {
	svc, _, _, _ := newTestServiceWithOAuth(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name: "github", McpUrl: "https://api.githubcopilot.com/mcp",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
	})
	alice := ctxAs(verifiers.SchemeUser, "alice")
	_, err := svc.CompleteConnectorOAuthFlow(alice, &nsv1.CompleteConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "github",
		Code: "x", State: "tampered.deadbeef",
		RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.FailedPrecondition)
}

func TestConnector_OAuthStateBoundToUserAndConnector(t *testing.T) {
	svc, _, _, _ := newTestServiceWithOAuth(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name: "github", McpUrl: "https://api.githubcopilot.com/mcp",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
	})
	_, _ = svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name: "linear", McpUrl: "https://mcp.linear.app/sse",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
	})
	alice := ctxAs(verifiers.SchemeUser, "alice")
	startResp, _ := svc.StartConnectorOAuthFlow(alice, &nsv1.StartConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "github", RedirectUrl: "http://app/cb",
	})
	// Try to use alice's github state to complete linear → reject.
	_, err := svc.CompleteConnectorOAuthFlow(alice, &nsv1.CompleteConnectorOAuthFlowRequest{
		UserId: "alice", ConnectorName: "linear",
		Code: "x", State: startResp.GetState(),
		RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.FailedPrecondition)

	// Try to use alice's state for bob → reject.
	bob := ctxAs(verifiers.SchemeUser, "bob")
	_, err = svc.CompleteConnectorOAuthFlow(bob, &nsv1.CompleteConnectorOAuthFlowRequest{
		UserId: "bob", ConnectorName: "github",
		Code: "x", State: startResp.GetState(),
		RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.FailedPrecondition)
}

func TestConnector_DeleteCascadesCreds(t *testing.T) {
	svc, sec, _ := newTestService(t)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	cr, err := svc.CreateConnector(admin, &nsv1.CreateConnectorRequest{
		Name:     "notion",
		McpUrl:   "https://mcp.notion.com/sse",
		AuthType: nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, u := range []string{"alice", "bob"} {
		ctx := ctxAs(verifiers.SchemeUser, u)
		if _, err := svc.SetConnectorStaticToken(ctx, &nsv1.SetConnectorStaticTokenRequest{
			UserId: u, ConnectorName: "notion", Token: "tok",
		}); err != nil {
			t.Fatalf("seed token %s: %v", u, err)
		}
	}
	if _, err := svc.DeleteConnector(admin, &nsv1.DeleteConnectorRequest{ConnectorId: cr.GetConnector().GetId()}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, u := range []string{"alice", "bob"} {
		if _, err := sec.Get(t.Context(), tokenPath(u, "notion")); err == nil {
			t.Fatalf("expected cascade-delete to remove %s/notion", u)
		}
	}
}
