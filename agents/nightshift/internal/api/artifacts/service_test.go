package artifacts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/objects/objectstest"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

const testBucket = "artifacts"

// fakeRunLookup maps run_id -> (user_id, session_id) for SchemeWorker
// tests. Returns records.ErrNotFound on miss so callerUserID surfaces
// NOT_FOUND. Plain string values (no session_id) are treated as
// {user_id: v, session_id: ""}.
type fakeRunLookup map[string]fakeRunMeta

type fakeRunMeta struct {
	userID    string
	sessionID string
}

func (m fakeRunLookup) LookupRunOwner(_ context.Context, runID string) (string, string, error) {
	if r, ok := m[runID]; ok {
		return r.userID, r.sessionID, nil
	}
	return "", "", records.ErrNotFound
}

func newTestService(t *testing.T, runs RunLookup) *Service {
	t.Helper()
	return newTestServiceWithDeployer(t, runs, nil)
}

// fakeAppDeployer records spec writes in memory + lets tests inspect
// the calls made by the Service. Implements runtime.AppDeployer.
type fakeAppDeployer struct {
	mu       sync.Mutex
	specs    map[string]runtime.AppSpec
	deployed []runtime.AppSpec
	updated  []runtime.AppSpec
	deleted  []string
}

func newFakeAppDeployer() *fakeAppDeployer {
	return &fakeAppDeployer{specs: map[string]runtime.AppSpec{}}
}

func (f *fakeAppDeployer) Deploy(_ context.Context, spec runtime.AppSpec) (runtime.AppDeployResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs[spec.ArtifactID] = spec
	f.deployed = append(f.deployed, spec)
	return runtime.AppDeployResult{ServiceURL: "http://fake/" + spec.ArtifactID}, nil
}

func (f *fakeAppDeployer) Update(_ context.Context, spec runtime.AppSpec) (runtime.AppDeployResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs[spec.ArtifactID] = spec
	f.updated = append(f.updated, spec)
	return runtime.AppDeployResult{ServiceURL: "http://fake/" + spec.ArtifactID}, nil
}

func (f *fakeAppDeployer) Delete(_ context.Context, artifactID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.specs, artifactID)
	f.deleted = append(f.deleted, artifactID)
	return nil
}

func (f *fakeAppDeployer) Close() error { return nil }

func newTestServiceWithDeployer(t *testing.T, runs RunLookup, deployer runtime.AppDeployer) *Service {
	t.Helper()
	rec, err := records.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec.Close() })

	obj := objectstest.New(t)

	return NewService(ServiceOptions{
		Records:     rec,
		Objects:     obj,
		Bucket:      testBucket,
		Runs:        runs,
		AppDeployer: deployer,
	})
}

func userCtx(userID string) context.Context {
	return verifiers.WithPrincipal(context.Background(), &verifiers.Principal{Scheme: verifiers.SchemeUser, ID: userID})
}

func workerCtx(runID string) context.Context {
	return verifiers.WithPrincipal(context.Background(), &verifiers.Principal{Scheme: verifiers.SchemeWorker, ID: runID, RunID: runID})
}

func mustCreate(t *testing.T, s *Service, ctx context.Context, name string, content []byte) *nsv1.Artifact {
	t.Helper()
	resp, err := s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:        name,
		Content:     content,
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("CreateObjectArtifact: %v", err)
	}
	return resp.GetArtifact()
}

func TestCreateGetObjectArtifact(t *testing.T) {
	s := newTestService(t, nil)
	ctx := userCtx("alice")

	art := mustCreate(t, s, ctx, "hello.txt", []byte("hello world"))
	if art.GetId() == "" {
		t.Fatal("no id assigned")
	}
	if art.GetOwnerId() != "alice" {
		t.Fatalf("owner=%q", art.GetOwnerId())
	}
	if art.GetSizeBytes() != int64(len("hello world")) {
		t.Fatalf("size=%d", art.GetSizeBytes())
	}

	got, err := s.GetArtifact(ctx, &nsv1.GetArtifactRequest{ArtifactId: art.GetId()})
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if got.GetArtifact().GetName() != "hello.txt" {
		t.Fatalf("name=%q", got.GetArtifact().GetName())
	}
}

