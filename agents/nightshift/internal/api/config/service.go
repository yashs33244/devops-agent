// Package config implements nightshift.v1.Config — agents/skills/connectors
// CRUD, the per-user resolver workers call at run start, and startup
// catalog reconciliation. Backed by internal/records (RecordStore) for
// durable definition state and internal/secrets for per-user connector
// credentials.
package config

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/secrets"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// ServiceOptions configures a Config Service. Records + Secrets +
// CallbackResolver are required; everything else has defaults.
type ServiceOptions struct {
	Records records.RecordStore
	Secrets secrets.Secrets

	// OAuth is the per-user OAuth-token-management surface used for
	// CONNECTOR_AUTH_TYPE_OAUTH connectors. Optional: nil disables
	// the OAuth flow (StartConnectorOAuthFlow + CompleteConnectorOAuthFlow
	// return Unimplemented; the resolver skips OAuth connectors).
	OAuth oauth.OAuthDispenser

	// StateSigningKey is the HMAC key used to sign OAuth state tokens
	// for the connector flow. Required when OAuth is non-nil.
	// Reference impl reuses the worker-HMAC key.
	StateSigningKey []byte

	// RunOwnerLookup returns the user_id of the run identified by
	// runID. Used by GetUserConfig to verify a worker credential
	// authorized for run R can only resolve config for R's owning user.
	// Required.
	RunOwnerLookup func(ctx context.Context, runID string) (string, error)

	// AdminGroup is the OIDC `groups` claim value that promotes a
	// SchemeUser principal to admin. Empty disables OIDC-based admin.
	AdminGroup string

	// AdminTokens is the set of static-token names whose principal is
	// treated as admin. Use a small operator-controlled set
	// (e.g. {"cli-admin"}).
	AdminTokens map[string]bool

	Logger *slog.Logger

	// Optional override for UUID generation. Tests supply a deterministic
	// generator.
	NewID func() string

	// Optional clock override (tests).
	Now func() time.Time
}

// Service is the nightshift.v1.ConfigServer implementation.
type Service struct {
	nsv1.UnimplementedConfigServer

	Records records.RecordStore
	Secrets secrets.Secrets
	OAuth   oauth.OAuthDispenser

	stateKey    []byte
	runOwner    func(ctx context.Context, runID string) (string, error)
	adminGroup  string
	adminTokens map[string]bool

	Logger *slog.Logger

	newID func() string
	now   func() time.Time
}

// NewService constructs a Service. Panics if required options are
// missing — wire-up errors should fail the binary at start, not silently.
func NewService(opts ServiceOptions) *Service {
	if opts.Records == nil {
		panic("config.NewService: Records required")
	}
	if opts.Secrets == nil {
		panic("config.NewService: Secrets required")
	}
	if opts.RunOwnerLookup == nil {
		panic("config.NewService: RunOwnerLookup required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	newID := opts.NewID
	if newID == nil {
		newID = func() string { return uuid.NewString() }
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if opts.OAuth != nil && len(opts.StateSigningKey) < 16 {
		panic("config.NewService: StateSigningKey >= 16 bytes required when OAuth is enabled")
	}
	return &Service{
		Records:     opts.Records,
		Secrets:     opts.Secrets,
		OAuth:       opts.OAuth,
		stateKey:    opts.StateSigningKey,
		runOwner:    opts.RunOwnerLookup,
		adminGroup:  opts.AdminGroup,
		adminTokens: opts.AdminTokens,
		Logger:      logger,
		newID:       newID,
		now:         now,
	}
}

// isAdmin reports whether p is an admin: matches the OIDC group OR
// matches a configured static-token admin name.
func (s *Service) isAdmin(p *verifiers.Principal) bool {
	if p == nil {
		return false
	}
	if s.adminGroup != "" && slices.Contains(p.Groups, s.adminGroup) {
		return true
	}
	if s.adminTokens != nil && s.adminTokens[p.ID] {
		return true
	}
	return false
}

// requireOwner ensures the principal is the resource owner OR an admin.
// Workers may be allowed elsewhere; this helper is for outer-surface
// (user/service) RPCs only.
func (s *Service) requireOwner(p *verifiers.Principal, ownerUserID string) error {
	if p == nil {
		return errUnauthenticated
	}
	if s.isAdmin(p) {
		return nil
	}
	if p.ID != ownerUserID {
		return errPermissionDenied
	}
	return nil
}

// requireAdmin ensures the principal is an admin.
func (s *Service) requireAdmin(p *verifiers.Principal) error {
	if p == nil {
		return errUnauthenticated
	}
	if !s.isAdmin(p) {
		return errPermissionDenied
	}
	return nil
}

// validName enforces a small character set on agent/skill/connector
// names: lowercase alnum, dash, underscore. Anything else risks
// breaking filesystem hydration (skills) or URL paths (connectors).
func validName(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// trimAndValidateName is used by Create/Update handlers.
func trimAndValidateName(name string) (string, bool) {
	n := strings.TrimSpace(name)
	return n, validName(n)
}

// transportFromURL derives the MCP transport from the connector URL.
// Per config.md §7: a "/sse" suffix is SSE; everything else is HTTP.
func transportFromURL(u string) nsv1.McpTransport {
	if strings.HasSuffix(strings.TrimRight(u, "/"), "/sse") {
		return nsv1.McpTransport_MCP_TRANSPORT_SSE
	}
	return nsv1.McpTransport_MCP_TRANSPORT_HTTP
}
