package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// fakeBao is a minimal OpenBao stub: implements /v1/auth/kubernetes/login,
// /v1/<mount>/data/<key> (GET/POST), /v1/<mount>/metadata/<key> (DELETE +
// GET ?list=true). Holds state in a map.
type fakeBao struct {
	mu        chan struct{}
	store     map[string]map[string]string
	logins    int32
	denyToken atomic.Bool // when true, every request returns 401 once
	clientTok string
}

func newFakeBao() *fakeBao {
	return &fakeBao{
		mu:        make(chan struct{}, 1),
		store:     map[string]map[string]string{},
		clientTok: "client-token-stub",
	}
}

func (f *fakeBao) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/kubernetes/login", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&f.logins, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{
				"client_token":   f.clientTok,
				"lease_duration": 3600,
			},
		})
	})
	mux.HandleFunc("/v1/secret/data/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != f.clientTok {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		if f.denyToken.CompareAndSwap(true, false) {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		key := r.URL.Path[len("/v1/secret/data/"):]
		switch r.Method {
		case http.MethodGet:
			data, ok := f.store[key]
			if !ok {
				http.Error(w, "{\"errors\":[]}", http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"data": data},
			})
		case http.MethodPost:
			var body struct {
				Data map[string]string `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			f.store[key] = body.Data
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/secret/metadata/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != f.clientTok {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		key := r.URL.Path[len("/v1/secret/metadata/"):]
		switch r.Method {
		case http.MethodDelete:
			delete(f.store, key)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			if r.URL.Query().Get("list") != "true" {
				http.Error(w, "unsupported", 400)
				return
			}
			children := map[string]struct{}{}
			prefix := key
			if prefix != "" && prefix[len(prefix)-1] != '/' {
				prefix = prefix + "/"
			}
			for k := range f.store {
				if !hasPrefix(k, prefix) {
					continue
				}
				rest := k[len(prefix):]
				if i := indexOf(rest, '/'); i >= 0 {
					rest = rest[:i]
				}
				if rest != "" {
					children[rest] = struct{}{}
				}
			}
			keys := make([]string, 0, len(children))
			for k := range children {
				keys = append(keys, k)
			}
			if len(keys) == 0 {
				http.Error(w, "{\"errors\":[]}", http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": keys},
			})
		}
	})
	return mux
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func newOpenBaoForTest(t *testing.T) (*OpenBao, *fakeBao) {
	t.Helper()
	bao := newFakeBao()
	srv := httptest.NewServer(bao.handler())
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "sa-token")
	if err := os.WriteFile(tokenPath, []byte("fake-jwt"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := NewOpenBao(OpenBaoConfig{
		Addr:        srv.URL,
		AuthRole:    "nightshift-api",
		KVMount:     "secret",
		SATokenPath: tokenPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, bao
}

func newOpenBaoStore(t *testing.T, fixtures map[string]map[string]string) Secrets {
	t.Helper()
	c, _ := newOpenBaoForTest(t)
	for path, kv := range fixtures {
		if err := c.Put(context.Background(), path, kv); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}
	return c
}

func TestOpenBaoCompliance(t *testing.T) {
	runSecretsComplianceSuite(t, newOpenBaoStore)
}

func TestOpenBao_RetriesOnExpiredToken(t *testing.T) {
	c, bao := newOpenBaoForTest(t)
	ctx := context.Background()
	// Prime: a put logs in once.
	if err := c.Put(ctx, "secret/nightshift/tokens/alice/notion", map[string]string{"v": "1"}); err != nil {
		t.Fatal(err)
	}
	loginsAfterFirst := atomic.LoadInt32(&bao.logins)
	// Now force the next request to 401 — the client should re-login + retry.
	bao.denyToken.Store(true)
	if _, err := c.Get(ctx, "secret/nightshift/tokens/alice/notion"); err != nil {
		t.Fatalf("get after token-expiry: %v", err)
	}
	if got := atomic.LoadInt32(&bao.logins); got <= loginsAfterFirst {
		t.Fatalf("expected re-login on 401; logins before=%d after=%d", loginsAfterFirst, got)
	}
}
