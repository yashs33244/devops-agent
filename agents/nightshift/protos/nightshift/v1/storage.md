# Nightshift Storage — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Storage` service. The proto file at
[`storage.proto`](storage.proto) is the authoritative wire contract; this
document describes what a conforming implementation must do and why.

## 1. Overview

Storage is the **persistence foundation** of Nightshift. Every durable
piece of state in a conforming control plane flows through Storage:

- Structured state (runs, events, user configs, skills, agents,
  connectors, billing events, schedules, artifact metadata, and so on)
  is stored as **Records**.
- Unstructured blob bytes (artifact content, warehouse exports, agent
  session state, and so on) are stored as **Objects**.

Domain services — Artifacts, Config, Workers, and any future layered
services — are thin wrappers that call Storage to persist their state.
A conforming implementation of Storage is therefore sufficient to back
the entire Nightshift control plane.

This design is deliberately two-primitive. A single abstraction would
either force opaque bytes on structured state (losing queryability) or
push row-at-a-time semantics onto blob storage (losing scale). Two
primitives cover both comfortably and let implementations back each half
with an appropriate engine: SQL/KV for Records, object storage for
Objects.

## 2. Record Model

A `Record` is a single structured entry in a named collection.

### Identity

- `(collection, key)` is the primary identity.
- `collection` is an opaque string. The spec does **not** declare which
  collections exist. Domain services and implementations name their own
  (`"runs"`, `"events"`, `"skills"`, `"user_configs"`, `"connectors"`,
  `"billing_events"`, `"schedules"`, `"artifacts"`, etc.). A naming
  convention across domain services is recommended but not mandated —
  see §10 for the suggested convention.

### Shape

- `attributes: map<string, string>` — **indexed** metadata. A conforming
  implementation MUST make attributes queryable via `ListRecords`.
- `data: bytes` — **opaque** payload. Storage MUST preserve the bytes
  exactly and MUST NOT inspect, validate, or interpret them.
- `content_type: string` — a hint for decoders (e.g.
  `application/x-protobuf`, `application/json`, `text/plain`). Storage
  does not act on the hint; callers use it to decode `data`.
- `version: int64` — a monotonic server-assigned version number, set
  on every successful write. Used with `if_version` for optimistic
  concurrency.
- `created_at`, `updated_at` — server-assigned timestamps.

### Attribute vs data split

Fields that callers need to **filter on** or **order by** go in
`attributes`. Fields that are part of the entity payload but not
queryable go in `data`. Example for a `runs` collection:

| Field | Location |
|---|---|
| user_id | `attributes["user_id"]` |
| status | `attributes["status"]` |
| created_at_bucket (e.g. `"2026-04"`) | `attributes["created_at_bucket"]` |
| prompt | inside `data` (serialized) |
| session_id | inside `data` |
| error | inside `data` |

Since attribute filtering is exact-match only in v1 (§9), callers who
want range queries on time must materialize a coarse-grained bucket
attribute (month, day) and filter on that.

## 3. Record Lifecycle

### `PutRecord`

- Writes or overwrites a record.
- If `if_version` is set:
  - `if_version == 0` means "the record must not exist". If it does,
    return `ALREADY_EXISTS`.
  - `if_version > 0` means "the current version must equal this value".
    If it does not, return `FAILED_PRECONDITION`.
- On success, the returned `Record.version` is strictly greater than
  the previous version (or 1 for first write), and
  `Record.updated_at` is the write time.
- `idempotency_key` is optional. If set, the server MUST deduplicate
  mutations from the same caller within an implementation-defined
  window (recommended: at least 24 hours), returning the original
  `PutRecordResponse` for replays of the same key.

### `GetRecord`

- Returns the current state of a record. If it does not exist, returns
  `NOT_FOUND`.

### `DeleteRecord`

- Removes a record. If it does not exist, returns `NOT_FOUND`.
- If `if_version` is set and does not match, returns
  `FAILED_PRECONDITION`.

### `ListRecords`

- Returns records from a single `collection`, optionally filtered by
  exact-match on one or more `attributes`, paginated, and ordered.
- `page_size` is a hint. The server MAY cap it. Callers MUST be
  prepared for the response to contain fewer records than requested
  even when more results are available.
- `next_page_token` is opaque. Clients pass it verbatim to the next
  call. Empty means there are no further results.
- `order_by` is a server-interpreted string. Conventional format is
  `"<field> <asc|desc>"` (e.g. `"created_at desc"`). An empty
  `order_by` means implementation-default order. Implementations SHOULD
  support at least `created_at asc` and `created_at desc`.

### Consistency and atomicity

- Single-record operations (`PutRecord`, `GetRecord`, `DeleteRecord`)
  MUST be atomic and strongly consistent: a successful write is
  immediately visible to subsequent reads by the same or any other
  caller.
