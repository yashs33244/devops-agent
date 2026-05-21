package runtime

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newFakeK8sLauncher(t *testing.T) *KubernetesLauncher {
	t.Helper()
	return NewKubernetesLauncherWithClient(fake.NewSimpleClientset(), "nightshift-test", "ns-worker")
}

func findVolume(vols []corev1.Volume, name string) *corev1.Volume {
	for i := range vols {
		if vols[i].Name == name {
			return &vols[i]
		}
	}
	return nil
}

func findMount(mounts []corev1.VolumeMount, name string) *corev1.VolumeMount {
	for i := range mounts {
		if mounts[i].Name == name {
			return &mounts[i]
		}
	}
	return nil
}

func TestKubernetesLaunchCreatesJob(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{
		RunID:                   "a1b2-c3d4-e5f6",
		UserID:                  "alice",
		SessionID:               "sess",
		Prompt:                  "do the thing",
		Image:                   "nightshift-worker:v1",
		CallbackURL:             "http://api",
		WorkerCredential:        "v1.a.1.s",
		TTLSecondsAfterFinished: 120,
		ActiveDeadlineSeconds:   1800,
		Resources:               ResourceReqs{CPU: "500m", Memory: "512Mi"},
	}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatalf("launch: %v", err)
	}
	name := jobName(spec.RunID)

	got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetLabels()["nightshift.io/run-id"] != spec.RunID {
		t.Fatalf("label run-id=%q", got.GetLabels()["nightshift.io/run-id"])
	}
	if *got.Spec.BackoffLimit != 0 {
		t.Fatalf("backoff limit=%d", *got.Spec.BackoffLimit)
	}
	if *got.Spec.ActiveDeadlineSeconds != 1800 {
		t.Fatalf("deadline=%d", *got.Spec.ActiveDeadlineSeconds)
	}
	if *got.Spec.TTLSecondsAfterFinished != 120 {
		t.Fatalf("ttl=%d", *got.Spec.TTLSecondsAfterFinished)
	}
	c := got.Spec.Template.Spec.Containers[0]
	if c.Image != "nightshift-worker:v1" {
		t.Fatalf("image=%q", c.Image)
	}
	if got.Spec.Template.Spec.ServiceAccountName != "ns-worker" {
		t.Fatalf("SA=%q", got.Spec.Template.Spec.ServiceAccountName)
	}

	envMap := map[string]string{}
	for _, e := range c.Env {
		envMap[e.Name] = e.Value
	}
	for k, want := range map[string]string{
		"NS_RUN_ID":            spec.RunID,
		"NS_USER_ID":           spec.UserID,
		"NS_SESSION_ID":        spec.SessionID,
		"NS_PROMPT":            spec.Prompt,
		"NS_API_URL":           spec.CallbackURL,
		"NS_WORKER_CREDENTIAL": spec.WorkerCredential,
	} {
		if envMap[k] != want {
			t.Fatalf("env %s=%q, want %q", k, envMap[k], want)
		}
	}
}

