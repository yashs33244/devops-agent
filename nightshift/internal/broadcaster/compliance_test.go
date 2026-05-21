package broadcaster

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// Profile flags optional contract clauses a particular backend
// satisfies. The single contract suite reads the profile to gate
// subtests that don't apply uniformly to every backend.
type Profile struct {
	// CrossPod is true when two factory-produced instances share
	// state through the underlying backend (e.g. Postgres LISTEN/
	// NOTIFY). The in-memory backend always leaves this false.
	CrossPod bool
}

// newBroadcasterFunc returns a fresh, isolated Broadcaster for one
// compliance subtest. Implementations must register cleanup via
// t.Cleanup (typically calling Close on the returned instance).
type newBroadcasterFunc func(t *testing.T) Broadcaster

// runBroadcasterComplianceSuite exercises the Broadcaster contract
// against the backend produced by newBcaster. Drivers (inmem_test.go,
// postgres_test.go, …) invoke this from a single TestXxxCompliance
// entry point so adding a backend cannot bypass the contract.
func runBroadcasterComplianceSuite(t *testing.T, newBcaster newBroadcasterFunc, profile Profile) {
	t.Helper()

	t.Run("PublishToSubscriber", func(t *testing.T) {
		b := newBcaster(t)
		runID := uuid.NewString()
		ch, unsub := b.Subscribe(runID)
		defer unsub()

		ev := makeEvent(1)
		publishHelper(t, b, runID, ev)

		got := mustRecv(t, ch, 2*time.Second)
		if got.GetIndex() != ev.GetIndex() {
			t.Fatalf("index=%d, want %d", got.GetIndex(), ev.GetIndex())
		}
	})

	t.Run("MultipleSubscribersAllReceive", func(t *testing.T) {
		b := newBcaster(t)
		runID := uuid.NewString()
		ch1, unsub1 := b.Subscribe(runID)
		defer unsub1()
		ch2, unsub2 := b.Subscribe(runID)
		defer unsub2()

		ev := makeEvent(7)
		publishHelper(t, b, runID, ev)

		got1 := mustRecv(t, ch1, 2*time.Second)
		got2 := mustRecv(t, ch2, 2*time.Second)
		if got1.GetIndex() != 7 || got2.GetIndex() != 7 {
			t.Fatalf("got1=%d got2=%d, want both 7", got1.GetIndex(), got2.GetIndex())
		}
	})

	t.Run("RunIsolation", func(t *testing.T) {
		b := newBcaster(t)
		runA := uuid.NewString()
		runB := uuid.NewString()
		ch, unsub := b.Subscribe(runA)
		defer unsub()

		publishHelper(t, b, runB, makeEvent(1))

		select {
		case got, ok := <-ch:
			if ok {
				t.Fatalf("subscriber on runA received event for runB: %+v", got)
			}
			t.Fatalf("subscriber on runA channel closed unexpectedly")
		case <-time.After(250 * time.Millisecond):
			// Expected: no delivery.
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		b := newBcaster(t)
		runID := uuid.NewString()
		ch, unsub := b.Subscribe(runID)

		unsub()

		// Re-subscribing on the same runID after unsub is fine.
		ch2, unsub2 := b.Subscribe(runID)
		defer unsub2()

		publishHelper(t, b, runID, makeEvent(1))

		// First (unsubscribed) channel must not deliver.
		select {
		case got, ok := <-ch:
			if ok {
				t.Fatalf("unsubscribed channel received event: %+v", got)
			}
			// Closed channel from unsub is acceptable.
		case <-time.After(250 * time.Millisecond):
			// Acceptable: nothing delivered.
		}

		// Second (live) channel must deliver.
		got := mustRecv(t, ch2, 2*time.Second)
		if got.GetIndex() != 1 {
			t.Fatalf("live channel got index=%d, want 1", got.GetIndex())
		}
	})

	t.Run("CloseRunSignalsSubscribers", func(t *testing.T) {
		b := newBcaster(t)
		runID := uuid.NewString()
		ch, unsub := b.Subscribe(runID)
		defer unsub()

		b.CloseRun(runID)

		select {
		case _, ok := <-ch:
			if ok {
				t.Fatalf("CloseRun should close the channel, got an event")
			}
			// Closed: pass.
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for CloseRun to close channel")
		}
	})

	t.Run("LateSubscriberSeesNoHistory", func(t *testing.T) {
		b := newBcaster(t)
		runID := uuid.NewString()

		publishHelper(t, b, runID, makeEvent(1))

		// Late subscribe: broadcaster doesn't persist, so no replay.
		ch, unsub := b.Subscribe(runID)
		defer unsub()

		select {
		case got, ok := <-ch:
			if ok {
				t.Fatalf("late subscriber unexpectedly got event: %+v", got)
			}
		case <-time.After(250 * time.Millisecond):
			// Expected.
		}
	})

	t.Run("SlowSubscriberDoesNotBlockOthers", func(t *testing.T) {
		b := newBcaster(t)
		runID := uuid.NewString()
		// Slow subscriber: never reads.
		_, unsubSlow := b.Subscribe(runID)
		defer unsubSlow()
		// Fast subscriber.
		fast, unsubFast := b.Subscribe(runID)
		defer unsubFast()

		// Publish more than the subscriber buffer. The slow sub's
		// queue fills + drops; the fast sub keeps up because we read
		// concurrently with publishing. The contract is "slow sub
		// does not block publish for everyone else"; whether the
		// drop happens at publish time or buffer-overflow time is
		// implementation-defined. Read concurrently so the fast
		// channel itself never fills.
		//
		// The deadline is a slack budget, not the property under test:
		// we want to detect "publish stalled on the slow sub forever",
		// not measure throughput. Under `-race` on shared CI runners
		// publish+receive of 300 events can drift well past a tight
		// budget without anything actually being blocked, so size it
		// generously. A real block hangs until the test timeout.
		const n = 300
		done := make(chan int, 1)
		go func() {
			seen := 0
			deadline := time.After(30 * time.Second)
			for seen < n {
				select {
				case ev := <-fast:
					if ev == nil {
						done <- seen
						return
					}
					seen++
				case <-deadline:
					done <- seen
					return
				}
			}
			done <- seen
		}()

		for i := 1; i <= n; i++ {
			publishHelper(t, b, runID, makeEvent(int64(i)))
		}

		seen := <-done
		if seen != n {
			t.Fatalf("fast subscriber received %d/%d events — slow sub appears to have blocked publish", seen, n)
		}
	})

	if profile.CrossPod {
		t.Run("CrossInstancePublish", func(t *testing.T) {
			b1 := newBcaster(t)
			b2 := newBcaster(t)

			runID := uuid.NewString()
			ch, unsub := b1.Subscribe(runID)
			defer unsub()

			ev := makeEvent(42)
			publishHelper(t, b2, runID, ev)

			got := mustRecv(t, ch, 5*time.Second)
			if got.GetIndex() != 42 {
				t.Fatalf("cross-instance index=%d, want 42", got.GetIndex())
			}
		})
	}
}

// publishHelper hooks the optional pre-publish event-store seeding.
// The Postgres driver overrides it via the package-level publishHook
// to write into the test EventFetcher's backing map BEFORE Publish
// triggers the NOTIFY. The in-memory driver leaves publishHook nil
// (no fetcher is involved) and falls through to a plain Publish.
func publishHelper(t *testing.T, b Broadcaster, runID string, ev *nsv1.StreamEvent) {
	t.Helper()
	if publishHook != nil {
		publishHook(t, runID, ev)
	}
	b.Publish(runID, ev)
}

// publishHook is set by integration drivers that need to seed the
// EventFetcher before a Publish (so that the cross-instance listener
// can resolve the NOTIFY back to a full event). Unit drivers leave it
// nil. Package-level (not per-test) because the Go testing model
// already serialises subtests of one test function and we tear it
// down via t.Cleanup at the test entry point.
var publishHook func(t *testing.T, runID string, ev *nsv1.StreamEvent)

func mustRecv(t *testing.T, ch <-chan *nsv1.StreamEvent, timeout time.Duration) *nsv1.StreamEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatalf("channel closed waiting for event")
		}
		return ev
	case <-time.After(timeout):
		t.Fatalf("timed out after %s waiting for event", timeout)
		return nil
	}
}

func makeEvent(index int64) *nsv1.StreamEvent {
	return &nsv1.StreamEvent{
		Index:     index,
		Type:      "compliance.test",
		Timestamp: timestamppb.Now(),
	}
}
