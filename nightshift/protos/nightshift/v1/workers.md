# Nightshift Workers — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Workers` service. The proto file at
[`workers.proto`](workers.proto) is the authoritative wire contract;
this document describes what a conforming implementation must do and
why.

## Overview

Workers is the load-bearing surface of Nightshift. Every interactive
and scheduled agent execution flows through this service and everything
else in the spec (Storage, Config, Secrets, Artifacts, Scheduling)
exists to make Workers useful.

The service has two clearly separated RPC groups on the same gRPC
service definition:

- **Outer surface** — user-facing and scheduler-facing RPCs for
  creating runs, observing their state, streaming events, and
  interrupting or deleting sessions.

- **Inner surface** — worker callback RPCs the run workload invokes
  to push events and terminal outcomes back to the control plane.
  These are a separate trust boundary (§8).

Workers persists durable state through Storage. It does NOT define
its own persistence primitive.

## Run Lifecycle

Every run transitions through the states declared in `RunStatus`. The full transition diagram:

```
                ┌──────────────────────► COMPLETED
                │
PENDING ───► RUNNING ──────────────────► ERROR
                │
                └──────────────────────► INTERRUPTED
```

Allowed transitions:

| From | To | Trigger |
|---|---|---|
| (absent) | PENDING | `CreateRun` |
| PENDING | RUNNING | Worker workload started (implementation signal) |
| PENDING | ERROR | Launch failure before the worker came up |
| PENDING | INTERRUPTED | `InterruptRun` called before RUNNING |
| RUNNING | COMPLETED | Worker called `CompleteRun` |
| RUNNING | ERROR | Worker called `FailRun`, or implementation-side deadline elapsed |
| RUNNING | INTERRUPTED | Worker observed cancellation and returned, or hard-kill fallback |

Terminal states (`COMPLETED`, `ERROR`, `INTERRUPTED`) are absorbing:
no further transitions are permitted. The associated `started_at`
and `ended_at` timestamps MUST be populated on the Run record at the
corresponding transitions.

### Resilience expectations

- If the control plane restarts while a run is RUNNING, it MUST
  reconcile on startup: any run in RUNNING whose worker has
  disappeared (for example the K8s Job is gone, or no events have
  arrived within an implementation-defined window) transitions to
  ERROR with an implementation-supplied message.
- Workers MAY be instructed to retry, but the spec does NOT define
  retry. Each `CreateRun` produces a new Run record with a fresh
  id.

## Invoker Attribution

Every Run records `invoker_type` + `invoker_id`. This is load-bearing
and implementations MAY use it for billing, audit, and rate limits.

- `INVOKER_TYPE_USER` — the creating call came from a human user.
  `invoker_id` is the user's id. Typically the caller's auth
  principal. Some implementations permit admins to create runs on
  another user's behalf, in which case the attribution should still
  point at the *target* user, not the admin.

- `INVOKER_TYPE_SCHEDULE` — a schedule fired. `invoker_id`
  is the schedule id. The Run's `user_id` MUST match the schedule's
  owning user.

Additional invoker types (for example `INVOKER_TYPE_API`,
`INVOKER_TYPE_WEBHOOK`) are reserved for future expansion. Clients
MUST treat unknown `InvokerType` values as opaque and forward-compat
friendly.

## Session Identity

Nightshift exposes a `session_id` that is **owned by the control
plane, not by the agent SDK**. This is a deliberate abstraction: the
Claude Agent SDK, for example, has its own internal session identifier
whose lifetime and semantics belong to the SDK which Nightshift wraps.

### Why

1. SDKs change. A Nightshift session should survive a change of
   agent SDK or a change of SDK session-id format.
2. Multi-SDK implementations. An implementation may want to support
   multiple agent SDKs; a single session_id namespace across them is
   cleaner than leaking each SDK's native id.
3. Session deletion (`DeleteSession`, §7) can then also clean up
   SDK-level state (session JSONL files, memory directories) by
   translating the Nightshift id through an implementation-held map.

### How

