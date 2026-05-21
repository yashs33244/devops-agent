package verifiers

import "strings"

// Method-policy table. Maps a fully-qualified gRPC method name to the
// set of Schemes whose principals are allowed to invoke it.
//
// The policy is deliberately coarse — two tiers in v1:
//
//   1. Inner surface (worker callbacks): SchemeWorker only. The
//      credential encodes a specific run_id; the handler additionally
//      calls RequireWorkerRunID to enforce that scope.
//
//   2. Outer surface (everything else under /nightshift.v1.*):
//      SchemeUser (OIDC) or SchemeService (static token).
//
// Anything outside /nightshift.v1.* — gRPC health, server reflection —
// is unauthenticated. Returning nil from AllowedSchemes signals the
// interceptor to pass the request through without a bearer check.
//
// Per-user RBAC (owner/editor/viewer on specific resources) is
// deferred to chunk 11 Config. This file only answers "is this caller
// allowed on the wire at all?"

// innerSurface lists the worker-callback RPCs. Worker credentials
// scope to a specific run_id; no other scheme may invoke these.
//
// Config.GetUserConfig is on the worker surface even though it lives
// on the Config service: it materializes the per-run user config and
// is called only by the worker at run start.
var innerSurface = map[string]struct{}{
	"/nightshift.v1.Workers/PostWorkerEvent":    {},
	"/nightshift.v1.Workers/CompleteRun":        {},
	"/nightshift.v1.Workers/FailRun":            {},
	"/nightshift.v1.Workers/GetRunCancellation": {},
	"/nightshift.v1.Config/GetUserConfig":       {},
}

// sharedSurface lists outer-surface RPCs that ALSO accept SchemeWorker
// (in addition to SchemeUser + SchemeService). The Artifacts service
// is the canonical case: workers create + update artifacts mid-run on
// behalf of their bound run's owner. Per-resource RBAC (owner /
// editor / viewer; worker-only-on-its-own-run) lives in the service
// handlers, not here — this table only answers "is this caller
// allowed on the wire at all?".
var sharedSurface = map[string]struct{}{
	"/nightshift.v1.Artifacts/CreateObjectArtifact":    {},
	"/nightshift.v1.Artifacts/CreateAppArtifact":       {},
	"/nightshift.v1.Artifacts/GetArtifact":             {},
	"/nightshift.v1.Artifacts/ListArtifacts":           {},
	"/nightshift.v1.Artifacts/UpdateArtifact":          {},
	"/nightshift.v1.Artifacts/DeleteArtifact":          {},
	"/nightshift.v1.Artifacts/GetArtifactDownloadURL":  {},
	"/nightshift.v1.Artifacts/GetArtifactPreviewURL":   {},
	"/nightshift.v1.Artifacts/ShareArtifact":           {},
	"/nightshift.v1.Artifacts/RevokeArtifactShare":     {},
	"/nightshift.v1.Artifacts/ListArtifactPermissions": {},
}

// AllowedSchemes returns the set of schemes permitted on fullMethod.
// An empty return means the method is unauthenticated (no bearer
// required). A nil return never happens — callers treat empty as
// "pass through".
func AllowedSchemes(fullMethod string) []Scheme {
	if _, ok := innerSurface[fullMethod]; ok {
		return []Scheme{SchemeWorker}
	}
	if _, ok := sharedSurface[fullMethod]; ok {
		return []Scheme{SchemeUser, SchemeService, SchemeWorker}
	}
	if strings.HasPrefix(fullMethod, "/nightshift.v1.") {
		return []Scheme{SchemeUser, SchemeService}
	}
	return nil
}

// IsInnerSurface reports whether fullMethod is a worker-callback RPC.
// Exposed for tests and for the interceptor's post-auth scope check.
func IsInnerSurface(fullMethod string) bool {
	_, ok := innerSurface[fullMethod]
	return ok
}
