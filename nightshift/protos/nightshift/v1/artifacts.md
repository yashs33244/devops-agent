# Nightshift Artifacts — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Artifacts` service. The proto file at
[`artifacts.proto`](artifacts.proto) is the authoritative wire contract;
this document describes what a conforming implementation must do and
why.

## Overview

Artifacts is a **domain service** built on top of Storage. It
defines two kinds of agent-produced outputs:

- **Object artifacts** which are arbitrary binary blobs (reports, images,
  sheets, presentations, CSV, JSON, …) whose bytes live in a Storage
  `Object`. Object artifacts may optionally carry a companion HTML
  preview, also stored as a Storage `Object`, for inline rendering of
  formats that browsers can't display natively.

- **App artifacts** are self-contained static sites (HTML/CSS/JS)
  deployed by the implementation as a hosted workload. Private apps
  are served through a permission-gated proxy; public apps are served
  at the implementation's public endpoint.

Artifacts persist no state outside Storage. Every `Artifact` is a
Storage `Record` in the `artifacts` collection. For example, an 
`ArtifactPermission` grant is a Storage `Record` in the
`artifact_permissions` collection. An artifact's bytes, and
every companion preview, are Storage `Object`s under an
implementation-defined bucket + key convention (see §8). 

App artifact workloads are managed by the implementation's runtime (for example
Kubernetes Deployments + Services + Ingresses) and are explicitly not
the Storage layer's concern.

A conforming implementation of Storage + Artifacts is sufficient to back
the artifact feature set of a Nightshift control plane.

## Artifact Types

### Object artifacts

- `type = ARTIFACT_TYPE_OBJECT`
- `object_bucket`, `object_key` reference the Storage `Object` holding
  the blob bytes
- `content_type` is the blob's MIME type
- `size_bytes` is the blob's size as reported by Storage after
  finalization
- `has_preview` is true if a companion `preview.html` was stored; see §4
- `app_url`, `app_status`, `app_status_detail` are empty / unspecified

### App artifacts

- `type = ARTIFACT_TYPE_APP`
- `app_url` is the caller-facing URL (permission-gated proxy for
  `public=false`, public ingress for `public=true`)
- `app_status` reflects the hosted workload's lifecycle (§5)
- `object_bucket`, `object_key`, `content_type`, `size_bytes`,
  `has_preview` are empty / zero / false

## Access Model

Artifacts are **private by default**. Access is controlled by:

1. **Ownership.** The creator's user id is stored as `owner_id`.
   Owners have full rights on the artifact (read, update, share,
   delete, toggle public/private).
2. **Permission grants.** An owner may grant another user
   `ARTIFACT_ROLE_VIEWER` (read) or `ARTIFACT_ROLE_EDITOR` (read +
   update) via `ShareArtifact`. `ARTIFACT_ROLE_OWNER` cannot be granted
   through the API in v1, ownership is implicit on the creator and
   immutable. 
3. **The `public` flag.** When true, the artifact's content is
   reachable without authentication. The semantics differ by type:

   | Artifact type | `public = false` (default) | `public = true` |
   |---|---|---|
   | object | GetArtifactDownloadURL requires viewer+ | Any caller may fetch the download URL |
   | app | `app_url` routes through a permission-gated proxy | `app_url` is a public ingress |

Listing (`ListArtifacts`) returns only artifacts the caller owns or has
been granted a role on. Public artifacts are reachable when the caller
knows the id, but are not enumerated implicitly.

### Role matrix

| Operation | viewer | editor | owner |
|---|---|---|---|
| GetArtifact | ✓ | ✓ | ✓ |
| GetArtifactDownloadURL | ✓ | ✓ | ✓ |
| GetArtifactPreviewURL | ✓ | ✓ | ✓ |
| ListArtifactPermissions | ✓ | ✓ | ✓ |
| UpdateArtifact (name, description) | | ✓ | ✓ |
| UpdateArtifact (html_content, content_bytes, preview_html) | | ✓ | ✓ |
| UpdateArtifact (public toggle) | | | ✓ |
| ShareArtifact / RevokeArtifactShare | | | ✓ |
| DeleteArtifact | | | ✓ |

