package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nightshiftco/nightshift/internal/objects"
)

// SessionStateDefaultMount is the canonical filesystem mount path for
// per-session state inside both API and worker pods. Workers see
// either a subPath-scoped mount (pvc backend) or a directly-scoped
// hostPath (host backend) at this path; the API mounts the full PVC
// here so DeleteSession can reach any user's session subdir.
const SessionStateDefaultMount = "/var/lib/nightshift/session-state"

// SessionObjectKeyPrefix is the bucket-key prefix for session state
// stored as Storage Objects. Layout:
//
//	<prefix>/<user_id>/<session_id>/...
//
// Identical layout on disk (under SessionStateDefaultMount) and in
// the object store, so DeleteSession's cascade code is symmetric.
const SessionObjectKeyPrefix = "sessions"

// SessionStateBackend selects how session state is persisted on the
// platform. Workers always read the resolved path from
// NS_SESSION_STATE_DIR; SessionStateBackend controls how the mount
// is established and where DeleteSession's cascade lands.
type SessionStateBackend string

const (
	// SessionStateBackendNone disables per-session state entirely:
	// no volume, no mount, no NS_SESSION_STATE_DIR env. The default.
	SessionStateBackendNone SessionStateBackend = "none"

	// SessionStateBackendPVC mounts a single shared RWX PVC into both
	// the API pod (full root) and worker pods (subPath-scoped to
	// <user_id>/<session_id>). Production-shaped.
	SessionStateBackendPVC SessionStateBackend = "pvc"

	// SessionStateBackendHost uses a hostPath on the launcher node.
	// Single-node only (kind quick-start, in-process stub launcher).
	SessionStateBackendHost SessionStateBackend = "host"

	// SessionStateBackendObject persists session state in a Storage
	// Object bucket. In chunk 13 the worker-side round-trip is not
	// yet wired (chunk 14); only the DeleteSession cascade applies.
	SessionStateBackendObject SessionStateBackend = "object"
)

// SessionStateConfig is the wiring carried on the Workers Service +
// LaunchSpec. Populated from NS_SESSION_STATE_* env in main.go.
type SessionStateConfig struct {
	Backend SessionStateBackend

	// MountPath is the path the worker container sees. Defaults to
	// SessionStateDefaultMount when empty.
	MountPath string

	// PVCName is the shared claim name (Backend == pvc).
	PVCName string

	// HostRoot is the absolute path on the launcher host that the
	// hostPath / stub launcher mounts subdirs of (Backend == host).
	HostRoot string

	// ObjectBucket is the bucket holding sessions/<u>/<s>/* objects
	// (Backend == object).
	ObjectBucket string
}

// Enabled reports whether the config is configured with a backend
// that the launcher should mount. Object backend mounts an emptyDir
// scratch volume; the worker fetches/uploads bytes via the API
// session-state endpoints (chunk 14 round-trip).
func (c SessionStateConfig) Enabled() bool {
	return c.Backend == SessionStateBackendPVC ||
		c.Backend == SessionStateBackendHost ||
		c.Backend == SessionStateBackendObject
}

// SessionSubPath is the relative path inside the shared PVC / host
// root that scopes a single session: "<user_id>/<session_id>".
// Returns an error if either id is empty or contains an unsafe
// segment (path separator, traversal, NUL). The user_id and
// session_id are server-allocated UUIDs in production; this is
// defense-in-depth.
func SessionSubPath(userID, sessionID string) (string, error) {
	if err := validateSessionIDPart("user_id", userID); err != nil {
		return "", err
	}
	if err := validateSessionIDPart("session_id", sessionID); err != nil {
		return "", err
	}
	return filepath.Join(userID, sessionID), nil
}

// SessionObjectPrefix returns the Object key prefix scoping a single
// session: "sessions/<user_id>/<session_id>/" — the trailing slash
// ensures List(prefix) doesn't accidentally match
// "sessions/<u>/<s>2/...".
func SessionObjectPrefix(userID, sessionID string) (string, error) {
	if err := validateSessionIDPart("user_id", userID); err != nil {
		return "", err
	}
	if err := validateSessionIDPart("session_id", sessionID); err != nil {
		return "", err
	}
	return SessionObjectKeyPrefix + "/" + userID + "/" + sessionID + "/", nil
}

func validateSessionIDPart(name, v string) error {
	if v == "" {
		return fmt.Errorf("session-state: %s required", name)
	}
	if strings.ContainsAny(v, "/\\\x00") {
		return fmt.Errorf("session-state: %s contains illegal character", name)
	}
	if v == "." || v == ".." || strings.Contains(v, "..") {
		return fmt.Errorf("session-state: %s rejected (traversal)", name)
	}
	return nil
}

// SessionCleaner is the interface DeleteSession calls to cascade
// session-state removal. Implementations MUST be idempotent —
// missing dirs/objects return nil.
type SessionCleaner interface {
	Clean(ctx context.Context, userID, sessionID string) error
}

// LocalFSCleaner removes the per-session subdir under Root.
//
// Used by the API pod (Root = SessionStateDefaultMount, the full PVC
// mount) AND by the stub launcher in dev (Root = the host root
// configured via NS_SESSION_STATE_HOST_ROOT).
type LocalFSCleaner struct{ Root string }

// Clean removes <Root>/<userID>/<sessionID> recursively. Missing dirs
// are not an error.
func (c *LocalFSCleaner) Clean(_ context.Context, userID, sessionID string) error {
	if c.Root == "" {
		return errors.New("session-state: LocalFSCleaner.Root not set")
	}
	sub, err := SessionSubPath(userID, sessionID)
	if err != nil {
		return err
	}
	target := filepath.Join(c.Root, sub)
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("session-state: remove %s: %w", target, err)
	}
	return nil
}

// ObjectCleaner deletes every Object under
// "sessions/<userID>/<sessionID>/" via the given ObjectStore.
type ObjectCleaner struct {
	Store  objects.ObjectStore
	Bucket string
}

// Clean enumerates the prefix and deletes each object. The
// underlying ObjectStore.Delete is idempotent (NotFound returns
// nil), so concurrent or repeated cleans are safe.
func (c *ObjectCleaner) Clean(ctx context.Context, userID, sessionID string) error {
	if c.Store == nil {
		return errors.New("session-state: ObjectCleaner.Store not set")
	}
	if c.Bucket == "" {
		return errors.New("session-state: ObjectCleaner.Bucket not set")
	}
	prefix, err := SessionObjectPrefix(userID, sessionID)
	if err != nil {
		return err
	}
	pageToken := ""
	for {
		page, next, err := c.Store.List(ctx, c.Bucket, prefix, pageToken, 500)
		if err != nil {
			return fmt.Errorf("session-state: list %s: %w", prefix, err)
		}
		for _, obj := range page {
			if err := c.Store.Delete(ctx, c.Bucket, obj.Key); err != nil {
				return fmt.Errorf("session-state: delete %s: %w", obj.Key, err)
			}
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return nil
}

// NoopCleaner is the cleaner used when no session-state backend is
// configured. Always returns nil.
type NoopCleaner struct{}

// Clean is a no-op.
func (NoopCleaner) Clean(_ context.Context, _, _ string) error { return nil }
