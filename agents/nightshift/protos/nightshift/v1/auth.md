# Nightshift Auth — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Auth` service. The proto file at
[`auth.proto`](auth.proto) is the authoritative wire contract; this
document describes what a conforming implementation must do and why.

## 1. Overview

Auth is the vendor-neutral **multi-method authentication plane** of
Nightshift. Each RPC group implements one auth method; today only one
method ships — the OAuth 2.0 dispenser for per-user MCP connector
credentials. Future methods (SAML, LDAP, custom token exchange, other
non-OIDC IdP flows) will land here as additional RPC groups.

Auth is split out from `Secrets` (the KV + workload-auth plane)
because operators may legitimately pick different backends for the
two concerns — for example, HashiCorp Vault for encrypted KV
alongside OpenBao's `oauthapp` plugin (or a future provider) for
per-user OAuth credential storage and refresh.

What is **not** in this interface:

- **Inbound bearer-token verification.** The surface that
  authenticates HTTP/gRPC requests *into* nightshift-api is a
  reference-implementation concern handled in `internal/verifiers/`
  (the `Verifier` interface, plus `OIDCVerifier`, `StaticVerifier`,
  `WorkerVerifier`). It is not a vendor-neutral contract — backends
  do not implement it, and it does not appear in the proto.
- **OIDC identity provider.** Issuing id_tokens for human users — the
  second role OpenBao plays in the reference deployment — is a
  reference-implementation concern. A conforming Auth implementation
  need not serve as an OIDC provider.

## 2. OAuth Token Dispenser

The dispenser implements the OAuth 2.0 authorization-code flow with
refresh and per-user storage. It is the read-side counterpart to
Config's connector-management RPCs.

### Flow

1. **Admin registration.** An operator calls
   `RegisterOAuthProvider(name, provider_type, client_id,
   client_secret)` (or pre-seeds the client_id/client_secret into KV
   under `secret/<org>/connectors/<name>` and lets the
   implementation's startup reconciler pick them up). `provider_type`
   is a backend-interpreted string (`"github"`, `"linear"`, …) the
   implementation's OAuth plugin uses to resolve authorize / token
   URLs.
2. **User authorize.** The user-facing frontend calls
   `GetOAuthAuthorizeURL(provider_name, user_id, redirect_url,
   scopes)` and redirects the user's browser to the returned
   `authorize_url`.
3. **User approval.** The upstream provider redirects back to
   `redirect_url` with `code` and the `state` token.
4. **Code exchange.** The HTTP callback handler calls
   `ExchangeOAuthCode(provider_name, user_id, code, state,
   redirect_url)`. The implementation MUST verify `state` matches the
   `state` issued in step 2 (CSRF defense). On success, the per-user
   grant (access token + refresh token + expiry) is stored under the
   dispenser's per-user convention.
5. **Dispense.** `GetOAuthToken(provider_name, user_id)` returns a
   fresh access token. The implementation MUST transparently refresh
   the underlying grant if the access token is near expiry, and MUST
   NOT surface refresh-token values to callers at any point.
6. **Disconnect.** `DeleteOAuthCredential(provider_name, user_id)`
   removes the per-user grant. The grant is NOT revoked upstream at
   the provider; users must revoke through the provider's UI to fully
   disconnect.

### Not-connected semantics

`GetOAuthToken` fails with `NOT_FOUND` when no grant exists for the
requested (provider, user) pair. This is the signal Config's
`GetUserConfig` uses to decide whether to include the connector in
the user's `mcp_servers` (§8 of `config.md`).

### Refresh semantics

The dispenser MUST refresh on read when the access token is expired
or within a provider-configured skew window before expiry. Callers
MUST NOT be required to handle refresh themselves; the simplicity of
the dispenser surface is its main value.

If a refresh fails (for example because the user revoked the grant
upstream), the dispenser MUST:

- Delete the stale grant from its per-user store.
- Return `FAILED_PRECONDITION` from the `GetOAuthToken` call with a
  message indicating the grant is no longer usable.

The user re-authorizes by going through steps 2–4 again.

## 3. Future auth methods

When non-OIDC IdP flows or other auth methods are added, each gets
its own section here, its own RPC group in `auth.proto` (with a verb
prefix that names the method — e.g. `RegisterSAMLProvider`,
`BeginLDAPBind`), and its own method-specific request/response
messages. New methods MUST NOT reuse OAuth-tagged messages or reuse
the `OAuthToken` entity.

## 4. Out of Scope for v1

- Token introspection (RFC 7662) for arbitrary upstream tokens — the
  dispenser is the read path for tokens *it issued / persisted*, not a
  generic OAuth introspection endpoint.
- OAuth 2.0 device-code flow — the authorization-code flow covers
  every connector type Nightshift integrates today.
- Cross-IdP federation (e.g. exchanging a SAML assertion for an OAuth
  token). If federation surfaces become useful, they will be a new
  RPC group in §3 rather than overloading the OAuth dispenser.
