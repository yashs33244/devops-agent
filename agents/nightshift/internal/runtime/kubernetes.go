package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

// workerNonRootUID is the UID/GID worker pods run as. Distroless
// nonroot images use 65532. Hardcoded — see chunk 9 threat model.
const workerNonRootUID int64 = 65532

// KubernetesLauncher runs each worker as a Kubernetes Job in its
// namespace. Job names are derived deterministically from RunID so
// Launch is idempotent on repeat calls.
type KubernetesLauncher struct {
	Client         kubernetes.Interface
	Namespace      string
	ServiceAccount string

	// Label values used on every Job the launcher creates. Enables
	// Interrupt lookup by run_id and future orphan reconciliation.
	LabelApp       string // default "nightshift"
	LabelComponent string // default "worker"
}

// NewKubernetesLauncher builds a KubernetesLauncher.
//
// kubeconfigPath is optional: empty means "use in-cluster config".
// Falls back to the default kubeconfig location (KUBECONFIG env, then
// ~/.kube/config) if kubeconfigPath is the literal string "auto".
func NewKubernetesLauncher(kubeconfigPath, namespace, serviceAccount string) (*KubernetesLauncher, error) {
	if namespace == "" {
		return nil, errors.New("runtime: k8s namespace required")
	}
	cfg, err := loadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("runtime: k8s clientset: %w", err)
	}
	return &KubernetesLauncher{
		Client:         cs,
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
		LabelApp:       "nightshift",
		LabelComponent: "worker",
	}, nil
}

// NewKubernetesLauncherWithClient is for tests — inject a fake Clientset.
func NewKubernetesLauncherWithClient(cs kubernetes.Interface, namespace, serviceAccount string) *KubernetesLauncher {
	return &KubernetesLauncher{
		Client:         cs,
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
		LabelApp:       "nightshift",
		LabelComponent: "worker",
	}
}

func loadRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("runtime: in-cluster config: %w", err)
		}
		return cfg, nil
	}
	if kubeconfigPath == "auto" {
		loader := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("runtime: kubeconfig: %w", err)
		}
		return cfg, nil
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

func (k *KubernetesLauncher) Close() error { return nil }

// Launch creates a Job for spec.RunID. Idempotent — if a Job already
// exists with the same name, Launch returns nil.
func (k *KubernetesLauncher) Launch(ctx context.Context, spec LaunchSpec) error {
	if spec.RunID == "" {
		return errors.New("runtime: LaunchSpec.RunID required")
	}
	if spec.Image == "" {
		return errors.New("runtime: LaunchSpec.Image required")
	}

	job := k.buildJob(spec)
	_, err := k.Client.BatchV1().Jobs(k.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("runtime: create Job: %w", err)
	}
	return nil
}

// Interrupt deletes the Job for runID with Background propagation.
// Missing Jobs are a no-op.
func (k *KubernetesLauncher) Interrupt(ctx context.Context, runID string) error {
	name := jobName(runID)
	propagation := metav1.DeletePropagationBackground
	if err := k.Client.BatchV1().Jobs(k.Namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("runtime: delete Job %s: %w", name, err)
	}
	return nil
}

