package sessionstate

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/nightshiftco/nightshift/internal/records"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// httpAuthErr carries a status code + message back to the dispatcher.
// Mirrors the proxyAuthErr pattern in internal/api/artifacts/proxy.go.
type httpAuthErr struct {
	code int
	msg  string
}

func (e httpAuthErr) Error() string { return e.msg }

// verifyWorkerForRun extracts the bearer credential from r, requires
// verifiers.SchemeWorker, and asserts the credential's run_id matches the
// one in the URL. On success returns the (userID, sessionID) read off
// the run record — these are authoritative for bucket prefix scoping.
//
// Error mapping (handled by caller):
//
//	missing/invalid bearer       -> 401
//	non-worker scheme            -> 403
//	scope mismatch (run X vs Y)  -> 403
//	run not found                -> 404
//	user_id/session_id missing   -> 500
func (s *Service) verifyWorkerForRun(ctx context.Context, r *http.Request, runID string) (userID, sessionID string, err error) {
	tok := bearerFromHeader(r.Header.Get("Authorization"))
	if tok == "" {
		return "", "", httpAuthErr{code: http.StatusUnauthorized, msg: "missing bearer"}
	}
	p, err := verifiers.VerifyBearer(ctx, tok, []verifiers.Scheme{verifiers.SchemeWorker}, s.verifiers)
	if err != nil {
		return "", "", httpAuthErr{code: http.StatusUnauthorized, msg: "unauthenticated"}
	}
	if p.Scheme != verifiers.SchemeWorker {
		return "", "", httpAuthErr{code: http.StatusForbidden, msg: "worker credential required"}
	}
	if p.RunID != runID {
		return "", "", httpAuthErr{code: http.StatusForbidden, msg: "credential does not authorize this run"}
	}
	rec, err := s.records.Get(ctx, "runs", runID)
	if err != nil {
		if errors.Is(err, records.ErrNotFound) {
			return "", "", httpAuthErr{code: http.StatusNotFound, msg: "run not found"}
		}
		return "", "", httpAuthErr{code: http.StatusInternalServerError, msg: "records error"}
	}
	userID = rec.Attributes[attrUserID]
	sessionID = rec.Attributes[attrSessionID]
	if userID == "" || sessionID == "" {
		return "", "", httpAuthErr{code: http.StatusInternalServerError, msg: "run missing user/session attributes"}
	}
	return userID, sessionID, nil
}

// bearerFromHeader is a copy of internal/api/artifacts/proxy.go's
// helper. Inlined here to keep this package free of cross-package
// helper imports.
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

// writeAuthErr writes the http response for an httpAuthErr-shaped
// error. Falls back to 500 for any other error type.
func writeAuthErr(w http.ResponseWriter, err error) {
	var ae httpAuthErr
	if errors.As(err, &ae) {
		http.Error(w, ae.msg, ae.code)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}
