package scheduling

import (
	"context"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// fakeScheduler records what the service applies/deletes/lists,
// without touching K8s. Implements runtime.Scheduler.
type fakeScheduler struct {
	mu      sync.Mutex
	specs   map[string]runtime.ScheduleSpec
	applied []runtime.ScheduleSpec
	deleted []string

	// failApply forces Apply to return an error for the next call.
	failApply error
}

func newFakeScheduler() *fakeScheduler {
	return &fakeScheduler{specs: map[string]runtime.ScheduleSpec{}}
}

func (f *fakeScheduler) Apply(_ context.Context, spec runtime.ScheduleSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failApply != nil {
		err := f.failApply
		f.failApply = nil
		return err
	}
	f.specs[spec.ID] = spec
	f.applied = append(f.applied, spec)
	return nil
}

func (f *fakeScheduler) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.specs, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeScheduler) List(_ context.Context) ([]runtime.ManagedSchedule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]runtime.ManagedSchedule, 0, len(f.specs))
	for id, sp := range f.specs {
		out = append(out, runtime.ManagedSchedule{ID: id, Suspend: !sp.Enabled})
	}
	return out, nil
}

func (f *fakeScheduler) Close() error { return nil }

func newTestService(t *testing.T) (*Service, *fakeScheduler) {
	t.Helper()
	rec, err := records.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rec.Close() })
	sched := newFakeScheduler()
	s := NewService(ServiceOptions{
		Records:     rec,
		Scheduler:   sched,
		APIURL:      "http://api.svc:8080",
		FireImage:   "curlimages/curl:latest",
		TokenSecret: "test-token",
	})
	return s, sched
}

func userCtx(id string) context.Context {
	return verifiers.WithPrincipal(context.Background(), &verifiers.Principal{Scheme: verifiers.SchemeUser, ID: id})
}

