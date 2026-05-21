// Package users exposes a small HTTP-only `GET /v1/users` endpoint
//
// This is HTTP-only (no proto/gRPC) because the only consumer is the
// UI's per-page user picker; adding a Users gRPC service would mean
// proto regen + a public surface we don't otherwise need.
package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/nightshiftco/nightshift/internal/identity"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// Service serves GET /v1/users. Exposed to anyone holding a valid
// bearer of any non-worker scheme — the share dialog must be usable
// by every signed-in user, but we don't expose this to worker pods
// (which never need to discover users).
type Service struct {
	Directory identity.Directory
	Verifiers verifiers.Set
	Logger    *slog.Logger

	// Group is the identity group whose members are listable.
	// Default "user" when zero — bootstrap creates this group and
	// assigns regular users to it.
	Group string
}

// concurrentLookups caps how many entity-resolution requests are
// in-flight at once. OpenBao tolerates a handful per group; cap so a
// huge group doesn't fan out unbounded.
const concurrentLookups = 8

// Handler returns the http.Handler for GET /v1/users. The caller
// (cmd/nightshift-api/main.go) mounts it on httpMux at the matching
// path; ServeMux's longer-prefix-wins routing keeps it from being
// shadowed by the artifacts proxy on /v1/.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(s.serveListUsers)
}

func (s *Service) serveListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	tok := bearerFromHeader(r.Header.Get("Authorization"))
	if tok == "" {
		http.Error(w, "missing bearer", http.StatusUnauthorized)
		return
	}
	// SchemeWorker omitted — workers should never hit user discovery.
	if _, err := verifiers.VerifyBearer(ctx, tok, []verifiers.Scheme{verifiers.SchemeUser, verifiers.SchemeService}, s.Verifiers); err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	group := s.Group
	if group == "" {
		group = "user"
	}
	memberIDs, err := s.Directory.ListGroupMembers(ctx, group)
	if err != nil {
		if errors.Is(err, identity.ErrNotFound) {
			// Group hasn't been created yet — operator hasn't run
			// bootstrap or hasn't seeded any users. Return empty so the
			// share dialog renders cleanly instead of erroring.
			writeUsersJSON(w, nil)
			return
		}
		s.logger().Warn("users: list group members failed", "group", group, "err", err)
		http.Error(w, "user directory unavailable", http.StatusBadGateway)
		return
	}
	if len(memberIDs) == 0 {
		writeUsersJSON(w, nil)
		return
	}

	users, err := s.resolveEntities(ctx, memberIDs)
	if err != nil {
		s.logger().Warn("users: resolve entities failed", "err", err)
		http.Error(w, "user directory unavailable", http.StatusBadGateway)
		return
	}
	// Stable order makes the picker UX predictable.
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	writeUsersJSON(w, users)
}

// userOut is the wire shape consumed by the UI's UserSummary type
// (`{id, name, email}`). Stable across this handler's lifetime so the
// adapter (cmd/nightshift-ui/lib/server/nightshift-proxy.ts) doesn't
// need response remapping.
type userOut struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Service) resolveEntities(ctx context.Context, ids []string) ([]userOut, error) {
	out := make([]userOut, 0, len(ids))
	var mu sync.Mutex
	sem := make(chan struct{}, concurrentLookups)
	var wg sync.WaitGroup
	var firstErr error
	var firstErrOnce sync.Once

	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			ent, err := s.Directory.GetEntity(ctx, id)
			if err != nil {
				if errors.Is(err, identity.ErrNotFound) {
					// Entity vanished between LIST and READ; skip it.
					return
				}
				firstErrOnce.Do(func() { firstErr = err })
				return
			}
			email := ent.Metadata["email"]
			if email == "" {
				email = fmt.Sprintf("%s@nightshift.local", ent.Name)
			}
			mu.Lock()
			out = append(out, userOut{ID: ent.ID, Name: ent.Name, Email: email})
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func writeUsersJSON(w http.ResponseWriter, users []userOut) {
	if users == nil {
		users = []userOut{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"users": users})
}

func bearerFromHeader(h string) string {
	const prefix = "bearer "
	if len(h) < len(prefix) {
		return ""
	}
	if !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}
