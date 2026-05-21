package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMintVerifyRoundTrip(t *testing.T) {
	secret := []byte("top-secret-key")
	exp := time.Now().Add(10 * time.Minute)
	cred := MintCredential(secret, "run-1", exp)

	runID, err := VerifyCredential(secret, cred, time.Now())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if runID != "run-1" {
		t.Fatalf("runID=%q, want run-1", runID)
	}
}

func TestVerifyTamperedSignature(t *testing.T) {
	secret := []byte("top-secret-key")
	cred := MintCredential(secret, "run-1", time.Now().Add(10*time.Minute))

	parts := strings.Split(cred, ".")
	parts[3] = "deadbeef"
	tampered := strings.Join(parts, ".")

	if _, err := VerifyCredential(secret, tampered, time.Now()); !errors.Is(err, ErrCredentialInvalid) {
		t.Fatalf("want ErrCredentialInvalid, got %v", err)
	}
}

func TestVerifyTamperedRunID(t *testing.T) {
	secret := []byte("top-secret-key")
	cred := MintCredential(secret, "run-1", time.Now().Add(10*time.Minute))
	parts := strings.Split(cred, ".")
	parts[1] = "run-other"
	tampered := strings.Join(parts, ".")
	if _, err := VerifyCredential(secret, tampered, time.Now()); !errors.Is(err, ErrCredentialInvalid) {
		t.Fatalf("want ErrCredentialInvalid, got %v", err)
	}
}

func TestVerifyExpired(t *testing.T) {
	secret := []byte("top-secret-key")
	cred := MintCredential(secret, "run-1", time.Now().Add(-1*time.Minute))
	if _, err := VerifyCredential(secret, cred, time.Now()); !errors.Is(err, ErrCredentialExpired) {
		t.Fatalf("want ErrCredentialExpired, got %v", err)
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	cred := MintCredential([]byte("secret-A"), "run-1", time.Now().Add(10*time.Minute))
	if _, err := VerifyCredential([]byte("secret-B"), cred, time.Now()); !errors.Is(err, ErrCredentialInvalid) {
		t.Fatalf("want ErrCredentialInvalid, got %v", err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	secret := []byte("k")
	cases := []string{
		"",
		"v1.run-1",
		"v1.run-1.123",
		"v2.run-1.123.abc",
		"v1..123.abc",
	}
	for _, c := range cases {
		if _, err := VerifyCredential(secret, c, time.Now()); err == nil {
			t.Fatalf("case %q: expected error", c)
		}
	}
}
