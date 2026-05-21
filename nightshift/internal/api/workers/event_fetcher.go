package workers

import (
	"context"
	"errors"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/broadcaster"
	"github.com/nightshiftco/nightshift/internal/records"
)

// recordsEventFetcher adapts records.RecordStore to
// broadcaster.EventFetcher. The Postgres broadcaster's listener uses
// it to resolve a NOTIFY (which carries only run_id + index) back to
// the full StreamEvent persisted in the events collection.
//
// This adapter lives in workers/ rather than broadcaster/ so the
// broadcaster package keeps a single inbound dependency direction
// (no import of records or workers).
type recordsEventFetcher struct {
	rs records.RecordStore
}

// NewRecordsEventFetcher returns a broadcaster.EventFetcher backed by
// the workers' records store. Reuses eventKey() and recordToEvent()
// from event_state.go so the broadcaster sees the same encoding
// PostWorkerEvent writes.
func NewRecordsEventFetcher(rs records.RecordStore) broadcaster.EventFetcher {
	return &recordsEventFetcher{rs: rs}
}

func (f *recordsEventFetcher) Fetch(ctx context.Context, runID string, index int64) (*nsv1.StreamEvent, error) {
	rec, err := f.rs.Get(ctx, eventsCollection, eventKey(runID, index))
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			return nil, broadcaster.ErrUnknownRun
		}
		return nil, err
	}
	return recordToEvent(rec)
}
