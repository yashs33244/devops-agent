package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBao mocks OpenBao's kubernetes-auth login endpoint plus the
// identity engine routes the OpenBao directory client consumes
// (/v1/identity/group/name/<name>, /v1/identity/entity/id/<id>).
//
// It accepts pre-seeded fixtures so the compliance suite's
// "ListGroupMembers_Seeded" and "GetEntity_Seeded" subtests can
// drive the client through the full HTTP path against deterministic
// data.
type fakeBao struct {
	clientTok string
	groups    map[string][]string // group name → entity IDs
	entities  map[string]Identity // entity ID → record
}

func newFakeBao(fx Fixtures) *fakeBao {
	groups := map[string][]string{}
	for k, v := range fx.Groups {
		cp := make([]string, len(v))
		copy(cp, v)
		groups[k] = cp
	}
	entities := map[string]Identity{}
	for k, v := range fx.Entities {
		entities[k] = v
	}
	return &fakeBao{
		clientTok: "client-token-stub",
		groups:    groups,
		entities:  entities,
	}
}

func (f *fakeBao) handler() http.Handler {
	mux := http.NewServeMux()

	// kubernetes-auth login: returns a fixed client_token + lease.
	mux.HandleFunc("/v1/auth/kubernetes/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{"client_token": f.clientTok, "lease_duration": 3600},
		})
	})

	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("X-Vault-Token") != f.clientTok {
			http.Error(w, "denied", http.StatusForbidden)
			return false
		}
		return true
	}

	// /v1/identity/group/name/<name> — return member entity IDs.
	mux.HandleFunc("/v1/identity/group/name/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/v1/identity/group/name/")
		members, ok := f.groups[name]
		if !ok {
			http.Error(w, "{\"errors\":[]}", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"member_entity_ids": members},
		})
	})

	// /v1/identity/entity/id/<id> — return the entity record.
	mux.HandleFunc("/v1/identity/entity/id/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/identity/entity/id/")
		ent, ok := f.entities[id]
		if !ok {
			http.Error(w, "{\"errors\":[]}", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       ent.ID,
				"name":     ent.Name,
				"metadata": ent.Metadata,
			},
		})
	})

	return mux
}

// newOpenBaoForCompliance wires fakeBao + an OpenBao directory
// client for one compliance subtest. Cleanup is registered via
// t.Cleanup.
func newOpenBaoForCompliance(t *testing.T, fx Fixtures) Directory {
	t.Helper()
	bao := newFakeBao(fx)
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
		SATokenPath: tokenPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOpenBaoCompliance(t *testing.T) {
	runDirectoryComplianceSuite(t, newOpenBaoForCompliance)
}
