// Package verifiers implements the nightshift-api authentication
// layer. Every inbound request is authenticated by a Verifier — a
// pluggable bearer-token validator that maps an opaque token to a
// Principal of a specific Scheme.
//
// Today three concrete verifiers ship: WorkerVerifier (per-run HMAC
// credentials), StaticVerifier (operator-supplied service tokens),
// and OIDCVerifier (id_tokens from a configured IdP). Adding a new
// auth method (e.g. SAML, mTLS) means writing a new Verifier
// implementation — no changes to the interceptor or call sites.
//
// Verifiers are bundled into a Set, registered in fastest-reject-first
// order, and tried by the gRPC interceptor for any RPC the policy
// table marks authenticated. Callers obtain the verified Principal
// from request context via FromContext.
package verifiers

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/metadata"
)

// Scheme identifies which kind of caller authenticated.
type Scheme int

const (
	SchemeUnspecified Scheme = iota
	SchemeWorker             // per-run HMAC credential minted by Workers.CreateRun
	SchemeService            // static bearer token for service-to-service clients
	SchemeUser               // OIDC id_token for human users
)

func (s Scheme) String() string {
	switch s {
	case SchemeWorker:
		return "worker"
	case SchemeService:
		return "service"
	case SchemeUser:
		return "user"
	}
	return "unspecified"
}

// Principal is the verified caller attached to every authenticated
// request context.
type Principal struct {
	Scheme Scheme

	// ID is the stable identifier of the principal:
	//   - Worker:  the run_id encoded in the credential
	//   - Service: the named key from the static-tokens map
	//   - User:    the OIDC id_token `sub` claim
	ID string

	// RunID is set only for SchemeWorker and equals ID. It is
	// duplicated so RequireWorkerRunID can read it without switching
	// on Scheme at every call site.
	RunID string

	// Groups is the list of group names the principal belongs to.
	// Populated only for SchemeUser, sourced from the OIDC id_token
	// `groups` claim. Empty for SchemeWorker and SchemeService.
	// Domain services (e.g. internal/api/config) consume this for
	// admin-tier RBAC checks.
	Groups []string
}

// Verifier validates a bearer token and yields the Principal it
// authenticates. Every concrete impl (OIDC, static, worker, future
// SAML/mTLS) satisfies this interface; new auth methods slot into
// the interceptor by adding to a Set.
//
// Verify MUST collapse every failure mode to ErrUnauthenticated so
// the interceptor never leaks token internals through error strings.
type Verifier interface {
	// Scheme is the Principal scheme this verifier produces. Used by
	// the interceptor to short-circuit verifiers whose scheme is not
	// permitted by the request's RPC policy.
	Scheme() Scheme

	// Verify validates token and returns the Principal on success.
	// Implementations that don't need ctx (e.g. WorkerVerifier, which
	// does pure HMAC math) ignore it; OIDCVerifier uses it for JWKS
	// fetches.
	Verify(ctx context.Context, token string) (*Principal, error)
}

// Set is an ordered list of Verifiers tried in registration order.
// Order matters for two reasons:
//
//  1. Fastest-reject-first minimizes wasted work on common tokens —
//     Worker tokens have a distinctive `v1.<runID>.<exp>.<hex>`
//     shape that won't collide with opaque service tokens or JWTs,
//     so the canonical wire-up is worker → static → OIDC.
//
//  2. The first Verifier that accepts the token wins; subsequent
//     ones are not consulted. Implementations therefore SHOULD reject
//     tokens that don't structurally match their format quickly,
//     without expensive cryptographic work.
type Set []Verifier

// ErrUnauthenticated is the only error every Verifier returns on
// failure. The interceptor maps it to gRPC codes.Unauthenticated
// without echoing the token or the underlying cause.
var ErrUnauthenticated = errors.New("unauthenticated")

type ctxKey struct{}

// WithPrincipal returns a new context carrying p.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the principal verified by the auth interceptor,
// or nil if the request was unauthenticated (either because the RPC is
// in the no-auth allowlist or because the interceptor was not wired).
func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}

// bearerFrom pulls a Bearer token out of incoming gRPC metadata.
// Returns "" when no usable authorization header is present. Accepts
// the standard "Bearer <token>" shape and is case-insensitive on the
// scheme per RFC 6750.
func bearerFrom(md metadata.MD) string {
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ""
	}
	raw := vals[0]
	sp := strings.IndexAny(raw, " \t")
	if sp < 0 {
		return ""
	}
	scheme := raw[:sp]
	tok := strings.TrimSpace(raw[sp+1:])
	if !strings.EqualFold(scheme, "Bearer") || tok == "" {
		return ""
	}
	return tok
}