- `CreateRun.session_id` is optional. Empty means "start a new
  conversation"; the implementation assigns a fresh Nightshift
  session_id at run creation and persists it on the Run when the
  worker calls `CompleteRun`.
- When a caller passes a non-empty `session_id` to `CreateRun`, the
  implementation maps it to whatever SDK-internal session identifier
  is needed and instructs the worker to resume that conversation.
- The SDK-internal session identifier is an implementation detail and
  MUST NOT appear on the **outer** (user-facing) surface — `Run`
  messages, `CreateRun` / `GetRun` / `ListRuns` responses, and the
  `StreamEvent` envelope all carry only the platform-owned
  `session_id`. Operators and end users never see the SDK id.
- The **inner** (worker callback) surface MAY carry the SDK id:
  `CompleteRunRequest.session_id` is defined as the SDK-internal id
  reported by the worker on terminal completion. The implementation
  persists that value out-of-band — for the reference implementation,
  as the `sdk_session_id` attribute on the Run record — and re-injects
  it as a launcher env var (`NS_SDK_SESSION_ID`) on follow-up runs of
  the same platform `session_id`. The worker never crosses the SDK id
  back over the outer surface; it only reports it on the way out and
  reads it on the way in.

The platform-owned `session_id` and the SDK-internal session id are
intentionally distinct values and MUST NOT appear in the same wire
field. `Run.session_id` is the platform's; `CompleteRunRequest.session_id`
is the SDK's. The implementation's job is to bridge them.

### Session runs vs session state

A `session_id` groups:

- The Run records (`runs` collection, filtered by `session_id`).
- The underlying SDK session state (session JSONL files on a per-
  user RWX volume, memory files, SDK settings). These are Storage
  `Object`s in the reference implementation.

`DeleteSession` MUST remove both.

## Event Stream

A run's events are a monotonic append-only log.

- `StreamEvent.index` starts at 0 and increments by 1 per event.
  `(run_id, index)` is unique.
- `timestamp` is the wallclock time the event was produced by the
  worker.
- `type` is a discriminator; `raw` is the structured payload.

### Well-known `type` values

Conforming implementations SHOULD use these strings when the payload
matches their shape, and MAY introduce additional values.

| `type` | Meaning | `raw` shape |
|---|---|---|
| `system.worker_started` | The run workload is alive and has begun consuming the prompt. | Implementation-defined |
| `user` | An SDK message from the user role. | SDK-shaped |
| `assistant` | An SDK message from the assistant role. | SDK-shaped |
| `result.<subtype>` | SDK result / status message. | SDK-shaped |
| `system.<subtype>` | SDK system message. | SDK-shaped |
| `rate_limit_event` | SDK rate-limit notification. | SDK-shaped |
| `artifact.created` / `artifact.updated` / `artifact.app_deployed` | Forwarded from the Artifacts service. | `ArtifactEvent` (see `artifacts.proto`) |

Subscribers MUST treat unknown `type` values as opaque and preserve
them during history replay. New values are additive and do not
constitute a breaking change to the spec.

### Payload opacity

`raw` is `google.protobuf.Struct`. The spec does NOT define the field
shape for each `type` because it varies by SDK. Conforming
implementations MUST preserve `raw` bit-for-bit from `PostWorkerEvent`
through persistence, streaming, and history replay. Clients MAY
interpret or MAY pass through.

### Streaming semantics

`StreamRunEvents` is a server-streaming RPC:

1. On subscription (and unless `include_history = false`), the
   implementation SHOULD replay all persisted events from
   `from_index` (default 0) before streaming live events.
2. Live events are delivered in order of index.
3. When the run reaches a terminal status, the stream SHOULD deliver
   any remaining buffered events then terminate with gRPC status OK.
4. Slow subscribers MAY be dropped; implementations SHOULD size
   per-subscription buffers generously but MUST NOT block other
   subscribers behind a slow one.

### Non-streaming history accessor

`ListRunEvents` is a paginated alternative for callers that prefer
snapshot semantics (tests, offline inspection). It MUST return the
same events in the same order `StreamRunEvents` would replay.

## Interrupt and Cancellation

Cancellation has two observers, the control plane (which records the
signal) and the worker (which must act on it).

