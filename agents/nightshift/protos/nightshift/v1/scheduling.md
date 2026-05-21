# Nightshift Scheduling — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Scheduling` service. The proto file at
[`scheduling.proto`](scheduling.proto) is the authoritative wire
contract; this document describes what a conforming implementation
must do and why.

## 1. Overview

Scheduling is a thin domain service layered over Workers. Each
`Schedule` is a declarative record of a recurring run — prompt, cron
expression, IANA timezone, optional session continuation, and the
resource bindings (agents, skills, connectors) the fire resolves
under the owning user's Config. When the schedule fires, the
implementation issues a `CreateRun` on the Workers service with
`invoker_type = INVOKER_TYPE_SCHEDULE` and `invoker_id = <schedule
id>`.

Scheduling does not hold credentials, does not materialize
per-connector tokens, and does not itself run agents. It plans when
things happen and delegates everything else.

Persisted state lives in Storage: one Record per Schedule in the
`schedules` collection. The cron runtime (Kubernetes CronJobs,
in-process timers, systemd timers, an external scheduler daemon, …)
is an implementation choice the spec does not prescribe.

## 2. Schedule Shape

Core fields:

- `user_id` — the owning user. Fires run under this user's Config;
  Secrets dispenses this user's tokens for the bound connectors.
- `prompt` — sent verbatim on every fire.
- `cron` — 5-field POSIX cron expression (`"minute hour dom month
  dow"`). Implementations MAY accept extensions but round-trip the
  input without normalization.
- `timezone` — IANA Time Zone Database identifier. Empty is treated
  as `"UTC"`.
- `enabled` — kill switch. False suspends the cron runtime without
  deleting it.
- `session_id` — optional Nightshift Workers session id. Non-empty
  reuses the same session on every fire (one long conversation);
  empty starts fresh every fire.
- `agent_ids`, `skill_ids`, `connector_ids` — ids into the Config
  service's `agents`, `skills`, `connectors` collections. The
  schedule stores ids only; Config materializes them into the live
  `AgentDefinition`, `SkillDefinition`, and `McpServerConfig` values
  at run start.

Fields NOT on the record by design:

- **No per-run output capture.** Results flow through Workers'
  normal event stream; the schedule does not accumulate per-fire
  results.
- **No credential material.** See §6.
- **No last-fired / next-fire-at timestamps.** The Workers `runs`
  collection is the source of truth for what has fired — filter
  `runs` by `invoker_type = SCHEDULE` and `invoker_id = <schedule
  id>` to reconstruct history. Implementations MAY surface
  next-fire-at as an auxiliary read; the spec does not mandate it.

## 3. Cron Semantics

Conforming implementations interpret `cron` as 5-field POSIX cron:

- Field 1 — minute (0–59)
- Field 2 — hour (0–23)
- Field 3 — day of month (1–31)
- Field 4 — month (1–12 or JAN–DEC)
- Field 5 — day of week (0–6 or SUN–SAT; 0 and 7 both mean Sunday)

Ranges (`1-5`), steps (`*/15`), and lists (`1,15,30`) are required.

The expression is interpreted in the schedule's `timezone`. If the
timezone undergoes a DST transition during a scheduled minute,
implementations SHOULD follow Kubernetes CronJob semantics (fire
once per wallclock occurrence, skip missing occurrences, do not
fire twice for fall-back hours).

Implementations MAY accept additional syntax (6-field with seconds,
`@hourly` / `@daily` aliases, etc.) as a best-effort convenience but
MUST preserve the caller's original string through `GetSchedule` and
`UpdateSchedule` without normalization — round-trips are lossless.

## 4. Concurrency Policy

Implementations MUST NOT allow a schedule's fires to stack
indefinitely. The reference policy is **Forbid**: if a previous fire
has not completed, a new fire is skipped. This matches cr0n-a's
cr0n-sched-* CronJobs (`concurrencyPolicy: Forbid`) and prevents
slow runs from queueing behind each other.

Forbid is the only policy the spec requires. Implementations MAY
offer Allow (stack) or Replace (cancel prior, start new) as
non-normative extensions; when they do, the policy is an
implementation-configured property, not a per-schedule field.

## 5. Suspend / Resume via `enabled`

- `enabled = true` (default on create) — the cron runtime is active;
  fires occur.
- `enabled = false` — the cron runtime is suspended but the record
  persists. Re-enabling resumes firing on the next matching tick.

Implementations MUST prefer suspension over deletion for disable
transitions. Kubernetes-backed implementations set `suspend: true`
on the CronJob; in-process schedulers skip ticks.

An implementation reconciling after a long disable window MUST NOT
fire "missed" runs catch-up; fires skipped while suspended are lost
by design.

## 6. Resource Bindings and Fire Semantics

On every fire, the implementation issues a `CreateRun` with:

