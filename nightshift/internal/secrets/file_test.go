package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secrets.yaml")
	if err := os.WriteFile(p, []byte(`
secret/api/key:
  value: abc123
secret/nightshift/worker-hmac:
  secret: deadbeef
  extra: meta
`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := NewFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Get(context.Background(), "secret/api/key")
	if err != nil {
		t.Fatal(err)
	}
	if got["value"] != "abc123" {
		t.Fatalf("got %+v", got)
	}
	got, err = f.Get(context.Background(), "secret/nightshift/worker-hmac")
	if err != nil {
		t.Fatal(err)
	}
	if got["secret"] != "deadbeef" || got["extra"] != "meta" {
		t.Fatalf("got %+v", got)
	}
}

func TestFileEnvFallback(t *testing.T) {
	f, err := NewFile("")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NS_SECRET_SECRET_NIGHTSHIFT_WORKER_HMAC", "fromenv")
	got, err := f.Get(context.Background(), "secret/nightshift/worker-hmac")
	if err != nil {
		t.Fatal(err)
	}
	if got["value"] != "fromenv" {
		t.Fatalf("got %+v", got)
	}
}

func TestFileNotFound(t *testing.T) {
	f, err := NewFile("")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Get(context.Background(), "secret/missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
