// Package broadcaster is the pluggable seam for per-run pub/sub
// fan-out of nsv1.StreamEvent.
//
// The in-process implementation (NewInMem) is fine when there is one
// API replica: every PostWorkerEvent and StreamRunEvents lands in the
// same process. With more than one replica behind a Service, the
// worker's POST and the UI's stream often land on different pods, so
// the fan-out has to cross pod boundaries — that is what NewPostgres
// adds via LISTEN/NOTIFY against the shared records database.
//
// Both implementations satisfy the same Broadcaster interface and the
// same compliance contract (compliance_test.go). The Postgres impl
// additionally satisfies the CrossInstancePublish subtest gated by
// Profile.CrossPod=true.
package broadcaster

import (
	"context"
	"errors"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// ErrUnknownRun is returned by EventFetcher.Fetch when the requested
// run/index is not known to the underlying store. The Postgres
// listener treats this as a recoverable miss (logs + drops the
// notification) rather than fatal.
var ErrUnknownRun = errors.New("broadcaster: unknown run/index")

// Broadcaster is a per-run pub/sub fan-out.
//
// Subscribe registers a new subscriber for runID. The returned unsub
// func cleans up the subscription; callers MUST call it.
//
// Publish fan-outs ev to every current subscriber of runID. A
// subscriber whose channel is full has its event dropped (best
// effort) and keeps the subscription — broadcaster does not persist
// events, so a dropped event is recovered by re-reading run history
// from the records store.
//
// CloseRun terminates the stream for every current subscriber of
// runID. Subsequent receives on their channels return ok=false. Used
// by terminal RPCs (CompleteRun / FailRun).
//
// Close shuts down the broadcaster instance itself — closes any
// per-run channels still open and releases any backend resources
// (goroutines, connections). Idempotent; safe to call multiple times.
//
// The broadcaster does NOT persist events; persistence is the
// records.RecordStore's job. A subscriber that misses a live event
// catches up via run history.
type Broadcaster interface {
	Subscribe(runID string) (<-chan *nsv1.StreamEvent, func())
	Publish(runID string, ev *nsv1.StreamEvent)
	CloseRun(runID string)
	Close() error
}

// EventFetcher resolves a (runID, index) back to the full
// nsv1.StreamEvent. The Postgres broadcaster uses this to look up
// events referenced by NOTIFY payloads — the payload carries only
// {pod, run, kind, index} (Postgres caps NOTIFY payloads at 8000
// bytes), so the listener re-fetches the event body from the
// authoritative records store before fanning it out locally.
//
// Implementations should return ErrUnknownRun when the requested
// run/index is not present (e.g. record not yet committed when the
// notification arrives — extremely unlikely in practice since Publish
// commits to records before NOTIFYing, but possible across crash
// recoveries).
type EventFetcher interface {
	Fetch(ctx context.Context, runID string, index int64) (*nsv1.StreamEvent, error)
}
