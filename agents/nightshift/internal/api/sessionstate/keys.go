package sessionstate

import (
	"errors"
	"strings"

	"github.com/nightshiftco/nightshift/internal/runtime"
)

// Run record attribute keys. Mirror internal/api/workers/run_state.go;
// stable wire-format strings, redeclared so this package doesn't
// import the workers service.
const (
	attrUserID    = "user_id"
	attrSessionID = "session_id"
)

// errBadKey is returned when a relative key fails sanitization. The
// HTTP layer maps it to 400.
var errBadKey = errors.New("session-state: invalid key")

// sanitizeKey validates a relative path supplied by the worker for
// download/upload. The returned string is safe to append to the
// run-scoped bucket prefix.
//
// Rules:
//   - empty -> reject
//   - leading "/", any "\", any NUL -> reject
//   - any segment of "." or ".." -> reject
//   - repeated "//" -> reject (keeps bucket layout predictable)
//   - allowed runes: ASCII alnum, '.', '_', '-', '/'
//
// Defense in depth: the final bucket key is composed by the server
// using runtime.SessionObjectPrefix, so even a forged key cannot
// escape the run-scoped prefix.
func sanitizeKey(rel string) (string, error) {
	if rel == "" {
		return "", errBadKey
	}
	if strings.HasPrefix(rel, "/") {
		return "", errBadKey
	}
	if strings.ContainsAny(rel, "\\\x00") {
		return "", errBadKey
	}
	if strings.Contains(rel, "//") {
		return "", errBadKey
	}
	for _, r := range rel {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-', r == '/':
			// allowed
		default:
			return "", errBadKey
		}
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", errBadKey
		}
	}
	return rel, nil
}

// objectKey composes the full bucket key for a sanitized relative
// path under <userID>/<sessionID>. Returns error if either id is
// empty or the relative path fails sanitization.
func objectKey(userID, sessionID, rel string) (string, error) {
	clean, err := sanitizeKey(rel)
	if err != nil {
		return "", err
	}
	prefix, err := runtime.SessionObjectPrefix(userID, sessionID)
	if err != nil {
		return "", err
	}
	return prefix + clean, nil
}
