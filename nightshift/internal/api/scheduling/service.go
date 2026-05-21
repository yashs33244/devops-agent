// Package scheduling implements nightshift.v1.Scheduling.
//
// The service is a thin layer over Records (one Record per Schedule
// in the `schedules` collection) plus a runtime.Scheduler that
// materializes schedules into a cron runtime (K8s CronJob in production,
// in-memory stub otherwise). cr0n-a's scheduler.py is the behavioral
// source of truth; the divergence is documented in chunk-17's row of
// MIGRATION.md.
//
// No event emission — chunks 15+16 set the parity precedent: cr0n
// doesn't emit lifecycle events, and the proto doesn't define schedule
// event types either.
package scheduling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/metrics"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// ServiceOptions configures a Service. Records + Scheduler + APIURL +
// FireImage + TokenSecret are required.
type ServiceOptions struct {
	Records     records.RecordStore
	Scheduler   runtime.Scheduler
	APIURL      string
	FireImage   string
	TokenSecret string
	Logger      *slog.Logger

	// Metrics is the chunk-18 recorder. Optional — nil-safe.
	Metrics metrics.Recorder

	// Test seams.
	NewID func() string
	Now   func() time.Time
}

// Service implements nsv1.SchedulingServer.
type Service struct {
	nsv1.UnimplementedSchedulingServer

	records     records.RecordStore
	scheduler   runtime.Scheduler
	apiURL      string
	fireImage   string
	tokenSecret string
	logger      *slog.Logger
	metrics     metrics.Recorder

	newID func() string
	now   func() time.Time
}

