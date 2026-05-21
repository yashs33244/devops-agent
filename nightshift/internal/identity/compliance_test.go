package identity

import (
	"context"
	"errors"
	"testing"
)

// Fixtures lets a compliance driver pre-seed a Directory backend
// with the data the suite needs to exercise. Implementations
// receiving Fixtures{} (the zero value) MUST stand up an empty
// directory.
type Fixtures struct {
	// Groups maps group name → entity IDs that belong to that group.
	// The compliance suite reads from these via ListGroupMembers.
	Groups map[string][]string

	// Entities maps entity ID → Identity record. The compliance
	// suite reads from these via GetEntity.
	Entities map[string]Identity
}

// newDirectoryFunc returns a fresh, isolated Directory pre-seeded
// with fixtures for one compliance subtest. Implementations must
// register cleanup via t.Cleanup.
type newDirectoryFunc func(t *testing.T, fixtures Fixtures) Directory

// runDirectoryComplianceSuite runs the full Directory contract
// against the directory produced by newDirectory. Drivers
// (openbao_test.go and future Keycloak / LDAP) invoke this from a
// single TestXxxCompliance entry point so adding a backend cannot
// bypass the contract.
//
// The contract:
//
//  1. ListGroupMembers on a seeded group returns the seeded member
//     IDs (order-insensitive).
//  2. ListGroupMembers on a non-existent group returns ErrNotFound.
//  3. GetEntity on a seeded ID returns the seeded Identity record
//     (ID + Name preserved; Metadata round-trips).
//  4. GetEntity on a non-existent ID returns ErrNotFound.
func runDirectoryComplianceSuite(t *testing.T, newDirectory newDirectoryFunc) {
	t.Helper()

	t.Run("ListGroupMembers_Seeded", func(t *testing.T) {
		d := newDirectory(t, Fixtures{
			Groups: map[string][]string{
				"user": {"alice-id", "bob-id", "charlie-id"},
			},
		})
		got, err := d.ListGroupMembers(context.Background(), "user")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d members, want 3 (%v)", len(got), got)
		}
		seen := map[string]bool{}
		for _, id := range got {
			seen[id] = true
		}
		for _, want := range []string{"alice-id", "bob-id", "charlie-id"} {
			if !seen[want] {
				t.Errorf("missing %q in %v", want, got)
			}
		}
	})

	t.Run("ListGroupMembers_NotFound", func(t *testing.T) {
		d := newDirectory(t, Fixtures{})
		if _, err := d.ListGroupMembers(context.Background(), "nonexistent-group"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("GetEntity_Seeded", func(t *testing.T) {
		d := newDirectory(t, Fixtures{
			Entities: map[string]Identity{
				"alice-id": {
					ID:       "alice-id",
					Name:     "alice",
					Metadata: map[string]string{"email": "alice@example.com"},
				},
			},
		})
		got, err := d.GetEntity(context.Background(), "alice-id")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.ID != "alice-id" {
			t.Errorf("ID=%q, want alice-id", got.ID)
		}
		if got.Name != "alice" {
			t.Errorf("Name=%q, want alice", got.Name)
		}
		if got.Metadata["email"] != "alice@example.com" {
			t.Errorf("Metadata[email]=%q, want alice@example.com", got.Metadata["email"])
		}
	})

	t.Run("GetEntity_NotFound", func(t *testing.T) {
		d := newDirectory(t, Fixtures{})
		if _, err := d.GetEntity(context.Background(), "nonexistent-id"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}