When a caller lacks the required role, implementations SHOULD return
`NOT_FOUND` rather than `PERMISSION_DENIED` to avoid leaking the
existence of private artifacts. `PERMISSION_DENIED` is appropriate only
when the caller is known to hold some access but not enough for the
requested operation (for example, an editor trying to toggle `public`).

## Preview Objects

Object artifacts may ship with a **companion HTML preview**. The common
case is agent-generated reports: a `.docx` or `.xlsx` binary is the
canonical deliverable, but a parallel `preview.html` gives the UI
something it can render inline without asking the user to download.

- On `CreateObjectArtifact`, if `preview_html` is non-empty the
  implementation stores it as a second Storage `Object` under a
  preview key derived from the artifact id (the convention is
  `preview.html` under the same artifact prefix; see §8). The
  `has_preview` field on the returned `Artifact` is set to true.
- `GetArtifactPreviewURL` returns a presigned URL to the preview
  object when `has_preview` is true, and fails with `NOT_FOUND`
  otherwise.
- `UpdateArtifact` accepts a replacement preview via the `preview_html`
  bytes field, gated by `set_preview_html`. When `set_preview_html` is
  true and `preview_html` is empty, the prior preview is deleted and
  `has_preview` flips to false. When `set_preview_html` is false, the
  preview is left untouched.
- `DeleteArtifact` deletes the preview object alongside the binary.

The preview object is intentionally transparent: it is a Storage
`Object` like any other, so implementations that want to index, scan,
or replicate previews do so without a second code path.

## App Deployment Lifecycle

The `app_status` field on an app `Artifact` reflects the state of the
hosted workload:

| State | Meaning |
|---|---|
| `DEPLOYMENT_STATE_PENDING` | The implementation has accepted the deploy and is creating / updating the workload. `app_url` may already be populated but the workload may not yet be reachable. |
| `DEPLOYMENT_STATE_READY` | The workload is reachable at `app_url`. |
| `DEPLOYMENT_STATE_FAILED` | The most recent deploy or update failed. `app_status_detail` carries an implementation-specific reason. |

`CreateAppArtifact` MAY return while `app_status` is still
`DEPLOYMENT_STATE_PENDING`. Callers that need to block on readiness
either:

1. Subscribe to the enclosing run's Workers event stream and wait for
   `ARTIFACT_EVENT_TYPE_APP_DEPLOYED` (recommended for agents during a
   run).
2. Poll `GetArtifact` until `app_status == DEPLOYMENT_STATE_READY`.

On `UpdateArtifact`, app artifacts transition back to `DEPLOYMENT_STATE_PENDING` during the re-deploy and emit a second
`ARTIFACT_EVENT_TYPE_APP_DEPLOYED` event when ready. Visibility toggles
via the `public` field cause the implementation to reconcile its
public/private routing (for example, creating or removing an Ingress in
Kubernetes); visibility toggles do not rewrite the HTML or bounce the
workload unless the implementation requires it.

`DeleteArtifact` for an app MUST remove the hosted workload and any
public ingress, not only the metadata record.

## Artifact Events

Workers emit three artifact lifecycle events through the Workers
streaming interface so frontends and downstream consumers can
react in real time.

| Enum | Wire string | Emitted |
|---|---|---|
| `ARTIFACT_EVENT_TYPE_CREATED` | `artifact.created` | Immediately after the artifact record is persisted. For apps, this is before the workload is necessarily live. |
| `ARTIFACT_EVENT_TYPE_UPDATED` | `artifact.updated` | After `UpdateArtifact` resolves, regardless of which fields changed. |
| `ARTIFACT_EVENT_TYPE_APP_DEPLOYED` | `artifact.app_deployed` | When an app artifact reaches `DEPLOYMENT_STATE_READY`, including after an update that triggered a re-deploy. |

The `ArtifactEvent` proto message is the wire shape. Workers forward
it unchanged into the streaming event envelope. 

