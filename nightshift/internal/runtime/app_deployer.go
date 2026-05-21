package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// AppSpec describes one app artifact deployment. The HTML bytes
// themselves are uploaded to Object storage by the Artifacts service
// before calling the deployer; the deployer itself only sees the
// presigned DownloadURL the init container fetches at pod start.
type AppSpec struct {
	ArtifactID  string
	Name        string
	DownloadURL string
}

// AppDeployResult is what the deployer returns to the Artifacts
// service. ServiceURL is persisted on the artifact record as `app_url`
// and used by the proxy handler as the upstream target.
type AppDeployResult struct {
	ServiceURL string
}

// AppDeployer is the seam the Artifacts service uses to materialize
// app artifacts. Two impls live here: KubernetesAppDeployer (real)
// and StubAppDeployer (no-K8s, in-memory; for stub runtime + tests
// without a Kubernetes API).
type AppDeployer interface {
	Deploy(ctx context.Context, spec AppSpec) (AppDeployResult, error)
	Update(ctx context.Context, spec AppSpec) (AppDeployResult, error)
	Delete(ctx context.Context, artifactID string) error
	Close() error
}

// AppDeployerConfig configures a KubernetesAppDeployer. Image fields
// have working defaults; the namespace must be supplied.
type AppDeployerConfig struct {
	NginxImage string // default "nginx:alpine"
	InitImage  string // default "curlimages/curl:latest"
}

// KubernetesAppDeployer materializes app artifacts as a Deployment +
// ClusterIP Service in a namespace the API has create/get/patch/delete
// permission on (deploy/charts/nightshift/templates/nightshift-api/rbac.yaml).
//
// The Deployment runs an init container that curls a presigned URL
// for the HTML into a shared emptyDir; nginx serves the dir read-only.
// No external storage credentials live in the pod — the URL signs the
// access. cr0n parity: matches artifact_routes.py:33-141, but uses
// curl + presigned URL instead of aws-cli + IRSA so the same code
// works for filesystem and S3 backends symmetrically.
type KubernetesAppDeployer struct {
	Client    kubernetes.Interface
	Namespace string
	Config    AppDeployerConfig
}

// NewKubernetesAppDeployer constructs a deployer against an existing
// clientset. Tests pass `fake.NewSimpleClientset()`.
func NewKubernetesAppDeployer(cs kubernetes.Interface, namespace string, cfg AppDeployerConfig) (*KubernetesAppDeployer, error) {
	if cs == nil {
		return nil, errors.New("runtime: app deployer Client required")
	}
	if namespace == "" {
		return nil, errors.New("runtime: app deployer namespace required")
	}
	if cfg.NginxImage == "" {
		// nginx-unprivileged listens on 8080 as uid 101 by default, so it
		// works alongside the hardened pod SecurityContext (RunAsNonRoot,
		// dropped caps, ReadOnlyRootFilesystem) without operator config.
		cfg.NginxImage = "nginxinc/nginx-unprivileged:1-alpine"
	}
	if cfg.InitImage == "" {
		cfg.InitImage = "curlimages/curl:latest"
	}
	return &KubernetesAppDeployer{Client: cs, Namespace: namespace, Config: cfg}, nil
}

func (k *KubernetesAppDeployer) Close() error { return nil }

// Deploy creates the Deployment + Service for spec. Idempotent on
// already-exists. Returns the in-cluster Service URL the proxy
// handler should target.
func (k *KubernetesAppDeployer) Deploy(ctx context.Context, spec AppSpec) (AppDeployResult, error) {
	if err := validateAppSpec(spec); err != nil {
		return AppDeployResult{}, err
	}
	dep := k.buildDeployment(spec)
	svc := k.buildService(spec)

	if _, err := k.Client.AppsV1().Deployments(k.Namespace).Create(ctx, dep, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return AppDeployResult{}, fmt.Errorf("runtime: create Deployment: %w", err)
		}
	}
	if _, err := k.Client.CoreV1().Services(k.Namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return AppDeployResult{}, fmt.Errorf("runtime: create Service: %w", err)
		}
	}
	return AppDeployResult{ServiceURL: serviceURL(spec.ArtifactID, k.Namespace)}, nil
}