func TestGetArtifact_NotFoundForStranger(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "secret.txt", []byte("ssh"))

	_, err := s.GetArtifact(userCtx("eve"), &nsv1.GetArtifactRequest{ArtifactId: art.GetId()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestGetArtifact_PublicVisibleToAuthenticated(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "report.pdf", []byte("data"))
	if _, err := s.UpdateArtifact(userCtx("alice"), &nsv1.UpdateArtifactRequest{
		ArtifactId: art.GetId(),
		Public:     ptrBool(true),
	}); err != nil {
		t.Fatalf("UpdateArtifact: %v", err)
	}
	if _, err := s.GetArtifact(userCtx("eve"), &nsv1.GetArtifactRequest{ArtifactId: art.GetId()}); err != nil {
		t.Fatalf("public Get from stranger: %v", err)
	}
}

func TestUpdateArtifact_RBACMatrix(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "doc.txt", []byte("v1"))

	// Stranger can't update.
	_, err := s.UpdateArtifact(userCtx("eve"), &nsv1.UpdateArtifactRequest{
		ArtifactId: art.GetId(),
		Name:       ptrString("hacked.txt"),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("stranger update: want NotFound, got %v", err)
	}

	// Grant viewer to bob.
	if _, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: art.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_VIEWER,
	}); err != nil {
		t.Fatalf("share viewer: %v", err)
	}

	// Viewer can't update.
	_, err = s.UpdateArtifact(userCtx("bob"), &nsv1.UpdateArtifactRequest{
		ArtifactId: art.GetId(),
		Name:       ptrString("renamed.txt"),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("viewer update: want PermissionDenied, got %v", err)
	}

	// Promote bob to editor.
	if _, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: art.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_EDITOR,
	}); err != nil {
		t.Fatalf("promote editor: %v", err)
	}

	// Editor can update name + description.
	if _, err := s.UpdateArtifact(userCtx("bob"), &nsv1.UpdateArtifactRequest{
		ArtifactId:  art.GetId(),
		Description: ptrString("updated by bob"),
	}); err != nil {
		t.Fatalf("editor update: %v", err)
	}

	// Editor can't toggle public.
	_, err = s.UpdateArtifact(userCtx("bob"), &nsv1.UpdateArtifactRequest{
		ArtifactId: art.GetId(),
		Public:     ptrBool(true),
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("editor publish: want PermissionDenied, got %v", err)
	}

	// Owner can.
	if _, err := s.UpdateArtifact(userCtx("alice"), &nsv1.UpdateArtifactRequest{
		ArtifactId: art.GetId(),
		Public:     ptrBool(true),
	}); err != nil {
		t.Fatalf("owner publish: %v", err)
	}
}

func TestDeleteArtifact_OwnerOnly(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "doomed.txt", []byte("x"))
	if _, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: art.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_EDITOR,
	}); err != nil {
		t.Fatal(err)
	}

	// Editor cannot delete.
	if _, err := s.DeleteArtifact(userCtx("bob"), &nsv1.DeleteArtifactRequest{ArtifactId: art.GetId()}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("editor delete: want PermissionDenied, got %v", err)
	}

	// Owner can.
	if _, err := s.DeleteArtifact(userCtx("alice"), &nsv1.DeleteArtifactRequest{ArtifactId: art.GetId()}); err != nil {
		t.Fatalf("owner delete: %v", err)
	}

	// Subsequent Get is NotFound.
	if _, err := s.GetArtifact(userCtx("alice"), &nsv1.GetArtifactRequest{ArtifactId: art.GetId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("after delete: want NotFound, got %v", err)
	}
}

