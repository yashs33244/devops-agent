package config

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/secrets"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// GetUserConfig materializes a user's runtime config. Worker-tier RPC:
// the calling Principal must be SchemeWorker, and the worker's run_id
// must be authorized for a run owned by req.user_id.
func (s *Service) GetUserConfig(ctx context.Context, req *nsv1.GetUserConfigRequest) (*nsv1.GetUserConfigResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}
	p := verifiers.FromContext(ctx)
	if p == nil || p.Scheme != verifiers.SchemeWorker {
		return nil, status.Error(codes.PermissionDenied, "GetUserConfig requires worker credential")
	}
	owner, err := s.runOwner(ctx, p.RunID)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "credential not authorized: %v", err)
	}
	if owner != req.GetUserId() {
		return nil, status.Errorf(codes.PermissionDenied, "credential authorized for run owned by a different user")
	}

	agents, err := s.collectAgents(ctx, req.GetUserId())
	if err != nil {
		return nil, recordErr(err)
	}
	skills, err := s.collectSkills(ctx, req.GetUserId())
	if err != nil {
		return nil, recordErr(err)
	}
	mcpServers, allowedTools, disallowedTools, err := s.collectMCPServers(ctx, req.GetUserId())
	if err != nil {
		return nil, recordErr(err)
	}

	return &nsv1.GetUserConfigResponse{
		Config: &nsv1.UserConfig{
			UserId:             req.GetUserId(),
			Agents:             agents,
			Skills:             skills,
			McpServers:         mcpServers,
			AllowedMcpTools:    allowedTools,
			DisallowedMcpTools: disallowedTools,
		},
	}, nil
}

func (s *Service) collectAgents(ctx context.Context, userID string) ([]*nsv1.AgentDefinition, error) {
	out := []*nsv1.AgentDefinition{}
	token := ""
	for {
		page, next, err := s.Records.List(ctx, records.ListQuery{
			Collection:       agentsCollection,
			AttributeFilters: map[string]string{attrUserID: userID},
			PageSize:         200,
			PageToken:        token,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range page {
			a, err := recordToAgent(r)
			if err != nil {
				return nil, err
			}
			out = append(out, &nsv1.AgentDefinition{
				Name:        a.GetName(),
				Description: a.GetDescription(),
				Prompt:      a.GetPrompt(),
				Tools:       a.GetTools(),
				Model:       a.GetModel(),
			})
		}
		if next == "" {
			return out, nil
		}
		token = next
	}
}

func (s *Service) collectSkills(ctx context.Context, userID string) ([]*nsv1.SkillDefinition, error) {
	out := []*nsv1.SkillDefinition{}
	token := ""
	for {
		page, next, err := s.Records.List(ctx, records.ListQuery{
			Collection:       skillsCollection,
			AttributeFilters: map[string]string{attrUserID: userID},
			PageSize:         200,
			PageToken:        token,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range page {
			sk, err := recordToSkill(r)
			if err != nil {
				return nil, err
			}
			out = append(out, &nsv1.SkillDefinition{
				Name:    sk.GetName(),
				Content: sk.GetContent(),
			})
		}
		if next == "" {
			return out, nil
		}
		token = next
	}
}

// collectMCPServers walks every connector and tries to resolve a
// per-user credential. Connectors with no credential are silently
// skipped (config.md §C). Returns the McpServerConfig map keyed by
// connector name plus the dedup'd union of allowed and disallowed
// tool names.
func (s *Service) collectMCPServers(ctx context.Context, userID string) (map[string]*nsv1.McpServerConfig, []string, []string, error) {
	servers := map[string]*nsv1.McpServerConfig{}
	allowed := []string{}
	allowedSeen := map[string]bool{}
	disallowed := []string{}
	disallowedSeen := map[string]bool{}

	connectors, err := s.listAllConnectors(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, c := range connectors {
		token, err := s.resolveCredential(ctx, userID, c)
		if err != nil {
			return nil, nil, nil, err
		}
		if token == "" {
			continue
		}
		servers[c.GetName()] = &nsv1.McpServerConfig{
			Name:      c.GetName(),
			Transport: transportFromURL(c.GetMcpUrl()),
			Url:       c.GetMcpUrl(),
			Headers: map[string]string{
				"Authorization": "Bearer " + token,
			},
		}
		for _, t := range c.GetMcpAllowedTools() {
			if !allowedSeen[t] {
				allowedSeen[t] = true
				allowed = append(allowed, t)
			}
		}
		for _, t := range c.GetMcpDisallowedTools() {
			if !disallowedSeen[t] {
				disallowedSeen[t] = true
				disallowed = append(disallowed, t)
			}
		}
	}
	return servers, allowed, disallowed, nil
}

// resolveCredential returns the per-user bearer token for c or "" if
// none is on file (which the caller treats as "skip this connector").
// Trust boundary: token freshness is the dispenser's responsibility
// (Native refreshes on Get with a 60s pre-expiry skew); we never
// refresh in this layer.
func (s *Service) resolveCredential(ctx context.Context, userID string, c *nsv1.Connector) (string, error) {
	if c.GetAuthType() == nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH {
		if s.OAuth == nil {
			return "", nil
		}
		tok, err := s.OAuth.GetOAuthToken(ctx, oauthCredName(userID, c.GetName()))
		if err != nil {
			if errors.Is(err, oauth.ErrNotFound) {
				return "", nil
			}
			return "", err
		}
		return tok, nil
	}
	kv, err := s.Secrets.Get(ctx, tokenPath(userID, c.GetName()))
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if v, ok := kv["access_token"]; ok && v != "" {
		return v, nil
	}
	if v, ok := kv["value"]; ok && v != "" {
		return v, nil
	}
	return "", nil
}
