package artifacts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/runtime"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// ProxyHandler returns an http.Handler that serves the viewer-facing
// artifact surfaces by intercepting them in front of the grpc-gateway
// mux:
//
//   - GET /v1/artifacts/{id}/view         → reverse-proxy or 302
//   - GET /v1/artifacts/{id}:downloadUrl  → JSON {download_url, expires_at}
//   - GET /v1/artifacts/{id}:previewUrl   → JSON {preview_url, expires_at}
//
// Everything else falls through to the supplied fallback (the
// grpc-gateway mux from main.go).
//
// Authentication policy (shared across all three suffixes):
//   - art.Public == true  → no auth required
//   - art.Public == false → Bearer token verified via Service.verifiers,
//     then canRead(principal, art, grants, viewerID); 401 on missing
//     bearer, 403 on permission denied, 404 on miss-or-no-access
//     (artifacts.md §3 collapse rule).
//
// Stub-runtime apps return 503 — the StubAppDeployer doesn't run a
// real backend.
func (s *Service) ProxyHandler(fallback http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		artifactID, suffix, ok := parseArtifactPath(r.URL.Path)
		if !ok {
			fallback.ServeHTTP(w, r)
			return
		}
		switch suffix {
		case "view":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.serveAppView(w, r, artifactID)
		case ":downloadUrl":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.serveDownloadURL(w, r, artifactID)
		case ":previewUrl":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.servePreviewURL(w, r, artifactID)
		default:
			fallback.ServeHTTP(w, r)
		}
	})
}

// loadAndAuthorize implements the shared /view-policy: public skips
// auth, private requires Bearer + canRead. On error it writes the
// response and returns ok=false. Used by all three viewer-facing
// suffixes so the policy lives in one place (#165).
func (s *Service) loadAndAuthorize(w http.ResponseWriter, r *http.Request, artifactID string) (*nsv1.Artifact, bool) {
	ctx := r.Context()
	rec, err := s.records.Get(ctx, artifactsCollection, artifactID)
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			http.NotFound(w, r)
			return nil, false
		}
		s.logger.Warn("proxy: records.Get", "id", artifactID, "err", err)
		http.Error(w, "records error", http.StatusInternalServerError)
		return nil, false
	}
	art, err := recordToArtifact(rec)
	if err != nil {
		s.logger.Warn("proxy: decode artifact", "id", artifactID, "err", err)
		http.Error(w, "decode error", http.StatusInternalServerError)
		return nil, false
	}
	if !art.GetPublic() {
		principal, viewerID, err := s.authenticateProxy(ctx, r)
		if err != nil {
			s.proxyDeny(w, err)
			return nil, false
		}
		grants, err := s.listGrants(ctx, art.GetId())
		if err != nil {
			http.Error(w, "permission lookup error", http.StatusInternalServerError)
			return nil, false
		}
		if !canRead(principal, art, grants, viewerID) {
			// Collapse miss-or-no-access into 404 (artifacts.md §3).
			http.NotFound(w, r)
			return nil, false
		}
	}
	return art, true
}