### Outer-surface signal

`InterruptRun` records a cancellation request on the Run. This is the
authoritative cross-process cancellation signal.

Effects:

1. The Run's cancellation flag is set in Storage.
2. For implementations with a hard-kill path (for example deleting
   the K8s Job), that path runs in parallel as a safety net.
3. The worker observes the signal via `GetRunCancellation` and
   gracefully drains.
4. The Run transitions to `RUN_STATUS_INTERRUPTED` when the worker
   calls `CompleteRun` / `FailRun` with terminal state, OR when the
   hard-kill path runs and the reconciler detects the worker is
   gone.

`InterruptRun` returns the Run after the signal has been recorded.
The returned status may still be `RUN_STATUS_RUNNING`; callers MUST
observe the transition to `RUN_STATUS_INTERRUPTED` via the event
stream or a subsequent `GetRun`.

### Worker polling

Workers SHOULD poll `GetRunCancellation` periodically during
execution — every N events or every K seconds. When `cancelled = true`:

1. Invoke the SDK's interrupt path.
2. Drain in-flight events (up to an implementation-defined deadline).
3. Call `CompleteRun` if the interrupt produced a clean result, or
   `FailRun` otherwise.

Implementations MAY deliver cancellation via a streaming RPC in a
future revision; for v1 the spec prescribes polling for simplicity.

### Reasons cancellation can fire

- Explicit user `InterruptRun`.
- Implementation-side deadline (for example `JOB_ACTIVE_DEADLINE`).
- Implementation-side quota or budget enforcement.

All three go through the same cancellation flag and appear identical
to the worker.

## Session Deletion

`DeleteSession(session_id)` is the counterpart to the session-
resume behavior described in §4. It:

1. Deletes every `runs` Record with matching `session_id`.
2. Deletes the associated `events` Records (cascade).
3. Deletes the underlying SDK session state. The reference
   implementation lays this out identically on disk and in object
   storage, keyed by the platform-owned `(user_id, session_id)`:
   - **Filesystem backend** (PVC or hostPath): the per-session
     subdirectory at `<mount>/<user_id>/<session_id>/...` is removed
     recursively.
   - **Object backend**: every Object under the prefix
     `sessions/<user_id>/<session_id>/` is enumerated and deleted.
4. Returns the count of run records removed.

Implementations MUST NOT delete runs that are still in
`RUN_STATUS_PENDING` or `RUN_STATUS_RUNNING` — they MUST return
`FAILED_PRECONDITION` and require the caller to interrupt first.
The state cascade (step 3) MUST run before the record cascade so a
failed cleanup leaves the index intact and a retry can resume; if
records were deleted first and cleanup failed, the platform would
leak files unindexed by any run.

Deleting a session that does not exist is a no-op; the response
returns `runs_deleted = 0` with gRPC status OK (not NOT_FOUND).
Likewise, deleting a session whose runs have all been individually
removed is a no-op — without a record there is no `user_id` to
cascade against.

## Worker Callback Trust Boundary

The worker callback RPCs (`PostWorkerEvent`, `CompleteRun`,
`FailRun`, `GetRunCancellation`) are a **separate trust boundary**
from the outer surface. Callers to these RPCs are the run workload,
not human users.

Implementations MUST authenticate worker callbacks distinctly. The
reference path is:

