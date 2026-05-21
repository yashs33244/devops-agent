// Package workers implements nightshift.v1.Workers.
//
// The service owns run lifecycle + event streaming, layering on
// internal/records for durable state and internal/runtime for the
// JobLauncher abstraction.
package workers

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/broadcaster"
	"github.com/nightshiftco/nightshift/internal/metrics"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// ServiceOptions configures a Workers Service. Fields without
// defaults (Records, Launcher, WorkerHMAC, CallbackURL) are required.
type ServiceOptions struct {
	Records     records.RecordStore
	Launcher    runtime.JobLauncher
	WorkerHMAC  []byte
	CallbackURL string
	WorkerImage string // required for K8s launcher; ignored by stub
	Logger      *slog.Logger

	// Broadcaster is the per-run event fan-out. Required: callers
	// pass either broadcaster.NewInMem() (single-replica / local
	// dev) or broadcaster.NewPostgres(...) (multi-replica via
	// LISTEN/NOTIFY). The Service does not own the broadcaster
	// lifecycle — main.go closes it at shutdown.
	Broadcaster broadcaster.Broadcaster

	// Credential TTL padding — worker credentials expire at
	// run.created_at + ActiveDeadline + CredentialTTLBuffer. Default 5m.
	CredentialTTLBuffer time.Duration

	// TTLAfterFinished maps to K8s TTLSecondsAfterFinished. Default 300s.
	TTLAfterFinished time.Duration

	// ActiveDeadline maps to K8s ActiveDeadlineSeconds. Default 1h.
	ActiveDeadline time.Duration

	// SessionState is forwarded onto every LaunchSpec so the launcher
	// can wire the per-session volume mount. Default: backend=none.
	SessionState runtime.SessionStateConfig

	// SessionStateCleaner is invoked by DeleteSession to cascade the
	// per-session state removal. Default: NoopCleaner.
	SessionStateCleaner runtime.SessionCleaner

	// MountWorkerServiceAccountToken controls whether worker pods
	// receive the default K8s SA token. False by default (chunk-9
	// hardening); chunk-14 nightshift-worker-claude flips it on to
	// enable OpenBao K8s-auth login.
	MountWorkerServiceAccountToken bool

	// WorkerExtraEnv is a chart-driven, opaque map of env vars the
	// API forwards onto every worker pod (via LaunchSpec.ExtraEnv).
	// The API does not interpret keys or values — this is the
	// passthrough seam that lets the chart configure backend-specific
	// worker config (e.g. the secrets-store address) without any
	// secrets-backend coupling in the API binary.
	WorkerExtraEnv map[string]string

	// Metrics is the chunk-18 recorder. Optional — nil-safe via
	// metrics.Get; tests pass nil and existing tests stay valid.
	Metrics metrics.Recorder

	// Optional override for UUID generation. Tests supply a
	// deterministic generator.
	NewID func() string

	// Optional override for clock (tests).
	Now func() time.Time
}

// Service is the nightshift.v1.WorkersServer implementation.
type Service struct {
	nsv1.UnimplementedWorkersServer

	Records     records.RecordStore
	Launcher    runtime.JobLauncher
	WorkerHMAC  []byte
	CallbackURL string
	WorkerImage string
	Logger      *slog.Logger

	CredentialTTLBuffer time.Duration
	TTLAfterFinished    time.Duration
	ActiveDeadline      time.Duration

	SessionState        runtime.SessionStateConfig
	SessionStateCleaner runtime.SessionCleaner

	MountWorkerServiceAccountToken bool

	WorkerExtraEnv map[string]string

	metrics metrics.Recorder

	broadcaster broadcaster.Broadcaster

	newID func() string
	now   func() time.Time
}

