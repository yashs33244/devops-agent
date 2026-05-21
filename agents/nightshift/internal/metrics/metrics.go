// Package metrics is the chunk-18 Prometheus collector layer.
//
// One Recorder interface, one PromRecorder impl that updates a fixed
// set of named collectors, one NoopRecorder for tests + nil-safe
// defaults. Hook sites in internal/api/workers + internal/api/artifacts
// + internal/api/scheduling call Recorder methods at the same points
// where they already persist state — emission and persistence happen
// together.
//
// All collectors live on a private *prometheus.Registry the Recorder
// owns. Handler() returns the http.Handler operators mount at
// /metrics on the chunk-18 dedicated listener (default :9090). Default
// Go runtime + process collectors are registered alongside.
//
// cr0n parity: the eight nightshift_* collectors mirror cr0n's metrics.py
// names + types, with two deliberate omissions:
//   - cr0n_run_cost{run_id, prompt} and cr0n_run_duration{run_id} are
//     NOT exported. Per-run id labels create unbounded cardinality and
//     leak the prompt into log shipping. Per-run cost is queryable on
//     the Run record itself via GetRun / ListRuns.
//   - cr0n_artifact_storage_bytes is NOT exported until the Artifact
//     proto carries size_bytes for app artifacts (today only object
//     artifacts populate the field).
package metrics

import (
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

// Recorder is the seam every domain service consumes. PromRecorder is
// the production impl; tests use NoopRecorder or pass nil and rely on
// the nil-safe Get helper.
type Recorder interface {
	RunCreated(invokerType nsv1.InvokerType)
	RunCompleted(status nsv1.RunStatus, invokerType nsv1.InvokerType, duration time.Duration, usage *nsv1.RunUsage)
	SessionCreated()
	ArtifactCreated(t nsv1.ArtifactType, outcome string)
	ArtifactDeleted(t nsv1.ArtifactType)
	AppDeployed(outcome string)
	ScheduleCreated()
	ScheduleDeleted()
	RecountActive(active int)
}

// Get returns r if non-nil, otherwise a no-op recorder. Hook sites
// call Get(s.metrics).RunCreated(...) so a Service constructed
// without a recorder doesn't panic.
func Get(r Recorder) Recorder {
	if r == nil {
		return NoopRecorder{}
	}
	return r
}

// PromRecorder updates the chunk-18 collectors backed by a private
// Registry. One PromRecorder per process — collectors are registered
// at construction and live for the lifetime of the binary.
type PromRecorder struct {
	registry *prometheus.Registry

	runsTotal       *prometheus.CounterVec
	runDuration     *prometheus.HistogramVec
	runCostUSDTotal prometheus.Counter
	runTokensTotal  *prometheus.CounterVec
	activeRuns      prometheus.Gauge

	artifactsTotal       *prometheus.CounterVec
	artifactCreatesTotal *prometheus.CounterVec
	appDeploysTotal      *prometheus.CounterVec

	schedulesCreatedTotal prometheus.Counter
	schedulesDeletedTotal prometheus.Counter

	sessionsCreatedTotal prometheus.Counter
}

// NewPromRecorder constructs a recorder + registers every collector
// on a fresh private Registry. Default Go runtime + process collectors
// are registered alongside so /metrics emits a complete picture.
func NewPromRecorder() *PromRecorder {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	factory := promauto.With(reg)
	r := &PromRecorder{registry: reg}

	r.runsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "nightshift_runs_total",
		Help: "Total runs that reached a terminal status, by status and invoker_type.",
	}, []string{"status", "invoker_type"})

	r.runDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nightshift_run_duration_seconds",
		Help:    "Wall-clock duration of completed runs, in seconds, by terminal status.",
		Buckets: []float64{10, 30, 60, 120, 300, 600, 1800, 3600},
	}, []string{"status"})

	r.runCostUSDTotal = factory.NewCounter(prometheus.CounterOpts{
		Name: "nightshift_run_cost_usd_total",
		Help: "Cumulative USD spend across all completed runs, as reported by workers in RunUsage.",
	})

	r.runTokensTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "nightshift_run_tokens_total",
		Help: "Cumulative tokens across all completed runs, by kind (input/output/cache_read/cache_creation).",
	}, []string{"kind"})

	r.activeRuns = factory.NewGauge(prometheus.GaugeOpts{
		Name: "nightshift_active_runs",
		Help: "Runs currently in PENDING or RUNNING state. Recounted from the runs collection at API startup.",
	})

	r.artifactsTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "nightshift_artifacts_total",
		Help: "Cumulative artifacts that successfully landed in storage, by type.",
	}, []string{"type"})

	r.artifactCreatesTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "nightshift_artifact_creates_total",
		Help: "Total CreateObjectArtifact + CreateAppArtifact attempts, by type and outcome (success|error).",
	}, []string{"type", "outcome"})

	r.appDeploysTotal = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "nightshift_app_deploys_total",
		Help: "Total app artifact K8s deploy attempts, by outcome (success|error). Distinct from artifact_creates_total because deploy is a separate K8s operation worth its own counter.",
	}, []string{"outcome"})

	r.schedulesCreatedTotal = factory.NewCounter(prometheus.CounterOpts{
		Name: "nightshift_schedules_created_total",
		Help: "Total schedules created.",
	})
	r.sessionsCreatedTotal = factory.NewCounter(prometheus.CounterOpts{
		Name: "nightshift_sessions_created_total",
		Help: "Total session ids minted via Workers.CreateSession.",
	})
	r.schedulesDeletedTotal = factory.NewCounter(prometheus.CounterOpts{
		Name: "nightshift_schedules_deleted_total",
		Help: "Total schedules deleted.",
	})

	return r
}

