package objects

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

// newObjectStoreFunc returns a fresh, isolated ObjectStore for one
// compliance subtest. Implementations must register cleanup via
// t.Cleanup.
type newObjectStoreFunc func(t *testing.T) ObjectStore

// runObjectStoreComplianceSuite runs the full ObjectStore contract
// against the backend produced by newStore. Drivers (s3_test.go,
// future backends) invoke this from a single TestXxxCompliance entry
// point so adding a backend cannot bypass the contract.
func runObjectStoreComplianceSuite(t *testing.T, newStore newObjectStoreFunc) {
	t.Helper()
	t.Run("PutBytesStat", func(t *testing.T) { testPutBytesStat(t, newStore(t)) })
	t.Run("InitiateLifecycle", func(t *testing.T) { testInitiateLifecycle(t, newStore(t)) })
	t.Run("DownloadBeforeFinalize", func(t *testing.T) { testDownloadBeforeFinalize(t, newStore(t)) })
	t.Run("Delete", func(t *testing.T) { testDelete_Object(t, newStore(t)) })
	t.Run("ListPrefix", func(t *testing.T) { testListPrefix(t, newStore(t)) })
}

func testPutBytesStat(t *testing.T, s ObjectStore) {
	ctx := context.Background()
	body := []byte("payload")
	obj, err := s.PutBytes(ctx, "bk", "p/file.txt", "text/plain", body)
	if err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	if obj.State != UploadStateReady {
		t.Fatalf("state=%v, want READY", obj.State)
	}
	if obj.SizeBytes != int64(len(body)) {
		t.Fatalf("size=%d, want %d", obj.SizeBytes, len(body))
	}

	stat, err := s.Stat(ctx, "bk", "p/file.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if stat.SizeBytes != int64(len(body)) {
		t.Fatalf("stat size=%d", stat.SizeBytes)
	}
	if stat.ContentType != "text/plain" {
		t.Fatalf("stat content-type=%q", stat.ContentType)
	}
}

func testInitiateLifecycle(t *testing.T, s ObjectStore) {
	ctx := context.Background()

	obj, uploadURL, headers, expires, err := s.Initiate(ctx, InitiateSpec{
		Bucket:      "bk",
		Key:         "path/to/file.txt",
		ContentType: "text/plain",
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if obj.State != UploadStatePending {
		t.Fatalf("state=%v, want PENDING", obj.State)
	}
	if time.Until(expires) <= 0 {
		t.Fatalf("expires not in future")
	}

	req, _ := http.NewRequest(http.MethodPut, uploadURL, bytes.NewBufferString("hello world"))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT status=%d body=%s", resp.StatusCode, body)
	}
	_ = resp.Body.Close()

	finalized, err := s.Finalize(ctx, "bk", "path/to/file.txt")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if finalized.State != UploadStateReady {
		t.Fatalf("state=%v, want READY", finalized.State)
	}
	if finalized.SizeBytes != int64(len("hello world")) {
		t.Fatalf("size=%d", finalized.SizeBytes)
	}

	dlURL, _, err := s.DownloadURL(ctx, "bk", "path/to/file.txt", time.Minute)
	if err != nil {
		t.Fatalf("download url: %v", err)
	}
	dlResp, err := http.Get(dlURL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = dlResp.Body.Close() }()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status=%d", dlResp.StatusCode)
	}
	got, _ := io.ReadAll(dlResp.Body)
	if string(got) != "hello world" {
		t.Fatalf("body=%q", got)
	}
}

func testDownloadBeforeFinalize(t *testing.T, s ObjectStore) {
	ctx := context.Background()
	if _, _, _, _, err := s.Initiate(ctx, InitiateSpec{Bucket: "bk", Key: "file"}); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if _, _, err := s.DownloadURL(ctx, "bk", "file", time.Minute); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("want ErrInvalidState, got %v", err)
	}
}

func testDelete_Object(t *testing.T, s ObjectStore) {
	ctx := context.Background()

	// Delete on a missing object is idempotent.
	if err := s.Delete(ctx, "bk", "missing"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}

	// Delete an existing object → Stat returns ErrNotFound.
	if _, err := s.PutBytes(ctx, "bk", "delete-me", "text/plain", []byte("x")); err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	if err := s.Delete(ctx, "bk", "delete-me"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Stat(ctx, "bk", "delete-me"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stat after delete: want ErrNotFound, got %v", err)
	}
}

func testListPrefix(t *testing.T, s ObjectStore) {
	ctx := context.Background()
	for _, k := range []string{"a/1", "a/2", "b/1"} {
		if _, _, _, _, err := s.Initiate(ctx, InitiateSpec{Bucket: "bk", Key: k}); err != nil {
			t.Fatalf("initiate %s: %v", k, err)
		}
	}
	all, _, err := s.List(ctx, "bk", "", "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d, want 3", len(all))
	}
	onlyA, _, err := s.List(ctx, "bk", "a/", "", 10)
	if err != nil {
		t.Fatalf("list a/: %v", err)
	}
	if len(onlyA) != 2 {
		t.Fatalf("prefix filter: got %d, want 2", len(onlyA))
	}
}
