//go:build integration

package broadcaster

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// stubFetcher is the EventFetcher used by the integration suite. The
// compliance helper publishHelper writes the event into stubFetcher's
// store BEFORE calling Publish, so the cross-instance listener can
// resolve the NOTIFY back to the full event body.
type stubFetcher struct {
	mu   sync.Mutex
	data map[string]*nsv1.StreamEvent // key = runID|index
}

func newStubFetcher() *stubFetcher {
	return &stubFetcher{data: map[string]*nsv1.StreamEvent{}}
}

func (s *stubFetcher) put(runID string, ev *nsv1.StreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[fetchKey(runID, ev.GetIndex())] = ev
}

func (s *stubFetcher) Fetch(_ context.Context, runID string, index int64) (*nsv1.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, ok := s.data[fetchKey(runID, index)]
	if !ok {
		return nil, ErrUnknownRun
	}
	return ev, nil
}

func fetchKey(runID string, index int64) string {
	// Local to the test so this package has no dependency on
	// internal/api/workers (which owns the production event-key
	// format).
	return fmt.Sprintf("%s|%d", runID, index)
}

// TestPostgresBroadcasterCompliance runs the full Broadcaster
// contract against the Postgres-backed implementation, including the
// CrossInstancePublish subtest gated by Profile.CrossPod=true.
func TestPostgresBroadcasterCompliance(t *testing.T) {
	// Per-test fetcher shared across both instances created during
	// CrossInstancePublish (so the listener on instance 1 can resolve
	// events published by instance 2). Reset before this test runs.
	fetcher := newStubFetcher()

	// Wire the publishHook so the suite seeds the fetcher before
	// every Publish. Cleared in t.Cleanup at the entry point.
	prev := publishHook
	publishHook = func(_ *testing.T, runID string, ev *nsv1.StreamEvent) {
		fetcher.put(runID, ev)
	}
	t.Cleanup(func() { publishHook = prev })

	factory := func(t *testing.T) Broadcaster {
		podID := "pod-" + uuid.NewString()
		ctx, cancel := context.WithCancel(context.Background())
		b, err := NewPostgres(ctx, pgDSN, podID, fetcher)
		if err != nil {
			cancel()
			t.Fatalf("NewPostgres: %v", err)
		}
		t.Cleanup(func() {
			_ = b.Close()
			cancel()
		})
		return b
	}

	runBroadcasterComplianceSuite(t, factory, Profile{CrossPod: true})
}
