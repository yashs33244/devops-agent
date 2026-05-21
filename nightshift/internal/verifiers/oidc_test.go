package verifiers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeIDP stands up a minimal OIDC provider: an OIDC-discovery doc,
// a JWKS endpoint, and a helper to mint signed id_tokens. Good enough
// for go-oidc's provider discovery + verification path.
type fakeIDP struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	kid    string
	issuer string
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	idp := &fakeIDP{srv: srv, key: key, kid: "test-key-1", issuer: srv.URL}

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                idp.issuer,
			"jwks_uri":                              idp.issuer + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"authorization_endpoint":                idp.issuer + "/auth",
			"token_endpoint":                        idp.issuer + "/token",
			"response_types_supported":              []string{"id_token"},
			"subject_types_supported":               []string{"public"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &key.PublicKey,
			KeyID:     idp.kid,
			Algorithm: "RS256",
			Use:       "sig",
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	return idp
}

type idClaims struct {
	jwt.Claims
	Email string `json:"email,omitempty"`
}

// mint signs an id_token with the IdP's key.
func (idp *fakeIDP) mint(t *testing.T, c idClaims) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: idp.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", idp.kid),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(c).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newOIDCVerifierForCompliance(t *testing.T) (Verifier, string) {
	t.Helper()
	idp := newFakeIDP(t)
	v, err := NewOIDCVerifier(context.Background(), idp.issuer, "nightshift-api")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tok := idp.mint(t, idClaims{Claims: jwt.Claims{
		Issuer:   idp.issuer,
		Subject:  "user-compliance",
		Audience: jwt.Audience{"nightshift-api"},
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
		IssuedAt: jwt.NewNumericDate(now),
	}})
	return v, tok
}

func TestOIDCCompliance(t *testing.T) {
	runVerifierComplianceSuite(t, newOIDCVerifierForCompliance)
}

// OIDC-specific edge cases below — token-shape-specific failure
// modes (wrong audience, expired, bad signature) that the generic
// compliance suite's "garbage token" doesn't exercise.

func TestOIDCVerifier_WrongAudience(t *testing.T) {
	idp := newFakeIDP(t)
	v, err := NewOIDCVerifier(context.Background(), idp.issuer, "nightshift-api")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tok := idp.mint(t, idClaims{Claims: jwt.Claims{
		Issuer:   idp.issuer,
		Subject:  "user-42",
		Audience: jwt.Audience{"some-other-aud"},
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
		IssuedAt: jwt.NewNumericDate(now),
	}})
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCVerifier_Expired(t *testing.T) {
	idp := newFakeIDP(t)
	v, err := NewOIDCVerifier(context.Background(), idp.issuer, "nightshift-api")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tok := idp.mint(t, idClaims{Claims: jwt.Claims{
		Issuer:   idp.issuer,
		Subject:  "user-42",
		Audience: jwt.Audience{"nightshift-api"},
		Expiry:   jwt.NewNumericDate(now.Add(-time.Minute)),
		IssuedAt: jwt.NewNumericDate(now.Add(-10 * time.Minute)),
	}})
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCVerifier_BadSignature(t *testing.T) {
	idp := newFakeIDP(t)
	v, err := NewOIDCVerifier(context.Background(), idp.issuer, "nightshift-api")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: other},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "bogus"),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(idClaims{Claims: jwt.Claims{
		Issuer:   idp.issuer,
		Subject:  "user-42",
		Audience: jwt.Audience{"nightshift-api"},
		Expiry:   jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(context.Background(), raw); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCVerifier_DisabledWhenIssuerEmpty(t *testing.T) {
	v, err := NewOIDCVerifier(context.Background(), "", "")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if v != nil {
		t.Fatalf("expected nil verifier when issuer empty")
	}
	if _, err := v.Verify(context.Background(), "anything"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCVerifier_AudienceRequired(t *testing.T) {
	_, err := NewOIDCVerifier(context.Background(), "https://example.test", "")
	if err == nil {
		t.Fatal("expected error when issuer set but audience blank")
	}
}

type idClaimsWithGroups struct {
	jwt.Claims
	Groups []string `json:"groups,omitempty"`
}

func (idp *fakeIDP) mintGroups(t *testing.T, c idClaimsWithGroups) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: idp.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", idp.kid),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(c).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOIDCVerifier_GroupsClaim(t *testing.T) {
	idp := newFakeIDP(t)
	v, err := NewOIDCVerifier(context.Background(), idp.issuer, "nightshift-api")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tok := idp.mintGroups(t, idClaimsWithGroups{
		Claims: jwt.Claims{
			Issuer:   idp.issuer,
			Subject:  "user-42",
			Audience: jwt.Audience{"nightshift-api"},
			Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt: jwt.NewNumericDate(now),
		},
		Groups: []string{"admin", "user"},
	})
	p, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if p.ID != "user-42" {
		t.Fatalf("sub=%q", p.ID)
	}
	if len(p.Groups) != 2 || p.Groups[0] != "admin" || p.Groups[1] != "user" {
		t.Fatalf("groups=%v", p.Groups)
	}
}
