package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nightshiftco/nightshift/internal/objects"
)

func TestSessionSubPath(t *testing.T) {
	got, err := SessionSubPath("alice", "sess-1")
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	if got != filepath.Join("alice", "sess-1") {
		t.Fatalf("got %q", got)
	}
}

func TestSessionSubPathRejectsBadInput(t *testing.T) {
	cases := []struct{ user, session string }{
		{"", "s"},
		{"u", ""},
		{"u/x", "s"},
		{"u", "s/x"},
		{"..", "s"},
		{"u", ".."},
		{"a/../b", "s"},
		{"u", "a/../b"},
		{"u\x00", "s"},
		{"u", "s\x00"},
	}
	for _, c := range cases {
		if _, err := SessionSubPath(c.user, c.session); err == nil {
			t.Errorf("SessionSubPath(%q,%q): expected error", c.user, c.session)
		}
	}
}

func TestSessionObjectPrefix(t *testing.T) {
	got, err := SessionObjectPrefix("alice", "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sessions/alice/sess-1/" {
		t.Fatalf("got %q", got)
	}
	if !strings.HasSuffix(got, "/") {
		t.Fatalf("prefix must end with / (else List would over-match)")
	}
}

func TestSessionStateConfigEnabled(t *testing.T) {
	cases := []struct {
		backend SessionStateBackend
		want    bool
	}{
		{SessionStateBackendNone, false},
		{SessionStateBackendPVC, true},
		{SessionStateBackendHost, true},
		{SessionStateBackendObject, true},
	}
	for _, c := range cases {
		got := SessionStateConfig{Backend: c.backend}.Enabled()
		if got != c.want {
			t.Errorf("Backend=%s Enabled()=%v, want %v", c.backend, got, c.want)
		}
	}
}

func TestLocalFSCleaner(t *testing.T) {
	root := t.TempDir()
	c := &LocalFSCleaner{Root: root}

	target := filepath.Join(root, "alice", "sess-1")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "state.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Sibling user dir must survive.
	sibling := filepath.Join(root, "bob", "sess-1")
	if err := os.MkdirAll(sibling, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := c.Clean(context.Background(), "alice", "sess-1"); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target should be gone, got %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("sibling should survive: %v", err)
	}
}

func TestLocalFSCleanerIdempotent(t *testing.T) {
	c := &LocalFSCleaner{Root: t.TempDir()}
	// No directory exists; cleaning is a no-op.
	if err := c.Clean(context.Background(), "alice", "missing"); err != nil {
		t.Fatalf("clean missing: %v", err)
	}
	// Second call still nil.
	if err := c.Clean(context.Background(), "alice", "missing"); err != nil {
		t.Fatalf("clean missing again: %v", err)
	}
}

func TestLocalFSCleanerRejectsBadInput(t *testing.T) {
	c := &LocalFSCleaner{Root: t.TempDir()}
	if err := c.Clean(context.Background(), "", "sess"); err == nil {
		t.Fatal("expected error for empty user_id")
	}
	if err := c.Clean(context.Background(), "u/x", "sess"); err == nil {
		t.Fatal("expected error for unsafe user_id")
	}
}

func TestLocalFSCleanerRequiresRoot(t *testing.T) {
	c := &LocalFSCleaner{}
	if err := c.Clean(context.Background(), "u", "s"); err == nil {
		t.Fatal("expected error for empty Root")
	}
}

// objectFake is a minimal in-memory ObjectStore for ObjectCleaner
// tests. Only List + Delete are exercised; the others panic so any
// future code that drifts onto them surfaces clearly.
type objectFake struct {
	objs map[string]map[string][]byte // bucket -> key -> data
}

func newObjectFake() *objectFake { return &objectFake{objs: map[string]map[string][]byte{}} }

func (f *objectFake) put(bucket, key string) {
	if f.objs[bucket] == nil {
		f.objs[bucket] = map[string][]byte{}
	}
	f.objs[bucket][key] = []byte("x")
}

func (f *objectFake) Initiate(_ context.Context, _ objects.InitiateSpec) (objects.Object, string, map[string]string, time.Time, error) {
	panic("Initiate not used in test")
}

func (f *objectFake) Finalize(_ context.Context, _, _ string) (objects.Object, error) {
	panic("Finalize not used")
}

func (f *objectFake) PutBytes(_ context.Context, _, _, _ string, _ []byte) (objects.Object, error) {
	panic("PutBytes not used")
}

func (f *objectFake) Stat(_ context.Context, _, _ string) (objects.Object, error) {
	panic("Stat not used")
}

func (f *objectFake) DownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, time.Time, error) {
	panic("DownloadURL not used")
}

