package runtime

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newFakeAppDeployer(t *testing.T) *KubernetesAppDeployer {
	t.Helper()
	d, err := NewKubernetesAppDeployer(fake.NewSimpleClientset(), "nightshift-test", AppDeployerConfig{})
	if err != nil {
		t.Fatalf("NewKubernetesAppDeployer: %v", err)
	}
	return d
}

func sampleAppSpec() AppSpec {
	return AppSpec{
		ArtifactID:  "art_a36196f5-894d-477d-8862-1e900a58ac19",
		Name:        "demo",
		DownloadURL: "http://example.test/_objects/abc?sig=def",
	}
}

func TestAppDeployerDeployCreatesDeploymentAndService(t *testing.T) {
	d := newFakeAppDeployer(t)
	ctx := context.Background()
	spec := sampleAppSpec()

	res, err := d.Deploy(ctx, spec)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if !strings.HasPrefix(res.ServiceURL, "http://ns-app-") || !strings.HasSuffix(res.ServiceURL, ".nightshift-test.svc:80") {
		t.Fatalf("ServiceURL=%q (unexpected shape)", res.ServiceURL)
	}

	dep, err := d.Client.AppsV1().Deployments(d.Namespace).Get(ctx, appResourceName(spec.ArtifactID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if *dep.Spec.Replicas != 1 {
		t.Fatalf("replicas=%d", *dep.Spec.Replicas)
	}

	pod := dep.Spec.Template.Spec
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Fatal("automountSAToken should be false")
	}

	// Init container shape.
	if len(pod.InitContainers) != 1 {
		t.Fatalf("init containers=%d", len(pod.InitContainers))
	}
	ic := pod.InitContainers[0]
	if ic.Name != "fetch-html" {
		t.Fatalf("init name=%q", ic.Name)
	}
	if !strings.Contains(strings.Join(ic.Command, " "), "curl -fsSL") {
		t.Fatalf("init cmd=%v", ic.Command)
	}
	var dlEnv string
	for _, e := range ic.Env {
		if e.Name == "DOWNLOAD_URL" {
			dlEnv = e.Value
		}
	}
	if dlEnv != spec.DownloadURL {
		t.Fatalf("DOWNLOAD_URL env=%q, want %q", dlEnv, spec.DownloadURL)
	}
	if ic.SecurityContext == nil || ic.SecurityContext.ReadOnlyRootFilesystem == nil || !*ic.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatal("init SC: readOnlyRootFilesystem must be true")
	}

	// Main container shape.
	if len(pod.Containers) != 1 {
		t.Fatalf("containers=%d", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.Name != "nginx" {
		t.Fatalf("main name=%q", c.Name)
	}
	if c.Image != "nginxinc/nginx-unprivileged:1-alpine" {
		t.Fatalf("nginx image=%q (default)", c.Image)
	}
	if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 8080 {
		t.Fatalf("ports=%v", c.Ports)
	}
	if c.SecurityContext == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatal("nginx SC: readOnlyRootFilesystem must be true")
	}

	// Volumes + mounts.
	if v := findVolume(pod.Volumes, "html"); v == nil || v.EmptyDir == nil {
		t.Fatal("html emptyDir volume missing")
	}
	if m := findMount(c.VolumeMounts, "html"); m == nil || m.MountPath != "/usr/share/nginx/html" || !m.ReadOnly {
		t.Fatalf("nginx html mount=%v", m)
	}

	// Service.
	svc, err := d.Client.CoreV1().Services(d.Namespace).Get(ctx, appResourceName(spec.ArtifactID), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if svc.Spec.Ports[0].Port != 80 || svc.Spec.Ports[0].TargetPort.IntValue() != 8080 {
		t.Fatalf("svc ports=%v", svc.Spec.Ports)
	}
	if svc.Spec.Selector["nightshift.io/artifact-id"] == "" {
		t.Fatalf("svc selector missing artifact-id label: %v", svc.Spec.Selector)
	}
}

func TestAppDeployerDeployIsIdempotent(t *testing.T) {
	d := newFakeAppDeployer(t)
	ctx := context.Background()
	spec := sampleAppSpec()
	if _, err := d.Deploy(ctx, spec); err != nil {
		t.Fatal(err)
	}
	// Second Deploy with same spec must not error (AlreadyExists tolerated).
	if _, err := d.Deploy(ctx, spec); err != nil {
		t.Fatalf("repeat deploy: %v", err)
	}
}

func TestAppDeployerUpdateBumpsRestartAnnotation(t *testing.T) {
	d := newFakeAppDeployer(t)
	ctx := context.Background()
	spec := sampleAppSpec()
	if _, err := d.Deploy(ctx, spec); err != nil {
		t.Fatal(err)
	}

	spec2 := spec
	spec2.DownloadURL = "http://example.test/_objects/NEW?sig=XYZ"
	if _, err := d.Update(ctx, spec2); err != nil {
		t.Fatalf("update: %v", err)
	}

	dep, err := d.Client.AppsV1().Deployments(d.Namespace).Get(ctx, appResourceName(spec.ArtifactID), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Template.Annotations["nightshift.io/restart-at"] == "" {
		t.Fatal("restart-at annotation not set")
	}
	var dlEnv string
	for _, e := range dep.Spec.Template.Spec.InitContainers[0].Env {
		if e.Name == "DOWNLOAD_URL" {
			dlEnv = e.Value
		}
	}
	if dlEnv != spec2.DownloadURL {
		t.Fatalf("init DOWNLOAD_URL not refreshed on update: %q", dlEnv)
	}
}

func TestAppDeployerUpdateOnMissingDeploys(t *testing.T) {
	d := newFakeAppDeployer(t)
	ctx := context.Background()
	// Update with no prior Deploy should fall through to Deploy.
	if _, err := d.Update(ctx, sampleAppSpec()); err != nil {
		t.Fatalf("update-as-deploy: %v", err)
	}
	if _, err := d.Client.AppsV1().Deployments(d.Namespace).Get(ctx, appResourceName(sampleAppSpec().ArtifactID), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected Deployment after fallback: %v", err)
	}
}

func TestAppDeployerDeleteRemovesResources(t *testing.T) {
	d := newFakeAppDeployer(t)
	ctx := context.Background()
	spec := sampleAppSpec()
	if _, err := d.Deploy(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(ctx, spec.ArtifactID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := d.Client.AppsV1().Deployments(d.Namespace).Get(ctx, appResourceName(spec.ArtifactID), metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("deployment should be gone: %v", err)
	}
	if _, err := d.Client.CoreV1().Services(d.Namespace).Get(ctx, appResourceName(spec.ArtifactID), metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("service should be gone: %v", err)
	}
	// Repeat delete is idempotent.
	if err := d.Delete(ctx, spec.ArtifactID); err != nil {
		t.Fatalf("repeat delete: %v", err)
	}
}

func TestAppDeployerValidatesSpec(t *testing.T) {
	d := newFakeAppDeployer(t)
	ctx := context.Background()
	if _, err := d.Deploy(ctx, AppSpec{ArtifactID: "x"}); err == nil {
		t.Fatal("expected error for missing DownloadURL")
	}
	if _, err := d.Deploy(ctx, AppSpec{DownloadURL: "http://x"}); err == nil {
		t.Fatal("expected error for missing ArtifactID")
	}
}

func TestStubAppDeployer(t *testing.T) {
	s := NewStubAppDeployer()
	ctx := context.Background()
	res, err := s.Deploy(ctx, sampleAppSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.ServiceURL, StubAppServicePrefix) {
		t.Fatalf("stub URL=%q", res.ServiceURL)
	}
	if err := s.Delete(ctx, sampleAppSpec().ArtifactID); err != nil {
		t.Fatal(err)
	}
}
