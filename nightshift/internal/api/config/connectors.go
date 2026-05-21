package config

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/secrets"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// CreateConnector registers a new MCP connector. Admin-only.
func (s *Service) CreateConnector(ctx context.Context, req *nsv1.CreateConnectorRequest) (*nsv1.CreateConnectorResponse, error) {
	if err := s.requireAdmin(verifiers.FromContext(ctx)); err != nil {
		return nil, err
	}
	name, ok := trimAndValidateName(req.GetName())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "name required (lowercase alnum + dash/underscore, ≤64)")
	}
	if req.GetMcpUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "mcp_url required")
	}
	switch req.GetAuthType() {
	case nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH,
		nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN:
		// ok
	default:
		return nil, status.Error(codes.InvalidArgument, "auth_type must be OAUTH or STATIC_TOKEN")
	}

	if existing, err := s.findConnectorByName(ctx, name); err != nil {
		return nil, recordErr(err)
	} else if existing != nil {
		return nil, status.Errorf(codes.AlreadyExists, "connector %q already exists", name)
	}

	now := s.now()
	c := &nsv1.Connector{
		Id:                 s.newID(),
		Name:               name,
		Description:        req.GetDescription(),
		AuthType:           req.GetAuthType(),
		McpUrl:             req.GetMcpUrl(),
		McpAllowedTools:    req.GetMcpAllowedTools(),
		McpDisallowedTools: req.GetMcpDisallowedTools(),
		OauthScopes:        req.GetOauthScopes(),
		AuthProviderRef:    req.GetAuthProviderRef(),
		CreatedAt:          timestamppb.New(now),
		UpdatedAt:          timestamppb.New(now),
	}
	rec, err := connectorToRecord(c)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	zero := int64(0)
	if _, err := s.Records.Put(ctx, rec, &zero, req.GetIdempotencyKey()); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.CreateConnectorResponse{Connector: c}, nil
}

// DeleteConnector removes a connector and cascades per-user credentials.
// Admin-only.
func (s *Service) DeleteConnector(ctx context.Context, req *nsv1.DeleteConnectorRequest) (*nsv1.DeleteConnectorResponse, error) {
	if err := s.requireAdmin(verifiers.FromContext(ctx)); err != nil {
		return nil, err
	}
	if req.GetConnectorId() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_id required")
	}
	rec, err := s.Records.Get(ctx, connectorsCollection, req.GetConnectorId())
	if err != nil {
		return nil, recordErr(err)
	}
	c, err := recordToConnector(rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", err.Error())
	}
	if err := s.cascadeDeleteCreds(ctx, c.GetName()); err != nil {
		return nil, status.Errorf(codes.Internal, "cascade delete creds: %v", err)
	}
	v := rec.Version
	if err := s.Records.Delete(ctx, connectorsCollection, req.GetConnectorId(), &v); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.DeleteConnectorResponse{}, nil
}

// ListConnectors returns the catalog with per-caller `connected` state
// when req.user_id is set.
func (s *Service) ListConnectors(ctx context.Context, req *nsv1.ListConnectorsRequest) (*nsv1.ListConnectorsResponse, error) {
	page, next, err := s.Records.List(ctx, records.ListQuery{
		Collection: connectorsCollection,
		PageSize:   req.GetPageSize(),
		PageToken:  req.GetPageToken(),
	})
	if err != nil {
		return nil, recordErr(err)
	}
	entries := make([]*nsv1.ConnectorCatalogEntry, 0, len(page))
	for _, r := range page {
		c, err := recordToConnector(r)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%s", err.Error())
		}
		entry := &nsv1.ConnectorCatalogEntry{Connector: c}
		// `configured`: static-token connectors don't need admin setup
		// (always true). OAuth connectors are configured iff admin
		// OAuth credentials exist in Secrets KV at the well-known path
		// (operator pre-seeded; catalog reconciliation registered it).
		switch c.GetAuthType() {
		case nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN:
			entry.Configured = true
		case nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH:
			if kv, err := s.Secrets.Get(ctx, connectorAdminPath(c.GetName())); err == nil {
				entry.Configured = kv["client_id"] != "" && kv["client_secret"] != ""
			} else if !errors.Is(err, secrets.ErrNotFound) {
				return nil, recordErr(err)
			}
		}
		// `connected`: per-user credential exists. Static-token via KV;
		// OAuth via the dispenser.
		if req.GetUserId() != "" {
			switch c.GetAuthType() {
			case nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN:
				if _, err := s.Secrets.Get(ctx, tokenPath(req.GetUserId(), c.GetName())); err == nil {
					entry.Connected = true
				} else if !errors.Is(err, secrets.ErrNotFound) {
					return nil, recordErr(err)
				}
			case nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH:
				if s.OAuth != nil {
					if _, err := s.OAuth.GetOAuthToken(ctx, oauthCredName(req.GetUserId(), c.GetName())); err == nil {
						entry.Connected = true
					} else if !errors.Is(err, oauth.ErrNotFound) {
						return nil, recordErr(err)
					}
				}
			}
		}
		entries = append(entries, entry)
	}
	return &nsv1.ListConnectorsResponse{Entries: entries, NextPageToken: next}, nil
}