func mustCreate(t *testing.T, s *Service, ctx context.Context, prompt, cron string) *nsv1.Schedule {
	t.Helper()
	resp, err := s.CreateSchedule(ctx, &nsv1.CreateScheduleRequest{
		Prompt: prompt,
		Cron:   cron,
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	return resp.GetSchedule()
}

func TestCreateScheduleHappyPath(t *testing.T) {
	s, sched := newTestService(t)
	ctx := userCtx("alice")
	got := mustCreate(t, s, ctx, "good morning", "0 9 * * *")
	if got.GetUserId() != "alice" {
		t.Fatalf("user_id=%q", got.GetUserId())
	}
	if got.GetTimezone() != "UTC" {
		t.Fatalf("timezone default=%q", got.GetTimezone())
	}
	// Regression for #169: enabled is unset on the request and must
	// default to true so the schedule actually fires.
	if !got.GetEnabled() {
		t.Fatal("enabled default-on: schedule came back disabled")
	}
	if len(sched.applied) != 1 || sched.applied[0].ID != got.GetId() {
		t.Fatalf("scheduler.Apply not called once: %+v", sched.applied)
	}
	if sched.applied[0].Enabled != true {
		t.Fatalf("cron runtime got Enabled=false; CronJob would be Suspend=true")
	}
}

// Regression for #169: an explicit `enabled: false` on Create still
// produces a paused schedule — the default-on convenience must not
// override caller intent.
func TestCreateScheduleEnabledFalseExplicit(t *testing.T) {
	s, sched := newTestService(t)
	ctx := userCtx("alice")
	enabled := false
	resp, err := s.CreateSchedule(ctx, &nsv1.CreateScheduleRequest{
		Prompt:  "paused at birth",
		Cron:    "0 9 * * *",
		Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}
	if resp.GetSchedule().GetEnabled() {
		t.Fatal("enabled=false on request did not stick")
	}
	if sched.applied[0].Enabled {
		t.Fatal("cron runtime got Enabled=true; expected the CronJob to be suspended")
	}
}

func TestCreateScheduleRollsBackOnApplyError(t *testing.T) {
	s, sched := newTestService(t)
	sched.failApply = ErrIntentionalFailure
	_, err := s.CreateSchedule(userCtx("alice"), &nsv1.CreateScheduleRequest{
		Prompt: "x", Cron: "* * * * *",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("want Internal, got %v", err)
	}
	// Record should NOT exist after rollback.
	list, lerr := s.ListSchedules(userCtx("alice"), &nsv1.ListSchedulesRequest{})
	if lerr != nil {
		t.Fatal(lerr)
	}
	if len(list.GetSchedules()) != 0 {
		t.Fatalf("rollback didn't clean up: %+v", list.GetSchedules())
	}
}

func TestGetScheduleHidesOthers(t *testing.T) {
	s, _ := newTestService(t)
	a := mustCreate(t, s, userCtx("alice"), "x", "* * * * *")

	if _, err := s.GetSchedule(userCtx("bob"), &nsv1.GetScheduleRequest{ScheduleId: a.GetId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("bob get-alice: want NotFound, got %v", err)
	}
}

func TestListSchedulesScopedToCaller(t *testing.T) {
	s, _ := newTestService(t)
	mustCreate(t, s, userCtx("alice"), "a1", "* * * * *")
	mustCreate(t, s, userCtx("alice"), "a2", "* * * * *")
	mustCreate(t, s, userCtx("bob"), "b1", "* * * * *")

	resp, err := s.ListSchedules(userCtx("alice"), &nsv1.ListSchedulesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetSchedules()) != 2 {
		t.Fatalf("alice list=%d", len(resp.GetSchedules()))
	}

	// Asking for someone else's schedules collapses to empty (no leak).
	resp, _ = s.ListSchedules(userCtx("alice"), &nsv1.ListSchedulesRequest{UserId: "bob"})
	if len(resp.GetSchedules()) != 0 {
		t.Fatalf("cross-owner leak=%d", len(resp.GetSchedules()))
	}
}

func TestUpdateScheduleReappliesOnCronChange(t *testing.T) {
	s, sched := newTestService(t)
	ctx := userCtx("alice")
	a := mustCreate(t, s, ctx, "p", "0 9 * * *")
	beforeApplied := len(sched.applied)

	newCron := "0 10 * * *"
	resp, err := s.UpdateSchedule(ctx, &nsv1.UpdateScheduleRequest{
		ScheduleId: a.GetId(),
		Cron:       &newCron,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if resp.GetSchedule().GetCron() != newCron {
		t.Fatalf("cron=%q", resp.GetSchedule().GetCron())
	}
	if len(sched.applied) != beforeApplied+1 {
		t.Fatalf("Apply not called on cron change: %d → %d", beforeApplied, len(sched.applied))
	}
}

func TestUpdateScheduleSuspendOnEnabledFlip(t *testing.T) {
	s, sched := newTestService(t)
	ctx := userCtx("alice")
	a := mustCreate(t, s, ctx, "p", "* * * * *")

	enabled := false
	if _, err := s.UpdateSchedule(ctx, &nsv1.UpdateScheduleRequest{
		ScheduleId: a.GetId(),
		Enabled:    &enabled,
	}); err != nil {
		t.Fatal(err)
	}
	// fakeScheduler reflects suspend via List.
	got, _ := sched.List(context.Background())
	if len(got) != 1 || !got[0].Suspend {
		t.Fatalf("expected suspended after enabled=false: %+v", got)
	}
}

func TestDeleteScheduleCascades(t *testing.T) {
	s, sched := newTestService(t)
	ctx := userCtx("alice")
	a := mustCreate(t, s, ctx, "doomed", "* * * * *")
	if _, err := s.DeleteSchedule(ctx, &nsv1.DeleteScheduleRequest{ScheduleId: a.GetId()}); err != nil {
		t.Fatal(err)
	}
	if len(sched.deleted) != 1 || sched.deleted[0] != a.GetId() {
		t.Fatalf("scheduler.Delete: %+v", sched.deleted)
	}
	// Get-after-delete → 404.
	if _, err := s.GetSchedule(ctx, &nsv1.GetScheduleRequest{ScheduleId: a.GetId()}); status.Code(err) != codes.NotFound {
		t.Fatalf("post-delete get: want NotFound, got %v", err)
	}
}

func TestUpdateForeignSchedule(t *testing.T) {
	s, _ := newTestService(t)
	a := mustCreate(t, s, userCtx("alice"), "x", "* * * * *")
	prompt := "hijacked"
	if _, err := s.UpdateSchedule(userCtx("bob"), &nsv1.UpdateScheduleRequest{
		ScheduleId: a.GetId(),
		Prompt:     &prompt,
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("bob update alice: want NotFound, got %v", err)
	}
}

// ── Reconciler ─────────────────────────────────────────────────────

func TestReconcileApplyForEachRecordReapsOrphans(t *testing.T) {
	s, sched := newTestService(t)
	ctx := userCtx("alice")

	// 1 schedule from records-side.
	a := mustCreate(t, s, ctx, "p", "* * * * *")

	// Inject an orphan into the scheduler that has no corresponding Record.
	orphanSpec := runtime.ScheduleSpec{
		ID:          "sch_orphan",
		UserID:      "alice",
		Prompt:      "ghost",
		Cron:        "* * * * *",
		Enabled:     true,
		APIURL:      "http://x",
		FireImage:   "curlimages/curl:latest",
		TokenSecret: "tok",
	}
	if err := sched.Apply(context.Background(), orphanSpec); err != nil {
		t.Fatal(err)
	}

	res, err := s.ReconcileSchedules(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Applied != 1 {
		t.Fatalf("applied=%d, want 1", res.Applied)
	}
	if res.Reaped != 1 {
		t.Fatalf("reaped=%d, want 1", res.Reaped)
	}

	// Verify the live record's CronJob remains.
	got, _ := sched.List(context.Background())
	if len(got) != 1 || got[0].ID != a.GetId() {
		t.Fatalf("post-reconcile state=%+v, want only %s", got, a.GetId())
	}
}

// ErrIntentionalFailure is the sentinel returned by fakeScheduler when
// failApply is set. Exported so tests can match against it.
var ErrIntentionalFailure = &intentionalErr{}

type intentionalErr struct{}

func (*intentionalErr) Error() string { return "intentional failure" }