func TestListArtifacts_OwnerScoped(t *testing.T) {
	s := newTestService(t, nil)
	mustCreate(t, s, userCtx("alice"), "a1.txt", []byte("a1"))
	mustCreate(t, s, userCtx("alice"), "a2.txt", []byte("a2"))
	mustCreate(t, s, userCtx("bob"), "b1.txt", []byte("b1"))

	list, err := s.ListArtifacts(userCtx("alice"), &nsv1.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.GetArtifacts()) != 2 {
		t.Fatalf("alice list len=%d, want 2", len(list.GetArtifacts()))
	}

	// Asking for bob's artifacts as alice collapses to empty (no leak).
	list, err = s.ListArtifacts(userCtx("alice"), &nsv1.ListArtifactsRequest{OwnerId: "bob"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.GetArtifacts()) != 0 {
		t.Fatalf("cross-owner list leaked %d", len(list.GetArtifacts()))
	}
}

func TestListArtifacts_IncludesSharedWithMe(t *testing.T) {
	s := newTestService(t, nil)

	owned := mustCreate(t, s, userCtx("bob"), "bob-owned.txt", []byte("bob"))
	shared := mustCreate(t, s, userCtx("alice"), "alice-shared.txt", []byte("alice"))
	private := mustCreate(t, s, userCtx("alice"), "alice-private.txt", []byte("private"))

	if _, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: shared.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_VIEWER,
	}); err != nil {
		t.Fatalf("share: %v", err)
	}

	list, err := s.ListArtifacts(userCtx("bob"), &nsv1.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	ids := make(map[string]bool, len(list.GetArtifacts()))
	for _, a := range list.GetArtifacts() {
		ids[a.GetId()] = true
	}
	if !ids[owned.GetId()] {
		t.Errorf("bob's own artifact %s missing from listing", owned.GetId())
	}
	if !ids[shared.GetId()] {
		t.Errorf("artifact shared with bob (%s) missing from listing", shared.GetId())
	}
	if ids[private.GetId()] {
		t.Errorf("alice's private artifact %s leaked into bob's listing", private.GetId())
	}
	if got := len(list.GetArtifacts()); got != 2 {
		t.Fatalf("listing len=%d, want 2 (owned + shared)", got)
	}

	// Revoking the grant removes it from the recipient's listing.
	if _, err := s.RevokeArtifactShare(userCtx("alice"), &nsv1.RevokeArtifactShareRequest{
		ArtifactId: shared.GetId(),
		UserId:     "bob",
	}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	list, err = s.ListArtifacts(userCtx("bob"), &nsv1.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("list after revoke: %v", err)
	}
	for _, a := range list.GetArtifacts() {
		if a.GetId() == shared.GetId() {
			t.Fatalf("revoked artifact %s still in bob's listing", shared.GetId())
		}
	}
}

func TestListArtifacts_TypeFilterAppliesToShared(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "shared.txt", []byte("v1"))
	if _, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: art.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_VIEWER,
	}); err != nil {
		t.Fatalf("share: %v", err)
	}

	// Object filter — shared object visible.
	list, err := s.ListArtifacts(userCtx("bob"), &nsv1.ListArtifactsRequest{
		TypeFilter: nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.GetArtifacts()) != 1 || list.GetArtifacts()[0].GetId() != art.GetId() {
		t.Fatalf("object filter: got %d artifacts, want shared object", len(list.GetArtifacts()))
	}

	// App filter — shared object is filtered out.
	list, err = s.ListArtifacts(userCtx("bob"), &nsv1.ListArtifactsRequest{
		TypeFilter: nsv1.ArtifactType_ARTIFACT_TYPE_APP,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.GetArtifacts()) != 0 {
		t.Fatalf("app filter leaked %d object artifact(s)", len(list.GetArtifacts()))
	}
}

func TestListArtifacts_Paginates(t *testing.T) {
	s := newTestService(t, nil)
	ctx := userCtx("alice")
	for i := 0; i < 5; i++ {
		mustCreate(t, s, ctx, "a.txt", []byte{byte('a' + i)})
	}

	first, err := s.ListArtifacts(ctx, &nsv1.ListArtifactsRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(first.GetArtifacts()) != 2 || first.GetNextPageToken() == "" {
		t.Fatalf("page1 len=%d token=%q", len(first.GetArtifacts()), first.GetNextPageToken())
	}
	second, err := s.ListArtifacts(ctx, &nsv1.ListArtifactsRequest{PageSize: 2, PageToken: first.GetNextPageToken()})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(second.GetArtifacts()) != 2 || second.GetNextPageToken() == "" {
		t.Fatalf("page2 len=%d token=%q", len(second.GetArtifacts()), second.GetNextPageToken())
	}
	third, err := s.ListArtifacts(ctx, &nsv1.ListArtifactsRequest{PageSize: 2, PageToken: second.GetNextPageToken()})
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if len(third.GetArtifacts()) != 1 || third.GetNextPageToken() != "" {
		t.Fatalf("page3 len=%d token=%q", len(third.GetArtifacts()), third.GetNextPageToken())
	}

	// No duplicates across pages.
	seen := make(map[string]bool)
	for _, page := range [][]*nsv1.Artifact{first.GetArtifacts(), second.GetArtifacts(), third.GetArtifacts()} {
		for _, a := range page {
			if seen[a.GetId()] {
				t.Fatalf("duplicate artifact across pages: %s", a.GetId())
			}
			seen[a.GetId()] = true
		}
	}
}

func TestIdempotency_ReplayAndConflict(t *testing.T) {
	s := newTestService(t, nil)
	ctx := userCtx("alice")

	first, err := s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:           "idem.txt",
		Content:        []byte("v1"),
		ContentType:    "text/plain",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}

	// Replay with same content → same artifact id.
	second, err := s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:           "idem.txt",
		Content:        []byte("v1"),
		ContentType:    "text/plain",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if second.GetArtifact().GetId() != first.GetArtifact().GetId() {
		t.Fatalf("replay returned different id: %s vs %s", second.GetArtifact().GetId(), first.GetArtifact().GetId())
	}

	// Replay with different content → INVALID_ARGUMENT.
	_, err = s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:           "idem.txt",
		Content:        []byte("v2-different"),
		ContentType:    "text/plain",
		IdempotencyKey: "k",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("conflicting replay: want InvalidArgument, got %v", err)
	}
}