// StartConnectorOAuthFlow returns the URL to redirect the user's
// browser to + an opaque state token the caller must echo back to
// CompleteConnectorOAuthFlow. Owner-tier (caller's principal must
// match req.user_id).
func (s *Service) StartConnectorOAuthFlow(ctx context.Context, req *nsv1.StartConnectorOAuthFlowRequest) (*nsv1.StartConnectorOAuthFlowResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetConnectorName() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_name required")
	}
	if req.GetRedirectUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "redirect_url required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth disabled (no OAuthDispenser configured); set NS_SECRETS_BACKEND=openbao")
	}
	c, err := s.findConnectorByName(ctx, req.GetConnectorName())
	if err != nil {
		return nil, recordErr(err)
	}
	if c == nil {
		return nil, status.Errorf(codes.NotFound, "connector %q not found", req.GetConnectorName())
	}
	if c.GetAuthType() != nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH {
		return nil, status.Errorf(codes.FailedPrecondition, "connector %q is not an OAuth connector", req.GetConnectorName())
	}
	state, err := s.signState(req.GetUserId(), req.GetConnectorName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sign state: %v", err)
	}
	authURL, err := s.OAuth.GetAuthorizeURL(ctx, req.GetConnectorName(), state, req.GetRedirectUrl(), parseScopes(c.GetOauthScopes()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "authorize url: %v", err)
	}
	return &nsv1.StartConnectorOAuthFlowResponse{AuthorizeUrl: authURL, State: state}, nil
}

// CompleteConnectorOAuthFlow exchanges the OAuth code for tokens via
// the dispenser. Owner-tier; verifies the state token matches the
// (user, connector) binding from StartConnectorOAuthFlow + the 10-min
// TTL. On success, the resolver picks up the new credential.
func (s *Service) CompleteConnectorOAuthFlow(ctx context.Context, req *nsv1.CompleteConnectorOAuthFlowRequest) (*nsv1.CompleteConnectorOAuthFlowResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetConnectorName() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_name required")
	}
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code required")
	}
	if req.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state required")
	}
	if req.GetRedirectUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "redirect_url required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}
	if s.OAuth == nil {
		return nil, status.Error(codes.Unimplemented, "OAuth disabled (no OAuthDispenser configured)")
	}
	if err := s.verifyState(req.GetState(), req.GetUserId(), req.GetConnectorName()); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "invalid state: %v", err)
	}
	c, err := s.findConnectorByName(ctx, req.GetConnectorName())
	if err != nil {
		return nil, recordErr(err)
	}
	if c == nil {
		return nil, status.Errorf(codes.NotFound, "connector %q not found", req.GetConnectorName())
	}
	if c.GetAuthType() != nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH {
		return nil, status.Errorf(codes.FailedPrecondition, "connector %q is not an OAuth connector", req.GetConnectorName())
	}
	cred := oauthCredName(req.GetUserId(), req.GetConnectorName())
	if err := s.OAuth.ExchangeOAuthCode(ctx, cred, req.GetConnectorName(), req.GetCode(), req.GetRedirectUrl(), req.GetState()); err != nil {
		return nil, status.Errorf(codes.Internal, "exchange code: %v", err)
	}
	return &nsv1.CompleteConnectorOAuthFlowResponse{}, nil
}