// NewService constructs a Service. Panics if required options are
// missing — misconfiguration is a programming error at wire-up time.
func NewService(opts ServiceOptions) *Service {
	if opts.Records == nil {
		panic("workers.NewService: Records required")
	}
	if opts.Launcher == nil {
		panic("workers.NewService: Launcher required")
	}
	if len(opts.WorkerHMAC) < 16 {
		panic("workers.NewService: WorkerHMAC must be >= 16 bytes")
	}
	if opts.CallbackURL == "" {
		panic("workers.NewService: CallbackURL required")
	}
	if opts.Broadcaster == nil {
		panic("workers.NewService: Broadcaster required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	newID := opts.NewID
	if newID == nil {
		newID = func() string { return uuid.NewString() }
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	cleaner := opts.SessionStateCleaner
	if cleaner == nil {
		cleaner = runtime.NoopCleaner{}
	}
	s := &Service{
		Records:                        opts.Records,
		Launcher:                       opts.Launcher,
		WorkerHMAC:                     opts.WorkerHMAC,
		CallbackURL:                    opts.CallbackURL,
		WorkerImage:                    opts.WorkerImage,
		Logger:                         logger,
		CredentialTTLBuffer:            opts.CredentialTTLBuffer,
		TTLAfterFinished:               opts.TTLAfterFinished,
		ActiveDeadline:                 opts.ActiveDeadline,
		SessionState:                   opts.SessionState,
		SessionStateCleaner:            cleaner,
		MountWorkerServiceAccountToken: opts.MountWorkerServiceAccountToken,
		WorkerExtraEnv:                 opts.WorkerExtraEnv,
		metrics:                        metrics.Get(opts.Metrics),
		broadcaster:                    opts.Broadcaster,
		newID:                          newID,
		now:                            now,
	}
	if s.CredentialTTLBuffer == 0 {
		s.CredentialTTLBuffer = 5 * time.Minute
	}
	return s
}

// -----------------------------------------------------------------------------
// Outer surface — run lifecycle
// -----------------------------------------------------------------------------

func (s *Service) CreateSession(_ context.Context, _ *nsv1.CreateSessionRequest) (*nsv1.CreateSessionResponse, error) {
	s.metrics.SessionCreated()
	return &nsv1.CreateSessionResponse{SessionId: "sess_" + s.newID()}, nil
}

func (s *Service) CreateRun(ctx context.Context, req *nsv1.CreateRunRequest) (*nsv1.CreateRunResponse, error) {
	if req.GetPrompt() == "" {
		return nil, status.Error(codes.InvalidArgument, "prompt required")
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	invoker := req.GetInvokerType()
	if invoker == nsv1.InvokerType_INVOKER_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "invoker_type required")
	}
	if req.GetInvokerId() == "" {
		return nil, status.Error(codes.InvalidArgument, "invoker_id required")
	}

	now := s.now()
	// session_id is owned by the platform: when the caller doesn't
	// supply one (fresh conversation) we mint it here so the launch-
	// time session subdirectory is stable. Workers may report an
	// SDK-internal session id via CompleteRun, but the directory key
	// is the platform's id, fixed at this point.
	sessionID := req.GetSessionId()
	if sessionID == "" {
		sessionID = "sess_" + s.newID()
	}
	run := &nsv1.Run{
		Id:          s.newID(),
		Prompt:      req.GetPrompt(),
		Status:      nsv1.RunStatus_RUN_STATUS_PENDING,
		SessionId:   sessionID,
		UserId:      req.GetUserId(),
		InvokerType: invoker,
		InvokerId:   req.GetInvokerId(),
		EventCount:  0,
		Usage:       &nsv1.RunUsage{},
		CreatedAt:   timestamppb.New(now),
	}

	rec, err := runToRecord(run, false)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build run record: %v", err)
	}
	zero := int64(0)
	persisted, err := s.Records.Put(ctx, rec, &zero, req.GetIdempotencyKey())
	if err != nil {
		return nil, recordErr(err)
	}
	run.CreatedAt = timestamppb.New(persisted.CreatedAt)
	s.metrics.RunCreated(invoker)

	// Mint worker credential and launch.
	credExp := now.Add(s.defaultActiveDeadline()).Add(s.CredentialTTLBuffer)
	credential := runtime.MintCredential(s.WorkerHMAC, run.GetId(), credExp)
	spec := s.buildLaunchSpec(run, credential)

	// Resume bookkeeping: when the caller supplied a non-empty
	// session_id, look up the SDK-internal session id reported by the
	// most recent terminal run for that session and forward it to the
	// worker as NS_SDK_SESSION_ID. Best-effort — a lookup failure
	// shouldn't block run creation; the worker just starts a fresh
	// SDK session.
	if req.GetSessionId() != "" {
		if sdkID, err := LookupResumeSDKSessionID(ctx, s.Records, req.GetSessionId()); err != nil {
			s.Logger.Warn("resume sdk-session-id lookup failed",
				"session_id", req.GetSessionId(), "err", err)
		} else {
			spec.SDKSessionID = sdkID
		}
	}

	// Transition PENDING -> RUNNING before launching so subscribers
	// don't race the worker.
	run.Status = nsv1.RunStatus_RUN_STATUS_RUNNING
	run.StartedAt = timestamppb.New(s.now())
	if _, err := s.putRun(ctx, run, false, &persisted.Version); err != nil {
		return nil, recordErr(err)
	}

	if err := s.Launcher.Launch(ctx, spec); err != nil {
		// Mark run as ERROR. Best-effort — if this Put fails, the
		// stale-run recovery path will clean up on next startup.
		s.Logger.Error("launcher.Launch failed", "run_id", run.GetId(), "err", err)
		run.Status = nsv1.RunStatus_RUN_STATUS_ERROR
		run.Error = "launch failed: " + err.Error()
		run.EndedAt = timestamppb.New(s.now())
		_, _ = s.putRun(ctx, run, false, nil)
		return nil, status.Errorf(codes.Internal, "launcher: %v", err)
	}

	s.Logger.Info("run created", "run_id", run.GetId(), "user_id", run.GetUserId(), "invoker", invoker.String())
	return &nsv1.CreateRunResponse{Run: run}, nil
}

func (s *Service) GetRun(ctx context.Context, req *nsv1.GetRunRequest) (*nsv1.GetRunResponse, error) {
	rec, err := s.Records.Get(ctx, runsCollection, req.GetRunId())
	if err != nil {
		return nil, recordErr(err)
	}
	run, _, err := recordToRun(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decode run: %v", err)
	}
	return &nsv1.GetRunResponse{Run: run}, nil
}

func (s *Service) ListRuns(ctx context.Context, req *nsv1.ListRunsRequest) (*nsv1.ListRunsResponse, error) {
	filters := map[string]string{}
	if v := req.GetUserId(); v != "" {
		filters[attrUserID] = v
	}
	if req.GetStatus() != nsv1.RunStatus_RUN_STATUS_UNSPECIFIED {
		filters[attrStatus] = req.GetStatus().String()
	}
	if v := req.GetSessionId(); v != "" {
		filters[attrSessionID] = v
	}
	if req.GetInvokerType() != nsv1.InvokerType_INVOKER_TYPE_UNSPECIFIED {
		filters[attrInvokerType] = req.GetInvokerType().String()
	}
	if v := req.GetInvokerId(); v != "" {
		filters[attrInvokerID] = v
	}

	page, next, err := s.Records.List(ctx, records.ListQuery{
		Collection:       runsCollection,
		AttributeFilters: filters,
		PageSize:         req.GetPageSize(),
		PageToken:        req.GetPageToken(),
		OrderBy:          req.GetOrderBy(),
	})
	if err != nil {
		return nil, recordErr(err)
	}
	out := &nsv1.ListRunsResponse{NextPageToken: next}
	for i := range page {
		r, _, err := recordToRun(page[i])
		if err != nil {
			return nil, status.Errorf(codes.Internal, "decode run: %v", err)
		}
		out.Runs = append(out.Runs, r)
	}
	return out, nil
}

func (s *Service) InterruptRun(ctx context.Context, req *nsv1.InterruptRunRequest) (*nsv1.InterruptRunResponse, error) {
	rec, err := s.Records.Get(ctx, runsCollection, req.GetRunId())
	if err != nil {
		return nil, recordErr(err)
	}
	run, cancelled, err := recordToRun(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decode run: %v", err)
	}
	if isTerminal(run.GetStatus()) {
		return nil, status.Errorf(codes.FailedPrecondition, "run already %s", run.GetStatus().String())
	}

	// Record the cancellation flag. Worker polls GetRunCancellation
	// and drains cooperatively.
	if !cancelled {
		if _, err := s.putRun(ctx, run, true, &rec.Version); err != nil {
			return nil, recordErr(err)
		}
	}
	// Fire-and-forget hard-stop path. Failures here don't fail the
	// RPC — the cancellation flag is authoritative.
	if err := s.Launcher.Interrupt(ctx, run.GetId()); err != nil {
		s.Logger.Warn("launcher.Interrupt failed", "run_id", run.GetId(), "err", err)
	}

	// Return the run as observed right now. The status transition to
	// INTERRUPTED happens when the worker's terminal RPC lands.
	fresh, err := s.getRun(ctx, run.GetId())
	if err != nil {
		return nil, err
	}
	return &nsv1.InterruptRunResponse{Run: fresh}, nil
}

func (s *Service) DeleteSession(ctx context.Context, req *nsv1.DeleteSessionRequest) (*nsv1.DeleteSessionResponse, error) {
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id required")
	}
	// First scan: reject if any matching run is still active.
	var all []records.Record
	pageToken := ""
	for {
		page, next, err := s.Records.List(ctx, records.ListQuery{
			Collection:       runsCollection,
			AttributeFilters: map[string]string{attrSessionID: req.GetSessionId()},
			PageSize:         500,
			PageToken:        pageToken,
		})
		if err != nil {
			return nil, recordErr(err)
		}
		all = append(all, page...)
		if next == "" {
			break
		}
		pageToken = next
	}

	for _, rec := range all {
		st := rec.Attributes[attrStatus]
		if st == nsv1.RunStatus_RUN_STATUS_PENDING.String() || st == nsv1.RunStatus_RUN_STATUS_RUNNING.String() {
			return nil, status.Errorf(codes.FailedPrecondition,
				"session %s has active run %s (%s); interrupt first",
				req.GetSessionId(), rec.Key, st)
		}
	}

	// Cascade per-session state cleanup BEFORE deleting records so
	// that a failed cleanup leaves the index intact and a retry can
	// resume. If there are no records the cleaner is skipped — there
	// is no user_id to cascade against, and a session with no runs
	// has no platform-managed state to clean.
	if len(all) > 0 {
		userID := all[0].Attributes[attrUserID]
		if userID != "" {
			if err := s.SessionStateCleaner.Clean(ctx, userID, req.GetSessionId()); err != nil {
				return nil, status.Errorf(codes.Internal, "session-state cleanup: %v", err)
			}
		}
	}

	deleted := int64(0)
	for _, rec := range all {
		if err := s.Records.Delete(ctx, runsCollection, rec.Key, nil); err != nil {
			if errors.Is(err, records.ErrNotFound) {
				continue
			}
			return nil, recordErr(err)
		}
		deleted++
	}
	return &nsv1.DeleteSessionResponse{RunsDeleted: deleted}, nil
}