func TestPreviewRoundTrip(t *testing.T) {
	s := newTestService(t, nil)
	ctx := userCtx("alice")
	resp, err := s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:        "report.pdf",
		Content:     []byte("%PDF-1.4 fake"),
		ContentType: "application/pdf",
		PreviewHtml: []byte("<html>preview</html>"),
	})
	if err != nil {
		t.Fatalf("create with preview: %v", err)
	}
	art := resp.GetArtifact()
	if !art.GetHasPreview() {
		t.Fatal("HasPreview not set")
	}
	url, err := s.GetArtifactPreviewURL(ctx, &nsv1.GetArtifactPreviewURLRequest{ArtifactId: art.GetId()})
	if err != nil {
		t.Fatalf("preview url: %v", err)
	}
	if url.GetPreviewUrl() == "" {
		t.Fatal("empty preview url")
	}

	// Clear the preview via Update.
	if _, err := s.UpdateArtifact(ctx, &nsv1.UpdateArtifactRequest{
		ArtifactId:     art.GetId(),
		SetPreviewHtml: true,
		PreviewHtml:    nil,
	}); err != nil {
		t.Fatalf("clear preview: %v", err)
	}
	if _, err := s.GetArtifactPreviewURL(ctx, &nsv1.GetArtifactPreviewURLRequest{ArtifactId: art.GetId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("after clear: want NotFound, got %v", err)
	}
}