func TestKubernetesLaunchHardensSecurityContext(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{RunID: "run-sc", Image: "img:latest"}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	pod := got.Spec.Template.Spec
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Errorf("AutomountServiceAccountToken: want false (chunk-9 default), got %v", pod.AutomountServiceAccountToken)
	}

	// Chunk 14: spec.MountServiceAccountToken=true flips it on.
	specMount := LaunchSpec{RunID: "run-mount", Image: "img:latest", MountServiceAccountToken: true}
	if err := l.Launch(ctx, specMount); err != nil {
		t.Fatal(err)
	}
	mountedJob, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(specMount.RunID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mountedPod := mountedJob.Spec.Template.Spec
	if mountedPod.AutomountServiceAccountToken == nil || !*mountedPod.AutomountServiceAccountToken {
		t.Errorf("AutomountServiceAccountToken: want true when MountServiceAccountToken set, got %v", mountedPod.AutomountServiceAccountToken)
	}

	psc := pod.SecurityContext
	if psc == nil {
		t.Fatal("PodSecurityContext: nil")
	}
	if psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("PodSecurityContext.RunAsNonRoot: want true")
	}
	if psc.RunAsUser == nil || *psc.RunAsUser != workerNonRootUID {
		t.Errorf("PodSecurityContext.RunAsUser: want %d, got %v", workerNonRootUID, psc.RunAsUser)
	}
	if psc.RunAsGroup == nil || *psc.RunAsGroup != workerNonRootUID {
		t.Errorf("PodSecurityContext.RunAsGroup: want %d", workerNonRootUID)
	}
	if psc.FSGroup == nil || *psc.FSGroup != workerNonRootUID {
		t.Errorf("PodSecurityContext.FSGroup: want %d", workerNonRootUID)
	}
	if psc.SeccompProfile == nil || psc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("PodSecurityContext.SeccompProfile: want RuntimeDefault")
	}

	csc := pod.Containers[0].SecurityContext
	if csc == nil {
		t.Fatal("Container.SecurityContext: nil")
	}
	if csc.AllowPrivilegeEscalation == nil || *csc.AllowPrivilegeEscalation {
		t.Errorf("AllowPrivilegeEscalation: want false")
	}
	if csc.ReadOnlyRootFilesystem == nil || !*csc.ReadOnlyRootFilesystem {
		t.Errorf("ReadOnlyRootFilesystem: want true")
	}
	if csc.RunAsNonRoot == nil || !*csc.RunAsNonRoot {
		t.Errorf("Container.RunAsNonRoot: want true")
	}
	if csc.RunAsUser == nil || *csc.RunAsUser != workerNonRootUID {
		t.Errorf("Container.RunAsUser: want %d", workerNonRootUID)
	}
	if csc.Capabilities == nil || len(csc.Capabilities.Drop) != 1 || csc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("Container.Capabilities.Drop: want [ALL], got %v", csc.Capabilities)
	}
	if csc.SeccompProfile == nil || csc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("Container.SeccompProfile: want RuntimeDefault")
	}
}

func TestKubernetesLaunchIdempotent(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{RunID: "run-idem", Image: "img:latest"}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatalf("second launch should be idempotent: %v", err)
	}
}