// NewService constructs a Service. Misconfiguration panics — wire-up
// errors should fail loudly at startup.
func NewService(opts ServiceOptions) *Service {
	if opts.Records == nil {
		panic("scheduling.NewService: Records required")
	}
	if opts.Scheduler == nil {
		panic("scheduling.NewService: Scheduler required")
	}
	if opts.APIURL == "" {
		panic("scheduling.NewService: APIURL required")
	}
	if opts.FireImage == "" {
		panic("scheduling.NewService: FireImage required")
	}
	if opts.TokenSecret == "" {
		panic("scheduling.NewService: TokenSecret required")
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
	return &Service{
		records:     opts.Records,
		scheduler:   opts.Scheduler,
		apiURL:      opts.APIURL,
		fireImage:   opts.FireImage,
		tokenSecret: opts.TokenSecret,
		logger:      logger,
		metrics:     metrics.Get(opts.Metrics),
		newID:       newID,
		now:         now,
	}
}

// -----------------------------------------------------------------------------
// CreateSchedule
// -----------------------------------------------------------------------------

func (s *Service) CreateSchedule(ctx context.Context, req *nsv1.CreateScheduleRequest) (*nsv1.CreateScheduleResponse, error) {
	if req.GetPrompt() == "" {
		return nil, status.Error(codes.InvalidArgument, "prompt required")
	}
	if req.GetCron() == "" {
		return nil, status.Error(codes.InvalidArgument, "cron required")
	}
	ownerID, err := s.callerOwner(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	tz := req.GetTimezone()
	if tz == "" {
		tz = "UTC"
	}

	id := "sch_" + s.newID()
	now := s.now()
	sch := &nsv1.Schedule{
		Id:           id,
		UserId:       ownerID,
		Prompt:       req.GetPrompt(),
		Cron:         req.GetCron(),
		Timezone:     tz,
		Enabled:      defaultEnabledOnCreate(req),
		SessionId:    req.GetSessionId(),
		AgentIds:     req.GetAgentIds(),
		SkillIds:     req.GetSkillIds(),
		ConnectorIds: req.GetConnectorIds(),
		CreatedAt:    timestamppb.New(now),
		UpdatedAt:    timestamppb.New(now),
	}

	rec, err := scheduleToRecord(sch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build schedule record: %v", err)
	}
	zero := int64(0)
	if _, err := s.records.Put(ctx, rec, &zero, req.GetIdempotencyKey()); err != nil {
		return nil, recordErr(err)
	}
	if err := s.scheduler.Apply(ctx, s.specFor(sch)); err != nil {
		// Roll back the Record so a failed Apply doesn't leave a phantom.
		_ = s.records.Delete(ctx, schedulesCollection, sch.GetId(), nil)
		return nil, status.Errorf(codes.Internal, "scheduler apply: %v", err)
	}
	s.metrics.ScheduleCreated()
	return &nsv1.CreateScheduleResponse{Schedule: sch}, nil
}

// defaultEnabledOnCreate returns the effective `enabled` for a Create
// request. The proto field is `optional bool`, so unset → true (the
// documented default) and an explicit `false` creates a paused schedule.
func defaultEnabledOnCreate(req *nsv1.CreateScheduleRequest) bool {
	if req.Enabled == nil {
		return true
	}
	return *req.Enabled
}

// -----------------------------------------------------------------------------
// GetSchedule
// -----------------------------------------------------------------------------

func (s *Service) GetSchedule(ctx context.Context, req *nsv1.GetScheduleRequest) (*nsv1.GetScheduleResponse, error) {
	if req.GetScheduleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id required")
	}
	sch, err := s.loadSchedule(ctx, req.GetScheduleId())
	if err != nil {
		return nil, err
	}
	ownerID, err := s.callerOwner(ctx, "")
	if err != nil {
		return nil, err
	}
	if sch.GetUserId() != ownerID {
		// Anti-leak: collapse miss-or-no-access into NOT_FOUND.
		return nil, notFound()
	}
	return &nsv1.GetScheduleResponse{Schedule: sch}, nil
}

// -----------------------------------------------------------------------------
// ListSchedules
// -----------------------------------------------------------------------------

func (s *Service) ListSchedules(ctx context.Context, req *nsv1.ListSchedulesRequest) (*nsv1.ListSchedulesResponse, error) {
	ownerID, err := s.callerOwner(ctx, "")
	if err != nil {
		return nil, err
	}
	wantOwner := req.GetUserId()
	if wantOwner == "" {
		wantOwner = ownerID
	}
	if wantOwner != ownerID {
		// Outer-surface callers may not list other users' schedules.
		return &nsv1.ListSchedulesResponse{}, nil
	}

	q := records.ListQuery{
		Collection: schedulesCollection,
		AttributeFilters: map[string]string{
			attrUserID: ownerID,
		},
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	}
	if req.Enabled != nil {
		q.AttributeFilters[attrEnabled] = boolAttr(*req.Enabled)
	}
	page, next, err := s.records.List(ctx, q)
	if err != nil {
		return nil, recordErr(err)
	}
	out := make([]*nsv1.Schedule, 0, len(page))
	for _, rec := range page {
		sch, err := recordToSchedule(rec)
		if err != nil {
			s.logger.Warn("scheduling: skipping malformed record", "key", rec.Key, "err", err)
			continue
		}
		out = append(out, sch)
	}
	return &nsv1.ListSchedulesResponse{Schedules: out, NextPageToken: next}, nil
}

// -----------------------------------------------------------------------------
// UpdateSchedule
// -----------------------------------------------------------------------------

func (s *Service) UpdateSchedule(ctx context.Context, req *nsv1.UpdateScheduleRequest) (*nsv1.UpdateScheduleResponse, error) {
	if req.GetScheduleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id required")
	}
	rec, err := s.records.Get(ctx, schedulesCollection, req.GetScheduleId())
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			return nil, notFound()
		}
		return nil, recordErr(err)
	}
	sch, err := recordToSchedule(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decode schedule: %v", err)
	}
	ownerID, err := s.callerOwner(ctx, "")
	if err != nil {
		return nil, err
	}
	if sch.GetUserId() != ownerID {
		return nil, notFound()
	}

	now := s.now()
	if req.Prompt != nil {
		sch.Prompt = *req.Prompt
	}
	if req.Cron != nil {
		sch.Cron = *req.Cron
	}
	if req.Timezone != nil {
		tz := *req.Timezone
		if tz == "" {
			tz = "UTC"
		}
		sch.Timezone = tz
	}
	if req.Enabled != nil {
		sch.Enabled = *req.Enabled
	}
	if req.GetSetSessionId() {
		sch.SessionId = req.GetSessionId()
	}
	if req.GetSetAgentIds() {
		sch.AgentIds = req.GetAgentIds()
	}
	if req.GetSetSkillIds() {
		sch.SkillIds = req.GetSkillIds()
	}
	if req.GetSetConnectorIds() {
		sch.ConnectorIds = req.GetConnectorIds()
	}
	sch.UpdatedAt = timestamppb.New(now)

	updated, err := scheduleToRecord(sch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build schedule record: %v", err)
	}
	updated.Version = rec.Version
	if _, err := s.records.Put(ctx, updated, &rec.Version, ""); err != nil {
		return nil, recordErr(err)
	}
	// Re-register the cron runtime. cron/timezone/enabled mutations
	// MUST be reflected before return per scheduling.md §3.
	if err := s.scheduler.Apply(ctx, s.specFor(sch)); err != nil {
		return nil, status.Errorf(codes.Internal, "scheduler apply: %v", err)
	}
	return &nsv1.UpdateScheduleResponse{Schedule: sch}, nil
}

// -----------------------------------------------------------------------------
// DeleteSchedule
// -----------------------------------------------------------------------------