func TestUpdateReplacesBlob(t *testing.T) {
	s := newTestService(t, nil)
	ctx := userCtx("alice")
	art := mustCreate(t, s, ctx, "f.txt", []byte("v1-short"))

	if _, err := s.UpdateArtifact(ctx, &nsv1.UpdateArtifactRequest{
		ArtifactId:   art.GetId(),
		ContentBytes: []byte("v2-much-longer"),
		ContentType:  "text/plain",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetArtifact(ctx, &nsv1.GetArtifactRequest{ArtifactId: art.GetId()})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetArtifact().GetSizeBytes() != int64(len("v2-much-longer")) {
		t.Fatalf("size after update=%d", got.GetArtifact().GetSizeBytes())
	}
}

func TestWorkerCredential_DerivesOwnerFromRun(t *testing.T) {
	runs := fakeRunLookup{"run-1": {userID: "alice"}}
	s := newTestService(t, runs)
	ctx := workerCtx("run-1")

	resp, err := s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:        "out.txt",
		Content:     []byte("agent output"),
		ContentType: "text/plain",
		// Worker forges a different user_id; should be ignored.
		OwnerId: "eve",
		RunId:   "ignored-by-server",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if resp.GetArtifact().GetOwnerId() != "alice" {
		t.Fatalf("owner=%q, want alice (worker forge ignored)", resp.GetArtifact().GetOwnerId())
	}
	if resp.GetArtifact().GetRunId() != "run-1" {
		t.Fatalf("run_id=%q, want run-1", resp.GetArtifact().GetRunId())
	}

	// User alice can see the artifact via GetArtifact even though she
	// never called Create directly.
	got, err := s.GetArtifact(userCtx("alice"), &nsv1.GetArtifactRequest{ArtifactId: resp.GetArtifact().GetId()})
	if err != nil {
		t.Fatalf("alice Get: %v", err)
	}
	if got.GetArtifact().GetId() != resp.GetArtifact().GetId() {
		t.Fatal("id mismatch")
	}
}

func TestWorkerCredential_ImpersonationRefused(t *testing.T) {
	s := newTestService(t, nil) // no RunLookup wired
	_, err := s.CreateObjectArtifact(workerCtx("run-x"), &nsv1.CreateObjectArtifactRequest{
		Name:        "x.txt",
		Content:     []byte("x"),
		ContentType: "text/plain",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("worker without RunLookup: want FailedPrecondition, got %v", err)
	}
}

func TestUserCallerCannotImpersonate(t *testing.T) {
	s := newTestService(t, nil)
	_, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "x.txt",
		Content:     []byte("x"),
		ContentType: "text/plain",
		OwnerId:     "bob",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("user setting other owner_id: want PermissionDenied, got %v", err)
	}
}

func TestShareUpsertReplacesRole(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "f.txt", []byte("x"))
	first, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: art.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_VIEWER,
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := s.ShareArtifact(userCtx("alice"), &nsv1.ShareArtifactRequest{
		ArtifactId: art.GetId(),
		UserId:     "bob",
		Role:       nsv1.ArtifactRole_ARTIFACT_ROLE_EDITOR,
	})
	if err != nil {
		t.Fatal(err)
	}
	// cr0n parity: same id retained on upsert.
	if first.GetPermission().GetId() != second.GetPermission().GetId() {
		t.Fatalf("upsert minted new id: %s vs %s", first.GetPermission().GetId(), second.GetPermission().GetId())
	}
	if second.GetPermission().GetRole() != nsv1.ArtifactRole_ARTIFACT_ROLE_EDITOR {
		t.Fatalf("upsert role=%v", second.GetPermission().GetRole())
	}

	list, err := s.ListArtifactPermissions(userCtx("alice"), &nsv1.ListArtifactPermissionsRequest{ArtifactId: art.GetId()})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetPermissions()) != 1 {
		t.Fatalf("perms=%d, want 1 (single grant after upsert)", len(list.GetPermissions()))
	}
}

func TestDownloadURLPermissionGated(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "f.txt", []byte("data"))

	if _, err := s.GetArtifactDownloadURL(userCtx("eve"), &nsv1.GetArtifactDownloadURLRequest{ArtifactId: art.GetId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("stranger download: want NotFound, got %v", err)
	}
	if _, err := s.GetArtifactDownloadURL(userCtx("alice"), &nsv1.GetArtifactDownloadURLRequest{ArtifactId: art.GetId()}); err != nil {
		t.Fatalf("owner download: %v", err)
	}
}

func TestRevokeIdempotent(t *testing.T) {
	s := newTestService(t, nil)
	art := mustCreate(t, s, userCtx("alice"), "f.txt", []byte("x"))
	// Revoke a never-granted user — no error.
	if _, err := s.RevokeArtifactShare(userCtx("alice"), &nsv1.RevokeArtifactShareRequest{
		ArtifactId: art.GetId(),
		UserId:     "carol",
	}); err != nil {
		t.Fatalf("revoke missing grant: %v", err)
	}
}

