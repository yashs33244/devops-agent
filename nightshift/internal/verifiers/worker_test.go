package verifiers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nightshiftco/nightshift/internal/runtime"
)

var testHMAC = []byte("0123456789abcdef0123456789abcdef")

func newWorkerVerifierForCompliance(t *testing.T) (Verifier, string) {
	t.Helper()
	now := time.Unix(1_000_000, 0)
	cred := runtime.MintCredential(testHMAC, "run-compliance", now.Add(5*time.Minute))
	return &WorkerVerifier{HMAC: testHMAC, Now: func() time.Time { return now }}, cred
}

func TestWorkerCompliance(t *testing.T) {
	runVerifierComplianceSuite(t, newWorkerVerifierForCompliance)
}

// Worker-specific edge cases below — covered by the compliance suite's
// VerifyGarbageTokenRejects in spirit, but pinned individually because
// each represents a distinct attack/misuse path.

func TestWorkerVerifier_BadSignature(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cred := runtime.MintCredential(testHMAC, "run-123", now.Add(5*time.Minute))
	other := []byte("ffffffffffffffffffffffffffffffff")
	v := &WorkerVerifier{HMAC: other, Now: func() time.Time { return now }}

	_, err := v.Verify(context.Background(), cred)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestWorkerVerifier_Expired(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cred := runtime.MintCredential(testHMAC, "run-123", now.Add(-time.Second))
	v := &WorkerVerifier{HMAC: testHMAC, Now: func() time.Time { return now }}

	_, err := v.Verify(context.Background(), cred)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestWorkerVerifier_Malformed(t *testing.T) {
	v := &WorkerVerifier{HMAC: testHMAC}
	for _, cred := range []string{"garbage", "v1.run.123", "v2.run.1.deadbeef"} {
		if _, err := v.Verify(context.Background(), cred); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("cred=%q err=%v", cred, err)
		}
	}
}

func TestWorkerVerifier_NilSelf(t *testing.T) {
	var v *WorkerVerifier
	if _, err := v.Verify(context.Background(), "anything"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestWorkerVerifier_EmptyHMAC(t *testing.T) {
	v := &WorkerVerifier{HMAC: nil}
	if _, err := v.Verify(context.Background(), "anything"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

// Sanity: a v1-shaped but obviously-bogus runID is still rejected by
// the signature check — no runID is lifted before signature verifies.
func TestWorkerVerifier_RunIDNotLiftedBeforeSignature(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cred := runtime.MintCredential(testHMAC, "real-run", now.Add(time.Minute))
	parts := strings.Split(cred, ".")
	parts[1] = "tampered-run"
	tampered := strings.Join(parts, ".")

	v := &WorkerVerifier{HMAC: testHMAC, Now: func() time.Time { return now }}
	if _, err := v.Verify(context.Background(), tampered); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("tampered runID accepted: err=%v", err)
	}
}
