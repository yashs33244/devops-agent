package workers

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/records"
)

// Storage collection names (see workers.md §9).
const (
	runsCollection   = "runs"
	eventsCollection = "events"
)

// Attribute keys on Run records.
const (
	attrUserID       = "user_id"
	attrStatus       = "status"
	attrSessionID    = "session_id"
	attrInvokerType  = "invoker_type"
	attrInvokerID    = "invoker_id"
	attrCancelled    = "cancelled"
	attrSDKSessionID = "sdk_session_id"
)

// Attribute key on Event records.
const attrRunID = "run_id"

// recordContentType is the proto-wire content type for Run / Event
// payloads. Documented in workers.md §9.
const recordContentType = "application/x-protobuf"

// runToRecord serializes r into a records.Record suitable for
// RecordStore.Put. Attributes mirror the fields used by ListRuns
// filters.
func runToRecord(r *nsv1.Run, cancelled bool) (records.Record, error) {
	if r.GetId() == "" {
		return records.Record{}, errors.New("workers: Run.id required")
	}
	data, err := proto.Marshal(r)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal run: %w", err)
	}
	return records.Record{
		Collection:  runsCollection,
		Key:         r.GetId(),
		Data:        data,
		ContentType: recordContentType,
		Attributes: map[string]string{
			attrUserID:      r.GetUserId(),
			attrStatus:      r.GetStatus().String(),
			attrSessionID:   r.GetSessionId(),
			attrInvokerType: r.GetInvokerType().String(),
			attrInvokerID:   r.GetInvokerId(),
			attrCancelled:   boolAttr(cancelled),
		},
	}, nil
}

// recordToRun reverses runToRecord. The cancelled flag is returned
// alongside so callers can distinguish a naturally-running run from
// one that has been marked for cancellation.
func recordToRun(rec records.Record) (*nsv1.Run, bool, error) {
	r := &nsv1.Run{}
	if err := proto.Unmarshal(rec.Data, r); err != nil {
		return nil, false, fmt.Errorf("unmarshal run: %w", err)
	}
	cancelled := rec.Attributes[attrCancelled] == "true"
	return r, cancelled, nil
}

func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// isTerminal reports whether a RunStatus is a terminal state.
func isTerminal(s nsv1.RunStatus) bool {
	switch s {
	case nsv1.RunStatus_RUN_STATUS_COMPLETED,
		nsv1.RunStatus_RUN_STATUS_ERROR,
		nsv1.RunStatus_RUN_STATUS_INTERRUPTED:
		return true
	}
	return false
}

// LookupRunOwner returns the user_id and session_id of the run
// identified by runID against an arbitrary RecordStore. Exposed so the
// Config service (chunk 11) can verify that a worker credential
// authorized for run R only resolves config for R's owning user; the
// artifacts service uses the session id to scope worker-produced
// artifacts to the run's session.
func LookupRunOwner(ctx context.Context, store records.RecordStore, runID string) (string, string, error) {
	rec, err := store.Get(ctx, runsCollection, runID)
	if err != nil {
		return "", "", err
	}
	owner := rec.Attributes[attrUserID]
	session := rec.Attributes[attrSessionID]
	if owner != "" {
		return owner, session, nil
	}
	r, _, err := recordToRun(rec)
	if err != nil {
		return "", "", err
	}
	return r.GetUserId(), r.GetSessionId(), nil
}

// CountActiveRuns returns the count of runs in PENDING or RUNNING
// state. Called once at API startup by the chunk-18 metrics layer to
// recount the active_runs gauge so it survives restarts. Listing the
// full collection per status is fine: pending+running runs are bounded
// by the launcher's concurrency, not by historical run volume.
func CountActiveRuns(ctx context.Context, store records.RecordStore) (int, error) {
	total := 0
	for _, st := range []nsv1.RunStatus{
		nsv1.RunStatus_RUN_STATUS_PENDING,
		nsv1.RunStatus_RUN_STATUS_RUNNING,
	} {
		var token string
		for {
			page, next, err := store.List(ctx, records.ListQuery{
				Collection:       runsCollection,
				AttributeFilters: map[string]string{attrStatus: st.String()},
				PageSize:         500,
				PageToken:        token,
			})
			if err != nil {
				return total, err
			}
			total += len(page)
			if next == "" {
				break
			}
			token = next
		}
	}
	return total, nil
}

// LookupResumeSDKSessionID finds the most recent terminal run for the
// given platform sessionID and returns the SDK-internal session id the
// worker reported on completion (stored as the attrSDKSessionID Record
// attribute by finalize). Used by CreateRun to inject NS_SDK_SESSION_ID
// onto the launcher's env when the caller is resuming a session.
//
// Returns "" with no error when:
//   - no runs exist for sessionID (fresh session)
//   - the most recent run never reported an SDK id (e.g. a failed run)
//
// Workers are ephemeral pods; this resume bridge lives on the API side
// (Record attribute), not in the worker's filesystem, so it works
// regardless of which session-state backend (chunk 13) is configured.
// Per workers.md §4, the SDK id appears only on the inner surface
// (CompleteRunRequest.session_id) and as out-of-band Record attribute
// state — never on the outer / user-facing surface.
func LookupResumeSDKSessionID(ctx context.Context, store records.RecordStore, sessionID string) (string, error) {
	if sessionID == "" {
		return "", nil
	}
	page, _, err := store.List(ctx, records.ListQuery{
		Collection:       runsCollection,
		AttributeFilters: map[string]string{attrSessionID: sessionID},
		PageSize:         50,
		OrderBy:          "created_at desc",
	})
	if err != nil {
		return "", fmt.Errorf("lookup sdk session id: %w", err)
	}
	completed := nsv1.RunStatus_RUN_STATUS_COMPLETED.String()
	interrupted := nsv1.RunStatus_RUN_STATUS_INTERRUPTED.String()
	for _, rec := range page {
		st := rec.Attributes[attrStatus]
		if st != completed && st != interrupted {
			// Only resume from runs that finished cleanly. ERROR runs
			// shouldn't define the SDK state; skip them.
			continue
		}
		if v := rec.Attributes[attrSDKSessionID]; v != "" {
			return v, nil
		}
	}
	return "", nil
}
