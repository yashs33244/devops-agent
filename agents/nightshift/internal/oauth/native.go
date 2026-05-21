package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nightshiftco/nightshift/internal/secrets"
)

// Native is an in-process OAuth 2.0 + PKCE/S256 client. It conforms
// to OAuthDispenser (so callers and the compliance suite drive it
// uniformly) and uses a secrets.Secrets backend as its only storage
// substrate — server configs, the per-flow code_verifier, and per-user
// tokens all live in KV, encrypted at rest by whatever backend the
// operator wired (internal/secrets/openbao, future K8s/Vault, etc.).
//
// Always uses PKCE/S256, the OAuth 2.1 default. Compatible with every
// OAuth 2.0 server in the Nightshift connector catalog: PKCE-required
// providers (HubSpot, Notion, Linear, Dropbox, Asana, monday.com) are
// satisfied by definition; PKCE-agnostic providers (GitHub, Google,
// Slack, Microsoft) silently ignore the additional code_challenge
// parameter per RFC 6749 §3.1.
type Native struct {
	kv   secrets.Secrets
	now  func() time.Time
	http *http.Client

	// One mutex per credName guards refresh so concurrent
	// GetOAuthToken calls near expiry don't double-refresh and
	// invalidate each other's refresh_token.
	mu       sync.Mutex
	credLock map[string]*sync.Mutex
}

// Storage paths. Kept under a dedicated prefix so the connectors
// service's static-token cascade doesn't conflate them with per-user
// connector tokens.
const (
	nativeServerPrefix  = "secret/nightshift/oauth-servers/"
	nativePendingPrefix = "secret/nightshift/oauth-pending/"
	nativeTokenPrefix   = "secret/nightshift/oauth-tokens/"
)

// pkceTTL bounds how long a stashed PKCE verifier remains valid
// between GetAuthorizeURL and ExchangeOAuthCode. Long enough to clear
// a slow consent screen; short enough that a stolen state token is
// useless after the user wandered off.
const pkceTTL = 10 * time.Minute

// defaultTokenTTL is applied when a provider's token response omits
// `expires_in`. Without a sane default, GetOAuthToken would treat
// every Get as past-expiry and hammer the refresh endpoint on every
// call. 1h is conservative — short enough that a revoked token
// surfaces an error within a usable window, long enough to avoid
// unnecessary refresh chatter.
const defaultTokenTTL = 1 * time.Hour

// NewNative builds a Native dispenser backed by kv. kv must be
// writable; pass a backend that satisfies secrets.Secrets with Put +
// Delete (the file backend is read-only and incompatible — Native
// will lazy-fail on first Put).
func NewNative(kv secrets.Secrets) *Native {
	return &Native{
		kv:       kv,
		now:      time.Now,
		http:     &http.Client{Timeout: 15 * time.Second},
		credLock: map[string]*sync.Mutex{},
	}
}

// builtinProviders maps Provider name → (auth_code_url, token_url).
// "custom" is special-cased to read URLs from ProviderOptions.
var builtinProviders = map[string][2]string{
	"google":    {"https://accounts.google.com/o/oauth2/v2/auth", "https://oauth2.googleapis.com/token"},
	"github":    {"https://github.com/login/oauth/authorize", "https://github.com/login/oauth/access_token"},
	"slack":     {"https://slack.com/oauth/v2/authorize", "https://slack.com/api/oauth.v2.access"},
	"microsoft": {"https://login.microsoftonline.com/common/oauth2/v2.0/authorize", "https://login.microsoftonline.com/common/oauth2/v2.0/token"},
	"hubspot":   {"https://app.hubspot.com/oauth/authorize", "https://api.hubapi.com/oauth/v1/token"},
	"dropbox":   {"https://www.dropbox.com/oauth2/authorize", "https://api.dropboxapi.com/oauth2/token"},
	"notion":    {"https://api.notion.com/v1/oauth/authorize", "https://api.notion.com/v1/oauth/token"},
	"linear":    {"https://linear.app/oauth/authorize", "https://api.linear.app/oauth/token"},
	"asana":     {"https://app.asana.com/-/oauth_authorize", "https://app.asana.com/-/oauth_token"},
	"monday":    {"https://auth.monday.com/oauth2/authorize", "https://auth.monday.com/oauth2/token"},
}

func resolveURLs(cfg OAuthServerConfig) (string, string, error) {
	if u, ok := builtinProviders[cfg.Provider]; ok {
		return u[0], u[1], nil
	}
	if cfg.Provider != "custom" {
		return "", "", fmt.Errorf("oauth: unknown provider %q (expected one of %s or \"custom\")", cfg.Provider, builtinNames())
	}
	auth := cfg.ProviderOptions["auth_code_url"]
	tok := cfg.ProviderOptions["token_url"]
	if auth == "" || tok == "" {
		return "", "", errors.New("oauth: provider=custom requires auth_code_url and token_url in provider_options")
	}
	return auth, tok, nil
}

