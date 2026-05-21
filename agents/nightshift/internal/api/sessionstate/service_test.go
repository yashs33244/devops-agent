package sessionstate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nightshiftco/nightshift/internal/objects/objectstest"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// hmacSecret is shared between MintCredential and the WorkerVerifier.
var hmacSecret = []byte("0123456789abcdef0123456789abcdef")

// fallback402 is a sentinel handler that returns 402 so tests can
// distinguish "request fell through to fallback" from any of the
// service's own status codes.
var fallback402 = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "fallback", http.StatusPaymentRequired)
})

// testHarness wires a Service against in-memory records + an
// in-process gofakes3-backed object store + a freshly-minted run.
// Returns the harness plus a worker bearer scoped to the run.
type testHarness struct {
	svc    *Service
	mux    http.Handler
	bucket string
	runID  string
	userID string
	sessID string
	bearer string
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	// In-memory records store.
	rec, err := records.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	t.Cleanup(func() { _ = rec.Close() })

	objStore := objectstest.New(t)

	// A run record with user_id + session_id attrs set.
	const (
		runID  = "run-abc"
		userID = "alice"
		sessID = "sess-1"
		bucket = "test-bucket"
	)
	if _, err := rec.Put(context.Background(), records.Record{
		Collection:  "runs",
		Key:         runID,
		Data:        []byte("{}"),
		ContentType: "application/json",
		Attributes: map[string]string{
			"user_id":    userID,
			"session_id": sessID,
		},
	}, nil, ""); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	vset := verifiers.Set{
		&verifiers.WorkerVerifier{HMAC: hmacSecret},
		verifiers.NewStaticVerifier(map[string]string{"svc": "svc-token"}),
	}
	svc := NewService(ServiceOptions{
		Records:   rec,
		Objects:   objStore,
		Bucket:    bucket,
		Verifiers: vset,
	})

	bearer := runtime.MintCredential(hmacSecret, runID, time.Now().Add(1*time.Hour))

	return &testHarness{
		svc:    svc,
		mux:    svc.Handler(fallback402),
		bucket: bucket,
		runID:  runID,
		userID: userID,
		sessID: sessID,
		bearer: bearer,
	}
}

func (h *testHarness) request(method, path, bearer string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, path, body)
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	return r
}

func TestParsePathRejectsNonMatching(t *testing.T) {
	cases := []string{
		"/v1/artifacts/abc/view",
		"/v1/internal/runs/",
		"/v1/internal/runs/foo",
		"/v1/internal/runs/foo/other",
		"/healthz",
	}
	for _, p := range cases {
		if _, _, ok := parsePath(p); ok {
			t.Errorf("parsePath(%q) ok=true, want false", p)
		}
	}
}

func TestHandlerFallthroughOnUnrelatedPath(t *testing.T) {
	h := newTestHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet, "/v1/artifacts/x/view", "", nil))
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected fallback 402, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestManifestEmpty(t *testing.T) {
	h := newTestHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/manifest", h.bearer, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got ManifestResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Entries) != 0 {
		t.Fatalf("expected empty entries, got %+v", got.Entries)
	}
}

func TestPutThenManifestThenGet(t *testing.T) {
	h := newTestHarness(t)

	// PUT.
	body := []byte(`{"hello":"world"}`)
	put := h.request(http.MethodPut,
		"/v1/internal/runs/"+h.runID+"/session-state/objects/foo.jsonl",
		h.bearer, bytes.NewReader(body))
	put.Header.Set("Content-Type", "application/jsonl")
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, put)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Manifest must include the entry.
	rec = httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/manifest", h.bearer, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("manifest status=%d body=%s", rec.Code, rec.Body.String())
	}
	var mf ManifestResponse
	if err := json.NewDecoder(rec.Body).Decode(&mf); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(mf.Entries) != 1 || mf.Entries[0].Key != "foo.jsonl" || mf.Entries[0].Size != int64(len(body)) {
		t.Fatalf("unexpected manifest: %+v", mf.Entries)
	}

	// GET should 302 to a presigned URL we can follow to retrieve bytes.
	rec = httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/objects/foo.jsonl", h.bearer, nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("GET status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatalf("no Location header")
	}
	// Follow the redirect against the presign HTTP server.
	resp, err := http.Get(loc)
	if err != nil {
		t.Fatalf("follow presign: %v", err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Fatalf("body mismatch:\n got=%q\nwant=%q", got, body)
	}
}

