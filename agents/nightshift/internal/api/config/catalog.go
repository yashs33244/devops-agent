package config

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/secrets"
)

//go:embed catalog/default.yaml
var defaultCatalogYAML []byte

// catalogFile is the on-disk YAML format the API parses at startup.
//
//	connectors:
//	  - name: notion
//	    description: Notion MCP server
//	    mcp_url: https://mcp.notion.com/sse
//	    auth_type: static_token
//	    mcp_allowed_tools: ["pages.*", "databases.*"]
//	  - name: github
//	    description: GitHub MCP server
//	    mcp_url: https://api.githubcopilot.com/mcp
//	    auth_type: oauth
//	    oauth_scopes: "repo,read:user"
//	    mcp_allowed_tools: ["repos.*"]
//
// Reconciliation refreshes catalog-managed fields on every run so that
// config edits propagate via deploy. Identity (id, name, created_at)
// is preserved; description, mcp_url, auth_type, mcp_allowed_tools,
// mcp_disallowed_tools, oauth_scopes and auth_provider_ref are all
// driven by the catalog. Catalog deletions do not cascade — operators
// remove connectors via DeleteConnector.
type catalogFile struct {
	Connectors []catalogEntry `yaml:"connectors"`
}

type catalogEntry struct {
	Name               string   `yaml:"name"`
	Description        string   `yaml:"description"`
	McpURL             string   `yaml:"mcp_url"`
	AuthType           string   `yaml:"auth_type"` // "oauth" | "static_token"
	OauthScopes        string   `yaml:"oauth_scopes"`
	McpAllowedTools    []string `yaml:"mcp_allowed_tools"`
	McpDisallowedTools []string `yaml:"mcp_disallowed_tools"`
	AuthProviderRef    string   `yaml:"auth_provider_ref"`

	// OAuth provider config; only meaningful when auth_type=oauth.
	// `provider` is one of Native's built-in names ("github", "google",
	// "slack", "microsoft", "hubspot", "dropbox", "notion", "linear",
	// "asana", "monday") or "custom" for inline endpoints.
	// `provider_options` carries auth_code_url + token_url for custom.
	// `auth_url_params` is provider-specific extras (e.g. Dropbox's
	// `token_access_type=offline`). Admin client_id + client_secret
	// are NOT in the catalog — operators pre-seed those at
	// `secret/nightshift/connectors/<name>` in the secrets backend.
	OAuth *catalogOAuthEntry `yaml:"oauth,omitempty"`
}

type catalogOAuthEntry struct {
	Provider        string            `yaml:"provider"`
	ProviderOptions map[string]string `yaml:"provider_options,omitempty"`
	AuthURLParams   map[string]string `yaml:"auth_url_params,omitempty"`
}

// LoadCatalog reads the catalog YAML from path. Empty path uses the
// embedded default (no entries).
func LoadCatalog(path string) (*catalogFile, error) {
	raw := defaultCatalogYAML
	if path != "" {
		var err error
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("config: read catalog %s: %w", path, err)
		}
	}
	var cf catalogFile
	if err := yaml.Unmarshal(raw, &cf); err != nil {
		return nil, fmt.Errorf("config: parse catalog: %w", err)
	}
	return &cf, nil
}

