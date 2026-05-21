// Package records implements the Storage Record persistence primitive
// defined by protos/nightshift/v1/storage.proto and specified by
// protos/nightshift/v1/storage.md.
//
// A Record is a structured entry in a named collection with an opaque
// payload, indexed attributes, a monotonic version, and server-managed
// timestamps. RecordStore is the pluggable seam: the reference impl is
// SQLite in sqlite.go; a Postgres impl can drop in with no call-site
// changes.
package records

import (
	"context"
	"errors"
	"time"
)

// Record is a single structured entry in a named collection.
// Matches nightshiftv1.Record on the wire; this is the native Go form
// used by the Storage service and every domain service layered on it.
type Record struct {
	Collection  string
	Key         string
	Attributes  map[string]string
	Data        []byte
	ContentType string
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListQuery is the input to RecordStore.List. Mirrors
// nightshiftv1.ListRecordsRequest without the wire noise.
type ListQuery struct {
	Collection       string
	AttributeFilters map[string]string
	PageSize         int32
	PageToken        string
	OrderBy          string
}

// Sentinel errors. API adapters translate these to gRPC status codes.
var (
	ErrNotFound        = errors.New("records: not found")
	ErrVersionConflict = errors.New("records: version mismatch")
	ErrAlreadyExists   = errors.New("records: already exists")
)

// RecordStore is the pluggable persistence seam for Records.
// Implementations MUST be safe for concurrent use.
type RecordStore interface {
	// Put writes a Record. If ifVersion is non-nil, the write fails
	// with ErrVersionConflict when the current version differs. Pass
	// &zero to require the record not already exist.
	//
	// idemKey is optional. When non-empty, a repeat call with the
	// same (collection, key, idemKey) tuple within the backend's
	// window returns the original response without re-writing.
	Put(ctx context.Context, r Record, ifVersion *int64, idemKey string) (Record, error)

	// Get returns the latest version of the record or ErrNotFound.
	Get(ctx context.Context, collection, key string) (Record, error)

	// Delete removes the record. ifVersion behaves as in Put.
	Delete(ctx context.Context, collection, key string, ifVersion *int64) error

	// List returns a page of records matching q. nextToken is empty
	// when there are no further results.
	List(ctx context.Context, q ListQuery) (page []Record, nextToken string, err error)

	// RecoverStaleRuns transitions RUNNING runs whose worker is no
	// longer live into ERROR. Called once on control-plane startup.
	// Returns the number of runs transitioned.
	//
	// v1: the implementation simply marks any RUNNING run whose
	// heartbeat is older than the staleness threshold. Future
	// versions may consult the JobLauncher for liveness.
	RecoverStaleRuns(ctx context.Context) (int, error)

	Close() error
}