// Update re-uploads the HTML implicitly (the Service-side caller
// PutBytes-overwrote the Object key before invoking) and triggers a
// rolling restart by patching the pod-template's restart annotation.
// The init container re-runs against the new presigned URL on the
// next pod, picking up the new HTML.
func (k *KubernetesAppDeployer) Update(ctx context.Context, spec AppSpec) (AppDeployResult, error) {
	if err := validateAppSpec(spec); err != nil {
		return AppDeployResult{}, err
	}
	name := appResourceName(spec.ArtifactID)
	dep, err := k.Client.AppsV1().Deployments(k.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return k.Deploy(ctx, spec)
		}
		return AppDeployResult{}, fmt.Errorf("runtime: get Deployment: %w", err)
	}

	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations["nightshift.io/restart-at"] = time.Now().UTC().Format(time.RFC3339Nano)

	// Refresh the init container's DOWNLOAD_URL — old presigned URLs
	// expire and would brick the rolling restart otherwise.
	for i := range dep.Spec.Template.Spec.InitContainers {
		ic := &dep.Spec.Template.Spec.InitContainers[i]
		for j := range ic.Env {
			if ic.Env[j].Name == "DOWNLOAD_URL" {
				ic.Env[j].Value = spec.DownloadURL
			}
		}
	}

	if _, err := k.Client.AppsV1().Deployments(k.Namespace).Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
		return AppDeployResult{}, fmt.Errorf("runtime: update Deployment: %w", err)
	}
	return AppDeployResult{ServiceURL: serviceURL(spec.ArtifactID, k.Namespace)}, nil
}

// Delete tears down the Deployment + Service. NotFound is idempotent.
func (k *KubernetesAppDeployer) Delete(ctx context.Context, artifactID string) error {
	if artifactID == "" {
		return errors.New("runtime: artifactID required")
	}
	name := appResourceName(artifactID)
	propagation := metav1.DeletePropagationBackground
	if err := k.Client.AppsV1().Deployments(k.Namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("runtime: delete Deployment %s: %w", name, err)
	}
	if err := k.Client.CoreV1().Services(k.Namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("runtime: delete Service %s: %w", name, err)
	}
	return nil
}

func (k *KubernetesAppDeployer) buildDeployment(spec AppSpec) *appsv1.Deployment {
	name := appResourceName(spec.ArtifactID)
	labels := map[string]string{
		"app.kubernetes.io/name":      "nightshift",
		"app.kubernetes.io/component": "app-artifact",
		"nightshift.io/app":           "nightshift-app",
		"nightshift.io/artifact-id":   labelValueSafe(spec.ArtifactID),
	}
	selector := map[string]string{
		"nightshift.io/app":         "nightshift-app",
		"nightshift.io/artifact-id": labelValueSafe(spec.ArtifactID),
	}

	// nginx-unprivileged runs as uid 101 by design; FSGroup=101 makes
	// the emptyDir owned by gid 101 so both the init container (curl
	// writing the HTML) and nginx (reading it) can access without
	// supplementary-group acrobatics. Both containers run as 101 to
	// keep file ownership stable across init→main.
	const nginxUnprivilegedUID int64 = 101
	podSC := &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		RunAsUser:      ptr.To(nginxUnprivilegedUID),
		RunAsGroup:     ptr.To(nginxUnprivilegedUID),
		FSGroup:        ptr.To(nginxUnprivilegedUID),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	initSC := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To(nginxUnprivilegedUID),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	// nginx-unprivileged is designed for a read-only rootfs: writable
	// state lives in /var/cache/nginx and /tmp which the image declares
	// as VOLUME, so emptyDir mounts there satisfy nginx without
	// loosening hardening.
	nginxSC := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To(nginxUnprivilegedUID),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}

	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      selector,
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext:              podSC,
					Volumes: []corev1.Volume{
						{Name: "html", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						// nginx-unprivileged needs /var/cache/nginx + /tmp
						// writable for working dirs; emptyDirs satisfy that
						// without loosening readOnlyRootFilesystem.
						{Name: "nginx-cache", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
					InitContainers: []corev1.Container{
						{
							Name:  "fetch-html",
							Image: k.Config.InitImage,
							Command: []string{
								"sh", "-c",
								// curl -fsSL: fail on HTTP errors, silent except errors,
								// follow redirects (presign handlers may 302). The init
								// container retries on container-start if curl fails
								// (Pod restartPolicy=Always for init containers).
								`curl -fsSL "$DOWNLOAD_URL" -o /data/index.html`,
							},
							Env: []corev1.EnvVar{
								{Name: "DOWNLOAD_URL", Value: spec.DownloadURL},
							},
							SecurityContext: initSC,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "html", MountPath: "/data"},
								{Name: "tmp", MountPath: "/tmp"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "nginx",
							Image:           k.Config.NginxImage,
							SecurityContext: nginxSC,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Name: "http"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "html", MountPath: "/usr/share/nginx/html", ReadOnly: true},
								{Name: "nginx-cache", MountPath: "/var/cache/nginx"},
								{Name: "tmp", MountPath: "/tmp"},
							},
						},
					},
				},
			},
		},
	}
}

