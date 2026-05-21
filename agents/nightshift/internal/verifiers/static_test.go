package verifiers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nightshiftco/nightshift/internal/secrets"
)

func newStaticVerifierForCompliance(t *testing.T) (Verifier, string) {
	t.Helper()
	const validToken = "tok-compliance-XYZ"
	return NewStaticVerifier(map[string]string{"compliance-svc": validToken}), validToken
}

func TestStaticCompliance(t *testing.T) {
	runVerifierComplianceSuite(t, newStaticVerifierForCompliance)
}

func newSecretsFile(t *testing.T, body string) secrets.Secrets {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := secrets.NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStaticVerifier_LoadAndVerify(t *testing.T) {
	s := newSecretsFile(t, `
secret/nightshift/static-tokens:
  scheduler: tok-aaa
  cli-admin: tok-bbb
`)
	v, err := LoadStaticVerifier(context.Background(), s, "secret/nightshift/static-tokens")
	if err != nil {
		t.Fatal(err)
	}
	if v.Len() != 2 {
		t.Fatalf("len=%d", v.Len())
	}

	p, err := v.Verify(context.Background(), "tok-aaa")
	if err != nil || p.ID != "scheduler" || p.Scheme != SchemeService {
		t.Fatalf("scheduler: principal=%+v err=%v", p, err)
	}
	p, err = v.Verify(context.Background(), "tok-bbb")
	if err != nil || p.ID != "cli-admin" {
		t.Fatalf("cli-admin: principal=%+v err=%v", p, err)
	}
	if _, err := v.Verify(context.Background(), "nope"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unknown token err=%v", err)
	}
}

func TestStaticVerifier_MissingPathIsEmpty(t *testing.T) {
	s := newSecretsFile(t, "")
	v, err := LoadStaticVerifier(context.Background(), s, "secret/nightshift/static-tokens")
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.Len() != 0 {
		t.Fatalf("want empty non-nil verifier, got %+v", v)
	}
	if _, err := v.Verify(context.Background(), "any-token"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty-verifier verify err=%v", err)
	}
}

func TestStaticVerifier_NilSelfAndEmpty(t *testing.T) {
	var v *StaticVerifier
	if _, err := v.Verify(context.Background(), "x"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("nil verifier err=%v", err)
	}
	if v.Len() != 0 {
		t.Fatalf("nil len=%d", v.Len())
	}
}

func TestStaticVerifier_IgnoresBlankEntries(t *testing.T) {
	s := newSecretsFile(t, `
secret/nightshift/static-tokens:
  scheduler: tok-aaa
  empty: ""
`)
	v, err := LoadStaticVerifier(context.Background(), s, "secret/nightshift/static-tokens")
	if err != nil {
		t.Fatal(err)
	}
	if v.Len() != 1 {
		t.Fatalf("len=%d, want 1", v.Len())
	}
}

func TestStaticVerifier_NilSecretsBackendIsEmpty(t *testing.T) {
	v, err := LoadStaticVerifier(context.Background(), nil, "x")
	if err != nil {
		t.Fatal(err)
	}
	if v.Len() != 0 {
		t.Fatalf("len=%d", v.Len())
	}
}
