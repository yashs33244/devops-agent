# Nightshift Config — Semantic Contract

This document is the normative behavioral specification of the
`nightshift.v1.Config` service. The proto file at
[`config.proto`](config.proto) is the authoritative wire contract; this
document describes what a conforming implementation must do and why.

## 1. Overview

Config is the Nightshift configuration interface. It persists three
kinds of declarative state — **Agents**, **Skills**, **Connectors** —
and exposes a single resolver, `GetUserConfig`, that workers invoke at
run start to materialize a user's effective runtime configuration
(agents, skills, and authorized MCP servers).

Like Artifacts, Config is a thin domain service over Storage. Every
row it persists is a Storage `Record`; no additional persistence
primitive is defined here. Credentials (OAuth tokens, static bearer
tokens) live in Secrets — Config holds references by convention, not
bytes.

Scope:

- Agent definitions (§2) — per-user sub-agents the agent SDK
  delegates to.
- Skill definitions (§3) — per-user SKILL.md content that is
  hydrated to the run's workspace filesystem.
- Connectors (§4, §5, §6, §7) — the admin-managed MCP server
  registry, its two auth paths, and the transport-from-URL rule.
- Config Dispenser (§8) — the `GetUserConfig` resolver.
- Storage layering (§9) — the `Record` collections Config writes to.
- RBAC (§10) — what "admin" vs "personal" means for each RPC.
- Catalog reconciliation (§11) — startup-time sync between a
  predefined catalog and persisted state.

## 2. Agents

An `Agent` is a sub-agent definition owned by one user.

- `(user_id, name)` is unique; the same user may not have two agents
  with the same `name`. Implementations enforce this via the
  `attributes["user_id"]` + `attributes["name"]` pair or an equivalent
  mechanism.
- `tools` is the list of allowed tool names the sub-agent may invoke.
  Conforming implementations do NOT validate that these are a subset
  of any parent agent's allowed tools; that's a run-time concern.
- `model` is an opaque string the worker interprets (for example
  `"haiku"`, `"sonnet"`, `"opus"`). Empty means run-level default.
- Agents reach the worker via `GetUserConfig`; cr0n-a's programmatic
  `AgentDefinition` passing pattern is preserved by `AgentDefinition`
  messages inside `UserConfig`.

Writes are owner-only. The caller's identity (the implementation's auth
principal) must resolve to the same `user_id` as the target agent.

## 3. Skills

A `Skill` is a SKILL.md content blob owned by one user.

- `(user_id, name)` is unique.
- `content` is the full SKILL.md text, including the YAML frontmatter
  block the agent SDK parses. Config does not inspect or validate the
  frontmatter — corrupted content surfaces as a run-time failure when
  the SDK tries to load the skill.
- At run start the worker hydrates each skill to
  `{workspace}/.claude/skills/{name}/SKILL.md` and enables the `Skill`
  tool. The spec does not prescribe the exact hydration mechanism; it
  prescribes only that `SkillDefinition.content` is passed through
  unchanged.

Writes are owner-only.

## 4. Connectors — shape

A `Connector` is one entry in the MCP server registry. Connectors are
**shared organizational state** — unlike agents and skills, exactly
one Connector with a given `name` exists across the control plane, and
every user shares the same registry.

Connector fields:

- `name` — display identifier, unique across the control plane. Used
  in URLs (for example `:startOAuthFlow`, `:disconnect`), user-facing
  labels, and MCP tool prefixes (`mcp__{name}__*`).
- `description` — free-form human text.
- `auth_type` — `CONNECTOR_AUTH_TYPE_OAUTH` or
  `CONNECTOR_AUTH_TYPE_STATIC_TOKEN`. See §5.
- `mcp_url` — remote MCP server URL. Transport is derived from this
  URL at resolve time; see §7.
- `mcp_allowed_tools` — tool-name wildcards the agent SDK is allowed
  to invoke on this connector (for example `["mcp__github__*"]`).
- `oauth_scopes` — OAuth-only comma-separated scope string.
- `auth_provider_ref` — an optional opaque hint the Secrets backend
  interprets (for example an OpenBao `oauthapp` server name). Empty
  when not needed.

`Connector` carries **no credentials**. Credentials live in Secrets
(chunk 5); Config references them by convention.

## 5. Connector Auth Paths

Two auth paths are supported. An implementation MUST support both.

### OAuth (`CONNECTOR_AUTH_TYPE_OAUTH`)

