package verifiers

import (
	"context"
	"time"

	"github.com/nightshiftco/nightshift/internal/runtime"
)

// WorkerVerifier validates per-run HMAC credentials minted by
// workers.Service.CreateRun. The wire format and verification rules
// live in runtime/credential.go; this type adapts that primitive to
// the Verifier interface.
type WorkerVerifier struct {
	// HMAC is the shared signing key. Must match the key passed to
	// runtime.MintCredential at the Workers service.
	HMAC []byte

	// Now overrides the clock for tests. Defaults to time.Now.
	Now func() time.Time
}

// Scheme reports SchemeWorker.
func (v *WorkerVerifier) Scheme() Scheme { return SchemeWorker }

// Verify returns a worker Principal whose ID + RunID encode the
// credential's bound run, or ErrUnauthenticated on any failure (bad
// version, signature mismatch, malformed, expired). The ctx is
// unused (no I/O).
func (v *WorkerVerifier) Verify(_ context.Context, cred string) (*Principal, error) {
	if v == nil || len(v.HMAC) == 0 {
		return nil, ErrUnauthenticated
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	runID, err := runtime.VerifyCredential(v.HMAC, cred, now())
	if err != nil {
		return nil, ErrUnauthenticated
	}
	return &Principal{Scheme: SchemeWorker, ID: runID, RunID: runID}, nil
}
