package verifiers

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/nightshiftco/nightshift/internal/secrets"
)

// StaticVerifier accepts a small, operator-supplied set of bearer
// tokens, each mapped to a named service principal. The source of
// truth is the Secrets backend at a well-known path; the map lives
// in memory for the lifetime of the process.
//
// KV shape at the configured path:
//
//	secret/nightshift/static-tokens:
//	  scheduler: <random-token-A>
//	  cli-admin: <random-token-B>
//
// Names (left side) appear in audit logs as the principal ID. Tokens
// (right side) are matched constant-time; an unknown token is not
// distinguishable from a misconfigured verifier.
type StaticVerifier struct {
	// tokenToName: reverse index for O(1) lookup. Not exported because
	// constant-time compare requires ranging over every entry anyway.
	tokenToName map[string]string
}

// NewStaticVerifier builds a verifier directly from a name→token
// map. Useful for tests and for operators who prefer to pass tokens
// through a non-Secrets channel (e.g. Helm-templated Secret volume).
func NewStaticVerifier(nameToToken map[string]string) *StaticVerifier {
	v := &StaticVerifier{tokenToName: map[string]string{}}
	for name, tok := range nameToToken {
		if name == "" || tok == "" {
			continue
		}
		v.tokenToName[tok] = name
	}
	return v
}

// LoadStaticVerifier reads the KV entry at path from the Secrets
// backend and builds a verifier. Returns an empty (but non-nil)
// verifier when the path is missing — static tokens are optional.
func LoadStaticVerifier(ctx context.Context, s secrets.Secrets, path string) (*StaticVerifier, error) {
	if s == nil || path == "" {
		return NewStaticVerifier(nil), nil
	}
	kv, err := s.Get(ctx, path)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return NewStaticVerifier(nil), nil
		}
		return nil, err
	}
	return NewStaticVerifier(kv), nil
}

// Scheme reports SchemeService.
func (v *StaticVerifier) Scheme() Scheme { return SchemeService }

// Verify returns the named service principal for a matching token,
// or ErrUnauthenticated on any miss. The ctx is unused (no I/O).
func (v *StaticVerifier) Verify(_ context.Context, cred string) (*Principal, error) {
	if v == nil || len(v.tokenToName) == 0 || cred == "" {
		return nil, ErrUnauthenticated
	}
	// Constant-time scan: compare against every entry so timing does
	// not leak which prefix of a token is correct.
	var match string
	credBytes := []byte(cred)
	for tok, name := range v.tokenToName {
		if subtle.ConstantTimeCompare([]byte(tok), credBytes) == 1 {
			match = name
		}
	}
	if match == "" {
		return nil, ErrUnauthenticated
	}
	return &Principal{Scheme: SchemeService, ID: match}, nil
}

// Len reports the number of loaded tokens (for startup logging).
func (v *StaticVerifier) Len() int {
	if v == nil {
		return 0
	}
	return len(v.tokenToName)
}