// parseScopes splits a comma- or space-separated scope string into a
// slice. Empty input → nil.
func parseScopes(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetConnectorStaticToken stores a per-user bearer token in Secrets.
func (s *Service) SetConnectorStaticToken(ctx context.Context, req *nsv1.SetConnectorStaticTokenRequest) (*nsv1.SetConnectorStaticTokenResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetConnectorName() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_name required")
	}
	if req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "token required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}
	c, err := s.findConnectorByName(ctx, req.GetConnectorName())
	if err != nil {
		return nil, recordErr(err)
	}
	if c == nil {
		return nil, status.Errorf(codes.NotFound, "connector %q not found", req.GetConnectorName())
	}
	if c.GetAuthType() != nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN {
		return nil, status.Errorf(codes.FailedPrecondition, "connector %q is not a static-token connector", req.GetConnectorName())
	}
	if err := s.Secrets.Put(ctx, tokenPath(req.GetUserId(), req.GetConnectorName()), map[string]string{
		"access_token": req.GetToken(),
	}); err != nil {
		return nil, recordErr(err)
	}
	return &nsv1.SetConnectorStaticTokenResponse{}, nil
}

// DisconnectConnector deletes the per-user credential for both auth types.
// Static-token credentials live in Secrets KV; OAuth credentials live
// in the OAuthDispenser. We probe the connector's auth_type and call
// the right backend, falling back to a best-effort double-delete when
// the connector record can't be loaded (e.g. concurrent delete).
func (s *Service) DisconnectConnector(ctx context.Context, req *nsv1.DisconnectConnectorRequest) (*nsv1.DisconnectConnectorResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	if req.GetConnectorName() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_name required")
	}
	if err := s.requireOwner(verifiers.FromContext(ctx), req.GetUserId()); err != nil {
		return nil, err
	}
	c, _ := s.findConnectorByName(ctx, req.GetConnectorName())
	authType := nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_UNSPECIFIED
	if c != nil {
		authType = c.GetAuthType()
	}
	switch authType {
	case nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH:
		if s.OAuth == nil {
			return nil, status.Error(codes.Unimplemented, "OAuth disabled (no OAuthDispenser configured)")
		}
		if err := s.OAuth.DeleteOAuthCredential(ctx, oauthCredName(req.GetUserId(), req.GetConnectorName())); err != nil {
			return nil, recordErr(err)
		}
	default:
		// STATIC_TOKEN or unknown — wipe the KV path. If the connector
		// type was unknown (no record), also try OAuth as a best-effort
		// cleanup so a stale credential can't outlive a recreated
		// connector under a different auth type.
		if err := s.Secrets.Delete(ctx, tokenPath(req.GetUserId(), req.GetConnectorName())); err != nil {
			return nil, recordErr(err)
		}
		if c == nil && s.OAuth != nil {
			_ = s.OAuth.DeleteOAuthCredential(ctx, oauthCredName(req.GetUserId(), req.GetConnectorName()))
		}
	}
	return &nsv1.DisconnectConnectorResponse{}, nil
}

// findConnectorByName scans the global connectors collection.
func (s *Service) findConnectorByName(ctx context.Context, name string) (*nsv1.Connector, error) {
	page, _, err := s.Records.List(ctx, records.ListQuery{
		Collection:       connectorsCollection,
		AttributeFilters: map[string]string{attrName: name},
		PageSize:         1,
	})
	if err != nil {
		return nil, err
	}
	if len(page) == 0 {
		return nil, nil
	}
	return recordToConnector(page[0])
}

// cascadeDeleteCreds removes every user's stored credential for the
// named connector. Tolerates Secrets backends that return
// ErrNotImplemented for List or Delete (file backend) — in that case
// the cascade is a no-op and operators must clean up manually.
func (s *Service) cascadeDeleteCreds(ctx context.Context, connectorName string) error {
	users, err := s.Secrets.List(ctx, tokenPathPrefix)
	if err != nil {
		if errors.Is(err, secrets.ErrNotImplemented) {
			return nil
		}
		return err
	}
	for _, u := range users {
		if err := s.Secrets.Delete(ctx, tokenPath(u, connectorName)); err != nil {
			if errors.Is(err, secrets.ErrNotImplemented) || errors.Is(err, secrets.ErrNotFound) {
				continue
			}
			return err
		}
	}
	return nil
}