// Registry returns the underlying registry. Tests use it to assert
// over the collected state.
func (r *PromRecorder) Registry() *prometheus.Registry {
	return r.registry
}

// Handler returns an http.Handler suitable for mounting at /metrics.
// Honors Accept-Encoding gzip per the Prometheus exposition spec.
func (r *PromRecorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	})
}

// RunCreated bumps active_runs and is called once per CreateRun.
// Note: invokerType isn't recorded as a label here because runs_total
// already carries it on the completion side, which is where the
// useful query lives (rate of completed runs by invoker).
func (r *PromRecorder) RunCreated(invokerType nsv1.InvokerType) {
	_ = invokerType
	r.activeRuns.Inc()
}

// RunCompleted records the terminal state. Decrements active_runs.
// usage may be nil for runs that error before the worker reports it;
// in that case cost + tokens are skipped.
func (r *PromRecorder) RunCompleted(s nsv1.RunStatus, inv nsv1.InvokerType, d time.Duration, usage *nsv1.RunUsage) {
	statusLbl := strings.TrimPrefix(s.String(), "RUN_STATUS_")
	invokerLbl := strings.TrimPrefix(inv.String(), "INVOKER_TYPE_")
	r.runsTotal.WithLabelValues(statusLbl, invokerLbl).Inc()
	r.runDuration.WithLabelValues(statusLbl).Observe(d.Seconds())
	r.activeRuns.Dec()
	if usage != nil {
		if usage.GetTotalCostUsd() > 0 {
			r.runCostUSDTotal.Add(usage.GetTotalCostUsd())
		}
		if v := usage.GetInputTokens(); v > 0 {
			r.runTokensTotal.WithLabelValues("input").Add(float64(v))
		}
		if v := usage.GetOutputTokens(); v > 0 {
			r.runTokensTotal.WithLabelValues("output").Add(float64(v))
		}
		if v := usage.GetCacheReadTokens(); v > 0 {
			r.runTokensTotal.WithLabelValues("cache_read").Add(float64(v))
		}
		if v := usage.GetCacheCreationTokens(); v > 0 {
			r.runTokensTotal.WithLabelValues("cache_creation").Add(float64(v))
		}
	}
}

// ArtifactCreated records the outcome of a CreateObjectArtifact /
// CreateAppArtifact. On success, also increments the cumulative
// artifacts_total gauge for at-rest count.
func (r *PromRecorder) ArtifactCreated(t nsv1.ArtifactType, outcome string) {
	typeLbl := artifactTypeLabel(t)
	r.artifactCreatesTotal.WithLabelValues(typeLbl, outcome).Inc()
	if outcome == "success" {
		r.artifactsTotal.WithLabelValues(typeLbl).Inc()
	}
}

// ArtifactDeleted is a no-op on artifacts_total (it's a Counter, not a
// Gauge — running totals don't decrement on delete). Kept on the
// interface so future per-type retention metrics can land without
// touching every call site.
func (r *PromRecorder) ArtifactDeleted(t nsv1.ArtifactType) {
	_ = t
}

// AppDeployed records the success/error of the K8s Deploy step. Always
// follows ArtifactCreated for app artifacts (an app create that
// succeeds in storage but fails the deploy is recorded as
// ArtifactCreated{outcome=error} + AppDeployed{outcome=error}).
func (r *PromRecorder) AppDeployed(outcome string) {
	r.appDeploysTotal.WithLabelValues(outcome).Inc()
}

func (r *PromRecorder) ScheduleCreated() { r.schedulesCreatedTotal.Inc() }
func (r *PromRecorder) ScheduleDeleted() { r.schedulesDeletedTotal.Inc() }
func (r *PromRecorder) SessionCreated()  { r.sessionsCreatedTotal.Inc() }

// RecountActive forces active_runs to a known value. Called once at
// API startup after loading the runs collection so the gauge survives
// restarts. Documented race: a concurrent CreateRun during recount can
// double-count by one — acceptable for an operator dashboard gauge.
func (r *PromRecorder) RecountActive(active int) {
	r.activeRuns.Set(float64(active))
}

func artifactTypeLabel(t nsv1.ArtifactType) string {
	switch t {
	case nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT:
		return "object"
	case nsv1.ArtifactType_ARTIFACT_TYPE_APP:
		return "app"
	}
	return "unspecified"
}

// -----------------------------------------------------------------------------
// NoopRecorder
// -----------------------------------------------------------------------------

// NoopRecorder satisfies Recorder without doing anything. Used by
// tests + as the nil-safe default returned by Get.
type NoopRecorder struct{}

func (NoopRecorder) RunCreated(nsv1.InvokerType)                                                  {}
func (NoopRecorder) RunCompleted(nsv1.RunStatus, nsv1.InvokerType, time.Duration, *nsv1.RunUsage) {}
func (NoopRecorder) ArtifactCreated(nsv1.ArtifactType, string)                                    {}
func (NoopRecorder) ArtifactDeleted(nsv1.ArtifactType)                                            {}
func (NoopRecorder) AppDeployed(string)                                                           {}
func (NoopRecorder) ScheduleCreated()                                                             {}
func (NoopRecorder) ScheduleDeleted()                                                             {}
func (NoopRecorder) SessionCreated()                                                              {}
func (NoopRecorder) RecountActive(int)                                                            {}
