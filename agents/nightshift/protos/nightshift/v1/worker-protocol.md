# Nightshift Worker-to-Platform Protocol

This document is the normative specification for the **agent-to-
platform protocol (A2P)** every Nightshift-conforming worker image must
speak. It is a companion to [`workers.md`](workers.md) §8, which
defines the Workers inner surface.

## Overview

A Nightshift worker is an opaque container image that:

1. Runs inside a runtime (Kubernetes Job, systemd service, or any
   equivalent) managed by the Workers service.
2. Speaks the RPCs in this document, the **inner surface** of
   `nightshift.v1.Workers`.
3. Authenticates itself with a **short-lived HMAC credential** scoped
   to a single run.

The worker is intentionally agent-framework-agnostic. Nothing in
this protocol is specific to Claude, Codex, or any particular Agent 
SDK. Implementations are free to embed any runtime (Python + Claude
Agent SDK, Go + Anthropic API, shell + external CLI, mock / simulator,
etc.) as long as they speak this protocol.

The canonical reference worker lives at [`cmd/nightshift-worker`](../../cmd/nightshift-worker) 
and is under active development. A simulation-only implementation that emits three synthetic events and
completes (we're going to make this better). 

Transport: gRPC (preferred), or HTTP/JSON via grpc-gateway.

## Environment Contract

The Workers service passes the following environment variables to the
worker container at launch:

| Variable | Required | Meaning |
|---|---|---|
| `NS_RUN_ID` | ✓ | The run id for every inner-surface RPC. |
| `NS_USER_ID` | ✓ | Owning user id. Used by the worker for logging and scoping. |
| `NS_SESSION_ID` |   | Nightshift session id to resume. Empty means "fresh conversation". |
| `NS_PROMPT` | ✓ | Task instruction from `CreateRunRequest.prompt`. |
| `NS_API_URL` | ✓ | Base URL of the Workers inner surface. Typically a host:port for gRPC in-cluster. |
| `NS_WORKER_CREDENTIAL` | ✓ | HMAC-signed bearer token scoped to `NS_RUN_ID`; see §3. |
| `NS_CANCEL_POLL_SECONDS` |   | Suggested polling interval (integer seconds). Default 5. |
| `NS_ACTIVE_DEADLINE_SECONDS` |   | Run-wide wall-clock deadline the runtime will enforce. Workers may honor this or not. |
| `NS_SESSION_STATE_DIR` |   | Absolute path to a per-session writable directory. Workers MAY persist agent-SDK transcripts, memory files, and other session state here. Files survive across runs of the same `NS_SESSION_ID`. The directory is platform-owned and reclaimed by `Workers.DeleteSession`; workers MUST NOT manually delete its contents. Empty / unset means the platform did not provision per-session state — workers MUST tolerate this and either operate statelessly or write to ephemeral storage. |

Implementations MAY pass additional `NS_*` env vars as extensions.
Workers MUST ignore unknown env vars.

## Worker Credential

### Wire format

```
v1.<runID>.<expUnix>.<hex(HMAC-SHA256(secret, "v1."+runID+"."+expUnix))>
```

- `v1` — protocol version. Unknown versions MUST be rejected.
- `<runID>` — the run id the credential is scoped to. MUST match
  `NS_RUN_ID`.
- `<expUnix>` — expiry as a Unix timestamp (seconds since epoch).
  After this instant the credential is invalid.
- `<hex(HMAC-SHA256(...))>` — hex-encoded HMAC-SHA256 signature over
  the preceding `v1.<runID>.<expUnix>` payload, keyed by a shared
  secret held only by the Workers service and its auth layer.

The credential is minted by the Workers service at `CreateRun` time.
Its expiry is `created_at + ActiveDeadline + credential_buffer`
(reference implementation uses a 5-minute buffer).

### Transport

The worker sends the credential on every inner-surface RPC as:

```
Authorization: Bearer <credential>
```

The Workers service verifies (a) the signature matches the shared
secret, (b) `<runID>` equals the `run_id` in the request, (c) the
expiry has not elapsed. Any failure returns `UNAUTHENTICATED`.

### Scope

The credential authorizes **only** the inner-surface RPCs
(`PostWorkerEvent`, `CompleteRun`, `FailRun`, `GetRunCancellation`)
for the run id encoded inside it. It does NOT grant access to other
runs, to the outer surface, or to any other Nightshift service.

## Event Shapes — `raw` payloads by `type`

Each `PostWorkerEvent` submits a `StreamEvent` whose `raw` field is a
`google.protobuf.Struct`. The shape of `raw` depends on `type`.
Design rule:

- **Nightshift-standard fields at the top level.** Every SDK-agnostic
  consumer (frontends, auditing, billing) reads from here.

- **An optional `sdk` field** holds vendor-specific extensions
  (native SDK dataclass dumps, model-specific metadata, etc.) that
  the Nightshift envelope does not standardize.

The platform preserves `raw` bit-for-bit through persistence,
streaming, and history replay. Workers port SDK messages by lifting
known fields to the envelope top level and putting the rest under
`sdk`.

### Content blocks (for `user` / `assistant`)

Discriminated by `type`. Additional per-block fields are permitted
and preserved opaquely.

```json
{"type": "text",        "text": "..."}
{"type": "thinking",    "thinking": "...", "signature": "..."}
{"type": "tool_use",    "id": "tu_...", "name": "...", "input": {...}}
{"type": "tool_result", "tool_use_id": "tu_...", "content": [...], "is_error": false}
```

`image` and other SDK-specific block types are permitted;
implementations MUST preserve unknown block types verbatim.

### `system.worker_started`

Lifecycle signal emitted at worker start. Optional but recommended.

```json
{
  "worker_id": "string",
  "image":     "string",
  "version":   "string"
}
```

### `user`

```json
{
  "content": [ <block>, ... ],
  "sdk":     { ... }
}
```

### `assistant`

```json
{
  "content": [ <block>, ... ],
  "model":   "string",
  "sdk":     { ... }
}
```

### `result.<subtype>` (canonical subtypes: `success`, `error`)

```json
{
  "subtype":        "success" | "error",
  "session_id":     "string",
  "usage": {
    "input_tokens":           0,
    "output_tokens":          0,
    "cache_read_tokens":      0,
    "cache_creation_tokens":  0
  },
  "total_cost_usd": 0.0,
  "duration_ms":    0,
  "sdk":            { ... }
}
```

Canonical token field names are the short forms (`cache_read_tokens`,
`cache_creation_tokens`) matching `RunUsage` in `workers.proto`.
Workers aggregating from SDKs that use different names (Claude SDK
emits `cache_read_input_tokens` / `cache_creation_input_tokens`)
MUST normalize.

### `system.<subtype>`

```json
{
  "subtype": "string",
  "sdk":     { ... }
}
```

### `rate_limit_event`

```json
{
  "retry_after_seconds": 0,
  "sdk":                 { ... }
}
```

### `stream_event` (SDK-level streaming delta passthrough)

```json
{
  "sdk": { ... }
}
```

Workers that aggregate turns into whole `assistant` messages SHOULD
NOT emit `stream_event`. Workers that forward raw SDK deltas MAY.

### `artifact.created` / `artifact.updated` / `artifact.app_deployed`

Matches `ArtifactEvent` in
[`artifacts.proto`](artifacts.proto):

```json
{
  "artifact_id":   "string",
  "artifact_type": "ARTIFACT_TYPE_OBJECT" | "ARTIFACT_TYPE_APP",
  "name":          "string",
  "url":           "string",
  "content_type":  "string"
}
```

Emitted by the worker after a successful Artifacts RPC during the
run.

## RPC Sequence

A well-behaved run looks like this:

1. Optional: `PostWorkerEvent` with `type="system.worker_started"`.
2. Per SDK message: `PostWorkerEvent` with a well-known `type`.
3. Every `NS_CANCEL_POLL_SECONDS` (or every N events):
   `GetRunCancellation`. On `cancelled=true`, the worker invokes its
   SDK interrupt, drains, and proceeds to step 4.
4. Exactly one terminal RPC:
   - **Success:** `PostWorkerEvent` with `type="result.<subtype>"`
     for the final message, then `CompleteRun` with the session id
     and `RunUsage`.
   - **Failure:** `FailRun` with a human-readable `error`. The
     terminal event MAY be omitted on failure.

## Invariants

- **Workers MUST NOT set `StreamEvent.index`.** The server assigns
  it and returns the assigned value in `PostWorkerEventResponse.index`.
- **Workers MUST set `StreamEvent.timestamp`** (RFC 3339 / proto
  `Timestamp`).
- **Ordering is by `index`.** `timestamp` is informational;
  subscribers use `index` for dedup and replay.
- **Exactly one terminal per run.** `CompleteRun` and `FailRun` are
  idempotent on already-terminal runs — a repeated terminal call on
  a run that is already COMPLETED / ERROR / INTERRUPTED MUST return
  OK without state change.
- **Cancellation is cooperative.** The platform's hard-stop
  (`JobLauncher.Interrupt` / SIGTERM) is a safety net; the
  authoritative signal is the `cancelled` attribute observed via
  `GetRunCancellation`. A worker that observes `cancelled=true`
  SHOULD `CompleteRun` (the platform then records
  RUN_STATUS_INTERRUPTED via the cooperative-cancellation path).
- **`raw` is preserved bit-for-bit** by the platform through
  persistence, streaming, and history replay. No canonicalization,
  no map re-ordering, no field stripping.
- **Deltas are deferred.** One `assistant` event per complete model
  turn is the v1 shape. `stream_event` is the permitted escape hatch
  for SDKs that cannot aggregate.

## Reference Implementation

[`cmd/nightshift-worker`](../../cmd/nightshift-worker) is the
canonical minimal conforming worker:

- Emits `system.worker_started` → one fake `assistant` → one
  `result.success`.
- Calls `CompleteRun` with a synthetic `RunUsage`.
- Observes cancellation via `GetRunCancellation` after each event;
  on `cancelled=true`, completes cleanly with empty usage so the
  platform records RUN_STATUS_INTERRUPTED.
- Does NOT call any LLM.

Any worker image whose behavior is observationally equivalent on the
wire is conforming. See
[`cmd/nightshift-worker/main.go`](../../cmd/nightshift-worker/main.go)
for the full reference.

## CR0N Port Notes (Informative)

When cr0n-a's Python `worker.py` is ported to the Nightshift protocol
(chunk 14 in [MIGRATION.md](../../MIGRATION.md)), the deltas from
its current behavior are:

- **Stop self-assigning `StreamEvent.index`.** cr0n-a currently
  increments a client-side counter and submits it on every event;
  the Nightshift protocol assigns indices server-side.
  `PostWorkerEventResponse.index` carries the assigned value.
- **Transform flat SDK-dataclass `raw` dumps into the envelope
  shape above.** cr0n-a today emits `raw = asdict(SDKMessage) +
  {type}`. Porting lifts `content`, `model`, `session_id`, `usage`,
  `total_cost_usd`, `duration_ms`, `subtype` to top-level envelope
  fields and pushes the remainder under `sdk`.
- **Rename `cache_read_input_tokens` / `cache_creation_input_tokens`
  → `cache_read_tokens` / `cache_creation_tokens`** on `result.*`
  and `CompleteRun`. cr0n-a uses the SDK's native names; Nightshift
  canonicalizes to the short forms.
- **Pass the worker credential via `Authorization: Bearer
  $NS_WORKER_CREDENTIAL`** instead of the cr0n-a
  `X-Worker-Secret` shared-secret header. The new credential is
  per-run, expires with the deadline, and is HMAC-signed so a leak
  does not compromise other runs.
