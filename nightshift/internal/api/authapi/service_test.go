package authapi

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
	"github.com/nightshiftco/nightshift/internal/oauth"
	"github.com/nightshiftco/nightshift/internal/verifiers"
)

// fakeDispenser mirrors the one in config tests but lives here so the
// authapi package can be tested in isolation.
type fakeDispenser struct {
	servers map[string]oauth.OAuthServerConfig
	creds   map[string]string
}

func newFakeDispenser() *fakeDispenser {
	return &fakeDispenser{
		servers: map[string]oauth.OAuthServerConfig{},
		creds:   map[string]string{},
	}
}

func (f *fakeDispenser) RegisterOAuthServer(_ context.Context, name string, cfg oauth.OAuthServerConfig) error {
	f.servers[name] = cfg
	return nil
}
func (f *fakeDispenser) GetAuthorizeURL(_ context.Context, server, _, _ string, _ []string) (string, error) {
	return "https://stub.test/authorize?server=" + server, nil
}
func (f *fakeDispenser) ExchangeOAuthCode(_ context.Context, credName, _, _, _, _ string) error {
	f.creds[credName] = "stub-token-" + credName
	return nil
}
func (f *fakeDispenser) GetOAuthToken(_ context.Context, credName string) (string, error) {
	tok, ok := f.creds[credName]
	if !ok {
		return "", oauth.ErrNotFound
	}
	return tok, nil
}
func (f *fakeDispenser) DeleteOAuthCredential(_ context.Context, credName string) error {
	delete(f.creds, credName)
	return nil
}

func ctxAs(scheme verifiers.Scheme, id string, groups ...string) context.Context {
	return verifiers.WithPrincipal(context.Background(), &verifiers.Principal{
		Scheme: scheme, ID: id, Groups: groups,
	})
}

func mustCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("code=%v want=%v err=%v", status.Code(err), want, err)
	}
}

func newSvcWithOAuth(disp oauth.OAuthDispenser) *Service {
	return &Service{
		OAuth:           disp,
		StateSigningKey: []byte("test-state-signing-key-must-be-16+-bytes"),
		AdminGroup:      "admin",
		AdminTokens:     map[string]bool{"cli-admin": true},
	}
}

func TestAuthAPI_DisabledWhenDispenserNil(t *testing.T) {
	s := newSvcWithOAuth(nil)
	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	_, err := s.RegisterOAuthProvider(admin, &nsv1.RegisterOAuthProviderRequest{
		Name: "github", ProviderType: "github", ClientId: "id", ClientSecret: "sec",
	})
	mustCode(t, err, codes.Unimplemented)
	_, err = s.GetOAuthAuthorizeURL(admin, &nsv1.GetOAuthAuthorizeURLRequest{
		ProviderName: "github", UserId: "alice", RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.Unimplemented)
}

func TestAuthAPI_RegisterOAuthProvider_AdminGate(t *testing.T) {
	disp := newFakeDispenser()
	s := newSvcWithOAuth(disp)

	user := ctxAs(verifiers.SchemeUser, "alice")
	_, err := s.RegisterOAuthProvider(user, &nsv1.RegisterOAuthProviderRequest{
		Name: "github", ProviderType: "github", ClientId: "id", ClientSecret: "sec",
	})
	mustCode(t, err, codes.PermissionDenied)

	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	if _, err := s.RegisterOAuthProvider(admin, &nsv1.RegisterOAuthProviderRequest{
		Name: "github", ProviderType: "github", ClientId: "id", ClientSecret: "sec",
	}); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if cfg := disp.servers["github"]; cfg.ClientID != "id" {
		t.Fatalf("server not registered: %v", disp.servers)
	}

	cli := ctxAs(verifiers.SchemeService, "cli-admin")
	if _, err := s.RegisterOAuthProvider(cli, &nsv1.RegisterOAuthProviderRequest{
		Name: "linear", ProviderType: "linear", ClientId: "id", ClientSecret: "sec",
	}); err != nil {
		t.Fatalf("cli-admin: %v", err)
	}
}

func TestAuthAPI_AuthorizeAndExchange_OwnerTier(t *testing.T) {
	disp := newFakeDispenser()
	s := newSvcWithOAuth(disp)

	alice := ctxAs(verifiers.SchemeUser, "alice")
	startResp, err := s.GetOAuthAuthorizeURL(alice, &nsv1.GetOAuthAuthorizeURLRequest{
		ProviderName: "github", UserId: "alice", RedirectUrl: "http://app/cb",
	})
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if _, err := s.ExchangeOAuthCode(alice, &nsv1.ExchangeOAuthCodeRequest{
		ProviderName: "github", UserId: "alice",
		Code: "ac", State: startResp.GetState(), RedirectUrl: "http://app/cb",
	}); err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if disp.creds["alice-github"] == "" {
		t.Fatalf("expected stored credential")
	}

	bob := ctxAs(verifiers.SchemeUser, "bob")
	_, err = s.GetOAuthAuthorizeURL(bob, &nsv1.GetOAuthAuthorizeURLRequest{
		ProviderName: "github", UserId: "alice", RedirectUrl: "http://app/cb",
	})
	mustCode(t, err, codes.PermissionDenied)
}

func TestAuthAPI_GetOAuthToken_AdminOnly(t *testing.T) {
	disp := newFakeDispenser()
	disp.creds["alice-github"] = "ghp_xxx"
	s := newSvcWithOAuth(disp)

	alice := ctxAs(verifiers.SchemeUser, "alice")
	_, err := s.GetOAuthToken(alice, &nsv1.GetOAuthTokenRequest{
		ProviderName: "github", UserId: "alice",
	})
	mustCode(t, err, codes.PermissionDenied)

	admin := ctxAs(verifiers.SchemeUser, "boss", "admin")
	resp, err := s.GetOAuthToken(admin, &nsv1.GetOAuthTokenRequest{
		ProviderName: "github", UserId: "alice",
	})
	if err != nil {
		t.Fatalf("admin get: %v", err)
	}
	if resp.GetToken().GetAccessToken() != "ghp_xxx" {
		t.Fatalf("token=%q", resp.GetToken().GetAccessToken())
	}

	_, err = s.GetOAuthToken(admin, &nsv1.GetOAuthTokenRequest{
		ProviderName: "github", UserId: "bob",
	})
	if !errors.Is(err, status.Error(codes.NotFound, "")) && status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestAuthAPI_DeleteOAuthCredential_OwnerTier(t *testing.T) {
	disp := newFakeDispenser()
	disp.creds["alice-github"] = "x"
	s := newSvcWithOAuth(disp)

	bob := ctxAs(verifiers.SchemeUser, "bob")
	_, err := s.DeleteOAuthCredential(bob, &nsv1.DeleteOAuthCredentialRequest{
		ProviderName: "github", UserId: "alice",
	})
	mustCode(t, err, codes.PermissionDenied)

	alice := ctxAs(verifiers.SchemeUser, "alice")
	if _, err := s.DeleteOAuthCredential(alice, &nsv1.DeleteOAuthCredentialRequest{
		ProviderName: "github", UserId: "alice",
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := disp.creds["alice-github"]; ok {
		t.Fatalf("credential still present")
	}
}
