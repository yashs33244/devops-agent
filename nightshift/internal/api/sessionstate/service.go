package sessionstate

import (
	"log/slog"
	"net/http"

	"github.com/nightshiftco/nightshift/internal/objects"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// DefaultMaxBytes caps the per-object PUT body size. SDK JSONL
// transcripts grow into the tens of megabytes for long sessions; 64
// MiB leaves comfortable headroom without exposing the API to OOM.
const DefaultMaxBytes int64 = 64 * 1024 * 1024

// DefaultDownloadTTL bounds the lifetime of presigned URLs returned
// by the GET endpoint. Workers follow the 302 immediately, so a short
// window is fine.
const defaultDownloadTTLSeconds = 60

// ServiceOptions configures a Service. Records, Objects, Bucket, and
// Verifiers are required; misconfiguration panics in NewService.
type ServiceOptions struct {
	Records   records.RecordStore
	Objects   objects.ObjectStore
	Bucket    string
	Verifiers verifiers.Set
	Logger    *slog.Logger
	MaxBytes  int64
}

// Service is the worker-internal session-state HTTP handler.
type Service struct {
	records   records.RecordStore
	objects   objects.ObjectStore
	bucket    string
	verifiers verifiers.Set
	logger    *slog.Logger
	maxBytes  int64
}

// NewService constructs a Service. Wire-up errors panic — these are
// startup-time misconfigurations, not runtime errors.
func NewService(opts ServiceOptions) *Service {
	if opts.Records == nil {
		panic("sessionstate.NewService: Records required")
	}
	if opts.Objects == nil {
		panic("sessionstate.NewService: Objects required")
	}
	if opts.Bucket == "" {
		panic("sessionstate.NewService: Bucket required")
	}
	if opts.Verifiers == nil {
		panic("sessionstate.NewService: Verifiers required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Service{
		records:   opts.Records,
		objects:   opts.Objects,
		bucket:    opts.Bucket,
		verifiers: opts.Verifiers,
		logger:    logger,
		maxBytes:  maxBytes,
	}
}

// Handler returns an http.Handler that intercepts the session-state
// path prefix and falls through to fallback for everything else.
// Mount this in front of the grpc-gateway mux in the API HTTP server.
func (s *Service) Handler(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runID, suffix, ok := parsePath(r.URL.Path)
		if !ok {
			fallback.ServeHTTP(w, r)
			return
		}
		s.dispatch(w, r, runID, suffix)
	})
}
