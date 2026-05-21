// Package authapi implements nightshift.v1.Auth — Nightshift's
// vendor-neutral authentication plane. Today it surfaces one auth
// method (the OAuth 2.0 dispenser for per-user MCP connector
// credentials); future iterations may add additional methods (SAML,
// LDAP, custom token exchange) as new RPC groups within the same
// proto service, each backed by an additional method-specific
// internal package.
package authapi

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// Service implements nightshift.v1.AuthServer. The OAuth dispenser
// RPCs delegate to an oauth.OAuthDispenser; future auth methods
// (SAML, LDAP, custom token exchange) will land here as additional
// fields + RPC groups.
type Service struct {
	nsv1.UnimplementedAuthServer

	// OAuth is required for the OAuth dispenser RPCs. When nil, every
	// dispenser RPC returns UNIMPLEMENTED. main.go wires this only
	// when NS_SECRETS_BACKEND=openbao.
	OAuth oauth.OAuthDispenser

	// StateSigningKey is the HMAC key used to sign OAuth state tokens
	// minted by GetOAuthAuthorizeURL and verified by ExchangeOAuthCode.
	// ≥16 bytes required when OAuth is non-nil; production reuses the
	// worker-HMAC key (domain-separated inside oauth.SignState so the
	// reuse is safe).
	StateSigningKey []byte

	// AdminGroup gates RegisterOAuthProvider on the OIDC `groups`
	// claim. Empty disables OIDC-based admin checking.
	AdminGroup string

	// AdminTokens is the set of static-token names that grant admin.
	AdminTokens map[string]bool

	// Now is a clock seam for tests; nil means time.Now.
	Now func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// RegisterOAuthProvider installs an OAuth provider entry. Admin-only.
func (s *Service) RegisterOAuthProvider(ctx context.Context, req *nsv1.RegisterOAuthProviderRequest) (*nsv1.RegisterOAuthProviderResponse, error) {
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth dispenser disabled (no OpenBao backend)")
	}
	if !s.isAdmin(verifiers.FromContext(ctx)) {
		return nil, status.Error(codes.PermissionDenied, "RegisterOAuthProvider is admin-only")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	if req.GetProviderType() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_type required")
	}
	if req.GetClientId() == "" || req.GetClientSecret() == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id + client_secret required")
	}
	cfg := oauth.OAuthServerConfig{
		Provider:     req.GetProviderType(),
		ClientID:     req.GetClientId(),
		ClientSecret: req.GetClientSecret(),
	}
	if err := s.OAuth.RegisterOAuthServer(ctx, name, cfg); err != nil {
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &nsv1.RegisterOAuthProviderResponse{}, nil
}

// GetOAuthAuthorizeURL returns the authorize URL + a CSRF state token.
// Owner-tier (caller must match req.user_id).
func (s *Service) GetOAuthAuthorizeURL(ctx context.Context, req *nsv1.GetOAuthAuthorizeURLRequest) (*nsv1.GetOAuthAuthorizeURLResponse, error) {
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth dispenser disabled")
	}
	if len(s.StateSigningKey) < 16 {
		return nil, status.Error(codes.Unimplemented, "OAuth state signing key not configured; refusing to issue replayable plaintext state")
	}
	if err := s.requireOwner(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	if req.GetProviderName() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_name required")
	}
	if req.GetRedirectUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "redirect_url required")
	}
	state, err := oauth.SignState(s.StateSigningKey, req.GetUserId(), req.GetProviderName(), s.now())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sign state: %v", err)
	}
	url, err := s.OAuth.GetAuthorizeURL(ctx, req.GetProviderName(), state, req.GetRedirectUrl(), req.GetScopes())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "authorize url: %v", err)
	}
	return &nsv1.GetOAuthAuthorizeURLResponse{AuthorizeUrl: url, State: state}, nil
}

// ExchangeOAuthCode finalizes an OAuth flow. Owner-tier. The state is
// verified against the HMAC + TTL — a stale or forged state is
// rejected before the dispenser exchange runs.
func (s *Service) ExchangeOAuthCode(ctx context.Context, req *nsv1.ExchangeOAuthCodeRequest) (*nsv1.ExchangeOAuthCodeResponse, error) {
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth dispenser disabled")
	}
	if len(s.StateSigningKey) < 16 {
		return nil, status.Error(codes.Unimplemented, "OAuth state signing key not configured")
	}
	if err := s.requireOwner(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	if req.GetProviderName() == "" || req.GetCode() == "" || req.GetState() == "" || req.GetRedirectUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_name, code, state, redirect_url required")
	}
	if err := oauth.VerifyState(s.StateSigningKey, req.GetState(), req.GetUserId(), req.GetProviderName(), s.now()); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "verify state: %v", err)
	}
	cred := req.GetUserId() + "-" + req.GetProviderName()
	if err := s.OAuth.ExchangeOAuthCode(ctx, cred, req.GetProviderName(), req.GetCode(), req.GetRedirectUrl(), req.GetState()); err != nil {
		return nil, status.Errorf(codes.Internal, "exchange: %v", err)
	}
	return &nsv1.ExchangeOAuthCodeResponse{}, nil
}

// GetOAuthToken returns a fresh access_token for a user. Service-tier
// (typically the Config Dispenser is the only consumer; off-process
// callers must present a service token whose principal is admin).
func (s *Service) GetOAuthToken(ctx context.Context, req *nsv1.GetOAuthTokenRequest) (*nsv1.GetOAuthTokenResponse, error) {
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth dispenser disabled")
	}
	if !s.isAdmin(verifiers.FromContext(ctx)) {
		return nil, status.Error(codes.PermissionDenied, "GetOAuthToken is service/admin-only")
	}
	if req.GetProviderName() == "" || req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_name + user_id required")
	}
	cred := req.GetUserId() + "-" + req.GetProviderName()
	tok, err := s.OAuth.GetOAuthToken(ctx, cred)
	if err != nil {
		if errors.Is(err, oauth.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "no credential for user/provider")
		}
		return nil, status.Errorf(codes.Internal, "get token: %v", err)
	}
	return &nsv1.GetOAuthTokenResponse{
		Token: &nsv1.OAuthToken{AccessToken: tok, TokenType: "Bearer"},
	}, nil
}

// DeleteOAuthCredential drops a user's stored grant. Owner-tier.
func (s *Service) DeleteOAuthCredential(ctx context.Context, req *nsv1.DeleteOAuthCredentialRequest) (*nsv1.DeleteOAuthCredentialResponse, error) {
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth dispenser disabled")
	}
	if err := s.requireOwner(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	if req.GetProviderName() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_name required")
	}
	cred := req.GetUserId() + "-" + req.GetProviderName()
	if err := s.OAuth.DeleteOAuthCredential(ctx, cred); err != nil {
		return nil, status.Errorf(codes.Internal, "delete: %v", err)
	}
	return &nsv1.DeleteOAuthCredentialResponse{}, nil
}

func (s *Service) isAdmin(p *verifiers.Principal) bool {
	if p == nil {
		return false
	}
	if s.AdminGroup != "" && slices.Contains(p.Groups, s.AdminGroup) {
		return true
	}
	return s.AdminTokens != nil && s.AdminTokens[p.ID]
}

func (s *Service) requireOwner(ctx context.Context, userID string) error {
	p := verifiers.FromContext(ctx)
	if p == nil {
		return status.Error(codes.Unauthenticated, "missing principal")
	}
	if s.isAdmin(p) {
		return nil
	}
	if p.ID != userID {
		return status.Error(codes.PermissionDenied, "caller does not own this resource")
	}
	return nil
}
