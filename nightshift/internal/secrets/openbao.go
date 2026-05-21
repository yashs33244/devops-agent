package secrets

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

// OpenBaoConfig configures the OpenBao KV-v2 backend.
type OpenBaoConfig struct {
	// Addr is the OpenBao API base URL (e.g.
	// http://openbao.nightshift.svc:8200). Required.
	Addr string

	// AuthRole is the kubernetes auth method role to log in as. The
	// chunk-10a bootstrap binds this to the nightshift-api SA in the
	// release namespace. Required.
	AuthRole string

	// KVMount is the mount path of the KV-v2 engine. Default "secret".
	KVMount string

	// SATokenPath is the file path to the projected service-account
	// token. Default /var/run/secrets/kubernetes.io/serviceaccount/token.
	// Tests can point this at a file containing any JWT.
	SATokenPath string

	// HTTPClient is the http.Client to use. Default is
	// &http.Client{Timeout: 10s}.
	HTTPClient *http.Client
}

// OpenBao is a KV-v2 Secrets backend backed by an in-cluster OpenBao.
// Authenticates via the kubernetes auth method on first request and
// re-authenticates on 401/403.
type OpenBao struct {
	addr        string
	authRole    string
	kvMount     string
	saTokenPath string
	http        *http.Client

	mu        sync.Mutex
	clientTok string
	expiresAt time.Time
}

// NewOpenBao builds an OpenBao Secrets client. Does not log in
// eagerly; the first request triggers authentication.
func NewOpenBao(cfg OpenBaoConfig) (*OpenBao, error) {
	if cfg.Addr == "" {
		return nil, errors.New("openbao: Addr required")
	}
	if cfg.AuthRole == "" {
		return nil, errors.New("openbao: AuthRole required")
	}
	mount := cfg.KVMount
	if mount == "" {
		mount = "secret"
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
		kvMount:     mount,
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
		return "", fmt.Errorf("openbao: read SA token %s: %w", b.saTokenPath, err)
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
		return "", fmt.Errorf("openbao: login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openbao: login http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var lr struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", fmt.Errorf("openbao: login decode: %w", err)
	}
	if lr.Auth.ClientToken == "" {
		return "", errors.New("openbao: login response missing client_token")
	}
	b.clientTok = lr.Auth.ClientToken
	// Renew before half-lease to avoid expiring mid-request.
	dur := time.Duration(lr.Auth.LeaseDuration) * time.Second
	if dur <= 0 {
		dur = 30 * time.Minute
	}
	b.expiresAt = time.Now().Add(dur / 2)
	return b.clientTok, nil
}

// invalidateToken clears the cached client_token so the next call
// re-logs in. Called on 401/403.
func (b *OpenBao) invalidateToken() {
	b.mu.Lock()
	b.clientTok = ""
	b.expiresAt = time.Time{}
	b.mu.Unlock()
}

// do executes an authed request, re-logging-in once on 401/403.
func (b *OpenBao) do(ctx context.Context, method, urlPath string, body any) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		tok, err := b.token(ctx)
		if err != nil {
			return nil, err
		}
		var rdr io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			rdr = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(ctx, method, b.addr+urlPath, rdr)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Vault-Token", tok)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
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
	return nil, errors.New("openbao: re-authentication failed")
}

// Get returns the data block at path under KV-v2.
func (b *OpenBao) Get(ctx context.Context, path string) (map[string]string, error) {
	urlPath := fmt.Sprintf("/v1/%s/data/%s", b.kvMount, strings.TrimLeft(stripMountPrefix(path, b.kvMount), "/"))
	resp, err := b.do(ctx, http.MethodGet, urlPath, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openbao: get %s http %d: %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var gr struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("openbao: decode %s: %w", path, err)
	}
	// KV-v2 distinguishes a soft-deleted key (200 with data:null) from
	// a present one. Treat null as not-found.
	if gr.Data.Data == nil {
		return nil, ErrNotFound
	}
	out := make(map[string]string, len(gr.Data.Data))
	for k, v := range gr.Data.Data {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out, nil
}

// Put writes the data block at path under KV-v2.
func (b *OpenBao) Put(ctx context.Context, path string, kv map[string]string) error {
	urlPath := fmt.Sprintf("/v1/%s/data/%s", b.kvMount, strings.TrimLeft(stripMountPrefix(path, b.kvMount), "/"))
	body := map[string]any{"data": kv}
	resp, err := b.do(ctx, http.MethodPost, urlPath, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openbao: put %s http %d: %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// Delete fully removes the metadata + all versions at path.
func (b *OpenBao) Delete(ctx context.Context, path string) error {
	urlPath := fmt.Sprintf("/v1/%s/metadata/%s", b.kvMount, strings.TrimLeft(stripMountPrefix(path, b.kvMount), "/"))
	resp, err := b.do(ctx, http.MethodDelete, urlPath, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("openbao: delete %s http %d: %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
}

// List returns the immediate children of prefix.
func (b *OpenBao) List(ctx context.Context, prefix string) ([]string, error) {
	urlPath := fmt.Sprintf("/v1/%s/metadata/%s", b.kvMount, strings.TrimLeft(stripMountPrefix(prefix, b.kvMount), "/"))
	// OpenBao's "LIST" verb is conventionally GET ?list=true.
	resp, err := b.do(ctx, http.MethodGet, urlPath+"?list=true", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openbao: list %s http %d: %s", prefix, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var lr struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, fmt.Errorf("openbao: list decode: %w", err)
	}
	out := make([]string, 0, len(lr.Data.Keys))
	for _, k := range lr.Data.Keys {
		// OpenBao returns directory entries with a trailing slash;
		// strip it so callers see leaf names uniformly.
		out = append(out, strings.TrimRight(k, "/"))
	}
	return out, nil
}

// stripMountPrefix collapses a callers-style path like
// "secret/nightshift/tokens/alice/notion" to "nightshift/tokens/alice/notion"
// so we can interpolate the right "/v1/<mount>/data/<rest>". Tolerates
// callers that already pass the suffix.
func stripMountPrefix(path, mount string) string {
	p := strings.TrimLeft(path, "/")
	if strings.HasPrefix(p, mount+"/") {
		return strings.TrimPrefix(p, mount+"/")
	}
	return p
}
