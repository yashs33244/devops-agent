package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nightshiftco/nightshift/internal/secrets"
)

// inMemKV is a goroutine-safe secrets.Secrets stub for tests. Native
// never calls List, so List returns nil.
type inMemKV struct {
	mu sync.RWMutex
	m  map[string]map[string]string
}

func newInMemKV() *inMemKV { return &inMemKV{m: map[string]map[string]string{}} }

func (k *inMemKV) Get(_ context.Context, p string) (map[string]string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.m[p]
	if !ok {
		return nil, secrets.ErrNotFound
	}
	out := make(map[string]string, len(v))
	for kk, vv := range v {
		out[kk] = vv
	}
	return out, nil
}

func (k *inMemKV) Put(_ context.Context, p string, kv map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	cp := make(map[string]string, len(kv))
	for kk, vv := range kv {
		cp[kk] = vv
	}
	k.m[p] = cp
	return nil
}

func (k *inMemKV) Delete(_ context.Context, p string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, p)
	return nil
}

func (k *inMemKV) List(context.Context, string) ([]string, error) { return nil, nil }

// fakeProvider is a minimal OAuth 2.0 + PKCE token endpoint. It records
// every request so PKCE-specific tests can inspect form fields, and
// returns sequential short-lived tokens so the compliance suite's
// GetRefreshesOnExpiry observes refresh through the interface alone.
type fakeProvider struct {
	mu       sync.Mutex
	requests []url.Values
	seq      int64
}

func newFakeProvider() *fakeProvider { return &fakeProvider{} }

func (f *fakeProvider) handler() http.Handler {
	mux := http.NewServeMux()
	// /login/oauth/authorize never gets fetched server-side — the
	// dispenser builds the URL and returns it; the browser is what
	// fetches it. We register a handler anyway in case a future test
	// asserts on the redirect.
	mux.HandleFunc("/login/oauth/authorize", func(w http.ResponseWriter, _ *http.Request) {})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.requests = append(f.requests, r.PostForm)
		f.mu.Unlock()
		// Sequential access_token + 1-second TTL so Native's 60s skew
		// window forces a refresh on every Get.
		n := atomic.AddInt64(&f.seq, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-" + itoa(int(n)),
			"refresh_token": "refresh-" + itoa(int(n)),
			"token_type":    "Bearer",
			"expires_in":    1,
		})
	})
	return mux
}

func (f *fakeProvider) lastRequest() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return nil
	}
	return f.requests[len(f.requests)-1]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// rewriteTransport intercepts every outbound HTTP request and rewrites
// its URL onto target's host. The compliance suite registers servers
// with Provider="github" which Native resolves to https://github.com/...;
// rewriting routes those to the local httptest fake without changing
// Native's production URL-resolution code.
type rewriteTransport struct {
	target *url.URL
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rewritten := *req.URL
	rewritten.Scheme = rt.target.Scheme
	rewritten.Host = rt.target.Host
	r2 := req.Clone(req.Context())
	r2.URL = &rewritten
	r2.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(r2)
}

// nativeForCompliance constructs a Native instance wired to a fake
// provider for the compliance suite. The driver hides the wiring from
// the suite — the suite calls only OAuthDispenser methods.
func nativeForCompliance(t *testing.T) *Native {
	t.Helper()
	provider := newFakeProvider()
	srv := httptest.NewServer(provider.handler())
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse fake URL: %v", err)
	}
	disp := NewNative(newInMemKV())
	disp.http = &http.Client{
		Timeout:   15 * time.Second,
		Transport: &rewriteTransport{target: target},
	}
	return disp
}

// TestNativeCompliance runs the universal OAuthDispenser contract
// against Native. In commit 4 (skeleton) all subtests except
// GetNotFoundForUnprovisioned and DeleteIdempotent fail. Commit 5
// (filling in Native) turns the suite green.
func TestNativeCompliance(t *testing.T) {
	runOAuthDispenserComplianceSuite(t, func(t *testing.T) OAuthDispenser {
		return nativeForCompliance(t)
	})
}

