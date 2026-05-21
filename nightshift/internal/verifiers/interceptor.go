package verifiers

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryInterceptor authenticates unary RPCs. It resolves the method
// policy, extracts the bearer, runs the registered Verifier chain in
// order (skipping verifiers whose scheme is not allowed for the
// method), confirms the resulting scheme is permitted, and injects
// the Principal into the handler context.
func UnaryInterceptor(set Set) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := authorize(ctx, info.FullMethod, set)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// StreamInterceptor authenticates server-streaming RPCs. Wraps the
// ServerStream so the verified Principal is visible via
// FromContext(stream.Context()) inside the handler.
func StreamInterceptor(set Set) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := authorize(ss.Context(), info.FullMethod, set)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

// authorize runs the common auth path for unary + stream. Returns the
// context to pass to the handler (with Principal attached), or a gRPC
// status error.
func authorize(ctx context.Context, fullMethod string, set Set) (context.Context, error) {
	allowed := AllowedSchemes(fullMethod)
	if len(allowed) == 0 {
		return ctx, nil // unauthenticated method
	}

	md, _ := metadata.FromIncomingContext(ctx)
	tok := bearerFrom(md)
	if tok == "" {
		return nil, status.Error(codes.Unauthenticated, "missing bearer token")
	}

	p, err := VerifyBearer(ctx, tok, allowed, set)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
	}
	return WithPrincipal(ctx, p), nil
}

// VerifyBearer runs verifiers in registration order, skipping any
// whose scheme is not in the allowed set for the method. The first
// verifier that accepts the token wins; if no verifier accepts,
// returns ErrUnauthenticated.
//
// Exposed (capitalized) so non-gRPC callers — e.g. the artifacts HTTP
// proxy handler — can run the same auth path the unary/stream
// interceptors do.
func VerifyBearer(ctx context.Context, token string, allowed []Scheme, set Set) (*Principal, error) {
	for _, v := range set {
		if v == nil || !schemeAllowed(v.Scheme(), allowed) {
			continue
		}
		if p, err := v.Verify(ctx, token); err == nil {
			return p, nil
		}
	}
	return nil, ErrUnauthenticated
}

func schemeAllowed(s Scheme, allowed []Scheme) bool {
	for _, a := range allowed {
		if a == s {
			return true
		}
	}
	return false
}