// -----------------------------------------------------------------------------
// Inner surface — worker callbacks
// -----------------------------------------------------------------------------

const postEventMaxRetries = 3

// PostWorkerEvent is the worker's push channel. Assigns a monotonic
// index server-side, persists the event as a Record, broadcasts to
// live subscribers.
func (s *Service) PostWorkerEvent(ctx context.Context, req *nsv1.PostWorkerEventRequest) (*nsv1.PostWorkerEventResponse, error) {
	if req.GetRunId() == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id required")
	}
	if err := verifiers.RequireWorkerRunID(ctx, req.GetRunId()); err != nil {
		return nil, err
	}
	if req.GetEvent() == nil {
		return nil, status.Error(codes.InvalidArgument, "event required")
	}

	var assigned int64
	for attempt := 0; attempt < postEventMaxRetries; attempt++ {
		rec, err := s.Records.Get(ctx, runsCollection, req.GetRunId())
		if err != nil {
			return nil, recordErr(err)
		}
		run, cancelled, err := recordToRun(rec)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "decode run: %v", err)
		}
		if isTerminal(run.GetStatus()) {
			return nil, status.Errorf(codes.FailedPrecondition, "run is %s", run.GetStatus().String())
		}

		// Clone so we don't mutate the caller's event.
		ev := &nsv1.StreamEvent{
			Index:     run.GetEventCount(),
			Type:      req.GetEvent().GetType(),
			Timestamp: req.GetEvent().GetTimestamp(),
			Raw:       req.GetEvent().GetRaw(),
		}
		if ev.Timestamp == nil {
			ev.Timestamp = timestamppb.New(s.now())
		}

		evRec, err := eventToRecord(req.GetRunId(), ev)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encode event: %v", err)
		}
		zero := int64(0)
		if _, err := s.Records.Put(ctx, evRec, &zero, ""); err != nil {
			if errors.Is(err, records.ErrVersionConflict) {
				// Slot already taken by a concurrent write; retry.
				continue
			}
			return nil, recordErr(err)
		}

		run.EventCount = ev.GetIndex() + 1
		if _, err := s.putRun(ctx, run, cancelled, &rec.Version); err != nil {
			if !errors.Is(err, records.ErrVersionConflict) {
				return nil, recordErr(err)
			}
			// Run record was updated concurrently (e.g. cancelled
			// attribute flipped). The event is persisted; the hint is
			// slightly stale; the next call will observe the true
			// event_count because the event-record PUT uses
			// ifVersion=0 and collides on the stale index.
		}

		s.broadcaster.Publish(req.GetRunId(), ev)
		assigned = ev.GetIndex()
		return &nsv1.PostWorkerEventResponse{Index: assigned}, nil
	}
	return nil, status.Error(codes.Internal, "event_count contention; exceeded retries")
}