// ── chunk 16 — app artifacts ────────────────────────────────────────

func TestCreateAppArtifact_HappyPath(t *testing.T) {
	dep := newFakeAppDeployer()
	s := newTestServiceWithDeployer(t, nil, dep)
	ctx := userCtx("alice")

	resp, err := s.CreateAppArtifact(ctx, &nsv1.CreateAppArtifactRequest{
		Name:        "demo",
		HtmlContent: "<h1>hi</h1>",
	})
	if err != nil {
		t.Fatalf("CreateAppArtifact: %v", err)
	}
	art := resp.GetArtifact()
	if art.GetType() != nsv1.ArtifactType_ARTIFACT_TYPE_APP {
		t.Fatalf("type=%v", art.GetType())
	}
	if art.GetAppUrl() == "" {
		t.Fatal("app_url not set")
	}
	if art.GetAppStatus() != nsv1.DeploymentState_DEPLOYMENT_STATE_READY {
		t.Fatalf("status=%v", art.GetAppStatus())
	}
	if len(dep.deployed) != 1 || dep.deployed[0].ArtifactID != art.GetId() {
		t.Fatalf("deployer call: %+v", dep.deployed)
	}
	if dep.deployed[0].DownloadURL == "" {
		t.Fatal("DownloadURL not forwarded to deployer")
	}
}

func TestCreateAppArtifact_NoDeployerReturnsUnimplemented(t *testing.T) {
	s := newTestService(t, nil) // no deployer wired
	_, err := s.CreateAppArtifact(userCtx("alice"), &nsv1.CreateAppArtifactRequest{
		Name:        "demo",
		HtmlContent: "<h1>hi</h1>",
	})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("want Unimplemented, got %v", err)
	}
}

