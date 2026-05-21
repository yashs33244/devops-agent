package verifiers

import "testing"

func TestAllowedSchemes_InnerSurface(t *testing.T) {
	cases := []string{
		"/nightshift.v1.Workers/PostWorkerEvent",
		"/nightshift.v1.Workers/CompleteRun",
		"/nightshift.v1.Workers/FailRun",
		"/nightshift.v1.Workers/GetRunCancellation",
		"/nightshift.v1.Config/GetUserConfig",
	}
	for _, m := range cases {
		got := AllowedSchemes(m)
		if len(got) != 1 || got[0] != SchemeWorker {
			t.Errorf("%s: got %v, want [worker]", m, got)
		}
		if !IsInnerSurface(m) {
			t.Errorf("%s: IsInnerSurface=false", m)
		}
	}
}

func TestAllowedSchemes_OuterSurface(t *testing.T) {
	cases := []string{
		"/nightshift.v1.Workers/CreateRun",
		"/nightshift.v1.Workers/GetRun",
		"/nightshift.v1.Workers/StreamRunEvents",
		"/nightshift.v1.Storage/PutRecord",
		"/nightshift.v1.Config/CreateAgent",
		"/nightshift.v1.Config/ListConnectors",
		"/nightshift.v1.Secrets/GetSecret",
		"/nightshift.v1.Scheduling/CreateSchedule",
	}
	for _, m := range cases {
		got := AllowedSchemes(m)
		if len(got) != 2 {
			t.Errorf("%s: len=%d, want 2", m, len(got))
			continue
		}
		hasUser, hasService := false, false
		for _, s := range got {
			if s == SchemeUser {
				hasUser = true
			}
			if s == SchemeService {
				hasService = true
			}
		}
		if !hasUser || !hasService {
			t.Errorf("%s: got %v, want [user service]", m, got)
		}
		if IsInnerSurface(m) {
			t.Errorf("%s: IsInnerSurface=true", m)
		}
	}
}

func TestAllowedSchemes_SharedSurface(t *testing.T) {
	// Artifacts methods are reachable by users (UI), services (admin
	// tooling), AND workers (mid-run artifact creation). Per-resource
	// RBAC is enforced inside the service handlers.
	cases := []string{
		"/nightshift.v1.Artifacts/CreateObjectArtifact",
		"/nightshift.v1.Artifacts/GetArtifact",
		"/nightshift.v1.Artifacts/ListArtifacts",
		"/nightshift.v1.Artifacts/UpdateArtifact",
		"/nightshift.v1.Artifacts/DeleteArtifact",
		"/nightshift.v1.Artifacts/GetArtifactDownloadURL",
		"/nightshift.v1.Artifacts/GetArtifactPreviewURL",
		"/nightshift.v1.Artifacts/ShareArtifact",
		"/nightshift.v1.Artifacts/RevokeArtifactShare",
		"/nightshift.v1.Artifacts/ListArtifactPermissions",
	}
	for _, m := range cases {
		got := AllowedSchemes(m)
		if len(got) != 3 {
			t.Errorf("%s: len=%d, want 3", m, len(got))
			continue
		}
		var hasUser, hasService, hasWorker bool
		for _, s := range got {
			switch s {
			case SchemeUser:
				hasUser = true
			case SchemeService:
				hasService = true
			case SchemeWorker:
				hasWorker = true
			}
		}
		if !hasUser || !hasService || !hasWorker {
			t.Errorf("%s: got %v, want [user service worker]", m, got)
		}
		if IsInnerSurface(m) {
			t.Errorf("%s: IsInnerSurface=true", m)
		}
	}
}

func TestAllowedSchemes_UnauthenticatedMethods(t *testing.T) {
	cases := []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/some.other.v1.Svc/RPC",
	}
	for _, m := range cases {
		if got := AllowedSchemes(m); len(got) != 0 {
			t.Errorf("%s: got %v, want empty (unauthenticated)", m, got)
		}
	}
}
