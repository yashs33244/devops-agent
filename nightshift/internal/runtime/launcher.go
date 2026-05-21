// Package runtime provides the JobLauncher abstraction and its
// reference backends — an in-process subprocess launcher for dev /
// tests (stub.go) and a Kubernetes Jobs launcher for production
// (kubernetes.go).
//
// The JobLauncher is the seam between the Workers gRPC service and
// the runtime that actually executes a worker image. Every worker
// invocation is described by a single LaunchSpec; implementations
// translate that into whatever their runtime requires.
package runtime

import (
	"context"
	"errors"
)

// Sentinel errors. Callers translate these into gRPC codes at the
// API boundary.
var (
	ErrAlreadyLaunched = errors.New("runtime: run already launched")
	ErrNotFound        = errors.New("runtime: run not found")
)

// ResourceReqs describes CPU / memory constraints. Empty strings mean
// "use the launcher default".
type ResourceReqs struct {
	CPU    string // e.g. "500m", "2"
	Memory string // e.g. "512Mi", "4Gi"
}

// LaunchSpec is the complete control surface a caller hands to a
// JobLauncher. Implementations translate the fields into their
// runtime's native shape.
type LaunchSpec struct {
	// Identity.
	RunID     string // required
	UserID    string // required
	SessionID string // optional
	Prompt    string // required

	// Image + callback.
	Image       string // container image URI (ignored by stub launcher)
	CallbackURL string // base URL the worker calls back to

	// Worker credential. HMAC-signed, scoped to RunID. The Workers
	// service mints this before invoking Launch.
	WorkerCredential string

	// Timeouts + resource constraints (honored by k8s launcher;
	// informational for stub).
	TTLSecondsAfterFinished int32
	ActiveDeadlineSeconds   int64
	Resources               ResourceReqs

	// Additional env vars merged on top of the NS_* convention.
	ExtraEnv map[string]string

	// SessionState describes the per-session volume the launcher
	// mounts into the worker pod (or process). Backend == "" / "none"
	// disables the mount. See volumes.go for the path conventions.
	SessionState SessionStateConfig

	// SDKSessionID is the agent-SDK-internal session id, looked up by
	// the API for resume continuity (chunk 14). Implementation-coupled
	// to the worker image; opaque to the platform. Empty for fresh
	// sessions or when the worker image doesn't support resume. The
	// launcher exports it as NS_SDK_SESSION_ID when non-empty.
	SDKSessionID string

	// MountServiceAccountToken controls whether the worker pod
	// gets the default Kubernetes ServiceAccount token mounted at
	// /var/run/secrets/kubernetes.io/serviceaccount/token.
	//
	// Default false (chunk-9 hardening). The Go simulated worker
	// authenticates to the API via NS_WORKER_CREDENTIAL HMAC and has
	// no need for an SA token. The chunk-14 Python worker sets this
	// to true so it can perform OpenBao Kubernetes-auth login to
	// fetch the Anthropic API key. The worker SA has no K8s API
	// permissions in the chart, so flipping this on does not expand
	// the worker's K8s attack surface — the token is only useful
	// against OpenBao via the bound auth role.
	MountServiceAccountToken bool
}

// JobLauncher is the pluggable runtime backend. Implementations MUST
// be safe for concurrent use.
type JobLauncher interface {
	// Launch starts the worker for spec.RunID. MUST be idempotent on
	// repeat calls with the same RunID — a second Launch returns nil
	// if the run is already in flight.
	Launch(ctx context.Context, spec LaunchSpec) error

	// Interrupt requests cancellation for runID. The worker observes
	// cancellation via the Workers.GetRunCancellation RPC; Interrupt
	// is the fallback hard-stop path (delete K8s Job / signal
	// subprocess).
	//
	// MUST be idempotent — calling Interrupt on an unknown or
	// already-terminated run returns nil.
	Interrupt(ctx context.Context, runID string) error

	// Close releases any resources held by the launcher (notably
	// subprocess trackers for the stub impl).
	Close() error
}
