package verifiers

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCVerifier validates id_tokens issued by an operator-configured
// OIDC provider. Nightshift-api consumes id_tokens; it does not issue
// them. The issuer URL and audience are set at startup via
// NS_OIDC_ISSUER_URL and NS_OIDC_AUDIENCE.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
	issuer   string
	audience string
}

// NewOIDCVerifier resolves the issuer's discovery document, builds a
// JWKS-caching verifier, and binds it to audience. A blank issuer
// disables OIDC (returns nil, nil). A non-blank issuer with a blank
// audience is a configuration error — audience is required to prevent
// cross-service token reuse.
func NewOIDCVerifier(ctx context.Context, issuer, audience string) (*OIDCVerifier, error) {
	if issuer == "" {
		return nil, nil
	}
	if audience == "" {
		return nil, fmt.Errorf("oidc: audience required when issuer is set")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery %s: %w", issuer, err)
	}
	cfg := &oidc.Config{ClientID: audience}
	return &OIDCVerifier{
		verifier: provider.Verifier(cfg),
		issuer:   issuer,
		audience: audience,
	}, nil
}

// Scheme reports SchemeUser.
func (v *OIDCVerifier) Scheme() Scheme { return SchemeUser }

// Verify validates the id_token against issuer, audience, signature,
// and expiry. On success returns a Principal with the token's `sub`
// claim as ID and the `groups` claim (or nil) for downstream RBAC.
// Any failure collapses to ErrUnauthenticated so the interceptor
// never leaks token internals.
func (v *OIDCVerifier) Verify(ctx context.Context, raw string) (*Principal, error) {
	if v == nil || v.verifier == nil {
		return nil, ErrUnauthenticated
	}
	tok, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if tok.Subject == "" {
		return nil, ErrUnauthenticated
	}
	var claims struct {
		Groups []string `json:"groups"`
	}
	// Missing or non-array groups claim is fine — leave Groups nil.
	_ = tok.Claims(&claims)
	return &Principal{
		Scheme: SchemeUser,
		ID:     tok.Subject,
		Groups: claims.Groups,
	}, nil
}

// Issuer reports the configured issuer URL (for startup logging).
func (v *OIDCVerifier) Issuer() string {
	if v == nil {
		return ""
	}
	return v.issuer
}
