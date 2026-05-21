package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

func TestRunCompletedIncrementsAllRunCollectors(t *testing.T) {
	r := NewPromRecorder()
	r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)

	usage := &nsv1.RunUsage{
		TotalCostUsd:        0.0125,
		InputTokens:         1234,
		OutputTokens:        567,
		CacheReadTokens:     89,
		CacheCreationTokens: 12,
	}
	r.RunCompleted(
		nsv1.RunStatus_RUN_STATUS_COMPLETED,
		nsv1.InvokerType_INVOKER_TYPE_USER,
		23*time.Second,
		usage,
	)

	if v := testutil.ToFloat64(r.runsTotal.WithLabelValues("COMPLETED", "USER")); v != 1 {
		t.Fatalf("runs_total=%v", v)
	}
	if v := testutil.ToFloat64(r.runCostUSDTotal); v != 0.0125 {
		t.Fatalf("cost_usd_total=%v", v)
	}
	if v := testutil.ToFloat64(r.runTokensTotal.WithLabelValues("input")); v != 1234 {
		t.Fatalf("tokens.input=%v", v)
	}
	if v := testutil.ToFloat64(r.runTokensTotal.WithLabelValues("output")); v != 567 {
		t.Fatalf("tokens.output=%v", v)
	}
	if v := testutil.ToFloat64(r.runTokensTotal.WithLabelValues("cache_read")); v != 89 {
		t.Fatalf("tokens.cache_read=%v", v)
	}
	if v := testutil.ToFloat64(r.runTokensTotal.WithLabelValues("cache_creation")); v != 12 {
		t.Fatalf("tokens.cache_creation=%v", v)
	}
	if v := testutil.ToFloat64(r.activeRuns); v != 0 {
		t.Fatalf("active_runs=%v (created+completed should net to 0)", v)
	}
	if c := testutil.CollectAndCount(r.runDuration); c != 1 {
		t.Fatalf("duration histogram series=%d", c)
	}
}

func TestActiveRunsTracksCreateAndComplete(t *testing.T) {
	r := NewPromRecorder()
	for i := 0; i < 3; i++ {
		r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)
	}
	if v := testutil.ToFloat64(r.activeRuns); v != 3 {
		t.Fatalf("after 3 creates, active=%v", v)
	}
	r.RunCompleted(nsv1.RunStatus_RUN_STATUS_COMPLETED, nsv1.InvokerType_INVOKER_TYPE_USER, time.Second, nil)
	if v := testutil.ToFloat64(r.activeRuns); v != 2 {
		t.Fatalf("after 1 complete, active=%v", v)
	}
}

func TestRecountActiveSetsGauge(t *testing.T) {
	r := NewPromRecorder()
	r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)
	r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)
	r.RecountActive(7)
	if v := testutil.ToFloat64(r.activeRuns); v != 7 {
		t.Fatalf("after Recount(7), active=%v", v)
	}
}

func TestRunCompletedNilUsageSkipsCostAndTokens(t *testing.T) {
	r := NewPromRecorder()
	r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)
	r.RunCompleted(nsv1.RunStatus_RUN_STATUS_ERROR, nsv1.InvokerType_INVOKER_TYPE_USER, time.Second, nil)
	if v := testutil.ToFloat64(r.runsTotal.WithLabelValues("ERROR", "USER")); v != 1 {
		t.Fatalf("runs_total[ERROR]=%v", v)
	}
	if v := testutil.ToFloat64(r.runCostUSDTotal); v != 0 {
		t.Fatalf("cost should stay 0 with nil usage, got %v", v)
	}
}

func TestArtifactCreatedSuccessIncrementsBoth(t *testing.T) {
	r := NewPromRecorder()
	r.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT, "success")
	r.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT, "success")
	r.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_APP, "error")

	if v := testutil.ToFloat64(r.artifactsTotal.WithLabelValues("object")); v != 2 {
		t.Fatalf("artifacts_total[object]=%v", v)
	}
	if v := testutil.ToFloat64(r.artifactsTotal.WithLabelValues("app")); v != 0 {
		t.Fatalf("artifacts_total[app] should not bump on error, got %v", v)
	}
	if v := testutil.ToFloat64(r.artifactCreatesTotal.WithLabelValues("object", "success")); v != 2 {
		t.Fatalf("creates[object,success]=%v", v)
	}
	if v := testutil.ToFloat64(r.artifactCreatesTotal.WithLabelValues("app", "error")); v != 1 {
		t.Fatalf("creates[app,error]=%v", v)
	}
}

func TestAppDeployedAndScheduleCounters(t *testing.T) {
	r := NewPromRecorder()
	r.AppDeployed("success")
	r.AppDeployed("success")
	r.AppDeployed("error")
	r.ScheduleCreated()
	r.ScheduleDeleted()
	r.ScheduleDeleted()

	if v := testutil.ToFloat64(r.appDeploysTotal.WithLabelValues("success")); v != 2 {
		t.Fatalf("app_deploys[success]=%v", v)
	}
	if v := testutil.ToFloat64(r.appDeploysTotal.WithLabelValues("error")); v != 1 {
		t.Fatalf("app_deploys[error]=%v", v)
	}
	if v := testutil.ToFloat64(r.schedulesCreatedTotal); v != 1 {
		t.Fatalf("schedules_created=%v", v)
	}
	if v := testutil.ToFloat64(r.schedulesDeletedTotal); v != 2 {
		t.Fatalf("schedules_deleted=%v", v)
	}
}

func TestHandlerExposesPrometheusFormat(t *testing.T) {
	r := NewPromRecorder()
	r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)
	r.RunCompleted(nsv1.RunStatus_RUN_STATUS_COMPLETED, nsv1.InvokerType_INVOKER_TYPE_USER, 5*time.Second,
		&nsv1.RunUsage{TotalCostUsd: 0.05, OutputTokens: 100})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"nightshift_runs_total",
		"nightshift_run_cost_usd_total",
		"nightshift_run_tokens_total",
		"nightshift_active_runs",
		"go_goroutines",
		`status="COMPLETED"`,
		`invoker_type="USER"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected /metrics body to contain %q\nbody:\n%s", want, body[:min(len(body), 4000)])
		}
	}
}

func TestNoopRecorderIsNilSafe(t *testing.T) {
	var r Recorder // nil
	r = Get(r)
	r.RunCreated(nsv1.InvokerType_INVOKER_TYPE_USER)
	r.RunCompleted(nsv1.RunStatus_RUN_STATUS_COMPLETED, nsv1.InvokerType_INVOKER_TYPE_USER, time.Second, nil)
	r.ArtifactCreated(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT, "success")
	r.ArtifactDeleted(nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT)
	r.AppDeployed("success")
	r.ScheduleCreated()
	r.ScheduleDeleted()
	r.RecountActive(0)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
