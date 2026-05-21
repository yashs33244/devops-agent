// Package secrets implements the minimal Secrets backend interface
// used internally by nightshift-api. Two impls today: the file backend
// (read-only KV from a YAML file with env-var fallback, used for
// bootstrap secrets) and the OpenBao backend (read-write KV-v2 against
// the bundled OpenBao IdP, used for per-user connector tokens).
//
// This is a much narrower surface than nightshift.v1.Secrets (the gRPC
// service) — that service is a stub until chunk 12.
package secrets

import (
	"context"
	"errors"
)

// Sentinel errors. API adapters translate these to gRPC status codes.
var (
	ErrNotFound       = errors.New("secrets: not found")
	ErrNotImplemented = errors.New("secrets: not implemented")
)

// Secrets is the internal credential-fetch interface used by the API
// server. Distinct from the gRPC Secrets service.
//
// Read methods (Get, List) MUST be safe for concurrent use. Write
// methods (Put, Delete) MAY return ErrNotImplemented from a read-only
// backend (the file backend does so).
type Secrets interface {
	// Get returns the KV payload at path or ErrNotFound.
	Get(ctx context.Context, path string) (map[string]string, error)

	// Put writes the KV payload at path, replacing any prior value.
	// Returns ErrNotImplemented from read-only backends.
	Put(ctx context.Context, path string, kv map[string]string) error

	// Delete removes the KV at path. Idempotent: deleting a missing
	// path is not an error.
	// Returns ErrNotImplemented from read-only backends.
	Delete(ctx context.Context, path string) error

	// List returns the immediate children of prefix (the segment
	// immediately following prefix, no recursion). For example, with
	// keys "secret/nightshift/tokens/alice/notion" and
	// "secret/nightshift/tokens/alice/github" stored, List on prefix
	// "secret/nightshift/tokens/alice" returns ["github", "notion"].
	// Returns an empty slice (not ErrNotFound) when no children
	// exist, matching OpenBao KV-v2 semantics.
	List(ctx context.Context, prefix string) ([]string, error)
}
