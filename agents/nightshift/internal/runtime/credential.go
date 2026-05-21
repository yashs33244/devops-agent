package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Worker credential wire format:
//
//	v1.<runID>.<expUnix>.<hex(HMAC-SHA256(secret, "v1."+runID+"."+expUnix))>
//
// Sent by the worker as `Authorization: Bearer <credential>`. Verified
// by internal/verifiers.WorkerVerifier at the gRPC interceptor, with a
// per-handler verifiers.RequireWorkerRunID check ensuring the encoded
// run_id matches the request's run_id.

var (
	ErrCredentialInvalid = errors.New("worker credential: invalid")
	ErrCredentialExpired = errors.New("worker credential: expired")
)

const credentialVersion = "v1"

// MintCredential produces a credential for runID that expires at exp.
func MintCredential(secret []byte, runID string, exp time.Time) string {
	expUnix := exp.UTC().Unix()
	payload := credentialVersion + "." + runID + "." + strconv.FormatInt(expUnix, 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// VerifyCredential checks the credential's signature and expiry.
// Returns the runID on success.
func VerifyCredential(secret []byte, cred string, now time.Time) (string, error) {
	parts := strings.Split(cred, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("%w: expected 4 segments, got %d", ErrCredentialInvalid, len(parts))
	}
	version, runID, expStr, sig := parts[0], parts[1], parts[2], parts[3]
	if version != credentialVersion {
		return "", fmt.Errorf("%w: unknown version %q", ErrCredentialInvalid, version)
	}
	if runID == "" {
		return "", fmt.Errorf("%w: empty runID", ErrCredentialInvalid)
	}
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("%w: exp parse: %v", ErrCredentialInvalid, err)
	}
	if now.UTC().Unix() > expUnix {
		return "", ErrCredentialExpired
	}

	// Recompute expected sig.
	payload := version + "." + runID + "." + expStr
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return "", fmt.Errorf("%w: signature mismatch", ErrCredentialInvalid)
	}
	return runID, nil
}
