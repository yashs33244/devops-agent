package verifiers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RequireWorkerRunID asserts the request context carries a
// worker-scheme principal whose encoded run_id matches runID. Called
// as the first line of every worker-callback handler
// (PostWorkerEvent, CompleteRun, FailRun, GetRunCancellation) to
// enforce worker-protocol §Scope: a credential for run A MUST NOT
// authorize RPCs against run B even if both are live.
//
// The interceptor has already guaranteed that only SchemeWorker
// reached this handler. This check is the run_id-matching half —
// separate because it requires inspecting the request body, which the
// interceptor deliberately does not do.
func RequireWorkerRunID(ctx context.Context, runID string) error {
	p := FromContext(ctx)
	if p == nil || p.Scheme != SchemeWorker {
		return status.Error(codes.Unauthenticated, "worker credential required")
	}
	if p.RunID != runID {
		return status.Error(codes.PermissionDenied, "worker credential does not authorize this run")
	}
	return nil
}