// nativeWithFake returns a Native pointed at a directly-addressed fake
// provider (no rewriteTransport). Native-specific tests use Provider:
// "custom" + ProviderOptions to inspect wire shape directly without
// the suite's hardcoded "github" provider.
func nativeWithFake(t *testing.T) (*Native, *fakeProvider, OAuthServerConfig) {
	t.Helper()
	provider := newFakeProvider()
	srv := httptest.NewServer(provider.handler())
	t.Cleanup(srv.Close)
	cfg := OAuthServerConfig{
		Provider:     "custom",
		ClientID:     "cid",
		ClientSecret: "csec",
		ProviderOptions: map[string]string{
			"auth_code_url": srv.URL + "/login/oauth/authorize",
			"token_url":     srv.URL + "/login/oauth/access_token",
		},
	}
	return NewNative(newInMemKV()), provider, cfg
}

// TestNative_PKCEEndToEnd asserts the full PKCE handshake: a SHA-256
// challenge appears in the authorize URL, and the matching verifier is
// sent on the token exchange.
func TestNative_PKCEEndToEnd(t *testing.T) {
	disp, prov, cfg := nativeWithFake(t)
	ctx := context.Background()
	if err := disp.RegisterOAuthServer(ctx, "fake", cfg); err != nil {
		t.Fatalf("register: %v", err)
	}
	const state = "state-pkce"
	authURL, err := disp.GetAuthorizeURL(ctx, "fake", state, "https://app/cb", nil)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	if got := u.Query().Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method=%q want S256", got)
	}
	challenge := u.Query().Get("code_challenge")
	if challenge == "" {
		t.Fatal("code_challenge missing from authorize URL")
	}
	if err := disp.ExchangeOAuthCode(ctx, "alice-fake", "fake", "code-X", "https://app/cb", state); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	last := prov.lastRequest()
	if got := last.Get("grant_type"); got != "authorization_code" {
		t.Fatalf("grant_type=%q want authorization_code", got)
	}
	verifier := last.Get("code_verifier")
	if verifier == "" {
		t.Fatal("code_verifier missing on token exchange")
	}
	if codeChallengeS256(verifier) != challenge {
		t.Fatalf("verifier→challenge mismatch: SHA256(%q) != %q", verifier, challenge)
	}
}

// TestNative_ExchangeWithoutAuthorizeIsNotFound: an attacker who
// intercepts a code can't replay it without first triggering authorize
// (which is what stashes the verifier).
func TestNative_ExchangeWithoutAuthorizeIsNotFound(t *testing.T) {
	disp, _, cfg := nativeWithFake(t)
	ctx := context.Background()
	if err := disp.RegisterOAuthServer(ctx, "fake", cfg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := disp.ExchangeOAuthCode(ctx, "alice-fake", "fake", "code-X", "https://app/cb", "never-stashed"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestNative_StateServerBinding: a state minted under server A cannot
// be used to exchange a code for server B. Defense in depth even when
// the API service signs state with a server-bound HMAC.
func TestNative_StateServerBinding(t *testing.T) {
	disp, _, cfgA := nativeWithFake(t)
	// Build a second server pointing at the same fake but registered
	// under a different name.
	cfgB := cfgA
	ctx := context.Background()
	if err := disp.RegisterOAuthServer(ctx, "serverA", cfgA); err != nil {
		t.Fatalf("register A: %v", err)
	}
	if err := disp.RegisterOAuthServer(ctx, "serverB", cfgB); err != nil {
		t.Fatalf("register B: %v", err)
	}
	const state = "shared-state"
	if _, err := disp.GetAuthorizeURL(ctx, "serverA", state, "https://app/cb", nil); err != nil {
		t.Fatalf("authorize A: %v", err)
	}
	// Try to exchange with the same state but bound to server B.
	if err := disp.ExchangeOAuthCode(ctx, "alice-B", "serverB", "code-X", "https://app/cb", state); err != ErrNotFound {
		t.Fatalf("cross-server exchange: want ErrNotFound, got %v", err)
	}
}