1. At run launch, the implementation mints an implementation-
   specific credential bound to the run id (for example a JWT signed
   by the control plane's secret, or a shared worker-secret header).
2. The worker presents that credential on every callback RPC.
3. The control plane verifies the credential and the run id match;
   mismatches fail with `UNAUTHENTICATED` or `PERMISSION_DENIED`.

The credential material itself is managed via the Secrets interface typically a per-run Secret in Secrets KV, or a
Kubernetes-auth'd AuthToken presented via `LoginWithKubernetes`.

`PostWorkerEvent` MUST:

- Assign `index` server-side (ignoring any `index` on the incoming
  event).
- Reject events for runs not in `RUN_STATUS_RUNNING` with
  `FAILED_PRECONDITION`.
- Persist the event and broadcast it to all live subscribers of
  `StreamRunEvents`.

`CompleteRun` and `FailRun` are terminal and idempotent at the Run
level — once a Run is in a terminal status, a repeat call MUST be a
no-op with gRPC status OK.

`GetRunCancellation` is read-only and may be called as frequently as
the worker's polling policy dictates.

## Storage Layering

### Collections

- `runs` — one Record per Run, keyed by run id. `Record.data` holds
  the serialized Run message. `Record.attributes` SHOULD include at
  minimum `user_id`, `status`, `session_id`, `invoker_type`,
  `invoker_id`, and a `created_at_bucket` (per Storage §2) so
  `ListRuns` filters work on indexed attributes.
- `events` — one Record per StreamEvent, keyed by `{run_id}:{index}`.
  `Record.data` holds the serialized StreamEvent message.
  `Record.attributes` SHOULD include `run_id` (for the event-listing
  queries).

### Objects

- Per-user / per-session SDK state is stored as Storage `Object`s
  under an implementation-defined bucket + key convention
  (`sessions/{user_id}/{session_id}/...`). `DeleteSession` enumerates
  and deletes these via Storage's Object API.

### Why not a Workers-specific DB

The reference implementation could run a separate database for runs
and events, but the spec explicitly requires Storage layering: a
conforming Storage backend is sufficient to back Workers. This
preserves the pluggability property — operators who swap SQLite for
Postgres move runs and events along with artifacts and configs.

## Error Code Mapping

| gRPC code | When |
|---|---|
| `NOT_FOUND` | Run does not exist, or caller lacks visibility. Also returned for `DeleteSession` internal cascade failures that would leak existence. |
| `PERMISSION_DENIED` | Caller has some access but not enough for the requested operation. |
| `UNAUTHENTICATED` | Missing or invalid auth credential on the worker callback surface. |
| `INVALID_ARGUMENT` | Missing required fields (empty `prompt`, empty `user_id`), unknown enum values, idempotency-key reuse with a different body. |
| `FAILED_PRECONDITION` | `PostWorkerEvent` on a terminal Run; `DeleteSession` on a session with active runs; `InterruptRun` on a terminal Run. |
| `RESOURCE_EXHAUSTED` | Per-user / per-org run quota exceeded. |
| `UNAVAILABLE` | Underlying Storage or workload runtime transiently unavailable. |
| `DEADLINE_EXCEEDED` | Streaming subscriber dropped due to server-side backpressure. |

## Idempotency

`CreateRun` accepts an optional `idempotency_key`. Semantics match
Storage §11: within an implementation-defined window, a repeat request
with the same `(caller, idempotency_key)` returns the original
response; a repeat with the same key but a different body fails with
`INVALID_ARGUMENT`.

`InterruptRun` and `DeleteSession` are resource-level idempotent.

`PostWorkerEvent` is naturally idempotent through server-assigned
indices — re-posting an event simply produces a new index;
implementations that need strict dedup across retries SHOULD carry an
opaque `event_id` inside `raw`.

## Versioning

`nightshift.v1.Workers` is part of the `v1` package. Additive changes
(new RPCs, new optional fields, new enum values, new well-known event
`type` strings) are permitted in-package.

New `RunStatus` and `InvokerType` values MUST be treated as forward-
compatible: unknown values fall back to `*_UNSPECIFIED`.

Wire-breaking changes require a `v2` service in a new proto file.

## Out of Scope for v1

- Synchronous run RPCs. Runs are inherently long-lived; there is no
  `ExecuteRunSync` that blocks until completion.
- Streaming cancellation (in place of polling). Deferred to a later
  revision.
- Multi-region run placement.
- Agent SDK metadata beyond the strings in `RunUsage` and
  well-known event `type`s. Specifically, per-model breakdowns
  (cr0n-a's `model_usage`) are not in v1 — implementations MAY
  encode them inside `raw` payloads on terminal events.
- Batch or scatter-gather run primitives.
- Any Kubernetes-specific knobs (CPU / memory limits, node
  selectors). Implementations choose these; the spec is
  intentionally runtime-neutral.
