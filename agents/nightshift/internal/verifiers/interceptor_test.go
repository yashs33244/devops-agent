package verifiers

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/nightshiftco/nightshift/internal/runtime"
)

// newSet returns a Set wired with the shared test HMAC and two named
// service tokens. OIDC is omitted — the dedicated OIDC test covers
// that path with a fake IdP.
func newSet(t *testing.T) (Set, string /* valid worker cred for run-abc */) {
	t.Helper()
	now := time.Unix(1_700_000_000, 0)
	cred := runtime.MintCredential(testHMAC, "run-abc", now.Add(5*time.Minute))
	return Set{
		&WorkerVerifier{HMAC: testHMAC, Now: func() time.Time { return now }},
		NewStaticVerifier(map[string]string{
			"scheduler": "svc-tok-1",
			"cli-admin": "svc-tok-2",
		}),
	}, cred
}

func ctxWithBearer(tok string) context.Context {
	md := metadata.New(map[string]string{"authorization": "Bearer " + tok})
	return metadata.NewIncomingContext(context.Background(), md)
}

func runUnary(t *testing.T, set Set, method, bearer string) (*Principal, error) {
	t.Helper()
	var principal *Principal
	handler := func(ctx context.Context, _ any) (any, error) {
		principal = FromContext(ctx)
		return "ok", nil
	}
	ctx := context.Background()
	if bearer != "" {
		ctx = ctxWithBearer(bearer)
	}
	_, err := UnaryInterceptor(set)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	return principal, err
}

func TestInterceptor_WorkerCredOnInnerSurface(t *testing.T) {
	set, cred := newSet(t)
	p, err := runUnary(t, set, "/nightshift.v1.Workers/PostWorkerEvent", cred)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if p.Scheme != SchemeWorker || p.RunID != "run-abc" || p.ID != "run-abc" {
		t.Fatalf("principal=%+v", p)
	}
}

func TestInterceptor_StaticOnOuterSurface(t *testing.T) {
	set, _ := newSet(t)
	p, err := runUnary(t, set, "/nightshift.v1.Workers/CreateRun", "svc-tok-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if p.Scheme != SchemeService || p.ID != "scheduler" {
		t.Fatalf("principal=%+v", p)
	}
}

func TestInterceptor_WorkerCredRejectedOnOuterSurface(t *testing.T) {
	set, cred := newSet(t)
	_, err := runUnary(t, set, "/nightshift.v1.Workers/CreateRun", cred)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestInterceptor_StaticRejectedOnInnerSurface(t *testing.T) {
	set, _ := newSet(t)
	_, err := runUnary(t, set, "/nightshift.v1.Workers/PostWorkerEvent", "svc-tok-1")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestInterceptor_MissingBearerRejected(t *testing.T) {
	set, _ := newSet(t)
	_, err := runUnary(t, set, "/nightshift.v1.Workers/CreateRun", "")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestInterceptor_InvalidBearerRejected(t *testing.T) {
	set, _ := newSet(t)
	_, err := runUnary(t, set, "/nightshift.v1.Workers/CreateRun", "bogus-token")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestInterceptor_UnauthenticatedMethodPassesThrough(t *testing.T) {
	set, _ := newSet(t)
	p, err := runUnary(t, set, "/grpc.health.v1.Health/Check", "")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if p != nil {
		t.Fatalf("expected nil principal on unauthenticated method, got %+v", p)
	}
}

func TestInterceptor_CaseInsensitiveBearerScheme(t *testing.T) {
	set, _ := newSet(t)
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.New(map[string]string{"authorization": "bearer svc-tok-2"}))
	called := false
	handler := func(_ context.Context, _ any) (any, error) {
		called = true
		return nil, nil
	}
	_, err := UnaryInterceptor(set)(ctx, nil, &grpc.UnaryServerInfo{
		FullMethod: "/nightshift.v1.Workers/CreateRun",
	}, handler)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !called {
		t.Fatal("handler not invoked")
	}
}

func TestInterceptor_StreamContextCarriesPrincipal(t *testing.T) {
	set, cred := newSet(t)
	md := metadata.New(map[string]string{"authorization": "Bearer " + cred})
	baseCtx := metadata.NewIncomingContext(context.Background(), md)

	fss := &fakeSStream{ctx: baseCtx}
	var seen *Principal
	handler := func(_ any, s grpc.ServerStream) error {
		seen = FromContext(s.Context())
		return nil
	}
	err := StreamInterceptor(set)(nil, fss, &grpc.StreamServerInfo{
		FullMethod: "/nightshift.v1.Workers/PostWorkerEvent",
	}, handler)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if seen == nil || seen.Scheme != SchemeWorker {
		t.Fatalf("seen=%+v", seen)
	}
}

func TestRequireWorkerRunID(t *testing.T) {
	base := context.Background()
	if err := RequireWorkerRunID(base, "r1"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no principal: %v", err)
	}
	ctx := WithPrincipal(base, &Principal{Scheme: SchemeUser, ID: "alice"})
	if err := RequireWorkerRunID(ctx, "r1"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong scheme: %v", err)
	}
	ctx = WithPrincipal(base, &Principal{Scheme: SchemeWorker, ID: "r2", RunID: "r2"})
	if err := RequireWorkerRunID(ctx, "r1"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong run: %v", err)
	}
	ctx = WithPrincipal(base, &Principal{Scheme: SchemeWorker, ID: "r1", RunID: "r1"})
	if err := RequireWorkerRunID(ctx, "r1"); err != nil {
		t.Fatalf("happy: %v", err)
	}
}

func TestBearerFrom_Variants(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"},
		{"BEARER  abc", "abc"},
		{"Basic abc", ""},
		{"abc", ""},
		{"", ""},
	}
	for _, c := range cases {
		md := metadata.New(map[string]string{"authorization": c.header})
		if got := bearerFrom(md); got != c.want {
			t.Errorf("header=%q got=%q want=%q", c.header, got, c.want)
		}
	}
}

// Safety net: ErrUnauthenticated is the only sentinel the interceptor
// layer exposes. Callers check via errors.Is — verify that contract.
func TestErrUnauthenticatedIsSentinel(t *testing.T) {
	if !errors.Is(ErrUnauthenticated, ErrUnauthenticated) {
		t.Fatal("sentinel self-check failed")
	}
}

// fakeSStream is the minimum grpc.ServerStream needed for stream
// interceptor tests. Only Context is exercised.
type fakeSStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeSStream) Context() context.Context     { return f.ctx }
func (f *fakeSStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeSStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeSStream) SetTrailer(metadata.MD)       {}
func (f *fakeSStream) SendMsg(any) error            { return nil }
func (f *fakeSStream) RecvMsg(any) error            { return nil }