func builtinNames() string {
	out := make([]string, 0, len(builtinProviders))
	for k := range builtinProviders {
		out = append(out, k)
	}
	return strings.Join(out, ", ")
}

// RegisterOAuthServer persists cfg under name. Idempotent.
func (n *Native) RegisterOAuthServer(ctx context.Context, name string, cfg OAuthServerConfig) error {
	if name == "" {
		return errors.New("oauth: server name required")
	}
	if _, _, err := resolveURLs(cfg); err != nil {
		return err
	}
	if cfg.ClientID == "" {
		return errors.New("oauth: client_id required")
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("oauth: marshal server cfg: %w", err)
	}
	return n.kv.Put(ctx, nativeServerPrefix+name, map[string]string{"config": string(raw)})
}

func (n *Native) loadServer(ctx context.Context, name string) (OAuthServerConfig, error) {
	kv, err := n.kv.Get(ctx, nativeServerPrefix+name)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return OAuthServerConfig{}, fmt.Errorf("oauth: server %q not registered: %w", name, ErrNotFound)
		}
		return OAuthServerConfig{}, err
	}
	var cfg OAuthServerConfig
	if err := json.Unmarshal([]byte(kv["config"]), &cfg); err != nil {
		return OAuthServerConfig{}, fmt.Errorf("oauth: decode server cfg: %w", err)
	}
	return cfg, nil
}

// GetAuthorizeURL builds the authorize URL with PKCE and stashes the
// verifier under the state key for ExchangeOAuthCode to find.
func (n *Native) GetAuthorizeURL(ctx context.Context, server, state, redirectURL string, scopes []string) (string, error) {
	if server == "" || state == "" || redirectURL == "" {
		return "", errors.New("oauth: server, state, redirect_url required")
	}
	cfg, err := n.loadServer(ctx, server)
	if err != nil {
		return "", err
	}
	authURL, _, err := resolveURLs(cfg)
	if err != nil {
		return "", err
	}

	verifier, err := newCodeVerifier()
	if err != nil {
		return "", err
	}
	challenge := codeChallengeS256(verifier)

	pending := map[string]string{
		"verifier":   verifier,
		"server":     server,
		"expires_at": strconv.FormatInt(n.now().Add(pkceTTL).Unix(), 10),
	}
	if err := n.kv.Put(ctx, nativePendingPrefix+state, pending); err != nil {
		return "", fmt.Errorf("oauth: stash pending state: %w", err)
	}

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURL)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	for k, v := range cfg.AuthURLParams {
		q.Set(k, v)
	}

	sep := "?"
	if strings.Contains(authURL, "?") {
		sep = "&"
	}
	return authURL + sep + q.Encode(), nil
}

// tokenResponse mirrors the standard RFC 6749 token endpoint payload.
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// ExchangeOAuthCode redeems the authorization code for tokens and
// persists them. Looks up the PKCE verifier by state.
func (n *Native) ExchangeOAuthCode(ctx context.Context, credName, server, code, redirectURL, state string) error {
	if credName == "" || server == "" || code == "" || redirectURL == "" || state == "" {
		return errors.New("oauth: credName, server, code, redirect_url, state required")
	}
	cfg, err := n.loadServer(ctx, server)
	if err != nil {
		return err
	}
	_, tokenURL, err := resolveURLs(cfg)
	if err != nil {
		return err
	}

	pending, err := n.kv.Get(ctx, nativePendingPrefix+state)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("oauth: load pending state: %w", err)
	}
	// A malformed or missing expires_at must NOT be interpreted as
	// "never expires" — that would leak a verifier indefinitely.
	// Native always writes a sane value; treat any parse failure /
	// non-positive as expired.
	exp, expErr := strconv.ParseInt(pending["expires_at"], 10, 64)
	if expErr != nil || exp <= 0 || n.now().Unix() >= exp {
		_ = n.kv.Delete(ctx, nativePendingPrefix+state)
		return ErrNotFound
	}
	// Bind state → server. Without this, a caller who can mint a state
	// for one server can reuse the same state to exchange a code for a
	// *different* server, mixing flows. The API service signs state so
	// this is also enforced upstream — but Native is a primitive, so it
	// must defend itself.
	if pending["server"] != server {
		return ErrNotFound
	}
	verifier := pending["verifier"]
	if verifier == "" {
		return ErrNotFound
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURL)
	form.Set("client_id", cfg.ClientID)
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	form.Set("code_verifier", verifier)

	tr, err := n.postToken(ctx, tokenURL, form)
	if err != nil {
		return err
	}
	if err := n.persistToken(ctx, credName, server, tr); err != nil {
		return err
	}
	_ = n.kv.Delete(ctx, nativePendingPrefix+state)
	return nil
}