- `prompt` — the schedule's `prompt`.
- `session_id` — the schedule's `session_id` (possibly empty).
- `user_id` — the schedule's `user_id`.
- `invoker_type` — `INVOKER_TYPE_SCHEDULE`.
- `invoker_id` — the schedule's id.

Workers then calls Config's `GetUserConfig` to materialize agents,
skills, and MCP server configs for the owning user. The schedule's
`agent_ids` / `skill_ids` / `connector_ids` fields MAY be used by
the implementation to narrow what Config returns for this fire —
the recommended shape is:

- If `agent_ids` is non-empty, only the listed agents load.
- If `skill_ids` is non-empty, only the listed skills load.
- If `connector_ids` is non-empty, only the listed connectors
  authenticate.
- Any empty list means "include all the user has" (the default
  `GetUserConfig` behavior).

This narrowing is advisory for v1 — implementations MAY load the
full user config and ignore the schedule's binding lists. Once
`GetUserConfigRequest` supports scoping fields in a later revision,
schedules will pass the ids through directly.

`session_id` interaction: if the schedule's `session_id` is set,
Workers resumes the same conversation on every fire. This is useful
for periodic "status check" agents that benefit from long-term
memory; it is not useful (and actively harmful) for independent,
stateless fires, which should leave `session_id` empty.

## 7. Startup Reconciliation

On startup the implementation MUST reconcile the `schedules`
collection against its cron runtime:

1. For each enabled `Schedule`, ensure a runtime entry exists with
   the current `cron` + `timezone` values. Create / update as needed.
2. For each disabled `Schedule`, ensure the runtime entry is
   suspended (or absent; the spec does not require suspended
   entries to persist as long as re-enabling recreates them).
3. Remove any runtime entries that do not correspond to a Schedule
   in the collection (orphan cleanup).

Reconciliation is idempotent and MUST be safe to run on every
control-plane startup. Implementations SHOULD scope orphan detection
by label or naming convention so schedules owned by other systems
sharing the same runtime are not accidentally removed.

## 8. RBAC

Scheduling RBAC is personal-tier:

| Operation | Who |
|---|---|
| `CreateSchedule` | The caller (must match `user_id`) |
| `GetSchedule`, `ListSchedules` | The owner; admins MAY see across users |
| `UpdateSchedule`, `DeleteSchedule` | The owner |

A caller attempting to create a schedule under a different
`user_id` MUST be denied with `PERMISSION_DENIED` unless the caller
holds admin privilege.

Admin schedules (schedules owned by an org-wide identity rather than
a human) are out of scope for v1.

## 9. Storage Layering

- Collection: `schedules`. One Record per Schedule, keyed by
  schedule id. `Record.data` holds the serialized Schedule message.
  `Record.attributes` SHOULD include `user_id` and `enabled` (as
  `"true"` / `"false"`) so `ListSchedules` filters work on indexed
  attributes.
- Cron runtime state lives outside Storage — it is implementation-
  managed. Orphan reconciliation is the only spec-mandated link
  between the two.

## 10. Error Code Mapping

| gRPC code | When |
|---|---|
| `NOT_FOUND` | Target schedule does not exist; or the caller lacks visibility (§8). |
| `PERMISSION_DENIED` | Caller is authenticated but not the owner / admin. |
| `INVALID_ARGUMENT` | Unparseable cron expression; unknown IANA timezone; empty prompt; idempotency key reuse with a different body. |
| `FAILED_PRECONDITION` | Referenced agent / skill / connector id does not exist in Config (implementations MAY choose to accept the write and surface the error only at fire time). |
| `UNAVAILABLE` | Cron runtime (for example Kubernetes API) transiently unavailable; Storage unavailable. |
| `RESOURCE_EXHAUSTED` | Per-user schedule quota exceeded. |

## 11. Idempotency

`CreateSchedule` accepts an optional `idempotency_key`. Semantics
match Storage §11: within an implementation-defined window, a repeat
request with the same `(caller, idempotency_key)` returns the
original response; a repeat with the same key but a different body
fails with `INVALID_ARGUMENT`.

`UpdateSchedule` and `DeleteSchedule` are resource-level idempotent.

## 12. Versioning

`nightshift.v1.Scheduling` is part of the `v1` package. Additive
changes (new RPCs, new optional fields) are permitted in-package.
Wire-breaking changes require a `v2` service in a new proto file.

## 13. Out of Scope for v1

- One-shot / run-once schedules (these are just a `CreateRun` on
  Workers).
- Dependency graphs across schedules.
- Dynamic cron expressions (for example cron on computed times).
- Per-schedule concurrency policies beyond the default Forbid.
- Per-schedule timezone-changing fires (the zone is fixed at
  update time).
- Per-schedule retry policies — if a fire's Run ends in ERROR, the
  schedule simply waits for the next cron tick. Operators who need
  retry wire it in at the agent-prompt layer.
- Catch-up runs after long suspension.
