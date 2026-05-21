package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// OpenBaoConfig configures the OpenBao identity-engine directory
// client. Field semantics mirror the secrets-side and oauth-side
// OpenBao configs — operators typically point all three at the same
// OpenBao instance, with all three authenticated by the same SA
// token. The clients are otherwise independent.
type OpenBaoConfig struct {
	// Addr is the OpenBao API base URL. Required.
	Addr string

	// AuthRole is the kubernetes auth method role to log in as.
	// Required.
	AuthRole string

	// SATokenPath is the file path to the projected service-account
	// token. Default /var/run/secrets/kubernetes.io/serviceaccount/token.
	SATokenPath string

	// HTTPClient is the http.Client to use. Default is
	// &http.Client{Timeout: 10s}.
	HTTPClient *http.Client
}

// OpenBao implements Directory by talking to OpenBao's identity
// engine at /v1/identity/group/name/<name> and /v1/identity/entity/id/<id>.
type OpenBao struct {
	addr        string
	authRole    string
	saTokenPath string
	http        *http.Client

	mu        sync.Mutex
	clientTok string
	expiresAt time.Time
}

// NewOpenBao builds an OpenBao directory client. Does not log in
// eagerly; the first request triggers authentication.
func NewOpenBao(cfg OpenBaoConfig) (*OpenBao, error) {
	if cfg.Addr == "" {
		return nil, errors.New("identity/openbao: Addr required")
	}
	if cfg.AuthRole == "" {
		return nil, errors.New("identity/openbao: AuthRole required")
	}
	saPath := cfg.SATokenPath
	if saPath == "" {
		saPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 10 * time.Second}
	}
	return &OpenBao{
		addr:        strings.TrimRight(cfg.Addr, "/"),
		authRole:    cfg.AuthRole,
		saTokenPath: saPath,
		http:        httpc,
	}, nil
}

// token returns a cached client_token, logging in if needed. Holds
// b.mu for the duration of any login.
func (b *OpenBao) token(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clientTok != "" && time.Now().Before(b.expiresAt) {
		return b.clientTok, nil
	}
	jwtBytes, err := os.ReadFile(b.saTokenPath)
	if err != nil {
		return "", fmt.Errorf("identity/openbao: read SA token %s: %w", b.saTokenPath, err)
	}
	jwt := strings.TrimSpace(string(jwtBytes))
	body, _ := json.Marshal(map[string]string{"role": b.authRole, "jwt": jwt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.addr+"/v1/auth/kubernetes/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("identity/openbao: login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("identity/openbao: login http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var lr struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", fmt.Errorf("identity/openbao: login decode: %w", err)
	}
	if lr.Auth.ClientToken == "" {
		return "", errors.New("identity/openbao: login response missing client_token")
	}
	b.clientTok = lr.Auth.ClientToken
	dur := time.Duration(lr.Auth.LeaseDuration) * time.Second
	if dur <= 0 {
		dur = 30 * time.Minute
	}
	b.expiresAt = time.Now().Add(dur / 2)
	return b.clientTok, nil
}

func (b *OpenBao) invalidateToken() {
	b.mu.Lock()
	b.clientTok = ""
	b.expiresAt = time.Time{}
	b.mu.Unlock()
}

// do executes an authed request, re-logging-in once on 401/403.
func (b *OpenBao) do(ctx context.Context, method, urlPath string) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		tok, err := b.token(ctx)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, b.addr+urlPath, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Vault-Token", tok)
		resp, err := b.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			_ = resp.Body.Close()
			b.invalidateToken()
			continue
		}
		return resp, nil
	}
	return nil, errors.New("identity/openbao: re-authentication failed")
}

// ListGroupMembers calls GET /v1/identity/group/name/<group> on
// OpenBao's identity engine.
func (b *OpenBao) ListGroupMembers(ctx context.Context, group string) ([]string, error) {
	resp, err := b.do(ctx, http.MethodGet, fmt.Sprintf("/v1/identity/group/name/%s", group))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("identity/openbao: group/name/%s http %d: %s", group, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var gr struct {
		Data struct {
			MemberEntityIDs []string `json:"member_entity_ids"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("identity/openbao: group decode: %w", err)
	}
	return gr.Data.MemberEntityIDs, nil
}

// GetEntity calls GET /v1/identity/entity/id/<id> on OpenBao's
// identity engine.
func (b *OpenBao) GetEntity(ctx context.Context, id string) (Identity, error) {
	resp, err := b.do(ctx, http.MethodGet, fmt.Sprintf("/v1/identity/entity/id/%s", id))
	if err != nil {
		return Identity{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return Identity{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return Identity{}, fmt.Errorf("identity/openbao: entity/id/%s http %d: %s", id, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var er struct {
		Data struct {
			ID       string            `json:"id"`
			Name     string            `json:"name"`
			Metadata map[string]string `json:"metadata"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return Identity{}, fmt.Errorf("identity/openbao: entity decode: %w", err)
	}
	return Identity{ID: er.Data.ID, Name: er.Data.Name, Metadata: er.Data.Metadata}, nil
}
