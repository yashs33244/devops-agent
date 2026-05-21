package artifacts

import (
	"fmt"
)

// Storage Record collections (artifacts.md §8).
const (
	artifactsCollection   = "artifacts"
	permissionsCollection = "artifact_permissions"
)

// Record attribute keys on artifacts.
const (
	attrType      = "type"
	attrOwnerID   = "owner_id"
	attrRunID     = "run_id"
	attrSessionID = "session_id"
	attrPublic    = "public"
	attrIdemKey   = "idem_key"
)

// Record attribute keys on permission grants.
const (
	attrPermArtifactID = "artifact_id"
	attrPermUserID     = "user_id"
)

// recordContentType is the proto-wire content type for Artifact +
// ArtifactPermission record payloads. Matches the convention used by
// internal/api/workers (`application/x-protobuf`).
const recordContentType = "application/x-protobuf"

// objectKey returns the Storage Object key for an artifact's blob.
// cr0n-a parity: `artifacts/{id}/{name}` (store.py:91-115).
func objectKey(artifactID, name string) string {
	return fmt.Sprintf("artifacts/%s/%s", artifactID, name)
}

// previewKey returns the Storage Object key for an artifact's
// companion HTML preview. cr0n-a parity: `artifacts/{id}/preview.html`.
func previewKey(artifactID string) string {
	return fmt.Sprintf("artifacts/%s/preview.html", artifactID)
}

// appObjectKey returns the Storage Object key for an app artifact's
// HTML body. cr0n-a parity: cr0n stores app HTML at `apps/{id}/index.html`.
func appObjectKey(artifactID string) string {
	return fmt.Sprintf("apps/%s/index.html", artifactID)
}

// permissionKey is the Record key for an (artifact_id, user_id) grant.
// Stable so re-granting the same pair upserts in place — matches cr0n's
// (id PK INSERT-OR-REPLACE on artifact_permissions).
func permissionKey(artifactID, userID string) string {
	return artifactID + ":" + userID
}