// CompleteRun finalizes a run as COMPLETED. Idempotent.
func (s *Service) CompleteRun(ctx context.Context, req *nsv1.CompleteRunRequest) (*nsv1.CompleteRunResponse, error) {
	if err := verifiers.RequireWorkerRunID(ctx, req.GetRunId()); err != nil {
		return nil, err
	}
	return &nsv1.CompleteRunResponse{}, s.finalize(ctx, req.GetRunId(), nsv1.RunStatus_RUN_STATUS_COMPLETED, "", req.GetSessionId(), req.GetUsage())
}

// FailRun finalizes a run as ERROR. Idempotent.
func (s *Service) FailRun(ctx context.Context, req *nsv1.FailRunRequest) (*nsv1.FailRunResponse, error) {
	if err := verifiers.RequireWorkerRunID(ctx, req.GetRunId()); err != nil {
		return nil, err
	}
	return &nsv1.FailRunResponse{}, s.finalize(ctx, req.GetRunId(), nsv1.RunStatus_RUN_STATUS_ERROR, req.GetError(), "", nil)
}

// finalize implements CompleteRun/FailRun. A run already in a
// terminal state returns nil (idempotent per workers.md §8). If the
// run has the cancelled attribute set, the terminal state is
// overridden to RUN_STATUS_INTERRUPTED.
func (s *Service) finalize(ctx context.Context, runID string, target nsv1.RunStatus, errorMsg, sessionID string, usage *nsv1.RunUsage) error {
	rec, err := s.Records.Get(ctx, runsCollection, runID)
	if err != nil {
		return recordErr(err)
	}
	run, cancelled, err := recordToRun(rec)
	if err != nil {
		return status.Errorf(codes.Internal, "decode run: %v", err)
	}
	if isTerminal(run.GetStatus()) {
		return nil // idempotent
	}
	finalStatus := target
	if cancelled && target == nsv1.RunStatus_RUN_STATUS_COMPLETED {
		finalStatus = nsv1.RunStatus_RUN_STATUS_INTERRUPTED
	}
	run.Status = finalStatus
	run.EndedAt = timestamppb.New(s.now())
	// NOTE: sessionID here is the SDK-internal session id reported by
	// the worker on its way out. workers.md §4 says it MUST NOT appear
	// on the outer (user-facing) surface — the platform-owned
	// Run.SessionId stays untouched. Persist the SDK id as a Record
	// attribute (attrSDKSessionID) so LookupResumeSDKSessionID can
	// inject it on follow-up runs of the same session via the
	// NS_SDK_SESSION_ID env var.
	if usage != nil {
		run.Usage = usage
	}
	if errorMsg != "" {
		run.Error = errorMsg
	}
	newRec, err := runToRecord(run, cancelled)
	if err != nil {
		return status.Errorf(codes.Internal, "build run record: %v", err)
	}
	// Preserve any existing SDK id on the record (a no-op if absent), then
	// overwrite with the worker's reported value when non-empty.
	if v := rec.Attributes[attrSDKSessionID]; v != "" {
		newRec.Attributes[attrSDKSessionID] = v
	}
	if sessionID != "" {
		newRec.Attributes[attrSDKSessionID] = sessionID
	}
	v := rec.Version
	if _, err := s.Records.Put(ctx, newRec, &v, ""); err != nil {
		return recordErr(err)
	}
	// Chunk-18 metrics: record terminal transition. Duration is
	// end - created (start may be zero on runs that error before launch).
	duration := run.GetEndedAt().AsTime().Sub(run.GetCreatedAt().AsTime())
	s.metrics.RunCompleted(finalStatus, run.GetInvokerType(), duration, run.GetUsage())
	// Close broadcaster AFTER the run transition is persisted so any
	// final event posted just before the terminal call races with us
	// and still delivers cleanly.
	s.broadcaster.CloseRun(runID)
	return nil
}

