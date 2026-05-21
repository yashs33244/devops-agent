package config

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/secrets"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// fakeSecrets is an in-memory Secrets backend for tests. Implements
// the full read/write surface so SetConnectorStaticToken,
// DisconnectConnector, and the resolver all run against it.
type fakeSecrets struct {
	mu   sync.Mutex
	data map[string]map[string]string
}

func newFakeSecrets() *fakeSecrets {
	return &fakeSecrets{data: map[string]map[string]string{}}
}

func (f *fakeSecrets) Get(_ context.Context, path string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[path]
	if !ok {
		return nil, secrets.ErrNotFound
	}
	out := map[string]string{}
	for k, val := range v {
		out[k] = val
	}
	return out, nil
}

func (f *fakeSecrets) Put(_ context.Context, path string, kv map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := map[string]string{}
	for k, v := range kv {
		cp[k] = v
	}
	f.data[path] = cp
	return nil
}

func (f *fakeSecrets) Delete(_ context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, path)
	return nil
}

func (f *fakeSecrets) List(_ context.Context, prefix string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := prefix
	if p != "" && p[len(p)-1] != '/' {
		p = p + "/"
	}
	seen := map[string]struct{}{}
	for k := range f.data {
		if len(k) <= len(p) || k[:len(p)] != p {
			continue
		}
		rest := k[len(p):]
		end := len(rest)
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				end = i
				break
			}
		}
		seen[rest[:end]] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	return out, nil
}

// newTestService spins up a Service against an in-memory SQLite
// Records store + a fakeSecrets. ID generator is deterministic so
// assertions can pin against names. OAuth dispenser is left nil; tests
// that exercise OAuth flow use newTestServiceWithOAuth instead.
func newTestService(t *testing.T) (*Service, *fakeSecrets, map[string]string) {
	svc, sec, runOwners, _ := newTestServiceFull(t, false)
	return svc, sec, runOwners
}

// newTestServiceWithOAuth additionally wires a fake OAuthDispenser.
func newTestServiceWithOAuth(t *testing.T) (*Service, *fakeSecrets, map[string]string, *fakeDispenser) {
	return newTestServiceFull(t, true)
}

func newTestServiceFull(t *testing.T, withOAuth bool) (*Service, *fakeSecrets, map[string]string, *fakeDispenser) {
	t.Helper()
	store, err := records.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sec := newFakeSecrets()
	runOwners := map[string]string{}
	var idCounter int
	newID := func() string {
		idCounter++
		return "id-" + strconv.Itoa(idCounter)
	}
	now := func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	opts := ServiceOptions{
		Records: store,
		Secrets: sec,
		RunOwnerLookup: func(_ context.Context, runID string) (string, error) {
			if u, ok := runOwners[runID]; ok {
				return u, nil
			}
			return "", errors.New("unknown run")
		},
		AdminGroup:  "admin",
		AdminTokens: map[string]bool{"cli-admin": true},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		NewID:       newID,
		Now:         now,
	}
	var disp *fakeDispenser
	if withOAuth {
		disp = newFakeDispenser()
		opts.OAuth = disp
		opts.StateSigningKey = []byte("0123456789abcdef0123456789abcdef")
	}
	svc := NewService(opts)
	return svc, sec, runOwners, disp
}

// fakeDispenser is an in-memory OAuthDispenser for handler tests.
// Records calls so tests can assert on the wire shape.
type fakeDispenser struct {
	mu      sync.Mutex
	servers map[string]oauth.OAuthServerConfig
	creds   map[string]string // credName → access_token
	calls   []string
}

func newFakeDispenser() *fakeDispenser {
	return &fakeDispenser{
		servers: map[string]oauth.OAuthServerConfig{},
		creds:   map[string]string{},
	}
}

func (f *fakeDispenser) RegisterOAuthServer(_ context.Context, name string, cfg oauth.OAuthServerConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.servers[name] = cfg
	f.calls = append(f.calls, "register:"+name)
	return nil
}

func (f *fakeDispenser) GetAuthorizeURL(_ context.Context, server, state, redirectURL string, scopes []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "authorize:"+server)
	return "https://stub.test/authorize?server=" + server + "&state=" + state, nil
}

func (f *fakeDispenser) ExchangeOAuthCode(_ context.Context, credName, server, code, redirectURL, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creds[credName] = "access-" + credName
	f.calls = append(f.calls, "exchange:"+credName)
	return nil
}

func (f *fakeDispenser) GetOAuthToken(_ context.Context, credName string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tok, ok := f.creds[credName]
	if !ok {
		return "", oauth.ErrNotFound
	}
	return tok, nil
}

func (f *fakeDispenser) DeleteOAuthCredential(_ context.Context, credName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.creds, credName)
	f.calls = append(f.calls, "delete:"+credName)
	return nil
}

func ctxAs(scheme verifiers.Scheme, id string, groups ...string) context.Context {
	return verifiers.WithPrincipal(context.Background(), &verifiers.Principal{
		Scheme: scheme,
		ID:     id,
		RunID:  id,
		Groups: groups,
	})
}

func mustCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := status.Code(err); got != want {
		t.Fatalf("code=%v want=%v err=%v", got, want, err)
	}
}
