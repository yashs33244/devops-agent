package artifacts

import (
	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// canRead reports whether p may view art (read metadata, list, fetch
// download/preview URLs). Public artifacts are visible to any
// authenticated principal. Owner is always allowed. Otherwise a
// matching grant of any role suffices. Worker-credentialed callers
// have access to artifacts they themselves produced (run_id matches
// their bound run).
func canRead(p *verifiers.Principal, art *nsv1.Artifact, grants []*nsv1.ArtifactPermission, viewerID string) bool {
	if art == nil {
		return false
	}
	if art.GetPublic() {
		return true
	}
	if p != nil && p.Scheme == verifiers.SchemeWorker && art.GetRunId() != "" && art.GetRunId() == p.RunID {
		return true
	}
	if viewerID != "" && art.GetOwnerId() == viewerID {
		return true
	}
	for _, g := range grants {
		if g.GetUserId() == viewerID && g.GetRole() != nsv1.ArtifactRole_ARTIFACT_ROLE_UNSPECIFIED {
			return true
		}
	}
	return false
}

// canEdit reports whether p may UpdateArtifact (metadata patches, blob
// replacement). Owner + EDITOR role qualify. Worker callers acting on
// behalf of the run's owner pass via the owner check (viewerID is
// derived from the run record by the handler).
func canEdit(art *nsv1.Artifact, grants []*nsv1.ArtifactPermission, viewerID string) bool {
	if art == nil {
		return false
	}
	if viewerID != "" && art.GetOwnerId() == viewerID {
		return true
	}
	for _, g := range grants {
		if g.GetUserId() != viewerID {
			continue
		}
		switch g.GetRole() {
		case nsv1.ArtifactRole_ARTIFACT_ROLE_EDITOR, nsv1.ArtifactRole_ARTIFACT_ROLE_OWNER:
			return true
		}
	}
	return false
}

// canAdmin reports whether p may toggle public/private, share, revoke,
// or delete. Owner only — matches cr0n-a's owner_id == user_id check
// in artifact_routes.py.
func canAdmin(art *nsv1.Artifact, viewerID string) bool {
	if art == nil {
		return false
	}
	return viewerID != "" && art.GetOwnerId() == viewerID
}