1. An admin pre-registers the OAuth app by writing
   `client_id` + `client_secret` into the Secrets backend at an
   implementation-specific convention path (a reference
   implementation's path is `secret/<impl>/connectors/{name}`).
2. A user calls `StartConnectorOAuthFlow(user_id, connector_name,
   redirect_url)`. The implementation returns `authorize_url` + an
   opaque `state` token; the caller redirects the user's browser to
   `authorize_url`.
3. The OAuth provider redirects back to `redirect_url` with `code`
   and the `state` token. The implementation's HTTP callback handler
   invokes `CompleteConnectorOAuthFlow(user_id, connector_name, code,
   state, redirect_url)`.
4. The implementation verifies `state`, exchanges `code` for per-user
   tokens, and stores them in Secrets under the per-user OAuth
   credential convention. Tokens are auto-refreshed by the Secrets
   backend.

### Static token (`CONNECTOR_AUTH_TYPE_STATIC_TOKEN`)

1. No admin setup is required; the connector is considered
   `configured = true` from startup.
2. A user calls `SetConnectorStaticToken(user_id, connector_name,
   token)`. The token is stored in Secrets under the per-user
   static-token convention.
3. There is no refresh flow. The token is sent verbatim as
   `Authorization: Bearer <token>` on every MCP request.

### Disconnect

`DisconnectConnector` deletes the per-user credential for either auth
type. Subsequent `GetUserConfig` calls will NOT include the connector
in the user's `mcp_servers`.

## 6. Connector Catalog

`ListConnectors` returns a `ConnectorCatalogEntry` per connector:

- `connector` — the persisted definition.
- `configured` — for OAuth: admin credentials are present in Secrets.
  For static-token: always true.
- `connected` — only populated when the request carries a `user_id`;
  true when that user has a usable credential in Secrets for this
  connector.

Frontends use `configured + connected` to render catalog state (for
example: "greyed out — not configured", "available — click to
connect", "connected").

Listing intentionally includes **all connectors**, including those the
calling user has not authorized. It is a catalog, not a personalized
view.

## 7. MCP Transport Derivation

The MCP transport (SSE vs streamable HTTP) is NOT a field on
`Connector`. It is derived from `mcp_url` at resolve time using the
following rule:

- If `mcp_url` ends with `/sse` (case-sensitive), transport is
  `MCP_TRANSPORT_SSE`.
- Otherwise, transport is `MCP_TRANSPORT_HTTP`.

This keeps the catalog free of transport metadata — the URL is the
source of truth. It is also what the cr0n-a reference implementation
does, and it matches the conventions current MCP providers use
(GitHub's streamable HTTP at `/mcp/`, Linear/Sentry/Asana/Atlassian's
SSE at `/sse`).

If a future provider violates this convention (for example streamable
HTTP on a `/sse` path), the recommended evolution is a new optional
`transport_override` field on `Connector`, preserving backward
compatibility with the derivation rule.

## 8. Config Dispenser (`GetUserConfig`)

`GetUserConfig(user_id)` is the resolver Workers call at run start.
It returns a `UserConfig` containing:

- `agents` — all of the user's `AgentDefinition`s, loaded from the
  `agents` Storage collection.
- `skills` — all of the user's `SkillDefinition`s, loaded from the
  `skills` Storage collection.
- `mcp_servers` — a map keyed by connector name. For each connector:
  1. If the user does NOT have a usable credential in Secrets, the
     connector is SKIPPED — it is not an error.
  2. Otherwise, the implementation constructs an `McpServerConfig`
     with:
     - `transport` derived from `url` per §7.
     - `url` copied from `Connector.mcp_url`.
     - `headers["Authorization"] = "Bearer " + <token>` where
       `<token>` is the fresh credential returned by Secrets.
- `allowed_mcp_tools` — the union of `mcp_allowed_tools` across every
  entry in `mcp_servers`.

`GetUserConfig` is the single point where Config touches Secrets. All
other Config RPCs MUST NOT read credentials — `ListConnectors` only
checks for *existence* of a credential (to populate `connected`), and
it does so via an implementation-defined existence probe that does not
dispense the token value.

Implementations MAY cache `UserConfig` briefly per user to reduce
Secrets backend load, but MUST invalidate on:

- Any mutation of the user's agents or skills.
- Any `SetConnectorStaticToken`, `CompleteConnectorOAuthFlow`, or
  `DisconnectConnector` for the user.
- Any `CreateConnector` or `DeleteConnector` (organizational event —
  invalidate all users' caches).

A zero-cache implementation is always compliant.

## 9. Storage Layering

### Collections

Suggested collection names (see Storage §10 on naming conventions).
Implementations MAY rename but conforming references use these.

- `agents` — one Record per Agent, keyed by agent id. Attributes
  SHOULD include at least `user_id` and `name`.
- `skills` — one Record per Skill, keyed by skill id. Attributes
  SHOULD include `user_id` and `name`.
- `connectors` — one Record per Connector, keyed by connector id.
  Attributes SHOULD include `name` (for the unique-name lookup) and
  `auth_type`.

### Data payloads

`Record.data` holds the serialized entity message
(`application/x-protobuf`). `Record.attributes` is the queryable
subset. Implementations that need other attribute splits may add
them; the listed attributes above are the minimum for the RPCs to
function.

### Credential references

Credentials are NOT Storage Records. They live in Secrets under
conventions the Secrets spec (chunk 5) defines. Config stores
connector metadata and references the credential by
`(connector_name, user_id)`; the Secrets backend is responsible for
the actual path, encryption, lease management, and refresh.

## 10. RBAC

Config enforces two RBAC tiers:

| Operation | Tier | Who |
|---|---|---|
| Agent CRUD | Personal | The owner (`user_id` matches caller's principal) |
| Skill CRUD | Personal | The owner |
| `CreateConnector`, `DeleteConnector` | Organizational | Admin only |
| `ListConnectors` | Read | Any authenticated caller |
| `StartConnectorOAuthFlow`, `CompleteConnectorOAuthFlow`, `SetConnectorStaticToken`, `DisconnectConnector` | Personal | The caller (the `user_id` on the request must match the caller's principal) |
| `GetUserConfig` | System | Workers on behalf of the enclosing run's user; implementations typically authenticate this via a worker-to-control-plane credential distinct from user sessions |

When a personal-tier write is attempted by a non-owner, implementations
SHOULD return `PERMISSION_DENIED`.

When an admin-tier write is attempted by a non-admin, implementations
SHOULD also return `PERMISSION_DENIED`. `NOT_FOUND` is not appropriate
here because connectors are shared and their existence is not
sensitive.

## 11. Catalog Reconciliation

Implementations typically ship with a **predefined connector catalog**
— a hard-coded list of well-known connectors (GitHub, Linear,
Atlassian, MotherDuck, Slack, …) with their `mcp_url`, `auth_type`,
and `mcp_allowed_tools` pre-populated. This is an implementation
concern; the spec does not mandate a catalog.

When a catalog is present, the implementation MUST reconcile it at
startup:

1. For each catalog entry, check if a connector with that `name`
   already exists in Storage.
2. If not, create it. For OAuth entries, require admin credentials in
   Secrets; if absent, SKIP creation (the connector remains
   `configured = false` in `ListConnectors`). For static-token
   entries, always create — no admin setup is needed.
3. Existing connector rows are NOT overwritten. Admins who have
   customized a connector (for example pointing `mcp_url` at a
   self-hosted instance) do not lose their edits on restart.

Catalog reconciliation is a startup-time concern and happens
out-of-band relative to the Config gRPC service; no RPC exposes it.

## 12. Idempotency

`CreateAgent`, `CreateSkill`, and `CreateConnector` accept an optional
`idempotency_key`. Semantics match Storage §11: within an
implementation-defined window, a repeat request with the same
`(caller, idempotency_key)` returns the original response; a repeat
with the same key but a different body fails with `INVALID_ARGUMENT`.

Update, delete, disconnect, and token-setting RPCs are inherently
idempotent at the resource level and do not take a key.

## 13. Error Code Mapping

| gRPC code | When |
|---|---|
| `NOT_FOUND` | Target resource does not exist. |
| `ALREADY_EXISTS` | Unique-name collision (`CreateAgent` / `CreateSkill` with a name the user already uses; `CreateConnector` with a name already taken). |
| `PERMISSION_DENIED` | RBAC tier check failed (see §10). |
| `INVALID_ARGUMENT` | Malformed request; wrong auth flow for connector auth_type (for example `SetConnectorStaticToken` on an OAuth connector); idempotency-key reuse with a different body. |
| `FAILED_PRECONDITION` | OAuth state mismatch on `CompleteConnectorOAuthFlow`; admin credentials missing when starting an OAuth flow on a configured-but-not-provisioned connector. |
| `UNAVAILABLE` | Transient Secrets or Storage backend failure. |

## 14. Versioning

`nightshift.v1.Config` is part of the `v1` package. Additive changes
(new RPCs, new optional fields, new enum values) are permitted
in-package; wire-breaking changes require a `v2` service in a new
proto file.

New `ConnectorAuthType` or `McpTransport` values MUST be treated as
forward-compatible by conforming clients: unknown values fall back to
`*_UNSPECIFIED` and the corresponding row is skipped rather than
rejected.

## 15. Out of Scope for v1

- Multi-tenant RBAC beyond a single admin / personal split.
- Teams, groups, or role inheritance.
- Connector-level rate limiting or quotas.
- Sharing agents or skills between users.
- History or versioning of agent/skill/connector rows beyond the
  Storage `Record` version.
- A programmatic catalog-publication API (catalogs are baked into the
  implementation).
- Non-MCP connector types.
- Connector secret rotation signals (the Secrets interface owns this).
