// Package oauth implements the per-user OAuth 2.0 credential plane
// for Nightshift connectors. It is the Go-side counterpart to
// nightshift.v1.Auth's OAuth dispenser RPCs (verifiers.proto).
//
// Architecture: API services consume the OAuthDispenser interface and
// are unaware of where tokens are stored. The canonical impl is
// Native — an in-process OAuth 2.0 + PKCE/S256 client whose only
// storage substrate is the secrets.Secrets interface. Pluggability
// lives at the secrets layer (OpenBao today, future K8s/Vault), and
// any conforming impl of secrets.Secrets transparently provides
// storage for OAuth without changes here.
//
// Two compliance suites pin the contracts independently:
//   - internal/oauth/compliance_test.go drives OAuthDispenser.
//   - internal/secrets/compliance_test.go drives Secrets.
//
// New OAuthDispenser impls (should anyone ever want one — Native
// covers the catalog today) MUST pass the compliance suite. Same for
// new secrets backends.
//
// Distinct from internal/auth (the inbound bearer-token verification
// plane: OIDCVerifier, StaticVerifier, WorkerVerifier). internal/auth
// authenticates requests INTO nightshift-api; internal/oauth manages
// OUTBOUND OAuth credentials nightshift-api holds on a user's behalf.
package oauth

import (
	"context"
	"errors"
)

// ErrNotFound is the sentinel returned when a credential or provider
// has never been provisioned. API adapters translate this to gRPC
// codes.NotFound.
var ErrNotFound = errors.New("oauth: not found")

// OAuthDispenser is the per-user OAuth-token-management surface used
// by the Config service to wire `CONNECTOR_AUTH_TYPE_OAUTH` connectors
// end-to-end. Backends without OAuth capability are simply absent
// (the Config service holds a nil dispenser, and OAuth flows return
// Unimplemented).
//
// Per the Nightshift contract (config.md §5, verifiers.md §6), token
// refresh is the dispenser's responsibility: `GetOAuthToken` MUST
// return a fresh access_token, transparently refreshing when needed.
// No Go-level refresh logic exists in nightshift-api.
type OAuthDispenser interface {
	// RegisterOAuthServer installs/updates an OAuth provider entry
	// under name. Idempotent: re-registering with the same config is
	// a no-op. Implementations may persist client_secret in encrypted
	// at-rest storage; conforming impls MUST NOT echo it back.
	RegisterOAuthServer(ctx context.Context, name string, cfg OAuthServerConfig) error

	// GetAuthorizeURL returns the URL the user's browser must visit
	// to start the OAuth code-flow, including the operator-supplied
	// CSRF state token verbatim. scopes is the list of OAuth scopes
	// to request (provider-specific format).
	GetAuthorizeURL(ctx context.Context, server, state, redirectURL string, scopes []string) (string, error)

	// ExchangeOAuthCode swaps an authorization code for tokens and
	// persists them under credName (the implementation's flat-namespace
	// key, e.g. "<user_id>-<connector_name>"). On success a subsequent
	// GetOAuthToken(credName) returns the access_token.
	//
	// state is the same opaque value the caller passed to GetAuthorizeURL.
	// PKCE-capable backends use it to look up the code_verifier they
	// stashed at authorize time; PKCE-agnostic backends MAY ignore it.
	// Callers MUST always supply it so backend selection stays
	// transparent at the call site.
	ExchangeOAuthCode(ctx context.Context, credName, server, code, redirectURL, state string) error

	// GetOAuthToken returns a fresh access_token for credName.
	// Implementations refresh transparently when the cached
	// access_token is near or past expiry. Returns ErrNotFound when
	// credName has never been provisioned (the resolver treats this as
	// "skip the connector").
	GetOAuthToken(ctx context.Context, credName string) (string, error)

	// DeleteOAuthCredential removes the local credential for credName.
	// Does NOT call the upstream provider's revocation endpoint —
	// users wanting to revoke at the provider must do so via the
	// provider's UI. Idempotent: deleting a missing credential is not
	// an error.
	DeleteOAuthCredential(ctx context.Context, credName string) error
}

// OAuthServerConfig is the operator-supplied configuration for one
// OAuth provider entry. The Native dispenser persists this verbatim
// at `secret/nightshift/oauth-servers/<name>` via the secrets backend.
type OAuthServerConfig struct {
	// Provider is one of Native's built-in names ("github", "google",
	// "slack", "microsoft", "hubspot", "dropbox", "notion", "linear",
	// "asana", "monday") or "custom" to declare endpoints inline via
	// ProviderOptions{auth_code_url, token_url}.
	Provider string

	// ClientID + ClientSecret are the per-provider OAuth app
	// credentials. The dispenser persists them; callers MUST treat
	// them as write-only.
	ClientID     string
	ClientSecret string

	// ProviderOptions carries provider-specific config. For
	// Provider="custom", set "auth_code_url" + "token_url" here.
	ProviderOptions map[string]string

	// AuthURLParams are extra query params appended to the
	// authorization URL on every flow start (e.g. Dropbox's
	// `token_access_type=offline`).
	AuthURLParams map[string]string
}