// buildJob is the pure manifest builder. Exported (via the struct
// method) so tests can assert on the produced spec.
func (k *KubernetesLauncher) buildJob(spec LaunchSpec) *batchv1.Job {
	name := jobName(spec.RunID)
	backoff := int32(0)
	ttl := spec.TTLSecondsAfterFinished
	if ttl == 0 {
		ttl = 300
	}
	deadline := spec.ActiveDeadlineSeconds
	if deadline == 0 {
		deadline = 3600
	}

	labels := map[string]string{
		"app.kubernetes.io/name":      k.LabelApp,
		"app.kubernetes.io/component": k.LabelComponent,
		"nightshift.io/run-id":        spec.RunID,
		"nightshift.io/user-id":       labelValueSafe(spec.UserID),
	}

	env := []corev1.EnvVar{
		{Name: "NS_RUN_ID", Value: spec.RunID},
		{Name: "NS_USER_ID", Value: spec.UserID},
		{Name: "NS_SESSION_ID", Value: spec.SessionID},
		{Name: "NS_PROMPT", Value: spec.Prompt},
		{Name: "NS_API_URL", Value: spec.CallbackURL},
		{Name: "NS_WORKER_CREDENTIAL", Value: spec.WorkerCredential},
	}

	// chunk-9 hardening sets readOnlyRootFilesystem=true on every
	// worker container. Real workloads (notably the chunk-14 Python
	// claude worker, which spawns the Claude Code CLI subprocess)
	// need writable /tmp (IPC sockets, caches) and a writable HOME
	// (~/.claude transcript dirs). Without them, subprocess startup
	// hangs and the SDK's initialize JSON-RPC never gets an ack.
	// emptyDir is per-pod ephemeral memory/disk — ideal for caches
	// that must not persist beyond a run, and harmless for the Go
	// simulated worker which never writes there.
	scratchVolumes := []corev1.Volume{
		{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		{Name: "home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
	scratchMounts := []corev1.VolumeMount{
		{Name: "tmp", MountPath: "/tmp"},
		{Name: "home", MountPath: "/home/nightshift"},
	}

	// Session-state volume. The pvc backend shares one RWX claim
	// across users; subPath isolation keeps a worker pod from reading
	// a sibling user's files. The host backend mounts an
	// already-scoped per-session host dir, so no subPath is needed.
	// The object backend uses an emptyDir; the worker pulls/pushes
	// bytes via the API session-state endpoints.
	var sessionVolumes []corev1.Volume
	var sessionMounts []corev1.VolumeMount
	mounted := false
	if spec.SessionState.Enabled() {
		mountPath := spec.SessionState.MountPath
		if mountPath == "" {
			mountPath = SessionStateDefaultMount
		}
		switch spec.SessionState.Backend {
		case SessionStateBackendPVC:
			subPath, err := SessionSubPath(spec.UserID, spec.SessionID)
			if err == nil && spec.SessionState.PVCName != "" {
				sessionVolumes = append(sessionVolumes, corev1.Volume{
					Name: "session-state",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: spec.SessionState.PVCName,
						},
					},
				})
				sessionMounts = append(sessionMounts, corev1.VolumeMount{
					Name:      "session-state",
					MountPath: mountPath,
					SubPath:   subPath,
				})
				env = append(env, corev1.EnvVar{Name: "NS_SESSION_STATE_DIR", Value: mountPath})
				mounted = true
			}
		case SessionStateBackendHost:
			subPath, err := SessionSubPath(spec.UserID, spec.SessionID)
			if err == nil && spec.SessionState.HostRoot != "" {
				hostDir := strings.TrimRight(spec.SessionState.HostRoot, "/") + "/" + subPath
				hostPathType := corev1.HostPathDirectoryOrCreate
				sessionVolumes = append(sessionVolumes, corev1.Volume{
					Name: "session-state",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostDir,
							Type: &hostPathType,
						},
					},
				})
				sessionMounts = append(sessionMounts, corev1.VolumeMount{
					Name:      "session-state",
					MountPath: mountPath,
				})
				env = append(env, corev1.EnvVar{Name: "NS_SESSION_STATE_DIR", Value: mountPath})
				mounted = true
			}
		case SessionStateBackendObject:
			sessionVolumes = append(sessionVolumes, corev1.Volume{
				Name:         "session-state",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			})
			sessionMounts = append(sessionMounts, corev1.VolumeMount{
				Name:      "session-state",
				MountPath: mountPath,
			})
			env = append(env, corev1.EnvVar{Name: "NS_SESSION_STATE_DIR", Value: mountPath})
			mounted = true
		}
		if mounted {
			env = append(env, corev1.EnvVar{
				Name:  "NS_SESSION_STATE_BACKEND",
				Value: string(spec.SessionState.Backend),
			})
		}
	}

	if spec.SDKSessionID != "" {
		env = append(env, corev1.EnvVar{Name: "NS_SDK_SESSION_ID", Value: spec.SDKSessionID})
	}

	for k, v := range spec.ExtraEnv {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	resReqs := corev1.ResourceRequirements{}
	if spec.Resources.CPU != "" || spec.Resources.Memory != "" {
		resReqs.Requests = corev1.ResourceList{}
		resReqs.Limits = corev1.ResourceList{}
		if spec.Resources.CPU != "" {
			cpu := resource.MustParse(spec.Resources.CPU)
			resReqs.Requests[corev1.ResourceCPU] = cpu
			resReqs.Limits[corev1.ResourceCPU] = cpu
		}
		if spec.Resources.Memory != "" {
			mem := resource.MustParse(spec.Resources.Memory)
			resReqs.Requests[corev1.ResourceMemory] = mem
			resReqs.Limits[corev1.ResourceMemory] = mem
		}
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

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			ActiveDeadlineSeconds:   &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					ServiceAccountName:           k.ServiceAccount,
					AutomountServiceAccountToken: ptr.To(spec.MountServiceAccountToken),
					SecurityContext:              podSC,
					Volumes:                      append(scratchVolumes, sessionVolumes...),
					Containers: []corev1.Container{
						{
							Name:            "worker",
							Image:           spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             env,
							Resources:       resReqs,
							SecurityContext: ctrSC,
							VolumeMounts:    append(scratchMounts, sessionMounts...),
						},
					},
				},
			},
		},
	}
}

// jobName produces a DNS-1123-safe Job name derived from RunID. Keeps
// the name ≤ 63 chars.
func jobName(runID string) string {
	// UUID form "a1b2c3d4-…" → strip dashes, lowercase.
	compact := strings.ToLower(strings.ReplaceAll(runID, "-", ""))
	if len(compact) > 50 {
		compact = compact[:50]
	}
	return "ns-run-" + compact
}

// labelValueSafe coerces v into a DNS-1123-safe label value.
// Unsafe chars collapse to '-'; leading/trailing '-' stripped. Max 63.
func labelValueSafe(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}
