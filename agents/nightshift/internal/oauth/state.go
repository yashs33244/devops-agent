package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// StateTTL caps how long an HMAC-signed OAuth state token remains
// valid between authorize and exchange. 10 minutes is long enough for
// a slow consent screen, short enough to bound replay risk.
const StateTTL = 10 * time.Minute

// stateDomain is prepended to the MAC input to decouple OAuth-state
// HMAC from any other HMAC use of the same key (workerHMAC is reused
// for worker-token signing AND state signing in the production wiring).
// Cross-protocol HMAC collisions become impossible: a token signed for
// one purpose cannot be made to verify for another even if the key
// material is shared.
var stateDomain = []byte("oauth-state\x00")

// stateClaims is the payload signed inside an OAuth state token.
// `Resource` is the connector name (Config service) or provider name
// (Auth service) — both bind so a state minted for (alice, github)
// can't be replayed against (bob, github) or (alice, slack).
type stateClaims struct {
	UserID    string `json:"uid"`
	Resource  string `json:"r"`
	Timestamp int64  `json:"t"` // unix seconds
}

// SignState produces an opaque state token of the form
//
//	<base64url(JSON{uid,r,t})>.<hex(HMAC-SHA256(domain || payload, key))>
//
// The full 256-bit MAC is preserved (RFC 2104 / NIST SP 800-107
// recommend ≥128 bits — there's no reason to truncate).
//
// `key` must be ≥16 bytes. `now` is a clock seam; pass time.Now in
// production.
func SignState(key []byte, userID, resource string, now time.Time) (string, error) {
	if len(key) < 16 {
		return "", errors.New("oauth: state signing key not configured (need ≥16 bytes)")
	}
	payload, err := json.Marshal(stateClaims{
		UserID:    userID,
		Resource:  resource,
		Timestamp: now.Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	mac.Write(stateDomain)
	mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyState parses a state token, checks the HMAC, and enforces the
// TTL + binding to (expectedUser, expectedResource). Returns a non-nil
// error on any failure; handlers convert these to gRPC status codes.
func VerifyState(key []byte, token, expectedUser, expectedResource string, now time.Time) error {
	if len(key) < 16 {
		return errors.New("oauth: state signing key not configured")
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return errors.New("oauth: malformed state token")
	}
	encoded, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, key)
	mac.Write(stateDomain)
	mac.Write([]byte(encoded))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return errors.New("oauth: state HMAC mismatch")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("oauth: state base64: %w", err)
	}
	var c stateClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("oauth: state json: %w", err)
	}
	if c.UserID != expectedUser {
		return errors.New("oauth: state user mismatch")
	}
	if c.Resource != expectedResource {
		return errors.New("oauth: state resource mismatch")
	}
	if now.Sub(time.Unix(c.Timestamp, 0)) > StateTTL {
		return errors.New("oauth: state expired")
	}
	return nil
}
