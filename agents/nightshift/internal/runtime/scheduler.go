package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// ScheduleSpec is the per-schedule input the Scheduler materializes
// into a cron runtime entry. The fire-time payload (prompt, user_id,
// invoker fields) is derived from these fields; APIURL / FireImage /
// TokenSecret are operator-configured and identical across all
// schedules in a deployment.
type ScheduleSpec struct {
	ID       string
	UserID   string
	Prompt   string
	Cron     string
	Timezone string
	Enabled  bool

	SessionID string

	// Operator-supplied. APIURL is the in-cluster URL the CronJob's
	// curl POSTs CreateRun to (e.g. http://nightshift-nightshift-api.nightshift.svc:8080).
	// FireImage is the container image with curl available.
	// TokenSecret is the K8s Secret name the bearer token lives in
	// (mounted as env var, key `token`).
	APIURL      string
	FireImage   string
	TokenSecret string
}

// ManagedSchedule is what Scheduler.List returns for the reconciler:
// just enough to set-diff against the Records-side view.
type ManagedSchedule struct {
	ID      string
	Suspend bool
}

// Scheduler is the runtime seam the Scheduling service uses to
// materialize schedules into a cron runtime. KubernetesScheduler maps
// each schedule onto a K8s CronJob; StubScheduler keeps an in-memory
// map for non-K8s runtime + tests without a Kubernetes API.
type Scheduler interface {
	Apply(ctx context.Context, spec ScheduleSpec) error
	Delete(ctx context.Context, scheduleID string) error
	List(ctx context.Context) ([]ManagedSchedule, error)
	Close() error
}

// KubernetesScheduler materializes each schedule as a K8s CronJob in
// a namespace the API has create/get/list/update/patch/delete
// permission on (deploy/charts/nightshift/templates/nightshift-api/rbac.yaml).
//
// Each CronJob runs a single curl-based pod that POSTs CreateRun
// to the in-cluster API service URL with the bearer token mounted
// from a K8s Secret. cr0n parity: matches scheduler.py:_build_cronjob
// modulo the OpenBao init-container fetch — chunk 17 keeps the bearer
// in a static Secret.
type KubernetesScheduler struct {
	Client    kubernetes.Interface
	Namespace string
}

// NewKubernetesScheduler constructs a scheduler against an existing
// clientset. Tests pass `fake.NewSimpleClientset()`.
func NewKubernetesScheduler(cs kubernetes.Interface, namespace string) (*KubernetesScheduler, error) {
	if cs == nil {
		return nil, errors.New("runtime: scheduler Client required")
	}
	if namespace == "" {
		return nil, errors.New("runtime: scheduler namespace required")
	}
	return &KubernetesScheduler{Client: cs, Namespace: namespace}, nil
}

func (k *KubernetesScheduler) Close() error { return nil }

