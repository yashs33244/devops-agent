package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newFakeScheduler(t *testing.T) *KubernetesScheduler {
	t.Helper()
	s, err := NewKubernetesScheduler(fake.NewSimpleClientset(), "nightshift-test")
	if err != nil {
		t.Fatalf("NewKubernetesScheduler: %v", err)
	}
	return s
}

func sampleScheduleSpec() ScheduleSpec {
	return ScheduleSpec{
		ID:          "sch_a36196f5-894d-477d-8862-1e900a58ac19",
		UserID:      "alice",
		Prompt:      "good morning",
		Cron:        "*/5 * * * *",
		Timezone:    "UTC",
		Enabled:     true,
		APIURL:      "http://api.svc:8080",
		FireImage:   "curlimages/curl:latest",
		TokenSecret: "scheduler-token",
	}
}

func TestSchedulerApplyCreatesCronJob(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	spec := sampleScheduleSpec()
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cj, err := k.Client.BatchV1().CronJobs(k.Namespace).Get(ctx, scheduleResourceName(spec.ID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get cronjob: %v", err)
	}
	if cj.Spec.Schedule != spec.Cron {
		t.Fatalf("schedule=%q", cj.Spec.Schedule)
	}
	if cj.Spec.TimeZone == nil || *cj.Spec.TimeZone != "UTC" {
		t.Fatalf("timezone=%v", cj.Spec.TimeZone)
	}
	if string(cj.Spec.ConcurrencyPolicy) != "Forbid" {
		t.Fatalf("concurrencyPolicy=%v", cj.Spec.ConcurrencyPolicy)
	}
	if cj.Spec.Suspend == nil || *cj.Spec.Suspend {
		t.Fatalf("suspend should be false for enabled spec")
	}
	if id := cj.Labels[scheduleIDLabel]; id == "" {
		t.Fatalf("schedule-id label missing: %v", cj.Labels)
	}

	// Container shape.
	pod := cj.Spec.JobTemplate.Spec.Template.Spec
	if len(pod.Containers) != 1 {
		t.Fatalf("containers=%d", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.Image != spec.FireImage {
		t.Fatalf("image=%q", c.Image)
	}
	if !strings.Contains(strings.Join(c.Args, " "), "curl -fsSL -X POST") {
		t.Fatalf("args=%v", c.Args)
	}
	envMap := map[string]string{}
	for _, e := range c.Env {
		envMap[e.Name] = e.Value
	}
	if envMap["NS_API_INTERNAL_URL"] != spec.APIURL {
		t.Fatalf("NS_API_INTERNAL_URL=%q", envMap["NS_API_INTERNAL_URL"])
	}
	// Verify the payload JSON encodes the right invoker fields.
	payloadRaw := envMap["NS_SCHEDULER_PAYLOAD"]
	if payloadRaw == "" {
		t.Fatal("NS_SCHEDULER_PAYLOAD missing")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if payload["invoker_type"] != "INVOKER_TYPE_SCHEDULE" {
		t.Fatalf("invoker_type=%v", payload["invoker_type"])
	}
	if payload["invoker_id"] != spec.ID {
		t.Fatalf("invoker_id=%v", payload["invoker_id"])
	}
	if payload["user_id"] != spec.UserID {
		t.Fatalf("user_id=%v", payload["user_id"])
	}
	if payload["prompt"] != spec.Prompt {
		t.Fatalf("prompt=%v", payload["prompt"])
	}

	// Token comes from a Secret.
	var tokenEnvVar bool
	for _, e := range c.Env {
		if e.Name == "NS_SCHEDULER_TOKEN" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			tokenEnvVar = true
			if e.ValueFrom.SecretKeyRef.Name != spec.TokenSecret {
				t.Fatalf("token secret name=%q", e.ValueFrom.SecretKeyRef.Name)
			}
		}
	}
	if !tokenEnvVar {
		t.Fatal("NS_SCHEDULER_TOKEN not sourced from secretKeyRef")
	}

	// Hardening.
	if c.SecurityContext == nil || c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatal("readOnlyRootFilesystem must be true")
	}
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Fatal("automountSAToken must be false")
	}
}

func TestSchedulerApplyIsIdempotent(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	if err := k.Apply(ctx, sampleScheduleSpec()); err != nil {
		t.Fatal(err)
	}
	if err := k.Apply(ctx, sampleScheduleSpec()); err != nil {
		t.Fatalf("repeat apply: %v", err)
	}
}

func TestSchedulerApplyDisabledSuspends(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	spec := sampleScheduleSpec()
	spec.Enabled = false
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatal(err)
	}
	cj, err := k.Client.BatchV1().CronJobs(k.Namespace).Get(ctx, scheduleResourceName(spec.ID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Fatal("expected suspend=true on disabled spec")
	}
}

func TestSchedulerApplyToggleEnabledRoundTrip(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	spec := sampleScheduleSpec()
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatal(err)
	}
	spec.Enabled = false
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatal(err)
	}
	cj, _ := k.Client.BatchV1().CronJobs(k.Namespace).Get(ctx, scheduleResourceName(spec.ID), metav1.GetOptions{})
	if !*cj.Spec.Suspend {
		t.Fatal("suspend should flip to true")
	}
	spec.Enabled = true
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatal(err)
	}
	cj, _ = k.Client.BatchV1().CronJobs(k.Namespace).Get(ctx, scheduleResourceName(spec.ID), metav1.GetOptions{})
	if *cj.Spec.Suspend {
		t.Fatal("suspend should flip back to false")
	}
}

func TestSchedulerDeleteRemovesCronJob(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	spec := sampleScheduleSpec()
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if err := k.Delete(ctx, spec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Client.BatchV1().CronJobs(k.Namespace).Get(ctx, scheduleResourceName(spec.ID), metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
	if err := k.Delete(ctx, spec.ID); err != nil {
		t.Fatalf("repeat delete: %v", err)
	}
}

func TestSchedulerListReturnsManaged(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	spec := sampleScheduleSpec()
	if err := k.Apply(ctx, spec); err != nil {
		t.Fatal(err)
	}
	got, err := k.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != spec.ID {
		t.Fatalf("list=%+v", got)
	}
	if got[0].Suspend {
		t.Fatal("expected non-suspended for enabled spec")
	}
}

func TestSchedulerSpecValidation(t *testing.T) {
	k := newFakeScheduler(t)
	ctx := context.Background()
	bad := sampleScheduleSpec()
	bad.UserID = ""
	if err := k.Apply(ctx, bad); err == nil {
		t.Fatal("expected error for missing UserID")
	}
}

func TestStubScheduler(t *testing.T) {
	s := NewStubScheduler()
	ctx := context.Background()
	if err := s.Apply(ctx, sampleScheduleSpec()); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("list=%+v", got)
	}
	if err := s.Delete(ctx, sampleScheduleSpec().ID); err != nil {
		t.Fatal(err)
	}
	got, _ = s.List(ctx)
	if len(got) != 0 {
		t.Fatalf("after delete: %+v", got)
	}
}
