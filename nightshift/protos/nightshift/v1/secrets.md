# Nightshift Secrets — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Secrets` service. The proto file at
[`secrets.proto`](secrets.proto) is the authoritative wire contract;
this document describes what a conforming implementation must do and
why.

## 1. Overview

Secrets is the vendor-neutral **credential plane** of Nightshift. It
is a peer of Storage, not a layer over it, because credentials carry
obligations Storage does not: encryption at rest, redaction from logs
and metrics, and short-lived lease-based issuance.

Two concerns live here:

- **KV** (§2, §3). Opaque `map<string, string>` payloads at paths the
  implementation scopes via ACL. Used for org-level secrets (API
  keys, shared worker secrets) and per-user static bearer tokens.
- **Workload authentication** (§4, §5). Kubernetes ServiceAccount
  auth plus lease lifecycle management — the contract by which
  control-plane and worker pods bootstrap into the Secrets backend.

Per-user OAuth credential management has moved to the separate
[`Auth` service](auth.md); operators may pick different backends for
KV vs. OAuth dispensing.

What is **not** in this interface:

- **OIDC identity provider.** Issuing id_tokens for human users — the
  second role OpenBao plays in the cr0n-a reference — is a
  reference-implementation concern, specified in chunk 8. A
  conforming Secrets implementation need not serve as an OIDC
  provider.
- **Policy definition.** The spec describes what a role maps to
  semantically, but the ACL policy language (which paths each role
  may read / write) is backend-specific.