func TestKubernetesInterrupt(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{RunID: "run-it", Image: "img:latest"}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if err := l.Interrupt(ctx, "run-it"); err != nil {
		t.Fatalf("interrupt: %v", err)
	}
	_, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName("run-it"), metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestKubernetesInterruptMissing(t *testing.T) {
	l := newFakeK8sLauncher(t)
	if err := l.Interrupt(context.Background(), "does-not-exist"); err != nil {
		t.Fatalf("should be no-op: %v", err)
	}
}

func TestKubernetesLaunchRequiresRunID(t *testing.T) {
	l := newFakeK8sLauncher(t)
	if err := l.Launch(context.Background(), LaunchSpec{Image: "x"}); err == nil {
		t.Fatal("expected error for empty RunID")
	}
}

func TestKubernetesLaunchRequiresImage(t *testing.T) {
	l := newFakeK8sLauncher(t)
	if err := l.Launch(context.Background(), LaunchSpec{RunID: "r"}); err == nil {
		t.Fatal("expected error for empty Image")
	}
}

func TestJobNameDNS1123(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc-def", "ns-run-abcdef"},
		{"UPPER-Case", "ns-run-uppercase"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "ns-run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	for _, c := range cases {
		got := jobName(c.in)
		if got != c.want {
			t.Errorf("jobName(%q)=%q, want %q", c.in, got, c.want)
		}
		if len(got) > 63 {
			t.Errorf("jobName(%q) exceeds 63 chars: len=%d", c.in, len(got))
		}
	}
}

func TestKubernetesLaunchPVCSessionState(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{
		RunID:     "run-pvc",
		UserID:    "alice",
		SessionID: "sess-1",
		Image:     "img:latest",
		SessionState: SessionStateConfig{
			Backend:   SessionStateBackendPVC,
			MountPath: "/var/lib/nightshift/session-state",
			PVCName:   "ns-session-state",
		},
	}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := got.Spec.Template.Spec
	sessionVol := findVolume(pod.Volumes, "session-state")
	if sessionVol == nil {
		t.Fatalf("session-state volume missing; got %+v", pod.Volumes)
	}
	if sessionVol.PersistentVolumeClaim == nil ||
		sessionVol.PersistentVolumeClaim.ClaimName != "ns-session-state" {
		t.Fatalf("PVC volume source: %+v", sessionVol.VolumeSource)
	}
	c := pod.Containers[0]
	m := findMount(c.VolumeMounts, "session-state")
	if m == nil {
		t.Fatalf("session-state mount missing; got %+v", c.VolumeMounts)
	}
	if m.MountPath != "/var/lib/nightshift/session-state" {
		t.Errorf("mountPath=%q", m.MountPath)
	}
	if m.SubPath != "alice/sess-1" {
		t.Errorf("subPath=%q, want alice/sess-1", m.SubPath)
	}
	envMap := map[string]string{}
	for _, e := range c.Env {
		envMap[e.Name] = e.Value
	}
	if envMap["NS_SESSION_STATE_DIR"] != "/var/lib/nightshift/session-state" {
		t.Errorf("NS_SESSION_STATE_DIR=%q", envMap["NS_SESSION_STATE_DIR"])
	}
	if envMap["NS_SESSION_STATE_BACKEND"] != "pvc" {
		t.Errorf("NS_SESSION_STATE_BACKEND=%q, want pvc", envMap["NS_SESSION_STATE_BACKEND"])
	}
}

func TestKubernetesLaunchHostSessionState(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{
		RunID:     "run-host",
		UserID:    "alice",
		SessionID: "sess-1",
		Image:     "img:latest",
		SessionState: SessionStateConfig{
			Backend:   SessionStateBackendHost,
			MountPath: "/var/lib/nightshift/session-state",
			HostRoot:  "/srv/ns-state",
		},
	}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := got.Spec.Template.Spec
	sessionVol := findVolume(pod.Volumes, "session-state")
	if sessionVol == nil || sessionVol.HostPath == nil {
		t.Fatalf("expected hostPath session-state volume, got %+v", pod.Volumes)
	}
	if sessionVol.HostPath.Path != "/srv/ns-state/alice/sess-1" {
		t.Errorf("hostPath.Path=%q", sessionVol.HostPath.Path)
	}
	if sessionVol.HostPath.Type == nil ||
		*sessionVol.HostPath.Type != corev1.HostPathDirectoryOrCreate {
		t.Errorf("hostPath.Type=%v", sessionVol.HostPath.Type)
	}
	c := pod.Containers[0]
	m := findMount(c.VolumeMounts, "session-state")
	if m == nil {
		t.Fatalf("session-state mount missing; got %+v", c.VolumeMounts)
	}
	if m.SubPath != "" {
		t.Errorf("host backend should not use subPath: %+v", m)
	}
	envMap := map[string]string{}
	for _, e := range c.Env {
		envMap[e.Name] = e.Value
	}
	if envMap["NS_SESSION_STATE_DIR"] != "/var/lib/nightshift/session-state" {
		t.Errorf("NS_SESSION_STATE_DIR=%q", envMap["NS_SESSION_STATE_DIR"])
	}
	if envMap["NS_SESSION_STATE_BACKEND"] != "host" {
		t.Errorf("NS_SESSION_STATE_BACKEND=%q, want host", envMap["NS_SESSION_STATE_BACKEND"])
	}
}

func TestKubernetesLaunchObjectSessionState(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{
		RunID:     "run-obj",
		UserID:    "alice",
		SessionID: "sess-1",
		Image:     "img:latest",
		SessionState: SessionStateConfig{
			Backend:      SessionStateBackendObject,
			MountPath:    "/var/lib/nightshift/session-state",
			ObjectBucket: "nightshift",
		},
	}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := got.Spec.Template.Spec

	// Volume must be an emptyDir (no PVC, no hostPath).
	sessionVol := findVolume(pod.Volumes, "session-state")
	if sessionVol == nil {
		t.Fatalf("session-state volume missing; got %+v", pod.Volumes)
	}
	if sessionVol.EmptyDir == nil {
		t.Fatalf("expected emptyDir volume source for object backend, got %+v", sessionVol.VolumeSource)
	}
	if sessionVol.PersistentVolumeClaim != nil || sessionVol.HostPath != nil {
		t.Fatalf("object backend should not use PVC/hostPath: %+v", sessionVol.VolumeSource)
	}

	c := pod.Containers[0]
	m := findMount(c.VolumeMounts, "session-state")
	if m == nil {
		t.Fatalf("session-state mount missing; got %+v", c.VolumeMounts)
	}
	if m.MountPath != "/var/lib/nightshift/session-state" {
		t.Errorf("mountPath=%q", m.MountPath)
	}
	if m.SubPath != "" {
		t.Errorf("object backend should not use subPath: %+v", m)
	}

	envMap := map[string]string{}
	for _, e := range c.Env {
		envMap[e.Name] = e.Value
	}
	if envMap["NS_SESSION_STATE_DIR"] != "/var/lib/nightshift/session-state" {
		t.Errorf("NS_SESSION_STATE_DIR=%q", envMap["NS_SESSION_STATE_DIR"])
	}
	if envMap["NS_SESSION_STATE_BACKEND"] != "object" {
		t.Errorf("NS_SESSION_STATE_BACKEND=%q, want object", envMap["NS_SESSION_STATE_BACKEND"])
	}
}

// Chunk 14 — NS_SDK_SESSION_ID is exported only when the resume
// lookup found a value. Empty is the fresh-session case.
func TestKubernetesLaunchPropagatesSDKSessionID(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()

	{
		spec := LaunchSpec{
			RunID:        "run-resume",
			Image:        "img:latest",
			SDKSessionID: "claude-xyz",
		}
		if err := l.Launch(ctx, spec); err != nil {
			t.Fatal(err)
		}
		got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		envMap := map[string]string{}
		for _, e := range got.Spec.Template.Spec.Containers[0].Env {
			envMap[e.Name] = e.Value
		}
		if envMap["NS_SDK_SESSION_ID"] != "claude-xyz" {
			t.Fatalf("NS_SDK_SESSION_ID=%q, want claude-xyz", envMap["NS_SDK_SESSION_ID"])
		}
	}
	{
		spec := LaunchSpec{RunID: "run-fresh", Image: "img:latest"}
		if err := l.Launch(ctx, spec); err != nil {
			t.Fatal(err)
		}
		got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range got.Spec.Template.Spec.Containers[0].Env {
			if e.Name == "NS_SDK_SESSION_ID" {
				t.Errorf("fresh-session run should not set NS_SDK_SESSION_ID, got %q", e.Value)
			}
		}
	}
}

func TestKubernetesLaunchNoSessionState(t *testing.T) {
	l := newFakeK8sLauncher(t)
	ctx := context.Background()
	spec := LaunchSpec{RunID: "run-none", Image: "img:latest"}
	if err := l.Launch(ctx, spec); err != nil {
		t.Fatal(err)
	}
	got, err := l.Client.BatchV1().Jobs(l.Namespace).Get(ctx, jobName(spec.RunID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := got.Spec.Template.Spec
	if findVolume(pod.Volumes, "session-state") != nil {
		t.Errorf("none backend should not add a session-state volume; got %+v", pod.Volumes)
	}
	// /tmp + /home/nightshift scratch volumes are always present
	// (chunk-14 fix to keep readOnlyRootFilesystem-compatible).
	if findVolume(pod.Volumes, "tmp") == nil || findVolume(pod.Volumes, "home") == nil {
		t.Errorf("scratch volumes (tmp, home) must always be present; got %+v", pod.Volumes)
	}
	c := pod.Containers[0]
	if findMount(c.VolumeMounts, "session-state") != nil {
		t.Errorf("none backend should not add a session-state mount; got %+v", c.VolumeMounts)
	}
	for _, e := range c.Env {
		if e.Name == "NS_SESSION_STATE_DIR" {
			t.Errorf("none backend should not set NS_SESSION_STATE_DIR")
		}
	}
}

func TestLabelValueSafe(t *testing.T) {
	cases := []struct{ in, want string }{
		{"alice@example.com", "alice-example.com"},
		{"Alice", "alice"},
		{"-leading-", "leading"},
		{"a_b.c", "a_b.c"},
	}
	for _, c := range cases {
		if got := labelValueSafe(c.in); got != c.want {
			t.Errorf("labelValueSafe(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}