func TestPutKeyTraversalRejected(t *testing.T) {
	h := newTestHarness(t)
	bads := []string{
		"../escape",
		"./dot",
		"with\\backslash",
		"with space",
		"a/../b",
	}
	for _, k := range bads {
		rec := httptest.NewRecorder()
		// URL-encode the key so unsafe chars don't break the test
		// request constructor; the server URL-decodes back to k.
		encoded := url.PathEscape(k)
		// Preserve embedded slashes — PathEscape escapes "/" too,
		// which we don't want for cases like "a/../b".
		encoded = strings.ReplaceAll(encoded, "%2F", "/")
		req := h.request(http.MethodPut,
			"/v1/internal/runs/"+h.runID+"/session-state/objects/"+encoded,
			h.bearer, bytes.NewReader([]byte("x")))
		h.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("key %q: status=%d, want 400 (body=%s)", k, rec.Code, rec.Body.String())
		}
	}
}

func TestPutBodyTooLarge(t *testing.T) {
	h := newTestHarness(t)
	// Shrink the cap so we don't allocate a huge buffer in tests.
	h.svc.maxBytes = 16
	body := bytes.Repeat([]byte("x"), 17)
	put := h.request(http.MethodPut,
		"/v1/internal/runs/"+h.runID+"/session-state/objects/big.bin",
		h.bearer, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, put)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthMissingBearer(t *testing.T) {
	h := newTestHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/manifest", "", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthRejectsServiceScheme(t *testing.T) {
	h := newTestHarness(t)
	rec := httptest.NewRecorder()
	// The static verifier registered "svc-token". It's a service-scheme
	// bearer; our handler must refuse it.
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/manifest", "svc-token", nil))
	if rec.Code != http.StatusUnauthorized {
		// VerifyBearer with []Scheme{SchemeWorker} returns
		// ErrUnauthenticated for any non-worker scheme — so we get 401,
		// not 403. Either is acceptable upstream; we assert the
		// observed shape.
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthRejectsWrongRunCredential(t *testing.T) {
	h := newTestHarness(t)
	otherCred := runtime.MintCredential(hmacSecret, "some-other-run", time.Now().Add(time.Hour))
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/manifest", otherCred, nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRunNotFound(t *testing.T) {
	h := newTestHarness(t)
	missingCred := runtime.MintCredential(hmacSecret, "missing-run", time.Now().Add(time.Hour))
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/missing-run/session-state/manifest", missingCred, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetObjectNotFound(t *testing.T) {
	h := newTestHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodGet,
		"/v1/internal/runs/"+h.runID+"/session-state/objects/nope.jsonl", h.bearer, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := newTestHarness(t)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, h.request(http.MethodPost,
		"/v1/internal/runs/"+h.runID+"/session-state/manifest", h.bearer, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewServicePanicsOnMissingDeps(t *testing.T) {
	t.Run("no records", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic")
			}
		}()
		NewService(ServiceOptions{Bucket: "b", Verifiers: verifiers.Set{}})
	})
}

func TestSanitizeKeyAccepts(t *testing.T) {
	good := []string{"a.jsonl", "abc/def.jsonl", "abc-def_ghi.123"}
	for _, k := range good {
		if _, err := sanitizeKey(k); err != nil {
			t.Errorf("sanitizeKey(%q) err=%v want nil", k, err)
		}
	}
}

func TestSanitizeKeyRejects(t *testing.T) {
	bad := []string{"", "/", "a//b", "a/./b", "a/../b", "..", ".", "a\x00b", "a\\b", "a b"}
	for _, k := range bad {
		if _, err := sanitizeKey(k); err == nil {
			t.Errorf("sanitizeKey(%q) accepted unexpectedly", k)
		}
	}
}

// ---- helpers ----

// readBody is a small helper for assertions printed on test failure.
//
//nolint:unused // present for interactive debugging
func readBody(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	return strings.TrimSpace(string(b))
}