func (s *Service) DeleteSchedule(ctx context.Context, req *nsv1.DeleteScheduleRequest) (*nsv1.DeleteScheduleResponse, error) {
	if req.GetScheduleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id required")
	}
	sch, err := s.loadSchedule(ctx, req.GetScheduleId())
	if err != nil {
		return nil, err
	}
	ownerID, err := s.callerOwner(ctx, "")
	if err != nil {
		return nil, err
	}
	if sch.GetUserId() != ownerID {
		return nil, notFound()
	}

	// Tear down cron runtime BEFORE the Record so a stuck CronJob can't
	// outlive its schedule. Idempotent on NotFound.
	if err := s.scheduler.Delete(ctx, sch.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "scheduler delete: %v", err)
	}
	if err := s.records.Delete(ctx, schedulesCollection, sch.GetId(), nil); err != nil {
		return nil, recordErr(err)
	}
	s.metrics.ScheduleDeleted()
	return &nsv1.DeleteScheduleResponse{}, nil
}

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------

// callerOwner resolves the user_id this caller is acting as. Outer-
// surface callers (User/Service) act as themselves; they may not
// impersonate another user via reqUserID. Worker scheme is not
// permitted on the Scheduling surface (verifiers.policy enforces).
func (s *Service) callerOwner(ctx context.Context, reqUserID string) (string, error) {
	p := verifiers.FromContext(ctx)
	if p == nil {
		return "", status.Error(codes.Unauthenticated, "missing principal")
	}
	if reqUserID != "" && reqUserID != p.ID {
		return "", status.Error(codes.PermissionDenied, "cannot act on behalf of another user")
	}
	return p.ID, nil
}

func (s *Service) loadSchedule(ctx context.Context, id string) (*nsv1.Schedule, error) {
	rec, err := s.records.Get(ctx, schedulesCollection, id)
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			return nil, notFound()
		}
		return nil, recordErr(err)
	}
	return recordToSchedule(rec)
}

// specFor builds the runtime.ScheduleSpec for sch using the Service's
// operator-supplied fire-time configuration.
func (s *Service) specFor(sch *nsv1.Schedule) runtime.ScheduleSpec {
	return runtime.ScheduleSpec{
		ID:          sch.GetId(),
		UserID:      sch.GetUserId(),
		Prompt:      sch.GetPrompt(),
		Cron:        sch.GetCron(),
		Timezone:    sch.GetTimezone(),
		Enabled:     sch.GetEnabled(),
		SessionID:   sch.GetSessionId(),
		APIURL:      s.apiURL,
		FireImage:   s.fireImage,
		TokenSecret: s.tokenSecret,
	}
}

// listAllSchedules paginates the entire `schedules` collection. Used
// by the reconciler.
func (s *Service) listAllSchedules(ctx context.Context) ([]*nsv1.Schedule, error) {
	var out []*nsv1.Schedule
	var token string
	for {
		page, next, err := s.records.List(ctx, records.ListQuery{
			Collection: schedulesCollection,
			PageSize:   500,
			PageToken:  token,
		})
		if err != nil {
			return nil, err
		}
		for _, rec := range page {
			sch, err := recordToSchedule(rec)
			if err != nil {
				s.logger.Warn("scheduling: skipping malformed record", "key", rec.Key, "err", err)
				continue
			}
			out = append(out, sch)
		}
		if next == "" {
			break
		}
		token = next
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// Marshal helpers
// -----------------------------------------------------------------------------

func scheduleToRecord(sch *nsv1.Schedule) (records.Record, error) {
	if sch.GetId() == "" {
		return records.Record{}, errors.New("scheduling: Schedule.id required")
	}
	data, err := proto.Marshal(sch)
	if err != nil {
		return records.Record{}, fmt.Errorf("marshal schedule: %w", err)
	}
	return records.Record{
		Collection: schedulesCollection,
		Key:        sch.GetId(),
		Attributes: map[string]string{
			attrUserID:  sch.GetUserId(),
			attrEnabled: boolAttr(sch.GetEnabled()),
			attrCron:    sch.GetCron(),
		},
		Data:        data,
		ContentType: recordContentType,
	}, nil
}

func recordToSchedule(rec records.Record) (*nsv1.Schedule, error) {
	sch := &nsv1.Schedule{}
	if err := proto.Unmarshal(rec.Data, sch); err != nil {
		return nil, fmt.Errorf("unmarshal schedule: %w", err)
	}
	return sch, nil
}

// -----------------------------------------------------------------------------
// Misc helpers
// -----------------------------------------------------------------------------

func notFound() error {
	return status.Error(codes.NotFound, "schedule not found")
}

func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func recordErr(err error) error {
	switch {
	case errors.Is(err, records.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, records.ErrVersionConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, records.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