// GetRunCancellation lets the worker poll for cooperative cancellation.
func (s *Service) GetRunCancellation(ctx context.Context, req *nsv1.GetRunCancellationRequest) (*nsv1.GetRunCancellationResponse, error) {
	if err := verifiers.RequireWorkerRunID(ctx, req.GetRunId()); err != nil {
		return nil, err
	}
	rec, err := s.Records.Get(ctx, runsCollection, req.GetRunId())
	if err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.GetRunCancellationResponse{Cancelled: rec.Attributes[attrCancelled] == "true"}, nil
}

// -----------------------------------------------------------------------------
// Events surface
// -----------------------------------------------------------------------------

// ListRunEvents returns a page of persisted events for a run.
func (s *Service) ListRunEvents(ctx context.Context, req *nsv1.ListRunEventsRequest) (*nsv1.ListRunEventsResponse, error) {
	page, next, err := s.Records.List(ctx, records.ListQuery{
		Collection:       eventsCollection,
		AttributeFilters: map[string]string{attrRunID: req.GetRunId()},
		PageSize:         req.GetPageSize(),
		PageToken:        req.GetPageToken(),
		OrderBy:          "created_at asc",
	})
	if err != nil {
		return nil, recordErr(err)
	}
	out := &nsv1.ListRunEventsResponse{NextPageToken: next}
	for i := range page {
		ev, err := recordToEvent(page[i])
		if err != nil {
			return nil, status.Errorf(codes.Internal, "decode event: %v", err)
		}
		if ev.GetIndex() < req.GetFromIndex() {
			continue
		}
		out.Events = append(out.Events, ev)
	}
	return out, nil
}

