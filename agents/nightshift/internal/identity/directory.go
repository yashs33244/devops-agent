// Package identity implements the user/group directory plane for
// nightshift. It is the Go-side counterpart to the IdP's directory
// API — what's used to populate the UI's share-dialog dropdown ("who
// can I share this artifact with?") and any future admin tooling
// that needs to enumerate users.
//
// Distinct from internal/verifiers (which authenticates *inbound*
// requests by validating bearer tokens — the OIDC role of the same
// IdP) and from internal/oauth (which dispenses *outbound* OAuth
// credentials nightshift holds on a user's behalf). All three are
// often backed by the same IdP host (OpenBao today; Keycloak / Okta
// in future deployments) but they are independent concerns:
//
//   - verifiers: per-request, signature-verified, no IdP query
//   - identity:  on-demand directory lookups
//   - oauth:     per-user delegated tokens to call third-party APIs
//
// Operators may pick different backends per concern.
package identity

import (
	"context"
	"errors"
)

// Identity is the projection of a directory entity that nightshift
// cares about: the entity ID (== OIDC `sub` claim for IdPs that also
// serve OIDC), the human-readable name, and any metadata (typically
// `email`).
type Identity struct {
	ID       string
	Name     string
	Metadata map[string]string
}

// ErrNotFound is the sentinel returned when a group or entity does
// not exist. API adapters translate this to gRPC codes.NotFound.
var ErrNotFound = errors.New("identity: not found")

// Directory is the read-side of an identity backend. Implementations
// MUST be safe for concurrent use.
type Directory interface {
	// ListGroupMembers returns the entity IDs registered as direct
	// members of the named group. Returns an empty slice when the
	// group exists but has no members; ErrNotFound when the group
	// itself does not exist.
	ListGroupMembers(ctx context.Context, group string) ([]string, error)

	// GetEntity returns the projected Identity for the given entity
	// ID. Returns ErrNotFound when the entity does not exist.
	GetEntity(ctx context.Context, id string) (Identity, error)
}
