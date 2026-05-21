package sessionstate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nightshiftco/nightshift/internal/objects"
	"github.com/nightshiftco/nightshift/internal/runtime"
)

// pathPrefix is the URL prefix this service intercepts. Anything else
// falls through to the configured fallback handler.
const pathPrefix = "/v1/internal/runs/"

// suffixManifest, suffixObjects are the per-run subpaths.
const (
	suffixManifest = "session-state/manifest"
	suffixObjects  = "session-state/objects/"
)

// parsePath splits "/v1/internal/runs/{run_id}/session-state/..."
// into the run_id and the remaining suffix. Returns ok=false if the
// shape doesn't match.
func parsePath(p string) (runID, suffix string, ok bool) {
	rest := strings.TrimPrefix(p, pathPrefix)
	if rest == p {
		return "", "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	if !strings.HasPrefix(parts[1], "session-state/") {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ManifestEntry is one row in the manifest response.
type ManifestEntry struct {
	Key   string `json:"key"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime"`
}

// ManifestResponse is returned by GET …/manifest.
type ManifestResponse struct {
	Entries []ManifestEntry `json:"entries"`
}

// dispatch routes a verified path to the correct handler. Auth is
// performed inside each handler so 401/403 status mapping is local.
func (s *Service) dispatch(w http.ResponseWriter, r *http.Request, runID, suffix string) {
	ctx := r.Context()
	switch {
	case suffix == suffixManifest:
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.serveManifest(ctx, w, r, runID)
	case strings.HasPrefix(suffix, suffixObjects):
		rel := strings.TrimPrefix(suffix, suffixObjects)
		switch r.Method {
		case http.MethodGet:
			s.serveGetObject(ctx, w, r, runID, rel)
		case http.MethodPut:
			s.servePutObject(ctx, w, r, runID, rel)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// serveManifest lists objects under sessions/<u>/<s>/ and returns
// them as ManifestResponse with relative keys (the per-session prefix
// is stripped).
func (s *Service) serveManifest(ctx context.Context, w http.ResponseWriter, r *http.Request, runID string) {
	userID, sessionID, err := s.verifyWorkerForRun(ctx, r, runID)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	prefix, err := runtime.SessionObjectPrefix(userID, sessionID)
	if err != nil {
		s.logger.Warn("session-state: prefix build failed", "run_id", runID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := ManifestResponse{Entries: []ManifestEntry{}}
	pageToken := ""
	for {
		page, next, err := s.objects.List(ctx, s.bucket, prefix, pageToken, 1000)
		if err != nil {
			s.logger.Warn("session-state: list failed", "run_id", runID, "err", err)
			http.Error(w, "list failed", http.StatusInternalServerError)
			return
		}
		for _, obj := range page {
			rel := strings.TrimPrefix(obj.Key, prefix)
			if rel == "" {
				continue
			}
			out.Entries = append(out.Entries, ManifestEntry{
				Key:   rel,
				Size:  obj.SizeBytes,
				Mtime: obj.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// serveGetObject mints a presigned download URL and 302-redirects.
// Worker httpx clients follow the redirect to the object store
// directly — zero-copy on the API side.
func (s *Service) serveGetObject(ctx context.Context, w http.ResponseWriter, r *http.Request, runID, rel string) {
	userID, sessionID, err := s.verifyWorkerForRun(ctx, r, runID)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	key, err := objectKey(userID, sessionID, rel)
	if err != nil {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}
	if _, err := s.objects.Stat(ctx, s.bucket, key); err != nil {
		if errors.Is(err, objects.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.Warn("session-state: stat failed", "run_id", runID, "err", err)
		http.Error(w, "stat failed", http.StatusInternalServerError)
		return
	}
	url, _, err := s.objects.DownloadURL(ctx, s.bucket, key, time.Duration(defaultDownloadTTLSeconds)*time.Second)
	if err != nil {
		s.logger.Warn("session-state: presign failed", "run_id", runID, "err", err)
		http.Error(w, "presign failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// servePutObject reads the request body up to MaxBytes and stores it
// at sessions/<u>/<s>/<sanitizedKey>. Content-Type defaults to
// application/octet-stream; SDK transcripts are JSONL but we don't
// enforce that — workers may write any per-session state.
func (s *Service) servePutObject(ctx context.Context, w http.ResponseWriter, r *http.Request, runID, rel string) {
	userID, sessionID, err := s.verifyWorkerForRun(ctx, r, runID)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	key, err := objectKey(userID, sessionID, rel)
	if err != nil {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}

	limited := http.MaxBytesReader(w, r.Body, s.maxBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		// MaxBytesReader returns *http.MaxBytesError on overflow.
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		s.logger.Warn("session-state: read body", "run_id", runID, "err", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, err := s.objects.PutBytes(ctx, s.bucket, key, contentType, body); err != nil {
		s.logger.Warn("session-state: put failed", "run_id", runID, "err", err)
		http.Error(w, "put failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
