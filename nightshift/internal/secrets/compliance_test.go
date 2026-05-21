package secrets

import (
	"context"
	"errors"
	"testing"
)

// newSecretsFunc returns a fresh, isolated Secrets backend for one
// compliance subtest, optionally pre-seeded with fixtures.
type newSecretsFunc func(t *testing.T, fixtures map[string]map[string]string) Secrets

// runSecretsComplianceSuite runs the full Secrets contract against the
// backend produced by newStore. Drivers (openbao_test.go and future
// Vault) invoke this from a single TestXxxCompliance entry point so
// adding a backend cannot bypass the contract.
func runSecretsComplianceSuite(t *testing.T, newStore newSecretsFunc) {
	t.Helper()
	t.Run("GetNotFound", func(t *testing.T) {
		s := newStore(t, nil)
		_, err := s.Get(context.Background(), "secret/missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("GetSeeded", func(t *testing.T) {
		s := newStore(t, map[string]map[string]string{
			"secret/api/key": {"value": "abc123"},
		})
		got, err := s.Get(context.Background(), "secret/api/key")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got["value"] != "abc123" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("ListEmpty", func(t *testing.T) {
		s := newStore(t, nil)
		keys, err := s.List(context.Background(), "secret/no/children/here")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(keys) != 0 {
			t.Fatalf("expected empty, got %v", keys)
		}
	})
	t.Run("ListChildren", func(t *testing.T) {
		s := newStore(t, map[string]map[string]string{
			"secret/tokens/alice/notion": {"v": "1"},
			"secret/tokens/alice/github": {"v": "1"},
			"secret/tokens/bob/notion":   {"v": "1"},
		})
		keys, err := s.List(context.Background(), "secret/tokens/alice")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %v", keys)
		}
		got := map[string]bool{}
		for _, k := range keys {
			got[k] = true
		}
		if !got["notion"] || !got["github"] {
			t.Fatalf("missing entries: %v", keys)
		}
	})
	t.Run("PutGetRoundTrip", func(t *testing.T) {
		s := newStore(t, nil)
		ctx := context.Background()
		if err := s.Put(ctx, "secret/k", map[string]string{"v": "1"}); err != nil {
			t.Fatalf("put: %v", err)
		}
		got, err := s.Get(ctx, "secret/k")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got["v"] != "1" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("DeleteIdempotent", func(t *testing.T) {
		s := newStore(t, nil)
		if err := s.Delete(context.Background(), "secret/missing"); err != nil {
			t.Fatalf("delete missing: %v", err)
		}
	})
	t.Run("GetAfterDelete", func(t *testing.T) {
		s := newStore(t, nil)
		ctx := context.Background()
		if err := s.Put(ctx, "secret/k", map[string]string{"v": "1"}); err != nil {
			t.Fatalf("put: %v", err)
		}
		if err := s.Delete(ctx, "secret/k"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.Get(ctx, "secret/k"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound after delete, got %v", err)
		}
	})
}