`ArtifactEvent.url` is the URL a frontend should use when rendering a preview tile (the proxy path for private objects, the public ingress URL for public apps, etc.) — the
implementation chooses what to put there based on the artifact's visibility at event time.

Artifact events are informational and non-load-bearing. Deletion of the event log is not observable to correctness; events MAY be dropped
under backpressure.

## Agent MCP Tool Surface

Agents interact with artifacts during a run through six MCP tools exposed by the worker. Tool names are `mcp__<implementation>__<tool>`;
the implementation chooses the `<implementation>` prefix. The agent SDK handles marshalling tool calls into MCP `tool_use` events and forwarding the results.

Five of the six tools map directly onto `Artifacts` service RPCs. The sixth, `show_preview_artifact`, is a **UI-only signal**. The tool
handler is a no-op on the backend and its entire effect is the resulting `tool_use` event that the frontend recognizes and renders as an
embedded preview tile.

| Tool | Backing RPC | Notes |
|---|---|---|
| `deploy_app` | `CreateAppArtifact` | Agent supplies `name`, `content` (HTML), optional `description`, optional `public`. |
| `deploy_object` | `CreateObjectArtifact` | Agent supplies `filename` (→ `name`), `content_base64` (→ `content`), `content_type`. |
| `list_artifacts` | `ListArtifacts` | Agent supplies optional `type` filter. Implementation scopes the listing to the agent's acting user. |
| `update_artifact` | `UpdateArtifact` | Agent supplies `artifact_id` + any changed fields. Content replacement, metadata, and `public` toggle share the same tool. |
| `share_artifact` | `ShareArtifact` or `RevokeArtifactShare` | Agent supplies `artifact_id`, `target_user_id`, `role`, optional `revoke`. When `revoke = true`, maps to `RevokeArtifactShare`. |
| `show_preview_artifact` | *none (UI-only)* | Agent supplies `artifact_id`, `name`, `type` (`app` or `object`), and either `url` (for apps) or `content_type` (for objects). Handler returns a short text ack; the frontend reads the structured input from the `tool_use` event and renders the preview. |

### Report generator tools (non-normative example)

