package records

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

// newStoreFunc returns a fresh, isolated RecordStore for one compliance
// subtest. Implementations must register cleanup via t.Cleanup.
type newStoreFunc func(t *testing.T) RecordStore

// runComplianceSuite runs the full RecordStore contract against the
// backend produced by newStore. Drivers (sqlite_test.go,
// postgres_test.go, ...) invoke this from a single TestXxxCompliance
// entry point so adding a backend cannot bypass the contract.
func runComplianceSuite(t *testing.T, newStore newStoreFunc) {
	t.Helper()
	t.Run("PutGet", func(t *testing.T) { testPutGet(t, newStore(t)) })
	t.Run("GetMissing", func(t *testing.T) { testGetMissing(t, newStore(t)) })
	t.Run("OptimisticConcurrency", func(t *testing.T) { testOptimisticConcurrency(t, newStore(t)) })
	t.Run("IdempotencyReplay", func(t *testing.T) { testIdempotencyReplay(t, newStore(t)) })
	t.Run("ListAttributeFilter", func(t *testing.T) { testListAttributeFilter(t, newStore(t)) })
	t.Run("ListPagination", func(t *testing.T) { testListPagination(t, newStore(t)) })
	t.Run("Delete", func(t *testing.T) { testDelete(t, newStore(t)) })
	t.Run("RecoverStaleRuns", func(t *testing.T) { testRecoverStaleRuns(t, newStore(t)) })
}

func mustPut(t *testing.T, s RecordStore, r Record) Record {
	t.Helper()
	out, err := s.Put(context.Background(), r, nil, "")
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	return out
}

func testPutGet(t *testing.T, s RecordStore) {
	r := Record{
		Collection:  "runs",
		Key:         "run-1",
		Attributes:  map[string]string{"user_id": "alice", "status": "PENDING"},
		Data:        []byte(`{"prompt":"hi"}`),
		ContentType: "application/json",
	}
	got := mustPut(t, s, r)
	if got.Version != 1 {
		t.Fatalf("version=%d, want 1", got.Version)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", got)
	}

	read, err := s.Get(context.Background(), "runs", "run-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(read.Data, r.Data) {
		t.Fatalf("data mismatch")
	}
	if read.Attributes["user_id"] != "alice" {
		t.Fatalf("attrs lost: %+v", read.Attributes)
	}
}

func testGetMissing(t *testing.T, s RecordStore) {
	_, err := s.Get(context.Background(), "runs", "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func testOptimisticConcurrency(t *testing.T, s RecordStore) {
	r := Record{Collection: "c", Key: "k"}
	mustPut(t, s, r)

	// require-not-exist must fail on existing
	zero := int64(0)
	_, err := s.Put(context.Background(), r, &zero, "")
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("want ErrVersionConflict, got %v", err)
	}

	// update with wrong version fails
	wrong := int64(42)
	_, err = s.Put(context.Background(), r, &wrong, "")
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("want ErrVersionConflict, got %v", err)
	}

	// update with right version succeeds and bumps
	ok := int64(1)
	out, err := s.Put(context.Background(), r, &ok, "")
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if out.Version != 2 {
		t.Fatalf("version=%d, want 2", out.Version)
	}
}

func testIdempotencyReplay(t *testing.T, s RecordStore) {
	ctx := context.Background()
	r := Record{Collection: "c", Key: "k", Data: []byte("payload"), ContentType: "text/plain"}

	first, err := s.Put(ctx, r, nil, "idem-1")
	if err != nil {
		t.Fatalf("put 1: %v", err)
	}
	second, err := s.Put(ctx, r, nil, "idem-1")
	if err != nil {
		t.Fatalf("put 2: %v", err)
	}
	if first.Version != second.Version {
		t.Fatalf("replay bumped version: %d -> %d", first.Version, second.Version)
	}

	// Different body with same idem key must fail.
	r2 := r
	r2.Data = []byte("different")
	_, err = s.Put(ctx, r2, nil, "idem-1")
	if err == nil {
		t.Fatalf("expected error on idempotency-key reuse with different body")
	}
}

func testListAttributeFilter(t *testing.T, s RecordStore) {
	ctx := context.Background()
	for i, user := range []string{"alice", "bob", "alice"} {
		mustPut(t, s, Record{
			Collection: "runs",
			Key:        "r" + string(rune('1'+i)),
			Attributes: map[string]string{"user_id": user},
			Data:       []byte(`{}`),
		})
	}
	page, next, err := s.List(ctx, ListQuery{
		Collection:       "runs",
		AttributeFilters: map[string]string{"user_id": "alice"},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("got %d, want 2; page=%+v", len(page), page)
	}
	if next != "" {
		t.Fatalf("unexpected next token: %q", next)
	}
}

func testListPagination(t *testing.T, s RecordStore) {
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		mustPut(t, s, Record{Collection: "c", Key: "k-" + string(rune('a'+i))})
	}

	page1, tok, err := s.List(ctx, ListQuery{Collection: "c", PageSize: 2})
	if err != nil {
		t.Fatalf("list1: %v", err)
	}
	if len(page1) != 2 || tok == "" {
		t.Fatalf("page1 len=%d tok=%q", len(page1), tok)
	}
	page2, tok2, err := s.List(ctx, ListQuery{Collection: "c", PageSize: 2, PageToken: tok})
	if err != nil {
		t.Fatalf("list2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 len=%d", len(page2))
	}
	page3, tok3, err := s.List(ctx, ListQuery{Collection: "c", PageSize: 2, PageToken: tok2})
	if err != nil {
		t.Fatalf("list3: %v", err)
	}
	if len(page3) != 1 || tok3 != "" {
		t.Fatalf("page3 len=%d tok=%q", len(page3), tok3)
	}
}

func testDelete(t *testing.T, s RecordStore) {
	ctx := context.Background()
	r := Record{Collection: "c", Key: "k", Data: []byte("x")}
	mustPut(t, s, r)

	if err := s.Delete(ctx, "c", "k", nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := s.Get(ctx, "c", "k")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
	if err := s.Delete(ctx, "c", "k", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}
}

func testRecoverStaleRuns(t *testing.T, s RecordStore) {
	ctx := context.Background()
	mustPut(t, s, Record{Collection: "runs", Key: "r1", Attributes: map[string]string{"status": "RUNNING"}})
	mustPut(t, s, Record{Collection: "runs", Key: "r2", Attributes: map[string]string{"status": "PENDING"}})
	mustPut(t, s, Record{Collection: "runs", Key: "r3", Attributes: map[string]string{"status": "COMPLETED"}})

	n, err := s.RecoverStaleRuns(ctx)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if n != 2 {
		t.Fatalf("recovered %d, want 2", n)
	}
	for _, k := range []string{"r1", "r2"} {
		rec, err := s.Get(ctx, "runs", k)
		if err != nil {
			t.Fatalf("get %s: %v", k, err)
		}
		if rec.Attributes["status"] != "ERROR" || rec.Attributes["recovered"] != "true" {
			t.Fatalf("%s not marked: %+v", k, rec.Attributes)
		}
	}
}