// StreamRunEvents is the server-streaming RPC. Subscribes first so
// live events during replay aren't missed, replays persisted events
// from the store, then fan-outs live events until the run terminates
// (broadcaster.Close causes the subscription channel to close).
func (s *Service) StreamRunEvents(req *nsv1.StreamRunEventsRequest, stream nsv1.Workers_StreamRunEventsServer) error {
	if req.GetRunId() == "" {
		return status.Error(codes.InvalidArgument, "run_id required")
	}

	ch, unsub := s.broadcaster.Subscribe(req.GetRunId())
	defer unsub()

	var lastSent int64 = -1
	if req.GetFromIndex() > 0 {
		lastSent = req.GetFromIndex() - 1
	}

	// Replay persisted events.
	if req.GetIncludeHistory() || !req.GetIncludeHistory() /* default true */ {
		pageToken := ""
		for {
			page, next, err := s.Records.List(stream.Context(), records.ListQuery{
				Collection:       eventsCollection,
				AttributeFilters: map[string]string{attrRunID: req.GetRunId()},
				PageSize:         500,
				PageToken:        pageToken,
				OrderBy:          "created_at asc",
			})
			if err != nil {
				return recordErr(err)
			}
			for i := range page {
				ev, err := recordToEvent(page[i])
				if err != nil {
					return status.Errorf(codes.Internal, "decode event: %v", err)
				}
				if ev.GetIndex() <= lastSent {
					continue
				}
				if err := stream.Send(&nsv1.StreamRunEventsResponse{Event: ev}); err != nil {
					return err
				}
				lastSent = ev.GetIndex()
			}
			if next == "" {
				break
			}
			pageToken = next
		}
	}

	// Check whether the run has already reached terminal state — if
	// so, no more events are coming; close the stream cleanly.
	r, err := s.getRun(stream.Context(), req.GetRunId())
	if err != nil {
		return err
	}
	if isTerminal(r.GetStatus()) {
		return nil
	}

	// Drain live subscription.
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil // broadcaster closed → terminal
			}
			if ev.GetIndex() <= lastSent {
				continue
			}
			if err := stream.Send(&nsv1.StreamRunEventsResponse{Event: ev}); err != nil {
				return err
			}
			lastSent = ev.GetIndex()
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// putRun serializes run + persists with the given cancelled flag. If
// ifVersion is non-nil, performs an optimistic-concurrency check.
func (s *Service) putRun(ctx context.Context, run *nsv1.Run, cancelled bool, ifVersion *int64) (records.Record, error) {
	rec, err := runToRecord(run, cancelled)
	if err != nil {
		return records.Record{}, err
	}
	return s.Records.Put(ctx, rec, ifVersion, "")
}

func (s *Service) getRun(ctx context.Context, runID string) (*nsv1.Run, error) {
	rec, err := s.Records.Get(ctx, runsCollection, runID)
	if err != nil {
		return nil, recordErr(err)
	}
	r, _, err := recordToRun(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decode run: %v", err)
	}
	return r, nil
}

// recordErr maps internal/records sentinel errors to gRPC codes.
func recordErr(err error) error {
	switch {
	case errors.Is(err, records.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, records.ErrVersionConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, records.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	}
	return status.Errorf(codes.Internal, "%s", err.Error())
}