// GetOAuthToken returns a fresh access_token, refreshing if expired.
func (n *Native) GetOAuthToken(ctx context.Context, credName string) (string, error) {
	mu := n.lockFor(credName)
	mu.Lock()
	defer mu.Unlock()

	stored, err := n.kv.Get(ctx, nativeTokenPrefix+credName)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	exp, _ := strconv.ParseInt(stored["expires_at"], 10, 64)
	// Refresh if expired or within 60s of expiry.
	if exp > 0 && n.now().Unix() < exp-60 {
		return stored["access_token"], nil
	}
	if stored["refresh_token"] == "" {
		// No refresh token and we're past expiry — credential is dead.
		return "", ErrNotFound
	}

	cfg, err := n.loadServer(ctx, stored["server"])
	if err != nil {
		return "", err
	}
	_, tokenURL, err := resolveURLs(cfg)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", stored["refresh_token"])
	form.Set("client_id", cfg.ClientID)
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}

	tr, err := n.postToken(ctx, tokenURL, form)
	if err != nil {
		if errors.Is(err, errProviderRejected) {
			// Provider rejected the refresh (rotated client_secret,
			// revoked grant, expired refresh window). Drop the
			// credential and surface ErrNotFound so callers report
			// "not connected." The user re-OAuths to recover.
			_ = n.kv.Delete(ctx, nativeTokenPrefix+credName)
			return "", ErrNotFound
		}
		// Transport / 5xx — transient. Preserve the credential and
		// surface the error so callers can retry later.
		return "", err
	}
	// Some providers (Dropbox) don't rotate refresh_token; preserve
	// the stored value when the response omits it.
	if tr.RefreshToken == "" {
		tr.RefreshToken = stored["refresh_token"]
	}
	if err := n.persistToken(ctx, credName, stored["server"], tr); err != nil {
		return "", err
	}
	return tr.AccessToken, nil
}

// DeleteOAuthCredential removes the per-user token. Idempotent. Also
// evicts the per-credName mutex so the lock map doesn't grow unbounded
// across the lifetime of the process.
func (n *Native) DeleteOAuthCredential(ctx context.Context, credName string) error {
	if credName == "" {
		return errors.New("oauth: credName required")
	}
	if err := n.kv.Delete(ctx, nativeTokenPrefix+credName); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return err
	}
	n.mu.Lock()
	delete(n.credLock, credName)
	n.mu.Unlock()
	return nil
}

// errProviderRejected wraps token-endpoint errors that are terminal
// for the credential — the provider explicitly refused the grant
// (4xx with an `error` body, invalid_grant, expired refresh_token,
// etc.). Callers use errors.Is to distinguish these from transport-
// level failures (DNS, TLS, 5xx) which are transient and MUST NOT
// cause the cached credential to be nuked.
var errProviderRejected = errors.New("oauth: provider rejected request")

// postToken POSTs an x-www-form-urlencoded body to the provider's
// token endpoint and decodes the response. 4xx provider rejections
// wrap errProviderRejected so the caller can decide to delete the
// credential; 5xx and transport errors surface as plain errors so
// the caller preserves the credential and retries.
func (n *Native) postToken(ctx context.Context, tokenURL string, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := n.http.Do(req)
	if err != nil {
		// Transport: DNS, TLS, connection refused, timeout. NOT a
		// provider-side rejection — return a plain error so callers
		// preserve the credential and retry later.
		return tokenResponse{}, fmt.Errorf("oauth: token endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return tokenResponse{}, fmt.Errorf("oauth: token endpoint http %d: decode: %w (body=%q)", resp.StatusCode, err, truncate(string(body), 256))
	}
	if tr.Error != "" {
		desc := tr.ErrorDescription
		if desc == "" {
			desc = tr.Error
		}
		return tokenResponse{}, fmt.Errorf("oauth: token endpoint: %s: %w", desc, errProviderRejected)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return tokenResponse{}, fmt.Errorf("oauth: token endpoint http %d: %s: %w", resp.StatusCode, truncate(string(body), 256), errProviderRejected)
	}
	if resp.StatusCode != http.StatusOK || tr.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("oauth: token endpoint http %d: %s", resp.StatusCode, truncate(string(body), 256))
	}
	return tr, nil
}

func (n *Native) persistToken(ctx context.Context, credName, server string, tr tokenResponse) error {
	d := time.Duration(tr.ExpiresIn) * time.Second
	if tr.ExpiresIn <= 0 {
		d = defaultTokenTTL
	}
	exp := n.now().Add(d).Unix()
	return n.kv.Put(ctx, nativeTokenPrefix+credName, map[string]string{
		"access_token":  tr.AccessToken,
		"refresh_token": tr.RefreshToken,
		"expires_at":    strconv.FormatInt(exp, 10),
		"server":        server,
	})
}

func (n *Native) lockFor(credName string) *sync.Mutex {
	n.mu.Lock()
	defer n.mu.Unlock()
	if mu, ok := n.credLock[credName]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	n.credLock[credName] = mu
	return mu
}

// newCodeVerifier returns a 64-byte high-entropy verifier encoded as
// 86 base64url chars (RFC 7636 §4.1: 43–128 character range).
func newCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: generate verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallengeS256 = base64url(SHA256(verifier)) per RFC 7636 §4.2.
func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
