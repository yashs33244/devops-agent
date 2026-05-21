package workers

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// workerCtx injects a worker-scheme principal for runID so tests can
// call inner-surface RPCs (PostWorkerEvent / CompleteRun / FailRun /
// GetRunCancellation) without routing through the gRPC interceptor.
// Unit tests exercise the service directly; interceptor behavior is
// covered in internal/verifiers.
func workerCtx(runID string) context.Context {
	return verifiers.WithPrincipal(context.Background(), &verifiers.Principal{
		Scheme: verifiers.SchemeWorker, ID: runID, RunID: runID,
	})
}

// fakeServerStream satisfies grpc.ServerStream. Only Send + Context
// are actually used by Service.StreamRunEvents; the rest panic if
// called, which is fine for unit tests.
type fakeServerStream struct {
	grpc.ServerStream
	ctx     context.Context
	mu      sync.Mutex
	items   []*nsv1.StreamRunEventsResponse
	sendErr error
}

func (f *fakeServerStream) Context() context.Context     { return f.ctx }
func (f *fakeServerStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeServerStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeServerStream) SetTrailer(metadata.MD)       {}
func (f *fakeServerStream) SendMsg(any) error            { return nil }
func (f *fakeServerStream) RecvMsg(any) error            { return nil }

func (f *fakeServerStream) Send(r *nsv1.StreamRunEventsResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return f.sendErr
	}
	f.items = append(f.items, r)
	return nil
}

func (f *fakeServerStream) collected() []*nsv1.StreamRunEventsResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*nsv1.StreamRunEventsResponse, len(f.items))
	copy(out, f.items)
	return out
}

func createTestRun(t *testing.T, svc *Service) string {
	t.Helper()
	resp, err := svc.CreateRun(context.Background(), &nsv1.CreateRunRequest{
		Prompt: "p", UserId: "u", InvokerType: nsv1.InvokerType_INVOKER_TYPE_USER, InvokerId: "u",
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	return resp.GetRun().GetId()
}

func mustPost(t *testing.T, svc *Service, runID, typ string) int64 {
	t.Helper()
	resp, err := svc.PostWorkerEvent(workerCtx(runID), &nsv1.PostWorkerEventRequest{
		RunId: runID,
		Event: &nsv1.StreamEvent{Type: typ, Timestamp: timestamppb.Now()},
	})
	if err != nil {
		t.Fatalf("PostWorkerEvent: %v", err)
	}
	return resp.GetIndex()
}

func TestPostWorkerEventMonotonicIndex(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)

	for i := int64(0); i < 5; i++ {
		got := mustPost(t, svc, runID, "assistant")
		if got != i {
			t.Fatalf("post %d: index=%d, want %d", i, got, i)
		}
	}
}

func TestPostWorkerEventOnTerminalRun(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)

	if _, err := svc.CompleteRun(workerCtx(runID), &nsv1.CompleteRunRequest{
		RunId: runID, Usage: &nsv1.RunUsage{InputTokens: 1},
	}); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
	_, err := svc.PostWorkerEvent(workerCtx(runID), &nsv1.PostWorkerEventRequest{
		RunId: runID,
		Event: &nsv1.StreamEvent{Type: "assistant", Timestamp: timestamppb.Now()},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", err)
	}
}

func TestCompleteRunIdempotent(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	req := &nsv1.CompleteRunRequest{RunId: runID, Usage: &nsv1.RunUsage{InputTokens: 10}}
	if _, err := svc.CompleteRun(workerCtx(runID), req); err != nil {
		t.Fatal(err)
	}
	// Second call MUST return OK without mutating state.
	if _, err := svc.CompleteRun(workerCtx(runID), req); err != nil {
		t.Fatalf("second CompleteRun: %v", err)
	}
	run, err := svc.getRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.GetStatus() != nsv1.RunStatus_RUN_STATUS_COMPLETED {
		t.Fatalf("status=%v", run.GetStatus())
	}
	// Second call MUST NOT overwrite usage from the first.
	if run.GetUsage().GetInputTokens() != 10 {
		t.Fatalf("usage lost: %v", run.GetUsage())
	}
}

func TestFailRunMarksError(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	if _, err := svc.FailRun(workerCtx(runID), &nsv1.FailRunRequest{
		RunId: runID, Error: "kaboom",
	}); err != nil {
		t.Fatal(err)
	}
	run, err := svc.getRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.GetStatus() != nsv1.RunStatus_RUN_STATUS_ERROR {
		t.Fatalf("status=%v", run.GetStatus())
	}
	if run.GetError() != "kaboom" {
		t.Fatalf("error=%q", run.GetError())
	}
}

func TestCompleteRunAfterCancelInterrupted(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)

	if _, err := svc.InterruptRun(context.Background(), &nsv1.InterruptRunRequest{RunId: runID}); err != nil {
		t.Fatal(err)
	}
	// Worker observes cancellation and calls CompleteRun (per the
	// protocol's cooperative-cancellation rule).
	if _, err := svc.CompleteRun(workerCtx(runID), &nsv1.CompleteRunRequest{
		RunId: runID, Usage: &nsv1.RunUsage{},
	}); err != nil {
		t.Fatal(err)
	}
	run, err := svc.getRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.GetStatus() != nsv1.RunStatus_RUN_STATUS_INTERRUPTED {
		t.Fatalf("status=%v (want INTERRUPTED via cancelled flag)", run.GetStatus())
	}
}