// ReconcileCatalog brings the connector store in line with cf. Entries
// that don't yet have a record are created; entries whose record's
// catalog-managed fields drift from the YAML are updated in place
// (description, mcp_url, auth_type, mcp_allowed_tools,
// mcp_disallowed_tools, oauth_scopes, auth_provider_ref). Identity
// (id, name, created_at) is preserved across updates. Returns the
// count of newly-created and updated records.
func (s *Service) ReconcileCatalog(ctx context.Context, cf *catalogFile) (created, updated int, err error) {
	if cf == nil {
		return 0, 0, nil
	}

	// Walk existing connector records, keeping name → (connector, version).
	type existingEntry struct {
		connector *nsv1.Connector
		version   int64
	}
	existing := map[string]existingEntry{}
	token := ""
	for {
		page, next, lerr := s.Records.List(ctx, records.ListQuery{
			Collection: connectorsCollection,
			PageSize:   200,
			PageToken:  token,
		})
		if lerr != nil {
			return 0, 0, fmt.Errorf("list connectors: %w", lerr)
		}
		for _, r := range page {
			c, perr := recordToConnector(r)
			if perr != nil {
				return 0, 0, perr
			}
			existing[c.GetName()] = existingEntry{connector: c, version: r.Version}
		}
		if next == "" {
			break
		}
		token = next
	}

	for _, e := range cf.Connectors {
		name := strings.TrimSpace(e.Name)
		if !validName(name) {
			return created, updated, fmt.Errorf("config: catalog entry %q: invalid name", e.Name)
		}
		auth, perr := parseAuthType(e.AuthType)
		if perr != nil {
			return created, updated, fmt.Errorf("config: catalog entry %q: %w", name, perr)
		}
		if e.McpURL == "" {
			return created, updated, fmt.Errorf("config: catalog entry %q: mcp_url required", name)
		}

		// OAuth server registration runs every reconcile (idempotent
		// PUT), even for already-known connectors — admin credentials
		// in KV may have arrived after the first reconcile, and the
		// dispenser's server registration is required for ListConnectors
		// to report `configured=true`.
		if auth == nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH && s.OAuth != nil && e.OAuth != nil {
			if rerr := s.registerOAuthServer(ctx, name, e); rerr != nil {
				// Don't fail the whole reconcile — log + continue.
				s.Logger.Warn("oauth server registration failed",
					"connector", name, "err", rerr.Error())
			}
		}

		if cur, ok := existing[name]; ok {
			// Drift-back: refresh catalog-managed fields if the
			// catalog has diverged from the record. Identity
			// (Id, Name, CreatedAt) is preserved.
			if catalogFieldsMatch(cur.connector, e, auth) {
				continue
			}
			desired := &nsv1.Connector{
				Id:                 cur.connector.GetId(),
				Name:               name,
				Description:        e.Description,
				AuthType:           auth,
				McpUrl:             e.McpURL,
				McpAllowedTools:    e.McpAllowedTools,
				McpDisallowedTools: e.McpDisallowedTools,
				OauthScopes:        e.OauthScopes,
				AuthProviderRef:    e.AuthProviderRef,
				CreatedAt:          cur.connector.GetCreatedAt(),
				UpdatedAt:          timestamppb.New(s.now()),
			}
			rec, merr := connectorToRecord(desired)
			if merr != nil {
				s.Logger.Warn("catalog drift-back: marshal failed",
					"connector", name, "err", merr.Error())
				continue
			}
			v := cur.version
			if _, perr := s.Records.Put(ctx, rec, &v, ""); perr != nil {
				if errors.Is(perr, records.ErrVersionConflict) {
					s.Logger.Warn("catalog drift-back: version conflict, skipping",
						"connector", name)
					continue
				}
				s.Logger.Warn("catalog drift-back: put failed",
					"connector", name, "err", perr.Error())
				continue
			}
			updated++
			continue
		}

		// Create new
		now := s.now()
		c := &nsv1.Connector{
			Id:                 s.newID(),
			Name:               name,
			Description:        e.Description,
			AuthType:           auth,
			McpUrl:             e.McpURL,
			McpAllowedTools:    e.McpAllowedTools,
			McpDisallowedTools: e.McpDisallowedTools,
			OauthScopes:        e.OauthScopes,
			AuthProviderRef:    e.AuthProviderRef,
			CreatedAt:          timestamppb.New(now),
			UpdatedAt:          timestamppb.New(now),
		}
		rec, merr := connectorToRecord(c)
		if merr != nil {
			return created, updated, merr
		}
		zero := int64(0)
		if _, perr := s.Records.Put(ctx, rec, &zero, ""); perr != nil {
			// Race: someone else created it between our list and put.
			if errors.Is(perr, records.ErrAlreadyExists) {
				continue
			}
			return created, updated, fmt.Errorf("put connector %q: %w", name, perr)
		}
		created++
	}
	return created, updated, nil
}

// catalogFieldsMatch returns true iff every catalog-managed field on
// `cur` already equals the catalog entry. Used by ReconcileCatalog to
// avoid a no-op write (and the resulting UpdatedAt churn).
func catalogFieldsMatch(cur *nsv1.Connector, e catalogEntry, auth nsv1.ConnectorAuthType) bool {
	return cur.GetDescription() == e.Description &&
		cur.GetMcpUrl() == e.McpURL &&
		cur.GetAuthType() == auth &&
		cur.GetOauthScopes() == e.OauthScopes &&
		cur.GetAuthProviderRef() == e.AuthProviderRef &&
		slices.Equal(cur.GetMcpAllowedTools(), e.McpAllowedTools) &&
		slices.Equal(cur.GetMcpDisallowedTools(), e.McpDisallowedTools)
}

// registerOAuthServer pulls the admin credential from
// `secret/nightshift/connectors/<name>` and registers an OAuth server
// with the dispenser. Missing creds are not an error — the connector
// simply stays configured=false until an operator seeds them.
func (s *Service) registerOAuthServer(ctx context.Context, name string, e catalogEntry) error {
	kv, err := s.Secrets.Get(ctx, connectorAdminPath(name))
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			s.Logger.Info("oauth admin credentials not seeded — connector stays unconfigured",
				"connector", name, "path", connectorAdminPath(name))
			return nil
		}
		return fmt.Errorf("read admin creds: %w", err)
	}
	clientID := kv["client_id"]
	clientSecret := kv["client_secret"]
	if clientID == "" || clientSecret == "" {
		s.Logger.Warn("oauth admin credentials present but client_id or client_secret empty",
			"connector", name, "path", connectorAdminPath(name))
		return nil
	}
	cfg := oauth.OAuthServerConfig{
		Provider:        e.OAuth.Provider,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		ProviderOptions: e.OAuth.ProviderOptions,
		AuthURLParams:   e.OAuth.AuthURLParams,
	}
	return s.OAuth.RegisterOAuthServer(ctx, name, cfg)
}

// listAllConnectors paginates through every connector record. Catalog
// reconciliation expects O(catalog size) connectors so this is fine
// without pagination on the public API.
func (s *Service) listAllConnectors(ctx context.Context) ([]*nsv1.Connector, error) {
	out := []*nsv1.Connector{}
	token := ""
	for {
		page, next, err := s.Records.List(ctx, records.ListQuery{
			Collection: connectorsCollection,
			PageSize:   200,
			PageToken:  token,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range page {
			c, err := recordToConnector(r)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		if next == "" {
			return out, nil
		}
		token = next
	}
}

func parseAuthType(s string) (nsv1.ConnectorAuthType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "oauth":
		return nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_OAUTH, nil
	case "static_token", "static-token", "token":
		return nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_STATIC_TOKEN, nil
	}
	return nsv1.ConnectorAuthType_CONNECTOR_AUTH_TYPE_UNSPECIFIED, fmt.Errorf("auth_type must be 'oauth' or 'static_token', got %q", s)
}
