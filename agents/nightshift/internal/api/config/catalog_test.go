package config

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// The default catalog ships with a small set of reference connectors
// (currently just motherduck) so that a fresh deployment is usable
// out of the box and contributors have a worked example to copy.
// This test pins that contract: loading with no override path
// succeeds and yields the embedded reference set.
func TestCatalog_DefaultLoadsReference(t *testing.T) {
	cf, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("default load: %v", err)
	}
	if len(cf.Connectors) == 0 {
		t.Fatal("expected default catalog to contain reference connectors, got 0")
	}
	var foundMotherduck bool
	for _, c := range cf.Connectors {
		if c.Name == "motherduck" {
			foundMotherduck = true
			break
		}
	}
	if !foundMotherduck {
		t.Fatalf("default catalog missing motherduck reference; got %+v", cf.Connectors)
	}
}

func TestCatalog_ReconcileCreatesMissing(t *testing.T) {
	svc, _, _ := newTestService(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	if err := os.WriteFile(path, []byte(`
connectors:
  - name: notion
    description: Notion
    mcp_url: https://mcp.notion.com/sse
    auth_type: static_token
    mcp_allowed_tools: ["pages.*"]
  - name: github
    description: GitHub
    mcp_url: https://api.githubcopilot.com/mcp
    auth_type: oauth
    oauth_scopes: "repo"
`), 0644); err != nil {
		t.Fatal(err)
	}
	cf, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	created, updated, err := svc.ReconcileCatalog(t.Context(), cf)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if created != 2 || updated != 0 {
		t.Fatalf("first-run created=%d updated=%d want created=2 updated=0", created, updated)
	}

	// Re-running with no catalog changes is a true no-op — neither
	// new records nor drift-back updates.
	created2, updated2, err := svc.ReconcileCatalog(t.Context(), cf)
	if err != nil {
		t.Fatalf("reconcile second: %v", err)
	}
	if created2 != 0 || updated2 != 0 {
		t.Fatalf("second-run created=%d updated=%d want both zero", created2, updated2)
	}

	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	lr, err := svc.ListConnectors(admin, &nsv1.ListConnectorsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lr.GetEntries()) != 2 {
		t.Fatalf("entries=%d want=2", len(lr.GetEntries()))
	}
}

// TestCatalog_ReconcileDriftBack pins the contract that catalog-managed
// fields on existing connector records track the catalog on every
// reconcile (mirrors cr0n's `update_connector_catalog_fields` —
// without it, scope/url/disallow edits get permanently stranded on
// rows that predate the change).
func TestCatalog_ReconcileDriftBack(t *testing.T) {
	svc, _, _ := newTestService(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	body := []byte(`
connectors:
  - name: dropbox
    description: Dropbox
    mcp_url: https://mcp.dropbox.com/mcp
    auth_type: oauth
    oauth_scopes: "files.metadata.read"
`)
	_ = os.WriteFile(path, body, 0644)
	cf, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if created, _, err := svc.ReconcileCatalog(t.Context(), cf); err != nil || created != 1 {
		t.Fatalf("seed reconcile: created=%d err=%v", created, err)
	}

	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	lr, err := svc.ListConnectors(admin, &nsv1.ListConnectorsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lr.GetEntries()) != 1 {
		t.Fatalf("entries=%d want=1", len(lr.GetEntries()))
	}
	originalID := lr.GetEntries()[0].GetConnector().GetId()
	originalCreated := lr.GetEntries()[0].GetConnector().GetCreatedAt().AsTime()

	// Mutate the catalog: broaden scopes, add a disallow list,
	// change description.
	body2 := []byte(`
connectors:
  - name: dropbox
    description: Dropbox — list, fetch, write, and share files
    mcp_url: https://mcp.dropbox.com/mcp
    auth_type: oauth
    oauth_scopes: "files.metadata.read sharing.read"
    mcp_disallowed_tools: ["mcp__dropbox__list_folder"]
`)
	_ = os.WriteFile(path, body2, 0644)
	cf2, err := LoadCatalog(path)
	if err != nil {
		t.Fatalf("load2: %v", err)
	}
	created, updated, err := svc.ReconcileCatalog(t.Context(), cf2)
	if err != nil {
		t.Fatalf("drift reconcile: %v", err)
	}
	if created != 0 || updated != 1 {
		t.Fatalf("drift reconcile created=%d updated=%d want created=0 updated=1", created, updated)
	}

	// New fields propagated; identity preserved.
	lr2, _ := svc.ListConnectors(admin, &nsv1.ListConnectorsRequest{})
	if len(lr2.GetEntries()) != 1 {
		t.Fatalf("entries=%d want=1", len(lr2.GetEntries()))
	}
	got := lr2.GetEntries()[0].GetConnector()
	if got.GetId() != originalID {
		t.Fatalf("Id changed: %q -> %q", originalID, got.GetId())
	}
	if !got.GetCreatedAt().AsTime().Equal(originalCreated) {
		t.Fatalf("CreatedAt changed: %v -> %v", originalCreated, got.GetCreatedAt().AsTime())
	}
	if got.GetDescription() != "Dropbox — list, fetch, write, and share files" {
		t.Fatalf("Description not refreshed: %q", got.GetDescription())
	}
	if got.GetOauthScopes() != "files.metadata.read sharing.read" {
		t.Fatalf("OauthScopes not refreshed: %q", got.GetOauthScopes())
	}
	if dis := got.GetMcpDisallowedTools(); len(dis) != 1 || dis[0] != "mcp__dropbox__list_folder" {
		t.Fatalf("McpDisallowedTools not refreshed: %v", dis)
	}

	// Reconciling a third time with no further changes must be a
	// true no-op — confirms diff-then-write (no UpdatedAt churn).
	created3, updated3, err := svc.ReconcileCatalog(t.Context(), cf2)
	if err != nil {
		t.Fatalf("idempotent reconcile: %v", err)
	}
	if created3 != 0 || updated3 != 0 {
		t.Fatalf("third-run created=%d updated=%d want both zero", created3, updated3)
	}
}

func TestCatalog_OAuthProviderRegistered(t *testing.T) {
	svc, sec, _, disp := newTestServiceWithOAuth(t)
	// Seed admin OAuth credentials at the well-known path before
	// reconcile runs.
	if err := sec.Put(t.Context(), connectorAdminPath("github"), map[string]string{
		"client_id":     "cid",
		"client_secret": "csec",
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	if err := os.WriteFile(path, []byte(`
connectors:
  - name: github
    mcp_url: https://api.githubcopilot.com/mcp
    auth_type: oauth
    oauth_scopes: "repo,read:user"
    oauth:
      provider: github
`), 0644); err != nil {
		t.Fatal(err)
	}
	cf, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ReconcileCatalog(t.Context(), cf); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// Dispenser should have the github server registered with the
	// admin credentials read from KV.
	got, ok := disp.servers["github"]
	if !ok {
		t.Fatalf("github not registered, calls=%v", disp.calls)
	}
	if got.ClientID != "cid" || got.ClientSecret != "csec" {
		t.Fatalf("server cfg=%+v", got)
	}
}

func TestCatalog_OAuthProviderSkippedWithoutAdminCreds(t *testing.T) {
	svc, _, _, disp := newTestServiceWithOAuth(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	_ = os.WriteFile(path, []byte(`
connectors:
  - name: github
    mcp_url: https://api.githubcopilot.com/mcp
    auth_type: oauth
    oauth:
      provider: github
`), 0644)
	cf, _ := LoadCatalog(path)
	if _, _, err := svc.ReconcileCatalog(t.Context(), cf); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, ok := disp.servers["github"]; ok {
		t.Fatalf("github should not be registered without admin creds")
	}
}

func TestCatalog_RejectsInvalidEntries(t *testing.T) {
	svc, _, _ := newTestService(t)
	cases := []struct {
		body string
		want codes.Code // 0 = expect plain error (not gRPC)
	}{
		{`connectors: [{name: "BAD!", mcp_url: x, auth_type: oauth}]`, 0},
		{`connectors: [{name: ok, mcp_url: "", auth_type: oauth}]`, 0},
		{`connectors: [{name: ok, mcp_url: x, auth_type: weird}]`, 0},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "c.yaml")
		_ = os.WriteFile(path, []byte(tc.body), 0644)
		cf, err := LoadCatalog(path)
		if err != nil {
			continue
		}
		if _, _, err := svc.ReconcileCatalog(t.Context(), cf); err == nil {
			t.Fatalf("expected reconcile error on %q", tc.body)
		}
	}
}