func TestUpdateAppArtifact_TriggersRedeploy(t *testing.T) {
	dep := newFakeAppDeployer()
	s := newTestServiceWithDeployer(t, nil, dep)
	ctx := userCtx("alice")

	create, err := s.CreateAppArtifact(ctx, &nsv1.CreateAppArtifactRequest{
		Name:        "demo",
		HtmlContent: "<h1>v1</h1>",
	})
	if err != nil {
		t.Fatal(err)
	}

	html := "<h1>v2</h1>"
	if _, err := s.UpdateArtifact(ctx, &nsv1.UpdateArtifactRequest{
		ArtifactId:  create.GetArtifact().GetId(),
		HtmlContent: &html,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(dep.updated) != 1 {
		t.Fatalf("update calls=%d, want 1", len(dep.updated))
	}
}

func TestDeleteAppArtifact_TearsDownK8s(t *testing.T) {
	dep := newFakeAppDeployer()
	s := newTestServiceWithDeployer(t, nil, dep)
	ctx := userCtx("alice")

	create, err := s.CreateAppArtifact(ctx, &nsv1.CreateAppArtifactRequest{
		Name:        "doomed",
		HtmlContent: "<h1>x</h1>",
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	if _, err := s.DeleteArtifact(ctx, &nsv1.DeleteArtifactRequest{ArtifactId: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(dep.deleted) != 1 || dep.deleted[0] != id {
		t.Fatalf("deleter calls: %+v", dep.deleted)
	}
}

func TestProxy_PublicSkipsAuth(t *testing.T) {
	dep := newFakeAppDeployer()
	s := newTestServiceWithDeployer(t, nil, dep)
	ctx := userCtx("alice")

	create, err := s.CreateAppArtifact(ctx, &nsv1.CreateAppArtifactRequest{
		Name:        "p",
		HtmlContent: "<h1>public</h1>",
		Public:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	// Replace AppUrl with a real httptest server we can reverse-proxy to.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("UPSTREAM"))
	}))
	t.Cleanup(upstream.Close)
	patchAppURL(t, s, id, upstream.URL)

	// Proxy with no auth header should still succeed (public=true).
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+"/view", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "UPSTREAM" {
		t.Fatalf("body=%q", body)
	}
}

func TestProxy_PrivateRequiresBearer(t *testing.T) {
	dep := newFakeAppDeployer()
	s := newTestServiceWithDeployer(t, nil, dep)
	ctx := userCtx("alice")

	create, err := s.CreateAppArtifact(ctx, &nsv1.CreateAppArtifactRequest{
		Name:        "secret",
		HtmlContent: "<h1>x</h1>",
		// public defaults false
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SHOULD-NOT-LEAK"))
	}))
	t.Cleanup(upstream.Close)
	patchAppURL(t, s, id, upstream.URL)

	// No bearer → 401 since verifiers aren't wired in this fixture.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+"/view", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestProxy_FallthroughForNonViewSuffix(t *testing.T) {
	s := newTestService(t, nil)
	called := false
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/abc/permissions", nil)
	s.ProxyHandler(fallback).ServeHTTP(rec, req)
	if !called {
		t.Fatal("fallback should have been invoked for non-/view suffix")
	}
}

// TestProxy_DownloadURL_PublicSkipsAuth covers issue #165: anonymous
// callers fetching `:downloadUrl` for a public object artifact should
// get a presigned URL JSON body, not a 401 from the gateway
// interceptor.
func TestProxy_DownloadURL_PublicSkipsAuth(t *testing.T) {
	s := newTestService(t, nil)
	create, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "report.pdf",
		Content:     []byte("PDF-BYTES"),
		ContentType: "application/pdf",
		Public:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+":downloadUrl", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "downloadUrl") {
		t.Fatalf("body lacks downloadUrl field: %s", body)
	}
}

func TestProxy_DownloadURL_PrivateRequiresBearer(t *testing.T) {
	s := newTestService(t, nil)
	create, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "secret.pdf",
		Content:     []byte("private"),
		ContentType: "application/pdf",
		// public defaults false
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+":downloadUrl", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxy_DownloadURL_AppArtifactRejected(t *testing.T) {
	dep := newFakeAppDeployer()
	s := newTestServiceWithDeployer(t, nil, dep)
	create, err := s.CreateAppArtifact(userCtx("alice"), &nsv1.CreateAppArtifactRequest{
		Name:        "app",
		HtmlContent: "<h1>x</h1>",
		Public:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+":downloadUrl", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxy_PreviewURL_PublicSkipsAuth(t *testing.T) {
	s := newTestService(t, nil)
	create, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "report.csv",
		Content:     []byte("a,b\n1,2"),
		ContentType: "text/csv",
		PreviewHtml: []byte("<table>...</table>"),
		Public:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+":previewUrl", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "previewUrl") {
		t.Fatalf("body lacks previewUrl field: %s", rec.Body.String())
	}
}

func TestProxy_PreviewURL_NoPreview(t *testing.T) {
	s := newTestService(t, nil)
	create, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "naked.pdf",
		Content:     []byte("data"),
		ContentType: "application/pdf",
		Public:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+":previewUrl", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxy_PreviewURL_PrivateRequiresBearer(t *testing.T) {
	s := newTestService(t, nil)
	create, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "private.csv",
		Content:     []byte("x"),
		ContentType: "text/csv",
		PreviewHtml: []byte("<table></table>"),
	})
	if err != nil {
		t.Fatal(err)
	}
	id := create.GetArtifact().GetId()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/artifacts/"+id+":previewUrl", nil)
	s.ProxyHandler(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxy_FallthroughForUnknownVerb(t *testing.T) {
	s := newTestService(t, nil)
	called := false
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/artifacts/abc:share", nil)
	s.ProxyHandler(fallback).ServeHTTP(rec, req)
	if !called {
		t.Fatal("fallback should have been invoked for unknown verb")
	}
}

// patchAppURL rewrites the artifact's app_url in storage for proxy
// tests against an httptest server.
func patchAppURL(t *testing.T, s *Service, id, newURL string) {
	t.Helper()
	rec, err := s.records.Get(context.Background(), artifactsCollection, id)
	if err != nil {
		t.Fatal(err)
	}
	art, err := recordToArtifact(rec)
	if err != nil {
		t.Fatal(err)
	}
	art.AppUrl = newURL
	updated, err := artifactToRecord(art, rec.Attributes[attrIdemKey], idemHashFromAttr(rec.Attributes))
	if err != nil {
		t.Fatal(err)
	}
	updated.Version = rec.Version
	if _, err := s.records.Put(context.Background(), updated, &rec.Version, ""); err != nil {
		t.Fatal(err)
	}
}

func ptrBool(b bool) *bool       { return &b }
func ptrString(s string) *string { return &s }

func TestCreateObjectArtifact_StampsSessionID(t *testing.T) {
	s := newTestService(t, nil)
	ctx := userCtx("alice")

	resp, err := s.CreateObjectArtifact(ctx, &nsv1.CreateObjectArtifactRequest{
		Name:        "upload.txt",
		Content:     []byte("payload"),
		ContentType: "text/plain",
		SessionId:   "sess_xyz",
	})
	if err != nil {
		t.Fatalf("CreateObjectArtifact: %v", err)
	}
	if got := resp.GetArtifact().GetSessionId(); got != "sess_xyz" {
		t.Fatalf("artifact.session_id = %q, want %q", got, "sess_xyz")
	}

	// Attribute index backs ListArtifacts(session_id=...); verify it
	// directly instead of relying on List as a transitive check.
	rec, err := s.records.Get(ctx, artifactsCollection, resp.GetArtifact().GetId())
	if err != nil {
		t.Fatalf("records.Get: %v", err)
	}
	if got := rec.Attributes[attrSessionID]; got != "sess_xyz" {
		t.Fatalf("record attr session_id = %q, want %q", got, "sess_xyz")
	}
}

func TestListArtifacts_FiltersBySession(t *testing.T) {
	const sessionID = "sess_xyz"
	const runID = "run_abc"

	s := newTestService(t, fakeRunLookup{runID: {userID: "alice", sessionID: sessionID}})

	userUpload, err := s.CreateObjectArtifact(userCtx("alice"), &nsv1.CreateObjectArtifactRequest{
		Name:        "user-upload.txt",
		Content:     []byte("from user"),
		ContentType: "text/plain",
		SessionId:   sessionID,
	})
	if err != nil {
		t.Fatalf("user CreateObjectArtifact: %v", err)
	}

	// Worker passes a foreign session_id; the service must ignore it
	// and bind the artifact to the run's actual session.
	workerOutput, err := s.CreateObjectArtifact(workerCtx(runID), &nsv1.CreateObjectArtifactRequest{
		Name:        "worker-output.txt",
		Content:     []byte("from worker"),
		ContentType: "text/plain",
		SessionId:   "sess_foreign",
	})
	if err != nil {
		t.Fatalf("worker CreateObjectArtifact: %v", err)
	}
	if got := workerOutput.GetArtifact().GetSessionId(); got != sessionID {
		t.Fatalf("worker session_id = %q, want %q (bound run's session, not request)", got, sessionID)
	}

	bySession, err := s.ListArtifacts(userCtx("alice"), &nsv1.ListArtifactsRequest{SessionId: sessionID})
	if err != nil {
		t.Fatalf("ListArtifacts(session): %v", err)
	}
	ids := map[string]bool{}
	for _, a := range bySession.GetArtifacts() {
		ids[a.GetId()] = true
	}
	if len(ids) != 2 || !ids[userUpload.GetArtifact().GetId()] || !ids[workerOutput.GetArtifact().GetId()] {
		t.Fatalf("session filter = %v, want both artifacts", ids)
	}

	byRun, err := s.ListArtifacts(userCtx("alice"), &nsv1.ListArtifactsRequest{RunId: runID})
	if err != nil {
		t.Fatalf("ListArtifacts(run): %v", err)
	}
	if len(byRun.GetArtifacts()) != 1 || byRun.GetArtifacts()[0].GetId() != workerOutput.GetArtifact().GetId() {
		t.Fatalf("run filter = %v, want only worker output %q", byRun.GetArtifacts(), workerOutput.GetArtifact().GetId())
	}
}
