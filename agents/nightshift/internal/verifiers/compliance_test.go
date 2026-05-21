package verifiers

import (
	"context"
	"errors"
	"testing"
)

// newVerifierFunc returns a fresh, isolated Verifier plus a token
// the verifier MUST accept (so the compliance suite can exercise the
// happy path) for one compliance subtest. Implementations must
// register cleanup via t.Cleanup.
type newVerifierFunc func(t *testing.T) (v Verifier, validToken string)

// runVerifierComplianceSuite runs the full Verifier contract against
// the verifier produced by newVerifier. Drivers (worker_test.go,
// static_test.go, oidc_test.go, future SAML/mTLS) invoke this from a
// single TestXxxCompliance entry point so adding a verifier cannot
// bypass the contract.
//
// The contract:
//
//  1. Scheme() returns a concrete (non-Unspecified) Scheme.
//  2. Verify of a known-valid token returns a non-nil Principal whose
//     Scheme equals Scheme() and whose ID is non-empty.
//  3. Verify of an empty token returns ErrUnauthenticated (not nil
//     Principal + nil error, and not a leaked error from below).
//  4. Verify of structurally-bogus garbage returns ErrUnauthenticated
//     — never panics, never returns a Principal.
func runVerifierComplianceSuite(t *testing.T, newVerifier newVerifierFunc) {
	t.Helper()
	t.Run("SchemeNotUnspecified", func(t *testing.T) {
		v, _ := newVerifier(t)
		if v.Scheme() == SchemeUnspecified {
			t.Fatal("Scheme() returned Unspecified — every verifier must declare its scheme")
		}
	})
	t.Run("VerifyValidTokenReturnsPrincipal", func(t *testing.T) {
		v, valid := newVerifier(t)
		p, err := v.Verify(context.Background(), valid)
		if err != nil {
			t.Fatalf("valid token rejected: %v", err)
		}
		if p == nil {
			t.Fatal("nil principal returned without error")
		}
		if p.Scheme != v.Scheme() {
			t.Fatalf("principal.Scheme=%v, verifier.Scheme()=%v", p.Scheme, v.Scheme())
		}
		if p.ID == "" {
			t.Fatal("principal ID is empty")
		}
	})
	t.Run("VerifyEmptyTokenRejects", func(t *testing.T) {
		v, _ := newVerifier(t)
		if _, err := v.Verify(context.Background(), ""); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("empty token: err=%v, want ErrUnauthenticated", err)
		}
	})
	t.Run("VerifyGarbageTokenRejects", func(t *testing.T) {
		v, _ := newVerifier(t)
		if _, err := v.Verify(context.Background(), "completely-bogus-token-xyz"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("garbage token: err=%v, want ErrUnauthenticated", err)
		}
	})
}