func TestGetRunCancellation(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)

	resp, err := svc.GetRunCancellation(workerCtx(runID), &nsv1.GetRunCancellationRequest{RunId: runID})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetCancelled() {
		t.Fatal("cancelled should be false initially")
	}

	if _, err := svc.InterruptRun(context.Background(), &nsv1.InterruptRunRequest{RunId: runID}); err != nil {
		t.Fatal(err)
	}
	resp, err = svc.GetRunCancellation(workerCtx(runID), &nsv1.GetRunCancellationRequest{RunId: runID})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetCancelled() {
		t.Fatal("cancelled should be true after Interrupt")
	}
}

func TestListRunEventsOrderedByIndex(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	for i := 0; i < 5; i++ {
		mustPost(t, svc, runID, "assistant")
	}
	resp, err := svc.ListRunEvents(context.Background(), &nsv1.ListRunEventsRequest{RunId: runID})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetEvents()) != 5 {
		t.Fatalf("got %d events", len(resp.GetEvents()))
	}
	for i, ev := range resp.GetEvents() {
		if ev.GetIndex() != int64(i) {
			t.Fatalf("events[%d].index=%d", i, ev.GetIndex())
		}
	}
}

func TestListRunEventsFromIndex(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	for i := 0; i < 5; i++ {
		mustPost(t, svc, runID, "assistant")
	}
	resp, err := svc.ListRunEvents(context.Background(), &nsv1.ListRunEventsRequest{
		RunId: runID, FromIndex: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetEvents()) != 2 {
		t.Fatalf("got %d events", len(resp.GetEvents()))
	}
	if resp.GetEvents()[0].GetIndex() != 3 {
		t.Fatalf("first index=%d", resp.GetEvents()[0].GetIndex())
	}
}

func TestStreamRunEventsReplayOnly(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	for i := 0; i < 3; i++ {
		mustPost(t, svc, runID, "assistant")
	}
	if _, err := svc.CompleteRun(workerCtx(runID), &nsv1.CompleteRunRequest{
		RunId: runID, Usage: &nsv1.RunUsage{},
	}); err != nil {
		t.Fatal(err)
	}
	stream := &fakeServerStream{ctx: context.Background()}
	if err := svc.StreamRunEvents(&nsv1.StreamRunEventsRequest{RunId: runID}, stream); err != nil {
		t.Fatalf("stream: %v", err)
	}
	items := stream.collected()
	if len(items) != 3 {
		t.Fatalf("got %d items", len(items))
	}
	for i, it := range items {
		if it.GetEvent().GetIndex() != int64(i) {
			t.Fatalf("items[%d].index=%d", i, it.GetEvent().GetIndex())
		}
	}
}

func TestStreamRunEventsLiveThenTerminal(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	stream := &fakeServerStream{ctx: context.Background()}

	done := make(chan error, 1)
	go func() {
		done <- svc.StreamRunEvents(&nsv1.StreamRunEventsRequest{RunId: runID}, stream)
	}()

	// Give the subscriber a moment to attach.
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 3; i++ {
		mustPost(t, svc, runID, "assistant")
	}
	if _, err := svc.CompleteRun(workerCtx(runID), &nsv1.CompleteRunRequest{
		RunId: runID, Usage: &nsv1.RunUsage{},
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stream returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not exit after CompleteRun")
	}

	items := stream.collected()
	if len(items) != 3 {
		t.Fatalf("got %d items; items=%v", len(items), items)
	}
}

func TestStreamRunEventsFromIndex(t *testing.T) {
	svc, _, _ := newTestService(t)
	runID := createTestRun(t, svc)
	for i := 0; i < 5; i++ {
		mustPost(t, svc, runID, "assistant")
	}
	if _, err := svc.CompleteRun(workerCtx(runID), &nsv1.CompleteRunRequest{
		RunId: runID, Usage: &nsv1.RunUsage{},
	}); err != nil {
		t.Fatal(err)
	}
	stream := &fakeServerStream{ctx: context.Background()}
	if err := svc.StreamRunEvents(&nsv1.StreamRunEventsRequest{RunId: runID, FromIndex: 3}, stream); err != nil {
		t.Fatal(err)
	}
	items := stream.collected()
	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].GetEvent().GetIndex() != 3 {
		t.Fatalf("first index=%d", items[0].GetEvent().GetIndex())
	}
}

// Broadcaster contract tests (slow-subscriber drop + CloseRun unblocks
// subscribers) live in internal/broadcaster's compliance suite, which
// runs against every backend (in-mem, Postgres). They used to live here
// when the broadcaster was local to this package.