func (k *KubernetesAppDeployer) buildService(spec AppSpec) *corev1.Service {
	name := appResourceName(spec.ArtifactID)
	labels := map[string]string{
		"app.kubernetes.io/name":      "nightshift",
		"app.kubernetes.io/component": "app-artifact",
		"nightshift.io/app":           "nightshift-app",
		"nightshift.io/artifact-id":   labelValueSafe(spec.ArtifactID),
	}
	selector := map[string]string{
		"nightshift.io/app":         "nightshift-app",
		"nightshift.io/artifact-id": labelValueSafe(spec.ArtifactID),
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)},
			},
		},
	}
}

// appResourceName produces a DNS-1123-safe Deployment/Service name
// from artifactID. K8s names cap at 63 chars; we use the first 8 chars
// of the UUID portion so the names stay short and human-glanceable.
func appResourceName(artifactID string) string {
	short := artifactID
	if i := strings.LastIndex(short, "_"); i >= 0 {
		short = short[i+1:]
	}
	short = strings.ReplaceAll(short, "-", "")
	if len(short) > 8 {
		short = short[:8]
	}
	short = strings.ToLower(short)
	return "ns-app-" + short
}

func serviceURL(artifactID, namespace string) string {
	return fmt.Sprintf("http://%s.%s.svc:80", appResourceName(artifactID), namespace)
}

func validateAppSpec(spec AppSpec) error {
	if spec.ArtifactID == "" {
		return errors.New("runtime: AppSpec.ArtifactID required")
	}
	if spec.DownloadURL == "" {
		return errors.New("runtime: AppSpec.DownloadURL required")
	}
	return nil
}

// -----------------------------------------------------------------------------
// StubAppDeployer
// -----------------------------------------------------------------------------

// StubAppDeployer is for NS_RUNTIME=stub. It records deployments in
// memory + returns a sentinel ServiceURL the proxy handler recognizes
// as "no real backend" and returns 503 for. Lets the API process boot
// + the artifact records persist in dev without a K8s API.
type StubAppDeployer struct {
	mu    sync.Mutex
	specs map[string]AppSpec
}

func NewStubAppDeployer() *StubAppDeployer {
	return &StubAppDeployer{specs: map[string]AppSpec{}}
}

func (s *StubAppDeployer) Deploy(_ context.Context, spec AppSpec) (AppDeployResult, error) {
	if err := validateAppSpec(spec); err != nil {
		return AppDeployResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.specs[spec.ArtifactID] = spec
	return AppDeployResult{ServiceURL: StubAppServiceURL(spec.ArtifactID)}, nil
}

func (s *StubAppDeployer) Update(ctx context.Context, spec AppSpec) (AppDeployResult, error) {
	return s.Deploy(ctx, spec)
}

func (s *StubAppDeployer) Delete(_ context.Context, artifactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.specs, artifactID)
	return nil
}

func (s *StubAppDeployer) Close() error { return nil }

// StubAppServicePrefix is the sentinel URL prefix the proxy handler
// uses to detect "this artifact was deployed by the stub deployer; no
// real backend exists." Returns 503 in that case.
const StubAppServicePrefix = "stub://nightshift-app/"

// StubAppServiceURL builds the sentinel URL the stub deployer returns
// in place of an in-cluster Service URL. Exported so the proxy handler
// can pattern-match.
func StubAppServiceURL(artifactID string) string {
	return StubAppServicePrefix + artifactID
}