func (f *objectFake) Delete(_ context.Context, bucket, key string) error {
	if m, ok := f.objs[bucket]; ok {
		delete(m, key)
	}
	return nil
}

func (f *objectFake) List(_ context.Context, bucket, prefix, _ string, _ int32) ([]objects.Object, string, error) {
	var out []objects.Object
	for k := range f.objs[bucket] {
		if strings.HasPrefix(k, prefix) {
			out = append(out, objects.Object{Bucket: bucket, Key: k})
		}
	}
	return out, "", nil
}

func TestObjectCleaner(t *testing.T) {
	f := newObjectFake()
	f.put("ns", "sessions/alice/sess-1/state.json")
	f.put("ns", "sessions/alice/sess-1/memory/scratch.txt")
	f.put("ns", "sessions/alice/sess-2/state.json") // sibling session — survives
	f.put("ns", "sessions/bob/sess-1/state.json")   // sibling user — survives
	f.put("ns", "artifacts/blob-x")                 // unrelated — survives

	c := &ObjectCleaner{Store: f, Bucket: "ns"}
	if err := c.Clean(context.Background(), "alice", "sess-1"); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if _, ok := f.objs["ns"]["sessions/alice/sess-1/state.json"]; ok {
		t.Errorf("alice/sess-1/state.json should be gone")
	}
	if _, ok := f.objs["ns"]["sessions/alice/sess-1/memory/scratch.txt"]; ok {
		t.Errorf("alice/sess-1/memory/scratch.txt should be gone")
	}
	if _, ok := f.objs["ns"]["sessions/alice/sess-2/state.json"]; !ok {
		t.Errorf("sibling session sess-2 should survive")
	}
	if _, ok := f.objs["ns"]["sessions/bob/sess-1/state.json"]; !ok {
		t.Errorf("sibling user bob should survive")
	}
	if _, ok := f.objs["ns"]["artifacts/blob-x"]; !ok {
		t.Errorf("unrelated object should survive")
	}
}

func TestObjectCleanerIdempotent(t *testing.T) {
	f := newObjectFake()
	c := &ObjectCleaner{Store: f, Bucket: "ns"}
	// No matching prefix → no-op.
	if err := c.Clean(context.Background(), "alice", "sess-1"); err != nil {
		t.Fatalf("clean missing: %v", err)
	}
	// Second call, still nil.
	if err := c.Clean(context.Background(), "alice", "sess-1"); err != nil {
		t.Fatalf("clean missing again: %v", err)
	}
}

func TestObjectCleanerValidation(t *testing.T) {
	// Both Store and Bucket are required.
	if err := (&ObjectCleaner{Bucket: "ns"}).Clean(context.Background(), "u", "s"); err == nil {
		t.Error("expected error when Store nil")
	}
	if err := (&ObjectCleaner{Store: newObjectFake()}).Clean(context.Background(), "u", "s"); err == nil {
		t.Error("expected error when Bucket empty")
	}
}

func TestNoopCleaner(t *testing.T) {
	if err := (NoopCleaner{}).Clean(context.Background(), "u", "s"); err != nil {
		t.Fatal(err)
	}
}