func (s *Service) serveAppView(w http.ResponseWriter, r *http.Request, artifactID string) {
	ctx := r.Context()
	art, ok := s.loadAndAuthorize(w, r, artifactID)
	if !ok {
		return
	}
	artType := art.GetType()
	if artType != nsv1.ArtifactType_ARTIFACT_TYPE_APP &&
		artType != nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT {
		http.Error(w, "unsupported artifact type", http.StatusBadRequest)
		return
	}

	// OBJECT artifacts: 302 to a short-lived presigned download URL.
	// Browsers / iframes follow the redirect and fetch bytes directly
	// from the object store. Matches cr0n's `/view` behavior for
	// non-app artifacts (cr0n streams through; we redirect — same
	// observable result for the UI but cheaper for the API).
	if artType == nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT {
		bucket := art.GetObjectBucket()
		key := art.GetObjectKey()
		if bucket == "" || key == "" {
			http.Error(w, "object has no storage location", http.StatusServiceUnavailable)
			return
		}
		dlURL, _, err := s.objects.DownloadURL(ctx, bucket, key, s.appDownloadTTL)
		if err != nil {
			s.logger.Warn("proxy: object download url", "id", artifactID, "err", err)
			http.Error(w, "download url error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, dlURL, http.StatusFound)
		return
	}

	upstream := art.GetAppUrl()
	if upstream == "" {
		http.Error(w, "app has no service url", http.StatusServiceUnavailable)
		return
	}
	if strings.HasPrefix(upstream, runtime.StubAppServicePrefix) {
		http.Error(w, "stub-runtime app: no backend to proxy to", http.StatusServiceUnavailable)
		return
	}

	target, err := url.Parse(upstream)
	if err != nil {
		s.logger.Warn("proxy: parse upstream", "upstream", upstream, "err", err)
		http.Error(w, "invalid app url", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Rewrite the request path so the upstream nginx sees the root,
	// not /v1/artifacts/{id}/view. Match cr0n's view route shape.
	pathPrefix := "/v1/artifacts/" + artifactID + "/view"
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, pathPrefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		// The upstream is a stateless static-file server; don't forward
		// the user's bearer.
		req.Header.Del("Authorization")
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		s.logger.Warn("proxy: upstream error", "upstream", upstream, "err", err)
		http.Error(rw, "upstream unreachable", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

// serveDownloadURL handles `GET /v1/artifacts/{id}:downloadUrl` with
// the shared /view policy (#165). The grpc-gateway request would
// otherwise 401 for anonymous viewers because its auth interceptor
// runs before art.Public is consulted.
func (s *Service) serveDownloadURL(w http.ResponseWriter, r *http.Request, artifactID string) {
	ctx := r.Context()
	art, ok := s.loadAndAuthorize(w, r, artifactID)
	if !ok {
		return
	}
	if art.GetType() != nsv1.ArtifactType_ARTIFACT_TYPE_OBJECT {
		// Mirror service.go:GetArtifactDownloadURL FailedPrecondition;
		// grpc-gateway maps that to 400.
		http.Error(w, "download_url only valid on object artifacts", http.StatusBadRequest)
		return
	}
	dlURL, exp, err := s.objects.DownloadURL(ctx, art.GetObjectBucket(), art.GetObjectKey(), s.downloadTTL)
	if err != nil {
		s.logger.Warn("proxy: download url", "id", artifactID, "err", err)
		http.Error(w, "download url error", http.StatusInternalServerError)
		return
	}
	writeProtoJSON(w, &nsv1.GetArtifactDownloadURLResponse{
		DownloadUrl: dlURL,
		ExpiresAt:   timestamppb.New(exp),
	})
}

// servePreviewURL handles `GET /v1/artifacts/{id}:previewUrl`.
func (s *Service) servePreviewURL(w http.ResponseWriter, r *http.Request, artifactID string) {
	ctx := r.Context()
	art, ok := s.loadAndAuthorize(w, r, artifactID)
	if !ok {
		return
	}
	if !art.GetHasPreview() {
		http.Error(w, "no preview", http.StatusNotFound)
		return
	}
	pvURL, exp, err := s.objects.DownloadURL(ctx, art.GetObjectBucket(), previewKey(art.GetId()), s.downloadTTL)
	if err != nil {
		s.logger.Warn("proxy: preview url", "id", artifactID, "err", err)
		http.Error(w, "preview url error", http.StatusInternalServerError)
		return
	}
	writeProtoJSON(w, &nsv1.GetArtifactPreviewURLResponse{
		PreviewUrl: pvURL,
		ExpiresAt:  timestamppb.New(exp),
	})
}

// writeProtoJSON serializes a proto with the same canonical-camelCase
// shape grpc-gateway emits, so UI callers see identical bodies whether
// the request hit the verb intercept (#165 path) or the gateway.
func writeProtoJSON(w http.ResponseWriter, msg proto.Message) {
	body, err := protojson.Marshal(msg)
	if err != nil {
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// authenticateProxy extracts the Bearer header, verifies via
// verifiers.Set, and returns (principal, viewerID, err). For
// SchemeWorker callers, viewerID is resolved via RunLookup so the
// chunk-15 worker-credential flow works through the proxy too.
func (s *Service) authenticateProxy(ctx context.Context, r *http.Request) (*verifiers.Principal, string, error) {
	if s.verifiers == nil {
		return nil, "", proxyAuthErr{code: http.StatusUnauthorized, msg: "auth not configured"}
	}
	tok := bearerFromHeader(r.Header.Get("Authorization"))
	if tok == "" {
		return nil, "", proxyAuthErr{code: http.StatusUnauthorized, msg: "missing bearer"}
	}
	p, err := verifiers.VerifyBearer(ctx, tok, []verifiers.Scheme{verifiers.SchemeUser, verifiers.SchemeService, verifiers.SchemeWorker}, s.verifiers)
	if err != nil {
		return nil, "", proxyAuthErr{code: http.StatusUnauthorized, msg: "unauthenticated"}
	}
	viewerID := p.ID
	if p.Scheme == verifiers.SchemeWorker && s.runs != nil {
		owner, _, err := s.runs.LookupRunOwner(ctx, p.RunID)
		if err == nil {
			viewerID = owner
		}
	}
	return p, viewerID, nil
}

func (s *Service) proxyDeny(w http.ResponseWriter, err error) {
	if pe, ok := err.(proxyAuthErr); ok {
		http.Error(w, pe.msg, pe.code)
		return
	}
	http.Error(w, "forbidden", http.StatusForbidden)
}

type proxyAuthErr struct {
	code int
	msg  string
}

func (e proxyAuthErr) Error() string { return e.msg }

// parseArtifactPath splits "/v1/artifacts/{id}/{suffix}" or
// "/v1/artifacts/{id}:{verb}" into id + suffix. For verb form the
// suffix is returned with the leading ":" so callers can disambiguate.
// Returns ok=false if the path doesn't match either shape.
func parseArtifactPath(p string) (id, suffix string, ok bool) {
	rest := strings.TrimPrefix(p, "/v1/artifacts/")
	if rest == p {
		return "", "", false
	}
	// Verb form: /v1/artifacts/{id}:verb (no '/' after id).
	if i := strings.IndexByte(rest, ':'); i > 0 && !strings.Contains(rest[:i], "/") {
		verb := rest[i:]
		if len(verb) <= 1 {
			return "", "", false
		}
		return rest[:i], verb, true
	}
	// Path form: /v1/artifacts/{id}/{suffix}.
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func bearerFromHeader(h string) string {
	const prefix = "bearer "
	if len(h) < len(prefix) {
		return ""
	}
	if !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
