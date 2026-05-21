package workers

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/broadcaster"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
)

// fakeLauncher is an in-process runtime.JobLauncher used by service
// tests. It records every Launch/Interrupt call and never actually
// runs a worker — the test drives the inner-surface RPCs directly
// when needed.
type fakeLauncher struct {
	mu        sync.Mutex
	launched  []runtime.LaunchSpec
	intrupted []string
	launchErr error
}

func (f *fakeLauncher) Launch(_ context.Context, spec runtime.LaunchSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.launchErr != nil {
		return f.launchErr
	}
	f.launched = append(f.launched, spec)
	return nil
}

func (f *fakeLauncher) Interrupt(_ context.Context, runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.intrupted = append(f.intrupted, runID)
	return nil
}

func (f *fakeLauncher) Close() error { return nil }

func newTestService(t *testing.T) (*Service, *fakeLauncher, records.RecordStore) {
	t.Helper()
	store, err := records.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	launcher := &fakeLauncher{}

	var idCounter int
	newID := func() string {
		idCounter++
		return "run-" + strconv.Itoa(idCounter)
	}
	bcaster := broadcaster.NewInMem()
	t.Cleanup(func() { _ = bcaster.Close() })
	svc := NewService(ServiceOptions{
		Records:     store,
		Launcher:    launcher,
		Broadcaster: bcaster,
		WorkerHMAC:  []byte("0123456789abcdef0123456789abcdef"),
		CallbackURL: "http://test-callback",
		WorkerImage: "nightshift-worker:test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		NewID:       newID,
		Now:         func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	})
	return svc, launcher, store
}

func TestCreateRunHappyPath(t *testing.T) {
	svc, launcher, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt:      "hello",
		UserId:      "alice",
		InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER,
		InvokerId:   "alice",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	run := resp.GetRun()
	if run.GetId() == "" {
		t.Fatal("no run id")
	}
	if run.GetStatus() != nsv1.RunStatus_RUN_STATUS_RUNNING {
		t.Fatalf("status=%v", run.GetStatus())
	}

	launcher.mu.Lock()
	defer launcher.mu.Unlock()
	if len(launcher.launched) != 1 {
		t.Fatalf("launched=%d", len(launcher.launched))
	}
	spec := launcher.launched[0]
	if spec.RunID != run.GetId() {
		t.Fatalf("spec runID=%s vs run.id=%s", spec.RunID, run.GetId())
	}
	if spec.WorkerCredential == "" {
		t.Fatal("credential empty")
	}
	if _, err := runtime.VerifyCredential(svc.WorkerHMAC, spec.WorkerCredential, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("credential verify: %v", err)
	}
	if spec.CallbackURL != "http://test-callback" {
		t.Fatalf("callback=%q", spec.CallbackURL)
	}
	if spec.Image != "nightshift-worker:test" {
		t.Fatalf("image=%q", spec.Image)
	}
}

// Server is the exclusive minter of session ids: two calls must
// return distinct ids drawn from the injected newID(), proving the
// value is neither hardcoded nor memoized.
func TestCreateSession_MintsFromServerNewID(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	first, err := svc.CreateSession(ctx, &nsv1.CreateSessionRequest{})
	if err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}
	if first.GetSessionId() != "sess_run-1" {
		t.Fatalf("first session_id = %q, want %q", first.GetSessionId(), "sess_run-1")
	}

	second, err := svc.CreateSession(ctx, &nsv1.CreateSessionRequest{})
	if err != nil {
		t.Fatalf("second CreateSession: %v", err)
	}
	if second.GetSessionId() != "sess_run-2" {
		t.Fatalf("second session_id = %q, want %q", second.GetSessionId(), "sess_run-2")
	}
}

func TestCreateRunValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	cases := []struct {
		name string
		req  *nsv1.CreateRunRequest
	}{
		{"no prompt", &nsv1.CreateRunRequest{UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u"}},
		{"no user_id", &nsv1.CreateRunRequest{Prompt: "p", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u"}},
		{"unspecified invoker type", &nsv1.CreateRunRequest{Prompt: "p", UserId: "u", InvokerId: "u"}},
		{"no invoker_id", &nsv1.CreateRunRequest{Prompt: "p", UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.CreateRun(ctx, c.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("want InvalidArgument, got %v", err)
			}
		})
	}
}

func TestGetRunRoundTrip(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt: "p", UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetRun(ctx, &nsv1.GetRunRequest{RunId: created.GetRun().GetId()})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetRun().GetId() != created.GetRun().GetId() {
		t.Fatalf("ids differ")
	}
	if got.GetRun().GetPrompt() != "p" {
		t.Fatalf("prompt=%q", got.GetRun().GetPrompt())
	}
}

func TestGetRunNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetRun(context.Background(), &nsv1.GetRunRequest{RunId: "nope"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestListRunsFilters(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	mk := func(user string) {
		if _, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
			Prompt: "p", UserId: user, InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: user,
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk("alice")
	mk("alice")
	mk("bob")

	byAlice, err := svc.ListRuns(ctx, &nsv1.ListRunsRequest{UserId: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byAlice.GetRuns()) != 2 {
		t.Fatalf("alice=%d", len(byAlice.GetRuns()))
	}

	all, err := svc.ListRuns(ctx, &nsv1.ListRunsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.GetRuns()) != 3 {
		t.Fatalf("all=%d", len(all.GetRuns()))
	}

	running, err := svc.ListRuns(ctx, &nsv1.ListRunsRequest{Status: nsv1.RunStatus_RUN_STATUS_RUNNING})
	if err != nil {
		t.Fatal(err)
	}
	if len(running.GetRuns()) != 3 {
		t.Fatalf("running=%d (all new runs are RUNNING after launch)", len(running.GetRuns()))
	}
}

func TestInterruptRunRecordsCancellation(t *testing.T) {
	svc, launcher, store := newTestService(t)
	ctx := context.Background()

	created, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt: "p", UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := created.GetRun().GetId()

	if _, err := svc.InterruptRun(ctx, &nsv1.InterruptRunRequest{RunId: runID}); err != nil {
		t.Fatalf("interrupt: %v", err)
	}

	launcher.mu.Lock()
	if len(launcher.intrupted) != 1 || launcher.intrupted[0] != runID {
		t.Fatalf("launcher interrupts=%v", launcher.intrupted)
	}
	launcher.mu.Unlock()

	rec, err := store.Get(ctx, runsCollection, runID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Attributes[attrCancelled] != "true" {
		t.Fatalf("cancelled attr=%q", rec.Attributes[attrCancelled])
	}
}

func TestInterruptTerminalRunRejected(t *testing.T) {
	svc, _, store := newTestService(t)
	ctx := context.Background()
	created, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt: "p", UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := created.GetRun().GetId()

	// Manually transition to COMPLETED.
	run := created.GetRun()
	run.Status = nsv1.RunStatus_RUN_STATUS_COMPLETED
	if _, err := svc.putRun(ctx, run, false, nil); err != nil {
		t.Fatal(err)
	}

	_, err = svc.InterruptRun(ctx, &nsv1.InterruptRunRequest{RunId: runID})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", err)
	}
	_ = store
}

func TestDeleteSessionCascade(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	var runIDs []string
	for i := 0; i < 3; i++ {
		resp, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
			Prompt:      "p",
			UserId:      "u",
			SessionId:   "sess-shared",
			InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER,
			InvokerId:   "u",
		})
		if err != nil {
			t.Fatal(err)
		}
		runIDs = append(runIDs, resp.GetRun().GetId())
	}

	// DeleteSession with runs still in RUNNING must fail.
	_, err := svc.DeleteSession(ctx, &nsv1.DeleteSessionRequest{SessionId: "sess-shared"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition on active runs, got %v", err)
	}

	// Force-complete each run.
	for _, id := range runIDs {
		r, err := svc.getRun(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		r.Status = nsv1.RunStatus_RUN_STATUS_COMPLETED
		if _, err := svc.putRun(ctx, r, false, nil); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := svc.DeleteSession(ctx, &nsv1.DeleteSessionRequest{SessionId: "sess-shared"})
	if err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if resp.GetRunsDeleted() != 3 {
		t.Fatalf("deleted=%d, want 3", resp.GetRunsDeleted())
	}

	// Second call is a no-op returning 0.
	resp, err = svc.DeleteSession(ctx, &nsv1.DeleteSessionRequest{SessionId: "sess-shared"})
	if err != nil {
		t.Fatalf("delete session (empty): %v", err)
	}
	if resp.GetRunsDeleted() != 0 {
		t.Fatalf("second delete=%d, want 0", resp.GetRunsDeleted())
	}
}

// Chunk 14 — `finalize` MUST NOT clobber Run.session_id (platform id)
// with the SDK id reported by the worker; it MUST persist the SDK id
// as a Record attribute so LookupResumeSDKSessionID can find it.
func TestFinalizeStoresSDKSessionIDAsAttribute(t *testing.T) {
	svc, _, store := newTestService(t)
	ctx := context.Background()

	created, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt:      "p",
		UserId:      "alice",
		SessionId:   "platform-abc",
		InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER,
		InvokerId:   "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := created.GetRun().GetId()

	// Worker reports its SDK session id on the way out.
	if err := svc.finalize(ctx, runID, nsv1.RunStatus_RUN_STATUS_COMPLETED, "", "claude-xyz", &nsv1.RunUsage{}); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	rec, err := store.Get(ctx, runsCollection, runID)
	if err != nil {
		t.Fatal(err)
	}
	// Platform session id MUST survive.
	if rec.Attributes[attrSessionID] != "platform-abc" {
		t.Errorf("Run.session_id (platform) clobbered: got %q", rec.Attributes[attrSessionID])
	}
	// SDK id MUST live in the attribute.
	if rec.Attributes[attrSDKSessionID] != "claude-xyz" {
		t.Errorf("attrSDKSessionID = %q, want claude-xyz", rec.Attributes[attrSDKSessionID])
	}
	// The proto field on Run still carries the platform id.
	r, _, err := recordToRun(rec)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetSessionId() != "platform-abc" {
		t.Errorf("Run.session_id proto: %q", r.GetSessionId())
	}
}

func TestFinalizeEmptySDKSessionIDPreservesPriorAttribute(t *testing.T) {
	// Worker reports a non-empty SDK id on Run 1, then a follow-up
	// finalize (e.g. cancellation path) reports empty. The previously-
	// stored attribute should NOT be wiped — empty means "nothing new
	// to report", not "clear the value".
	svc, _, store := newTestService(t)
	ctx := context.Background()

	created, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt: "p", UserId: "u", SessionId: "platform",
		InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := created.GetRun().GetId()

	if err := svc.finalize(ctx, runID, nsv1.RunStatus_RUN_STATUS_COMPLETED, "", "claude-xyz", nil); err != nil {
		t.Fatal(err)
	}
	// Second finalize is a no-op (run is terminal) — attribute still set.
	_ = svc.finalize(ctx, runID, nsv1.RunStatus_RUN_STATUS_COMPLETED, "", "", nil)
	rec, err := store.Get(ctx, runsCollection, runID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Attributes[attrSDKSessionID] != "claude-xyz" {
		t.Errorf("attrSDKSessionID lost: %q", rec.Attributes[attrSDKSessionID])
	}
}

func TestLookupResumeSDKSessionIDReturnsMostRecent(t *testing.T) {
	svc, _, store := newTestService(t)
	ctx := context.Background()

	mk := func(sdkID string) string {
		resp, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
			Prompt:      "p",
			UserId:      "u",
			SessionId:   "platform-1",
			InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER,
			InvokerId:   "u",
		})
		if err != nil {
			t.Fatal(err)
		}
		runID := resp.GetRun().GetId()
		if err := svc.finalize(ctx, runID, nsv1.RunStatus_RUN_STATUS_COMPLETED, "", sdkID, &nsv1.RunUsage{}); err != nil {
			t.Fatal(err)
		}
		return runID
	}
	mk("sdk-old")
	mk("sdk-mid")
	mk("sdk-newest")

	got, err := LookupResumeSDKSessionID(ctx, store, "platform-1")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != "sdk-newest" {
		t.Fatalf("got %q, want sdk-newest", got)
	}

	// Unknown session → empty, no error.
	got, err = LookupResumeSDKSessionID(ctx, store, "doesnt-exist")
	if err != nil || got != "" {
		t.Fatalf("expected empty/nil, got %q / %v", got, err)
	}
}

func TestCreateRunResumeInjectsSDKSessionID(t *testing.T) {
	svc, launcher, _ := newTestService(t)
	ctx := context.Background()

	// Run 1 — fresh session. CreateRun mints a platform id; worker
	// reports SDK id on completion.
	first, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt:      "p",
		UserId:      "u",
		InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER,
		InvokerId:   "u",
	})
	if err != nil {
		t.Fatal(err)
	}
	platformID := first.GetRun().GetSessionId()
	if platformID == "" {
		t.Fatal("platform session_id should be auto-minted")
	}
	if err := svc.finalize(ctx, first.GetRun().GetId(), nsv1.RunStatus_RUN_STATUS_COMPLETED, "", "claude-xyz", &nsv1.RunUsage{}); err != nil {
		t.Fatal(err)
	}

	// First CreateRun's LaunchSpec carried no SDK id (fresh).
	launcher.mu.Lock()
	if got := launcher.launched[0].SDKSessionID; got != "" {
		t.Errorf("first run SDKSessionID should be empty, got %q", got)
	}
	launcher.mu.Unlock()

	// Run 2 — resume by passing the platform session_id. The launch
	// spec MUST carry SDKSessionID = "claude-xyz".
	if _, err := svc.CreateRun(ctx, &nsv1.CreateRunRequest{
		Prompt:      "follow-up",
		UserId:      "u",
		SessionId:   platformID,
		InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER,
		InvokerId:   "u",
	}); err != nil {
		t.Fatal(err)
	}

	launcher.mu.Lock()
	defer launcher.mu.Unlock()
	if len(launcher.launched) != 2 {
		t.Fatalf("launched=%d", len(launcher.launched))
	}
	if got := launcher.launched[1].SDKSessionID; got != "claude-xyz" {
		t.Errorf("resume run SDKSessionID = %q, want claude-xyz", got)
	}
	if got := launcher.launched[1].SessionID; got != platformID {
		t.Errorf("resume run SessionID (platform) = %q, want %q", got, platformID)
	}
}

func TestLauncherFailureMarksRunAsError(t *testing.T) {
	svc, launcher, _ := newTestService(t)
	launcher.launchErr = context.DeadlineExceeded
	_, err := svc.CreateRun(context.Background(), &nsv1.CreateRunRequest{
		Prompt: "p", UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("want Internal, got %v", err)
	}
	// The run record is still there, marked as ERROR.
	all, _ := svc.ListRuns(context.Background(), &nsv1.ListRunsRequest{Status: nsv1.RunStatus_RUN_STATUS_ERROR})
	if len(all.GetRuns()) != 1 {
		t.Fatalf("error runs=%d", len(all.GetRuns()))
	}
}