[CR0N](https://cr0n.sh) ships four convenience tools internally. `create_pdf`, `create_docx`,
`create_xlsx`, and `create_pptx` that generate a binary report from structured input, emit both the binary and a companion HTML preview,
and call `CreateObjectArtifact` with `preview_html` populated. Conforming Nightshift implementations MAY expose equivalent tools;
they are not required by the spec. 

## Storage Layering

### Collections

Collection names are a suggested convention; implementations MAY rename
them, but conforming references use these names.

- `artifacts` one Record per `Artifact`, keyed by artifact id.
  Attributes SHOULD include at least `owner_id`, `type`,
  `public` (as `"true"` or `"false"`), `run_id`, `created_at_bucket`,
  and `app_status` (for apps). The `data` payload is the serialized
  `Artifact` message.
- `artifact_permissions` one Record per `ArtifactPermission`, keyed
  by permission id. Attributes SHOULD include `artifact_id` and
  `user_id`. The `data` payload is the serialized `ArtifactPermission`
  message. Implementations enforce at most one grant per
  `(artifact_id, user_id)` pair at the domain-service layer; Storage
  does not model this constraint.

### Buckets and key conventions

- Object artifact bytes: `<artifacts-bucket>/artifacts/{artifact_id}/{name}`.
- Object artifact preview: `<artifacts-bucket>/artifacts/{artifact_id}/preview.html`.
- App artifact source HTML: `<artifacts-bucket>/apps/{artifact_id}/index.html`.

The `<artifacts-bucket>` is pre-created out of band by operators (cf.
Storage §6) and is the same conforming-implementation-wide bucket used
for all artifact content in a given deployment. Implementations that
prefer per-type buckets (for example separate S3 buckets for apps and
objects) MAY do so; the key convention above should be preserved within
each bucket.

### Why not a dedicated table

Artifacts deliberately does not define its own persistence primitive.
The design choice is that every durable byte the control plane writes
flows through Storage and is the same interface Config, Workers, Scheduling,
and future services use. A conforming Storage backend therefore covers
artifacts for free; swapping the Storage backend moves artifact
metadata alongside runs, events, configs, and secrets metadata without
touching the Artifacts service.

## Auth, Identity, and `owner_id`

User identity is modeled opaquely. The `owner_id` field on `Artifact`
and the `user_id` fields on `ArtifactPermission`, `ShareArtifact`, and
`RevokeArtifactShare` are caller-defined strings, typically the OIDC
`sub` claim, but the spec does not require OIDC.

Implementations derive the acting user from their auth mechanism (a
user session, a worker-secret header, an mTLS client certificate, a
bearer token, etc.). 

For worker-originated calls during a run, the worker typically presents an implementation-specific credential and
passes the run's user id as `owner_id`; the implementation verifies the worker is permitted to act on that user's behalf.

`owner_id` SHOULD be treated as immutable after creation. Transferring
ownership is out of scope for v1.

## Idempotency

`CreateObjectArtifact` and `CreateAppArtifact` accept an optional
`idempotency_key`. Within an implementation-defined window (24h is a
reasonable reference floor), a repeated request with the same
`(caller, idempotency_key)` tuple MUST return the original response
unchanged, including the original artifact id. Requests with the same
key but different bodies MUST fail with `INVALID_ARGUMENT`.

Idempotency is recommended for agent tool handlers since SDK retries on
transient errors would otherwise produce duplicate artifacts on repeat
success.

`UpdateArtifact`, `DeleteArtifact`, `ShareArtifact`, and
`RevokeArtifactShare` are inherently idempotent at the resource level
and do not require an explicit key.

## Error Code Mapping

| gRPC code | When |
|---|---|
| `NOT_FOUND` | Artifact does not exist, or caller lacks any access (see §3). |
| `PERMISSION_DENIED` | Caller has some access but not enough for the requested operation (e.g. editor calling `ShareArtifact`). |
| `INVALID_ARGUMENT` | Malformed request — missing required fields, `content_type` missing when `content_bytes` is set, `role = ARTIFACT_ROLE_OWNER` on `ShareArtifact`, idempotency key reuse with a different body, etc. |
| `ALREADY_EXISTS` | Implementation-specific — for example, a natural-key collision on a named artifact if the implementation enforces name uniqueness. |
| `FAILED_PRECONDITION` | App deploy pipeline rejects the content (malformed HTML, asset too large, etc.) synchronously. Async failures surface as `DEPLOYMENT_STATE_FAILED` instead. |
| `UNAVAILABLE` | Backing Storage or workload runtime transiently unavailable. |
| `RESOURCE_EXHAUSTED` | Per-user artifact quota or per-request size limit exceeded. |

## Versioning

`nightshift.v1.Artifacts` is part of the `v1` package. Wire-breaking
changes (renamed fields, renumbered tags, removed RPCs) require a
`v2` service in a new proto file. Additive changes (new RPCs, new
optional fields, new enum values) remain in-package and are permitted.

New enum values in particular additions to `ArtifactType`,
`ArtifactEventType`, and `DeploymentState` MUST be considered
forward-compatible: clients that encounter an unknown value SHOULD
treat it as `*_UNSPECIFIED` and fall back gracefully.

## Out of Scope for v1 but on the roadmap

- Artifact ownership transfer.
- `ARTIFACT_ROLE_OWNER` as a grantable role via `ShareArtifact`.
- Group or role-based sharing (grants are per-user only).
- Artifact versioning / history beyond the Storage `Record` version.
- App artifacts with server-side logic and persistence (only static HTML/CSS/JS is
  defined, that being said it's not impossible for the Agent to figure out how to do this in the current state).
- Cross-tenant or public listing of artifacts.
- A dedicated upload-by-presigned-URL flow that bypasses inline
  `content` bytes. This is expected to arrive as a
  `RegisterObjectArtifact(bucket, key)` RPC in a later revision.
