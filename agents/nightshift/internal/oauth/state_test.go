package oauth

import (
	"strings"
	"testing"
	"time"
)

var testStateKey = []byte("test-state-signing-key-must-be-16+-bytes")

func TestState_RoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tok, err := SignState(testStateKey, "alice", "github", now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !strings.Contains(tok, ".") {
		t.Fatalf("token missing separator: %q", tok)
	}
	if err := VerifyState(testStateKey, tok, "alice", "github", now); err != nil {
		t.Fatalf("verify same instant: %v", err)
	}
	// Within TTL.
	if err := VerifyState(testStateKey, tok, "alice", "github", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("verify within TTL: %v", err)
	}
}

func TestState_TTLExpired(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tok, err := SignState(testStateKey, "alice", "github", now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyState(testStateKey, tok, "alice", "github", now.Add(StateTTL+time.Second)); err == nil {
		t.Fatal("expected expired error, got nil")
	}
}

func TestState_HMACMismatch(t *testing.T) {
	now := time.Now()
	tok, err := SignState(testStateKey, "alice", "github", now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	otherKey := []byte("different-key-but-still-16+-bytes-long")
	if err := VerifyState(otherKey, tok, "alice", "github", now); err == nil {
		t.Fatal("expected HMAC mismatch, got nil")
	}
}

func TestState_UserMismatch(t *testing.T) {
	now := time.Now()
	tok, _ := SignState(testStateKey, "alice", "github", now)
	if err := VerifyState(testStateKey, tok, "bob", "github", now); err == nil {
		t.Fatal("expected user mismatch, got nil")
	}
}

func TestState_ResourceMismatch(t *testing.T) {
	now := time.Now()
	tok, _ := SignState(testStateKey, "alice", "github", now)
	if err := VerifyState(testStateKey, tok, "alice", "slack", now); err == nil {
		t.Fatal("expected resource mismatch, got nil")
	}
}

func TestState_Malformed(t *testing.T) {
	now := time.Now()
	cases := []string{
		"",
		"no-separator",
		".",
		"only-payload.",
		".only-sig",
		"!@#$%^.deadbeef",
	}
	for _, tok := range cases {
		if err := VerifyState(testStateKey, tok, "alice", "github", now); err == nil {
			t.Fatalf("expected error for malformed token %q, got nil", tok)
		}
	}
}

func TestState_KeyTooShort(t *testing.T) {
	now := time.Now()
	short := []byte("too-short")
	if _, err := SignState(short, "alice", "github", now); err == nil {
		t.Fatal("expected error on short key sign, got nil")
	}
	if err := VerifyState(short, "any.token", "alice", "github", now); err == nil {
		t.Fatal("expected error on short key verify, got nil")
	}
}

// TestState_DomainSeparation pins the domain prefix: a MAC computed
// over the same encoded payload but without the prefix MUST NOT verify.
// This is what makes the worker-HMAC reuse safe.
func TestState_DomainSeparation(t *testing.T) {
	// Sign normally.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tok, err := SignState(testStateKey, "alice", "github", now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Forge a token with a MAC computed without the domain prefix
	// (i.e., plain HMAC(key, encoded)). Verify must fail.
	parts := strings.SplitN(tok, ".", 2)
	encoded := parts[0]
	forged := encoded + ".0000000000000000000000000000000000000000000000000000000000000000"
	if err := VerifyState(testStateKey, forged, "alice", "github", now); err == nil {
		t.Fatal("expected forged HMAC to fail, got nil")
	}
}