// Apply creates-or-updates the CronJob for spec. Idempotent: a second
// Apply with the same spec produces no observable change beyond the
// last-updated annotation. cr0n's `replace_namespaced_cron_job` with
// 404 fallback is the pattern.
func (k *KubernetesScheduler) Apply(ctx context.Context, spec ScheduleSpec) error {
	if err := validateScheduleSpec(spec); err != nil {
		return err
	}
	cj, err := k.buildCronJob(spec)
	if err != nil {
		return err
	}

	existing, err := k.Client.BatchV1().CronJobs(k.Namespace).Get(ctx, cj.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		if _, err := k.Client.BatchV1().CronJobs(k.Namespace).Create(ctx, cj, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("runtime: create CronJob: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("runtime: get CronJob: %w", err)
	}
	cj.ResourceVersion = existing.ResourceVersion
	if _, err := k.Client.BatchV1().CronJobs(k.Namespace).Update(ctx, cj, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("runtime: update CronJob: %w", err)
	}
	return nil
}

// Delete removes the CronJob for scheduleID. NotFound is idempotent.
func (k *KubernetesScheduler) Delete(ctx context.Context, scheduleID string) error {
	if scheduleID == "" {
		return errors.New("runtime: scheduleID required")
	}
	name := scheduleResourceName(scheduleID)
	propagation := metav1.DeletePropagationBackground
	if err := k.Client.BatchV1().CronJobs(k.Namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("runtime: delete CronJob %s: %w", name, err)
	}
	return nil
}

// List returns every CronJob the scheduler manages in its namespace.
// Uses the canonical chunk-17 component label so unrelated CronJobs
// (operator-installed cluster jobs, etc.) are filtered out.
func (k *KubernetesScheduler) List(ctx context.Context) ([]ManagedSchedule, error) {
	cjs, err := k.Client.BatchV1().CronJobs(k.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=schedule,app.kubernetes.io/name=nightshift",
	})
	if err != nil {
		return nil, fmt.Errorf("runtime: list CronJobs: %w", err)
	}
	out := make([]ManagedSchedule, 0, len(cjs.Items))
	for _, cj := range cjs.Items {
		id := cj.Labels[scheduleIDLabel]
		if id == "" {
			continue
		}
		out = append(out, ManagedSchedule{
			ID:      id,
			Suspend: cj.Spec.Suspend != nil && *cj.Spec.Suspend,
		})
	}
	return out, nil
}

const scheduleIDLabel = "nightshift.io/schedule-id"

func (k *KubernetesScheduler) buildCronJob(spec ScheduleSpec) (*batchv1.CronJob, error) {
	name := scheduleResourceName(spec.ID)
	labels := map[string]string{
		"app.kubernetes.io/name":      "nightshift",
		"app.kubernetes.io/component": "schedule",
		scheduleIDLabel:               labelValueSafe(spec.ID),
	}

	// Build the CreateRun JSON payload baked into the CronJob's env.
	// Workers' CreateRun expects InvokerType as the proto string
	// constant ("INVOKER_TYPE_SCHEDULE") on the JSON path.
	payload := map[string]any{
		"prompt":       spec.Prompt,
		"user_id":      spec.UserID,
		"invoker_type": "INVOKER_TYPE_SCHEDULE",
		"invoker_id":   spec.ID,
	}
	if spec.SessionID != "" {
		payload["session_id"] = spec.SessionID
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("runtime: marshal scheduler payload: %w", err)
	}

	tz := spec.Timezone
	if tz == "" {
		tz = "UTC"
	}

	podSC := &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		RunAsUser:      ptr.To(workerNonRootUID),
		RunAsGroup:     ptr.To(workerNonRootUID),
		FSGroup:        ptr.To(workerNonRootUID),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	ctrSC := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To(workerNonRootUID),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}

	suspend := !spec.Enabled
	successHist := int32(1)
	failHist := int32(1)
	backoff := int32(1)
	ttl := int32(120)

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   spec.Cron,
			TimeZone:                   ptr.To(tz),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			Suspend:                    &suspend,
			SuccessfulJobsHistoryLimit: &successHist,
			FailedJobsHistoryLimit:     &failHist,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: batchv1.JobSpec{
					BackoffLimit:            &backoff,
					TTLSecondsAfterFinished: &ttl,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							RestartPolicy:                corev1.RestartPolicyNever,
							AutomountServiceAccountToken: ptr.To(false),
							SecurityContext:              podSC,
							Volumes: []corev1.Volume{
								{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
							},
							Containers: []corev1.Container{
								{
									Name:    "fire",
									Image:   spec.FireImage,
									Command: []string{"sh", "-c"},
									Args: []string{
										// Single-line so the embedded JSON survives shell quoting.
										// curl -fsSL: fail-fast on HTTP error, silent except errors,
										// follow redirects. The payload + bearer arrive via env.
										`exec curl -fsSL -X POST "$NS_API_INTERNAL_URL/v1/runs" ` +
											`-H "Authorization: Bearer $NS_SCHEDULER_TOKEN" ` +
											`-H "Content-Type: application/json" ` +
											`-d "$NS_SCHEDULER_PAYLOAD"`,
									},
									Env: []corev1.EnvVar{
										{Name: "NS_API_INTERNAL_URL", Value: spec.APIURL},
										{Name: "NS_SCHEDULER_PAYLOAD", Value: string(payloadBytes)},
										{
											Name: "NS_SCHEDULER_TOKEN",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{Name: spec.TokenSecret},
													Key:                  "token",
												},
											},
										},
									},
									SecurityContext: ctrSC,
									VolumeMounts: []corev1.VolumeMount{
										{Name: "tmp", MountPath: "/tmp"},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// scheduleResourceName produces a DNS-1123-safe CronJob name from a
// schedule id. Stable so reconciliation is idempotent.
func scheduleResourceName(scheduleID string) string {
	short := scheduleID
	if i := strings.LastIndex(short, "_"); i >= 0 {
		short = short[i+1:]
	}
	short = strings.ReplaceAll(short, "-", "")
	if len(short) > 12 {
		short = short[:12]
	}
	short = strings.ToLower(short)
	return "ns-sched-" + short
}

func validateScheduleSpec(spec ScheduleSpec) error {
	if spec.ID == "" {
		return errors.New("runtime: ScheduleSpec.ID required")
	}
	if spec.UserID == "" {
		return errors.New("runtime: ScheduleSpec.UserID required")
	}
	if spec.Prompt == "" {
		return errors.New("runtime: ScheduleSpec.Prompt required")
	}
	if spec.Cron == "" {
		return errors.New("runtime: ScheduleSpec.Cron required")
	}
	if spec.APIURL == "" {
		return errors.New("runtime: ScheduleSpec.APIURL required")
	}
	if spec.FireImage == "" {
		return errors.New("runtime: ScheduleSpec.FireImage required")
	}
	if spec.TokenSecret == "" {
		return errors.New("runtime: ScheduleSpec.TokenSecret required")
	}
	return nil
}

// -----------------------------------------------------------------------------
// StubScheduler
// -----------------------------------------------------------------------------

// StubScheduler is the no-K8s impl: keeps an in-memory map of applied
// specs and lists them as ManagedSchedules. The stub never actually
// fires runs — operators without a Kubernetes runtime can still
// CreateSchedule / GetSchedule / etc., but no fire happens.
type StubScheduler struct {
	mu    sync.Mutex
	specs map[string]ScheduleSpec
}

func NewStubScheduler() *StubScheduler {
	return &StubScheduler{specs: map[string]ScheduleSpec{}}
}

func (s *StubScheduler) Apply(_ context.Context, spec ScheduleSpec) error {
	if err := validateScheduleSpec(spec); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.specs[spec.ID] = spec
	return nil
}

func (s *StubScheduler) Delete(_ context.Context, scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.specs, scheduleID)
	return nil
}

func (s *StubScheduler) List(_ context.Context) ([]ManagedSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ManagedSchedule, 0, len(s.specs))
	for id, sp := range s.specs {
		out = append(out, ManagedSchedule{ID: id, Suspend: !sp.Enabled})
	}
	return out, nil
}

func (s *StubScheduler) Close() error { return nil }
