// Package objects defines the pure-Go interface (ObjectStore) that the nightshift.v1.Storage gRPC service delegates
// to for blob persistence. The proto-facing adapter lives in internal/api/storage.
//
// An Object is a blob of bytes identified by (bucket, key) and served
// via presigned URLs. Two backends live here:
//
//   - filesystem: a local rooted dir with HMAC-signed URLs served by
//     an embedded HTTP handler (presign_http.go).
//   - s3: MinIO-compatible S3 via aws-sdk-go-v2 (chunk 8b).
//
// Callers interact only with the ObjectStore interface; the backend is
// chosen via config.
package objects

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors. API adapters translate these to gRPC status codes.
var (
	ErrNotFound      = errors.New("objects: not found")
	ErrAlreadyExists = errors.New("objects: already exists")
	ErrInvalidState  = errors.New("objects: invalid upload state")
)

// UploadState is the lifecycle of an Object during and after upload.
// Matches nightshiftv1.UploadState on the wire.
type UploadState int

const (
	UploadStateUnspecified UploadState = iota
	UploadStatePending
	UploadStateReady
	UploadStateFailed
)

// Object is the metadata record for a blob.
type Object struct {
	Bucket      string
	Key         string
	ContentType string
	SizeBytes   int64
	ETag        string
	Metadata    map[string]string
	State       UploadState
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// InitiateSpec is the input to ObjectStore.Initiate.
type InitiateSpec struct {
	Bucket      string
	Key         string
	ContentType string
	SizeBytes   int64
	Metadata    map[string]string
	TTL         time.Duration // upload URL validity
}

// ObjectStore is the pluggable seam. Implementations MUST be safe for
// concurrent use.
type ObjectStore interface {
	// Initiate reserves an Object in PENDING state and returns a
	// presigned URL the caller PUTs bytes to.
	Initiate(ctx context.Context, spec InitiateSpec) (obj Object, uploadURL string, uploadHeaders map[string]string, expires time.Time, err error)

	// Finalize transitions PENDING -> READY after the PUT completes.
	Finalize(ctx context.Context, bucket, key string) (Object, error)

	// PutBytes is an in-band counterpart to Initiate+PUT+Finalize for
	// callers that already have the bytes in memory and live in the
	// same process as the API. It writes body in one shot and returns
	// the resulting Object in READY state. Overwrites any existing
	// object at (bucket, key).
	PutBytes(ctx context.Context, bucket, key, contentType string, body []byte) (Object, error)

	// Stat returns metadata without bytes.
	Stat(ctx context.Context, bucket, key string) (Object, error)

	// DownloadURL returns a presigned GET URL.
	DownloadURL(ctx context.Context, bucket, key string, ttl time.Duration) (url string, expires time.Time, err error)

	// Delete removes the object. NotFound is idempotent — returns nil.
	Delete(ctx context.Context, bucket, key string) error

	// List returns objects in a bucket matching prefix.
	List(ctx context.Context, bucket, prefix, pageToken string, pageSize int32) ([]Object, string, error)
}