- **Encryption key management.** Secrets opaquely encrypts at rest;
  how keys are managed, rotated, and sealed is implementation-defined
  (OpenBao's auto-unseal sidecar is one option).

## 2. KV Model

A `Secret` is one versioned KV payload at a `path`. The shape matches
OpenBao's and Vault's KV v2 engine:

- `path` — opaque string. Conforming implementations interpret paths
  hierarchically; see §3 for the recommended conventions.
- `data` — `map<string, string>`. All values are strings; binary
  payloads are encoded (for example base64) by the caller.
- `version` — monotonic counter assigned by the backend on each
  `PutSecret`. Used with `if_version` for optimistic concurrency and
  to select historical versions from `GetSecret`.
- `created_at`, `updated_at` — server-assigned timestamps on the
  first and most recent writes.

## 3. Path Conventions

Paths are opaque to the spec; authorization is enforced by the ACL
policy bound to the caller's role. Conforming implementations SHOULD
adopt the following convention — every reference implementation in
this tree does:

- `secret/<org>/...` — org-level shared secrets. Read-scoped to the
  org's service role (for example `cr0n-a`) and its workers.
- `secret/<org>/connectors/<connector_name>` — OAuth application
  credentials (`client_id`, `client_secret`) pre-seeded by admins
  before users can authorize the connector. See
  [`auth.md`](auth.md) for how the Auth service consumes these.
- `secret/<org>/tokens/<user_id>/<connector_name>` — per-user static
  bearer tokens written via Config's `SetConnectorStaticToken`. The
  `access_token` field under this path is read by Config's
  `GetUserConfig` at run start.

OAuth grant storage — the per-user access + refresh tokens for the
authorization-code flow — is handled by the Auth service's OAuth
dispenser ([`auth.md`](auth.md)) and does NOT live under `secret/`.
Implementations typically back it with a dedicated plugin (for
example OpenBao's `oauthapp`) whose path is outside the KV engine's
path space.

## 4. Workload Authentication

Nightshift standardizes on **Kubernetes ServiceAccount auth** as the
bootstrap path. Every pod that needs to call Secrets RPCs:

1. Mounts a projected ServiceAccount token via the pod spec.
2. Calls `LoginWithKubernetes(role, jwt)` once at startup.
3. Uses the returned `AuthToken.token` as
   `Authorization: Bearer <token>` on all subsequent Secrets RPCs.
4. Renews the token before `expires_at` via `RenewLease`, or calls
   Login again when the lease is no longer renewable.

The `role` parameter names an ACL policy binding in the backend.
Implementations MUST refuse login when the presenting
(namespace, service-account) pair is not bound to the requested role.

Nightshift does NOT prescribe the JWT audience for the projected
token — cluster operators configure projection to match the Secrets
backend's configuration. A typical audience is the Secrets service's
DNS name (`openbao.<namespace>.svc`).

### Why Kubernetes auth as the only spec'd method

Kubernetes auth works on any conforming Kubernetes cluster without
pre-provisioning long-lived shared tokens. It preserves the property
that no application-level secret is present in a pod's environment or
mounted as a Kubernetes Secret — the only credential a pod needs is
the one Kubernetes mints automatically for its ServiceAccount.

Implementations MAY offer additional auth methods (approle, JWT, TLS)
for callers that cannot present a Kubernetes SA JWT; those methods
are out-of-scope for the spec.

## 5. Lease Lifecycle

Every bearer credential Secrets issues — including the AuthToken from
Login — is lease-bound:

- `expires_at` on the AuthToken is the lease TTL. After that instant
  the token is invalid.
- `renewable = true` means the lease may be extended via `RenewLease`
  one or more times, up to the backend's configured maximum TTL.
- `renewable = false` means the caller must re-authenticate (call
  `LoginWithKubernetes` again) for continued access.

`RenewLease` takes an optional `increment_seconds`. The backend caps
the renewal at its policy-configured maximum; the actual new expiry
is returned. Implementations MUST return the effective new expiry
even when it is smaller than requested.

`RevokeLease` immediately invalidates the lease and any tokens bound
to it. Revocation SHOULD propagate to outstanding requests presenting
the revoked token — implementations MAY continue to accept the token
briefly (bounded by the backend's cache TTL), but MUST NOT accept it
indefinitely.

Revoking a lease that does not exist is a no-op.

## 6. ACL Responsibilities

The spec intentionally does not define an ACL policy language.
Instead, it defines the SEMANTIC expectations an implementation must
uphold:

- **Org-level paths** (e.g. `secret/<org>/*`) MUST be readable only
  by the control plane's service role, the worker role, and the
  scheduler role — as configured by the operator.
- **Per-user paths** (e.g. `secret/<org>/tokens/<user_id>/*`) MUST be
  readable only by the control plane's service role; end-user HTTP
  auth (OIDC) is translated into Secrets RPCs *only* through the
  control plane, never directly.
- **Admin writes** (organization-level `PutSecret` under
  `secret/<org>/*`) MUST be restricted to an admin role distinct
  from the service role.

ACL responsibilities for the OAuth dispenser's per-user grant store
are documented in [`auth.md`](auth.md). Implementations that back
both KV and OAuth dispensing on OpenBao MAY use the policy examples
referenced in §3 — `cr0n-a` (full `secret/cr0n/*`),
`cr0n-worker` (narrow read), `<customer>` (namespace-scoped) — as a
starting reference.

## 7. Logging and Masking

Implementations MUST NOT log secret values in cleartext. Specifically:

- `Secret.data` values MUST be redacted in request/response logs.
- `AuthToken.token` MUST be redacted.
- `Secret.path` MAY be logged.

Error messages MUST NOT echo secret values. NOT_FOUND and
PERMISSION_DENIED responses MUST NOT leak which specific path failed
authorization vs. did not exist — implementations SHOULD return
NOT_FOUND for both to avoid enumeration attacks.

## 8. Idempotency

KV writes (`PutSecret`) are naturally idempotent at the path level
through `if_version`.

Lease operations (`RenewLease`, `RevokeLease`) are idempotent at the
resource level.

## 9. Error Code Mapping

| gRPC code | When |
|---|---|
| `NOT_FOUND` | Path does not exist; historical version does not exist; lease does not exist. Also returned for PERMISSION_DENIED conditions per §7 when the implementation cannot distinguish without leaking. |
| `PERMISSION_DENIED` | Caller authenticated but ACL forbids the operation (and the implementation has decided not to collapse this into NOT_FOUND). |
| `UNAUTHENTICATED` | No Bearer token presented or token is invalid / expired. Callers respond by calling `LoginWithKubernetes` and retrying. |
| `INVALID_ARGUMENT` | Malformed path, data payload, etc. |
| `FAILED_PRECONDITION` | `PutSecret` `if_version` mismatch. |
| `ALREADY_EXISTS` | `PutSecret` with `if_version=0` on an existing path. |
| `UNAVAILABLE` | Backend unreachable / sealed; Kubernetes TokenReview API unreachable. |
| `RESOURCE_EXHAUSTED` | Per-path, per-user, or per-role rate / quota exceeded. |

## 10. Versioning

`nightshift.v1.Secrets` is part of the `v1` package. Additive changes
(new RPCs, new optional fields) are permitted in-package. Wire-
breaking changes require a `v2` service in a new proto file.

New auth methods (beyond `LoginWithKubernetes`) will arrive as
additional login RPCs alongside the existing one, not as fields on
it.

## 11. Out of Scope for v1

- OIDC identity-provider capability (issuing id_tokens for human
  users). Deferred to chunk 8 as a reference-implementation concern.
- Dynamic secret engines (on-the-fly database credentials, cloud IAM
  roles). Nightshift's v1 KV plane is static; dynamic engines are a
  reasonable v2 extension.
- Transit / encryption-as-a-service operations (sign, verify, encrypt
  arbitrary data). Out of scope for the v1 credential plane.
- Mesh / envoy-style secret dissemination. The init-container
  pattern cr0n-a uses for customer workloads is an operator recipe,
  not a spec concern.
- Fine-grained audit APIs. Implementations SHOULD emit audit logs
  but the Nightshift spec does not define their shape.
