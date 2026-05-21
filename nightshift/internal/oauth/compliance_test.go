package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// newOAuthDispenserFunc returns a fresh, isolated OAuthDispenser for
// one compliance subtest. Implementations must register cleanup via
// t.Cleanup.
type newOAuthDispenserFunc func(t *testing.T) OAuthDispenser

// runOAuthDispenserComplianceSuite runs the full OAuthDispenser
// contract against the dispenser produced by newDispenser. Drivers
// (openbao_test.go, native_test.go, future Vault) invoke this from a
// single TestXxxCompliance entry point so adding a backend cannot
// bypass the contract.
//
// PKCE-specific wire-shape assertions (verifier in token POST,
// code_challenge in URL) live in backend-specific tests; the universal
// "Authorize → Exchange → Get → Delete" round-trip plus refresh-on-
// expiry MUST work for every backend. Drivers whose upstream needs
// authorize-first state stashing (PKCE) are exercised correctly by the
// suite because every Exchange/Delete path issues a GetAuthorizeURL
// call first.
func runOAuthDispenserComplianceSuite(t *testing.T, newDispenser newOAuthDispenserFunc) {
	t.Helper()

	t.Run("RegisterServer", func(t *testing.T) {
		d := newDispenser(t)
		if err := d.RegisterOAuthServer(context.Background(), "github", OAuthServerConfig{
			Provider:     "github",
			ClientID:     "id",
			ClientSecret: "sec",
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
	})

	t.Run("AuthorizeURLContainsState", func(t *testing.T) {
		d := newDispenser(t)
		ctx := context.Background()
		if err := d.RegisterOAuthServer(ctx, "github", OAuthServerConfig{
			Provider: "github", ClientID: "id", ClientSecret: "sec",
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
		url, err := d.GetAuthorizeURL(ctx, "github", "state-xyz", "https://app/cb", []string{"repo"})
		if err != nil {
			t.Fatalf("auth url: %v", err)
		}
		if !strings.Contains(url, "state-xyz") {
			t.Fatalf("state token not preserved in URL: %q", url)
		}
	})

	t.Run("GetNotFoundForUnprovisioned", func(t *testing.T) {
		d := newDispenser(t)
		if _, err := d.GetOAuthToken(context.Background(), "alice-github"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("AuthorizeExchangeThenGet", func(t *testing.T) {
		d := newDispenser(t)
		ctx := context.Background()
		if err := d.RegisterOAuthServer(ctx, "github", OAuthServerConfig{
			Provider: "github", ClientID: "id", ClientSecret: "sec",
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
		const state = "state-xyz"
		if _, err := d.GetAuthorizeURL(ctx, "github", state, "https://app/cb", []string{"repo"}); err != nil {
			t.Fatalf("authorize: %v", err)
		}
		const cred = "alice-github"
		if err := d.ExchangeOAuthCode(ctx, cred, "github", "code-X", "https://app/cb", state); err != nil {
			t.Fatalf("exchange: %v", err)
		}
		tok, err := d.GetOAuthToken(ctx, cred)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if tok == "" {
			t.Fatal("empty access token after exchange")
		}
	})

	t.Run("DeleteIdempotent", func(t *testing.T) {
		d := newDispenser(t)
		// Delete on a credential that was never provisioned is a no-op.
		if err := d.DeleteOAuthCredential(context.Background(), "never-existed"); err != nil {
			t.Fatalf("delete missing: %v", err)
		}
	})

	t.Run("DeleteThenGetNotFound", func(t *testing.T) {
		d := newDispenser(t)
		ctx := context.Background()
		if err := d.RegisterOAuthServer(ctx, "github", OAuthServerConfig{
			Provider: "github", ClientID: "id", ClientSecret: "sec",
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
		const state = "state-xyz"
		if _, err := d.GetAuthorizeURL(ctx, "github", state, "https://app/cb", nil); err != nil {
			t.Fatalf("authorize: %v", err)
		}
		const cred = "alice-github"
		if err := d.ExchangeOAuthCode(ctx, cred, "github", "code-X", "https://app/cb", state); err != nil {
			t.Fatalf("exchange: %v", err)
		}
		if err := d.DeleteOAuthCredential(ctx, cred); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := d.GetOAuthToken(ctx, cred); !errors.Is(err, ErrNotFound) {
			t.Fatalf("after delete: want ErrNotFound, got %v", err)
		}
	})

	// GetRefreshesOnExpiry pins the refresh contract: GetOAuthToken
	// MUST transparently refresh when the cached access_token is past
	// (or near) expiry. Observable through the interface alone — the
	// driver is responsible for configuring its upstream so that
	// consecutive Gets resolve to different access_tokens. For Native,
	// that means the fake provider issues short-lived (`expires_in: 1`)
	// sequential tokens; Native's 60s pre-expiry skew window forces a
	// refresh on every Get.
	t.Run("GetRefreshesOnExpiry", func(t *testing.T) {
		d := newDispenser(t)
		ctx := context.Background()
		if err := d.RegisterOAuthServer(ctx, "github", OAuthServerConfig{
			Provider: "github", ClientID: "id", ClientSecret: "sec",
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
		const state = "state-refresh"
		if _, err := d.GetAuthorizeURL(ctx, "github", state, "https://app/cb", nil); err != nil {
			t.Fatalf("authorize: %v", err)
		}
		const cred = "alice-github"
		if err := d.ExchangeOAuthCode(ctx, cred, "github", "code-X", "https://app/cb", state); err != nil {
			t.Fatalf("exchange: %v", err)
		}
		first, err := d.GetOAuthToken(ctx, cred)
		if err != nil {
			t.Fatalf("first get: %v", err)
		}
		second, err := d.GetOAuthToken(ctx, cred)
		if err != nil {
			t.Fatalf("second get: %v", err)
		}
		if first == second {
			t.Fatalf("GetOAuthToken returned %q twice — refresh did not fire on expiry", first)
		}
	})
}