- There are **no multi-record transactions in v1**. Callers that need
  atomicity across multiple records must design around it (for example,
  single-record state machines, idempotent operations, or saga-style
  compensations).
- `ListRecords` results are eventually consistent relative to concurrent
  writes. Implementations SHOULD provide read-your-own-writes for the
  same session but are not required to.

## 4. Object Model

An `Object` is a blob of bytes identified by `(bucket, key)`.

### Identity

- `(bucket, key)` is the primary identity.
- `bucket` is a pre-created logical grouping. The spec does **not**
  provide bucket CRUD in v1 — operators create buckets out of band.
  Implementations map buckets to S3 buckets, GCS buckets, MinIO
  buckets, filesystem directories, or any other backend-specific
  concept.
- `key` MAY contain slashes and MAY be multi-segment (for example
  `artifacts/abc123/report.pdf`).

### Shape

- `content_type` — MIME type; set at `InitiateObjectUpload` time.
- `size_bytes` — expected at `InitiateObjectUpload`, authoritative
  after `FinalizeObjectUpload`.
- `etag` — implementation-defined content identity (typically the
  backend's ETag or a content hash).
- `metadata: map<string, string>` — user-defined metadata preserved
  verbatim by the implementation.
- `upload_state` — PENDING, READY, or FAILED. See §5.
- `created_at`, `updated_at` — server-assigned timestamps.

## 5. Object Lifecycle

Objects use a **two-step signed-URL upload flow**. Storage never carries
blob bytes over the gRPC wire.

### Upload

1. **`InitiateObjectUpload`** — The client declares intent (`bucket`,
   `key`, `content_type`, `size_bytes`, `metadata`). The server:
   - Registers the object in `UPLOAD_STATE_PENDING`.
   - Generates a presigned URL the client will PUT the bytes to
     directly.
   - Returns the URL, any required `upload_headers`, and an
     `expires_at`.
2. **Client PUT** — The client sends a single HTTP PUT to `upload_url`
   with the declared headers. This is **not a Storage RPC** — it's a
   direct upload to the backend (S3/GCS/MinIO/etc).
3. **`FinalizeObjectUpload`** — The client notifies Storage that the
   PUT succeeded. The server verifies the object is present at the
   backend, populates `etag` and authoritative `size_bytes`, and
   transitions the upload state to `UPLOAD_STATE_READY`.

### Failure and orphans

- If the client never calls `FinalizeObjectUpload`, the object remains
  PENDING. Implementations MUST garbage-collect PENDING objects after
  an implementation-defined TTL (recommended: at least 24 hours).
- If the backend reports the PUT failed at finalization time, the
  server MAY transition the object to `UPLOAD_STATE_FAILED` and return
  `FAILED_PRECONDITION`. FAILED objects MUST be garbage-collected the
  same way as PENDING.
- Calling `FinalizeObjectUpload` on an object that is not PENDING
  returns `FAILED_PRECONDITION`.

### Read

- **`GetObject`** — returns only the metadata. No bytes.
- **`GetObjectDownloadURL`** — returns a presigned URL the client GETs
  directly. The presigned URL is short-lived; `expires_at` is
  authoritative.
- **`ListObjects`** — paginated list over a `bucket`, optionally
  filtered by `key_prefix`. Returns metadata only.

### Delete

- **`DeleteObject`** — removes the object and its bytes. Idempotent
  in the sense that a subsequent `DeleteObject` returns `NOT_FOUND`.

## 6. Buckets

- Pre-created by operators out of band. No `CreateBucket`,
  `DeleteBucket`, or `ListBuckets` RPC in v1.
- Implementations may map bucket names to backend-native concepts any
  way they choose. A common mapping:
  - `bucket="artifacts"` → an S3 bucket or a key prefix in a shared bucket.
  - `bucket="warehouse"` → a separate S3 bucket or prefix.
  - `bucket="claude-state"` → a separate S3 bucket, prefix, or even an
    EFS-backed path depending on the implementation.
- A request to an unknown bucket returns `NOT_FOUND`.

## 7. Auth and Metadata Keys

Storage is authenticated. The full auth wire format is defined in the
Secrets chunk (chunk 5); Storage chunk 2 only reserves the following
gRPC metadata keys:

- **`nightshift-caller-id`** — the authenticated principal. Storage
  does not inspect this directly in v1; domain services above Storage
  use it for their own scoping.
- **`nightshift-impersonate-user-id`** — set by service-account callers
  (for example Workers running a scheduled job) to indicate they are
  acting on behalf of another user. Storage does not inspect this in
  v1; domain services use it for the same scoping.

v1 Storage has **no `user_id` field** in any request or response. Per-
caller isolation is an implementation concern (§8).

## 8. Isolation — Non-Normative Operator Guidance

The spec does not mandate a specific isolation model. Real deployments
will need one. Recommended patterns, in order of typical preference:

1. **Bucket policies.** Give each tenant/user their own bucket, enforce
   per-bucket access in the backend. Strongest isolation.
2. **Prefix-scoped credentials.** Use a shared bucket but issue
   scoped-prefix credentials per tenant (e.g. IAM policies limiting
   access to `s3://shared/{user_id}/*`).
3. **Collection prefix conventions.** For Records, prefix collection
   names with tenant ID (e.g. `t-123.runs` instead of `runs`). Weaker
   than backend-enforced isolation but trivial to implement.
4. **Virtual buckets / collections.** Implementation transparently
   prepends the tenant ID to bucket/collection names based on the
   `nightshift-caller-id` metadata. Simplest for callers; requires
   implementation discipline.

Implementations SHOULD document which of these they use and SHOULD NOT
silently expose cross-tenant data.

## 9. Pagination and Filtering

- **Pagination** follows [AIP-158](https://google.aip.dev/158). Tokens
  are opaque strings with implementation-defined encoding. Clients
  pass them verbatim.
- **Filtering** in v1 is **exact-match only** via `attribute_filters`
  on Records and `key_prefix` on Objects. No range queries, no JOINs,
  no full-text search.
  - **Workaround for range queries on time**: materialize a bucketed
    attribute (e.g. `attributes["created_at_day"] = "2026-04-10"`) and
    filter on it.
  - **Workaround for multi-field filters**: multiple entries in
    `attribute_filters` are combined with logical AND.

## 10. Collection Naming Convention

The spec does not enforce a naming convention for `collection`, but
recommends that domain services name their collections with their
service name as a prefix to avoid cross-service collision. Example
recommendation:

| Service | Collection names |
|---|---|
| Workers | `workers.runs`, `workers.events`, `workers.schedules` |
| Config | `config.agents`, `config.skills`, `config.connectors`, `config.user_configs` |
| Artifacts | `artifacts.items`, `artifacts.permissions` |
| Secrets | `secrets.metadata` (if used) |
| Billing | `billing.events`, `billing.rates` |

Implementations MAY enforce this convention, MAY normalize it, or MAY
ignore it.

## 11. Idempotency

Mutations accept an optional `idempotency_key` field on:

- `PutRecord`
- `InitiateObjectUpload`

When set, the server MUST deduplicate mutations from the same caller
within an implementation-defined window (recommended: at least 24
hours). A replay with the same key and same request body returns the
original response. A replay with the same key but a different request
body returns `ALREADY_EXISTS`. The caller identity used for
deduplication is `nightshift-caller-id` (if present) or the transport
connection identity.

## 12. Error Code Mapping

Storage uses standard gRPC status codes (`google.rpc.Status`). The
following mapping is normative:

| Code | Meaning in Storage |
|---|---|
| `OK` | Success. |
| `NOT_FOUND` | No such record, object, or bucket. |
| `ALREADY_EXISTS` | `PutRecord` with `if_version=0` on an existing record; idempotency-key replay with a different body. |
| `INVALID_ARGUMENT` | Missing required field (`collection`, `key`, `bucket`); malformed pagination token; invalid `order_by`. |
| `FAILED_PRECONDITION` | `if_version` mismatch; `FinalizeObjectUpload` on a non-PENDING object; attempt to finalize when backend reports the PUT failed. |
| `PERMISSION_DENIED` | Caller is not authorized for the bucket, collection, or key. |
| `RESOURCE_EXHAUSTED` | Quota exceeded (implementation-defined). |
| `UNAVAILABLE` | Backend is temporarily unreachable. Clients SHOULD retry with backoff. |
| `INTERNAL` | Implementation bug or unexpected backend error. |

Implementations MAY add richer detail via `google.rpc.ErrorInfo` in
`google.rpc.Status.details`, but MUST NOT change the primary code.

## 13. Versioning

The `nightshift.v1` package is **pre-release stable** until chunk 6
(Workers) lands. Between chunks 2 and 6, only additive changes are
made — no field renumbering, no method removal. `buf breaking` is
configured at `FILE` level to enforce this.

After chunk 6 merges, `v1` becomes stable. Breaking changes require a
`v2` package.

## 14. Out of Scope for v1

The following are explicitly not part of Storage v1. Some may arrive
in a later version; others are permanent non-goals.

- **Multi-record transactions.** Future work; may become a `BatchWrite`
  or `Transact` RPC in v2.
- **Range queries, secondary indexes, JOINs, full-text search** on
  Records. Workarounds exist via bucketed attributes; real query power
  is a v2 concern.
- **Bucket CRUD.** Operators create buckets out of band.
- **Multipart upload mechanics, copy, move.** Implementation concerns.
- **Streaming RPCs for blob bytes.** Rejected in favor of signed URLs
  at the RPC boundary.
- **Warehouse / time-series / retention policies.** Implementation
  concerns outside the spec.
- **Quotas and rate limiting.** Implementation concerns; Storage
  surfaces them via `RESOURCE_EXHAUSTED`.
