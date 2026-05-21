# cr0n UI Specification

Frontend for the cr0n-a agent orchestration platform. Provides the user-facing interface for running Claude agents, managing configurations (agents, skills, connectors), viewing artifacts, scheduling automated runs, and monitoring usage/billing.

## Architecture Overview

**Framework**: Next.js 15 (App Router, React 19, Turbopack)  
**Language**: TypeScript 5 (strict mode)  
**Styling**: Tailwind CSS 4 + shadcn/ui (New York style)  
**Auth**: better-auth (email/password, session-based)  
**UI Database**: SQLite via better-sqlite3 — auth tables only  
**Backend**: cr0n-a FastAPI service (single source of truth for all domain data)  
**Real-time**: Server-Sent Events (SSE) for live run streaming  
**Package Manager**: Bun

### Core Principle: cr0n-a Is the Source of Truth

The UI is a **thin view layer** over cr0n-a. All domain data, 
runs, events, agents, skills, connectors, artifacts, schedules, billing,
lives in cr0n-a and is fetched on demand. The UI database stores **only** better-auth session/account data. 
No conversations, messages, tasks, or domain state is duplicated in the UI.

If the UI needs data that cr0n-a doesn't expose, the answer is a new cr0n-a endpoint, not a UI-side table.

### Terminology

The user-facing term is **"Tasks"**. A task maps to a cr0n-a **run** (or a session of runs). 
The UI labels, navigation, and components use "Tasks". 
"runs" is an implementation detail. Never change the designer's labels.

### System Boundary

```
Browser ──→ Next.js (better-auth sessions, API proxy routes, SSR)
                │
                ├── SQLite (auth only: users, sessions, accounts)
                │
                └── cr0n-a (FastAPI) ← single source of truth
                      ├── Runs + Sessions (execution, event streams)
                      ├── Agents (per-user sub-agent definitions)
                      ├── Skills (per-user SKILL.md files)
                      ├── Connectors (OAuth integrations, MCP servers)
                      ├── Artifacts (S3 objects + K8s app deployments)
                      ├── Schedules (K8s CronJobs)
                      └── Billing (Prometheus-based usage metering)
```

### Authentication Flow

> **Canonical auth doc**: the full four-layer stack (Kubernetes → OpenBao → cr0n-a → cr0n-ui) lives in `src/cr0n-a/SPEC.md` under **Authorization Policy Stack (implemented)**. The summary below covers only the UI-specific pieces; for policy attachments, K8s service accounts, JWT validation, and the admin-vs-user matrix, read that section.

Production auth is **OpenBao OIDC via better-auth's `genericOAuth` plugin**. Email/password is still enabled behind `AUTH_EMAIL_PASSWORD_ENABLED` as a rollback hatch during migration; it will be removed.

1. User clicks **"Sign in with OpenBao"** on `/login`.
2. better-auth initiates an authorization-code flow against OpenBao:
   - Browser 302s to `https://auth-cr0n.ns-apps.com/ui/vault/identity/oidc/provider/cr0n/authorize?…` with PKCE + state + nonce.
   - User logs in at OpenBao (userpass). OpenBao redirects back to `/api/auth/oauth2/callback/openbao?code=…&state=…` on the cr0n UI origin.
3. better-auth server-side exchanges the code for `{id_token, access_token}` at the in-cluster token endpoint (`http://openbao.cr0n.svc:8200/v1/identity/oidc/provider/cr0n/token` in prod; the public auth host in local dev).
4. `getUserInfo` decodes the id_token locally (OpenBao's `/userinfo` endpoint isn't exposed publicly) and returns `{id: sub, name, email, role}`. better-auth creates the `user` + `account` rows; the id_token is stored on `account.id_token`.
5. A database hook (`databaseHooks.account.create.after` + `.update.after`) re-decodes the stored id_token on every login and mirrors the `groups` claim into `user.role` via Drizzle: `role="admin"` if groups contains `admin`, else `role="user"`. This is the single source of truth for admin UX gating in the UI.
6. Session cookie is set; browser lands on `/tasks`.
7. API proxy routes (`lib/server/cron-proxy.ts`) call cr0n-a with an `Authorization: Bearer <id_token>` header — cr0n-a validates the signature against OpenBao's JWKS and enforces `groups`-based admin routes. *(Step 4 of the auth rollout; in flight — see the plan file.)*

**No refresh tokens.** OpenBao 2.2 does not issue a `refresh_token` in authcode flow. When the 1h id_token expires, the proxy triggers a silent `prompt=none` re-authorization against OpenBao's 24h session cookie; if that cookie is also expired, the UI redirects to `/login`.

**Identity shape to know**:
- `user.id` — better-auth-generated opaque ID (NOT the OIDC sub).
- `user.email` — synthesized as `<sub>@openbao.local` unless the entity has email metadata set in OpenBao.
- `user.name` — the OpenBao entity name (e.g. `gianni`, `alice`).
- `user.role` — `"admin"` or `"user"`, derived from the id_token's `groups` claim.
- `account.provider_id="openbao"`, `account.account_id=<OIDC sub>`, `account.id_token=<current JWT>`.

`requireAdminAuth()` in `lib/server/tenant.ts` checks `user.role === "admin"` for admin-only UI surfaces. No `admin` claim? Fall through to 403.

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `BETTER_AUTH_SECRET` | Auth signing key (min 32 chars) |
| `BETTER_AUTH_URL` | Auth origin URL |
| `CRON_API_URL` | cr0n-a base URL (e.g., `http://localhost:8080`) |
| `TENANT_ID` | Organization/tenant ID |
| `TENANT_NAME` | Organization display name |
| `OIDC_CLIENT_ID` | OIDC client ID (from the `openbao-oidc-client` K8s secret). Setting this **and** `OIDC_CLIENT_SECRET` enables the genericOAuth plugin. |
| `OIDC_CLIENT_SECRET` | OIDC client secret (same secret). Never log. |
| `OIDC_AUTHORIZATION_URL` | Browser-facing authorize endpoint at `auth-cr0n.ns-apps.com/ui/vault/identity/oidc/provider/cr0n/authorize`. |
| `OIDC_TOKEN_URL` | Token exchange endpoint. In-cluster in prod; public in local dev. |
| `OIDC_USERINFO_URL` | Userinfo endpoint. Configured for completeness; the plugin uses `getUserInfo` to decode the id_token locally instead of calling `/userinfo`. |
| `AUTH_EMAIL_PASSWORD_ENABLED` | `"false"` disables the email/password path. Default `true` during rollout. |
| `CR0N_MASCOT` | Optional. Picks the animated SVG mascot family for the right-panel prompt input. Valid values: `clawd`, `gemini`, `codex`. Unset/empty/unknown → no mascot rendered. Currently only `clawd` ships with assets. Surfaced to client code via the `env` block in `next.config.mjs`, so it does NOT need the `NEXT_PUBLIC_` prefix. |

---

## 1. Task Console (implemented)

The central interface for interacting with Claude agents. 
Users write prompts from the left panel, execute runs, 
and observe real-time streaming output in the right panel with chain-of-thought rendering.

### 1.1 Conversation Model

A **conversation** (displayed as a "task") is a sequence of runs linked by `session_id`. 
The UI derives conversation threads entirely from cr0n-a, there is no UI-side storage:

- `useChat` hook calls `api.listSessions()` → builds the task list in the left panel
- `useChat.loadMessages(sessionId)` calls `api.listSessionRuns(sessionId)` + `api.getRunEvents(runId)` for each run **in parallel** via `Promise.all` 
which builds the message thread
- Each run's events are rendered directly by `EventStream`, no intermediate markdown conversion

**First message**: User submits prompt from left panel → `sendMessage(null, prompt)` → 
`api.createRun(prompt)` returns the real `session_id` synchronously (cr0n owns session ids) → 
sidebar entry + user/assistant messages inserted keyed on that id → URL routed to 
`/tasks/{session_id}` → SSE subscribed to stream events into the right panel.

**Follow-up messages**: User types in the right panel chat input → `sendMessage(sessionId, prompt)` 
→ `api.createRun(prompt, sessionId)` → backend looks up the latest `claude_session_id` on 
that session and passes it to the worker as `options.resume` → agent resumes with full 
Claude SDK history.

### 1.2 Layout

Two-panel layout (three-panel when artifact viewer is open):
- **Left panel (480px)**: "All tasks" header + prompt input + scrollable task list
- **Right panel (flex-1)**: Active conversation header + message thread + chat input
- **Artifact panel (optional, resizable)**: iframe viewer for deployed artifacts, opened by clicking artifact badges

### 1.2a Prompt Mascot

A small animated SVG mascot hovers above the right-hand prompt input — both the welcome-screen card (`ChatWelcome`) and the in-thread follow-up input (`ChatInput`). It exists purely for character; it doesn't gate or interact with anything.

- **Component**: `src/ui/components/chat/prompt-mascot.tsx` — `PromptMascot({ seed, className })`. Picks a random file from the active mascot family on mount, deferred to a client-side `useEffect` (so SSR renders nothing and avoids a hydration mismatch from `Math.random()` running differently on server vs client).
- **Position**: `absolute bottom-[calc(100%+4px)] right-6 z-10`, 80×80px (`size-20`), with a drop shadow. Sits fully above (outside) the input card border with a 4px gap, anchored to the card's top-right corner. The parent card needs `position: relative`.
- **Reroll trigger** — the `seed` prop drives the `useMemo`/`useEffect` reroll:
  - In `ChatWelcome`, no seed is passed; the component remounts whenever `ThreadPanel` returns to the welcome screen, so each visit picks a fresh mascot.
  - In `ChatInput`, `ThreadPanel` passes `seed={\`${conversationId}:${userMessageCount}\`}`. That means the mascot rerolls when the user switches tasks AND when they send any new follow-up prompt within the same task — `userMessageCount` is the number of `role === "user"` messages, so streaming assistant responses don't trigger a reroll.
- **Family registry** — mascot files are organized by family in `MASCOT_FAMILIES`:
  - `clawd` — 22 animations from [marciogranzotto/clawd-tank](https://github.com/marciogranzotto/clawd-tank), used with explicit permission. Excludes `clawd-disconnected.svg` and `clawd-static-base.svg` (the disconnected vibe doesn't fit a healthy "ready to work" state, and the static base is a non-animated stub).
  - `gemini`, `codex` — reserved keys for future asset sets, currently empty.
- **Env-var gating** — the active family is read from `process.env.CR0N_MASCOT` at module load (Next.js inlines it via the `env` block in `next.config.mjs` so the variable does NOT need the `NEXT_PUBLIC_` prefix). When the value is unset, empty, unknown, or maps to an empty manifest, `pickRandom()` returns `null` and the component renders nothing — no DOM, no fetch, no layout shift. Set `CR0N_MASCOT=clawd` in `.env.local` to enable.
- **Assets** live under `src/ui/public/animations/` (served by Next.js at `/animations/*.svg`). Attribution lives in `public/animations/NOTICE.md`.

### 1.3 Run Lifecycle

```
User writes prompt in left panel input
  → handleNewTask() → sendMessage(null, prompt)
  → api.createRun(prompt) → { run_id, session_id } returned
  → sidebar row + user/assistant messages inserted keyed on session_id
  → URL routed to /tasks/{session_id}
  → SSE subscribes to /runs/{run_id}/events/sse
  → Events stream via RAF batching → EventStream renders chain-of-thought
  → ingest_event backfills claude_session_id on the first event that carries it
  → result.success event → run finalized → SSE closed
```

### 1.4 Event Rendering (Chain of Thought)

Events are rendered using the `EventStream` component (`components/chat/event-stream.tsx`) which uses the `ChainOfThought` component from `@ai-elements/chain-of-thought`. The renderer segments events into:

**Text segments** — Rendered inline as markdown via `MessageResponse` (Streamdown).

**Activity segments** — Collapsible `ChainOfThought` sections grouping consecutive tool calls. Each step shows a tool-specific icon + label + optional description/metadata:

| Tool | Icon | Label | Description |
|------|------|-------|-------------|
| `WebSearch` | Globe | The search query text | — |
| `WebFetch` | Globe | "Fetching {domain}" | Full URL |
| `Bash` | Terminal | Command description or "Running command" | The command text |
| `Read`/`Edit`/`Write` | Eye/Pencil/FileText | "Reading/Editing/Writing {filename}" | Full file path |
| `Grep`/`Glob` | Search/FolderSearch | Search pattern | — |
| `Agent` | Bot | Agent description | — |
| `ToolSearch` | Search | "Looking up tools" | "Found: {tool_names}" (from result) |
| `mcp__cr0n__deploy_app` | Globe | "Deploying app: {name}" | — |
| `mcp__cr0n__deploy_object` | FileUp | "Uploading {filename}" | — |
| `mcp__cr0n__show_preview_artifact` | Monitor | "Preview: {name}" | Breaks the activity group and emits an inline preview segment — renders either an iframe (apps) or an `ObjectViewer` (objects) depending on the `type` param in `tool_use.input` |

**Search results** — WebSearch tool results rendered as rows with Google favicons + title + domain, truncated to 5 with "+N more".

**Artifact badges** — Clickable badges from `mcp__cr0n__deploy_app` results. Clicking opens the artifact viewer panel. Text content parsed for "Artifact ID:" pattern to extract name, URL, public status.

**Inline preview tiles** — When the agent calls `mcp__cr0n__show_preview_artifact`, the event-stream renderer flushes the current activity group and emits a full-width `InlinePreview` segment instead of a chain-of-thought step. The tile has two variants driven by the `type` field in `tool_use.input`:

- **App previews** (`type="app"`): the tile embeds the artifact's `url` in a sandboxed iframe with a 16:10 aspect ratio. The agent passes the `app_url` returned by `deploy_app`.
- **Object previews** (`type="object"`): the tile renders an `<ObjectViewer variant="inline">` sourced from `/api/artifacts/${id}/view`. The agent passes `content_type` (MIME from `deploy_object`); the UI builds the URL itself.

Both variants use a `pointer-events-none` sandboxed content layer plus an absolute click-catcher so the whole tile is a single click target. Clicking it calls `onOpenArtifact(view: ArtifactView)`, promoting the preview into the right-side `ArtifactPanel`. The tool is a no-op on the backend — all metadata comes directly from the `tool_use.input` payload.

**Auto-collapse behavior** — During streaming, the latest activity group stays **open**. When new content arrives (text or new activity), previous groups **auto-collapse**. After completion, all groups are collapsed. Manual user toggles take precedence.

**Result footer** — Shows "Completed — $cost, duration" or "Error" with cost/time from `result.success`/`result.error` events.

### 1.5 Artifact Viewer Panel

When a user clicks an artifact badge or an inline preview tile, a resizable right panel opens. The panel dispatches on the tagged `ArtifactView` union:

- **Header**: Globe / FileText icon + artifact name + optional content-type label (objects) + "Open" link (new tab) + X close button
- **Body (apps)**: `<iframe>` loading the artifact's URL (e.g., `https://xxxx-app-cr0n.ns-apps.com`)
- **Body (objects)**: `<ObjectViewer variant="panel">` rendered at full height
- **Drag handle**: Vertical 1px bar between chat and artifact panels, draggable to resize (25%-80% range)
- **Close**: X button or navigating back dismisses the panel

**Private app previews** — apps deployed with `public: false` (the default) come back from cr0n-a with a relative `app_url` of `/artifacts/{id}/view`, which is the cr0n-a backend route path, not a Next.js route. The UI resolves that through `resolveAppUrl()` in `src/ui/lib/artifacts.ts`, which rewrites cr0n-a-relative paths to the UI proxy path (`/api/artifacts/{id}/view`) so iframes and "Open" links work without needing a public ingress. Absolute URLs (public apps at `https://{id}-app-cr0n.ns-apps.com`) pass through unchanged. The resolver is idempotent and called in every render layer that consumes an app URL — `InlinePreview` (event-stream), `ArtifactPanel` (iframe + header Open link), and the legacy `ArtifactBadge` click handler.

`ArtifactView` is a discriminated union:

```ts
type ArtifactView =
  | { kind: "app"; id: string; name: string; url: string }
  | { kind: "object"; id: string; name: string; contentType: string };
```

#### Object Viewer (`components/chat/object-viewer.tsx`)

`ObjectViewer` is a single dispatcher component that selects a sub-renderer based on MIME type. It takes `{ id, name, contentType, variant?: "inline" | "panel" }` and builds its view URL as `/api/artifacts/${id}/view` (the UI proxy forwards to cr0n-a's permission-gated `/artifacts/{id}/view` endpoint).

| Content type | Renderer | Notes |
|---|---|---|
| `image/*` | `<img>` | Native decode + lazy loading |
| `application/pdf` | `<iframe>` | Chrome's built-in PDF viewer — no PDF.js dep |
| `application/vnd.openxmlformats-officedocument.wordprocessingml.document` (docx) | `DocxView` → `PreviewIframeView` | Fetches `/api/artifacts/{id}/preview` (server-generated HTML companion), renders via `<iframe srcDoc sandbox="allow-scripts">` |
| `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` (xlsx) | `XlsxView` → `PreviewIframeView` | Same pattern as DOCX; preview HTML is a self-contained tabbed multi-sheet viewer |
| `application/vnd.openxmlformats-officedocument.presentationml.presentation` (pptx) | `PptxView` → `PreviewIframeView` | Same pattern; preview HTML renders each slide as a 16:9 card in a vertical scroll list |
| `text/markdown`, `text/x-markdown` | `MessageResponse` (streamdown) | Fetches content client-side, reuses the assistant-message renderer |
| `application/json` | `MessageResponse` in a ` ```json ` code fence | Pretty-prints with `JSON.stringify(…, 2)`; falls back to raw if parse fails |
| `text/csv`, `application/csv` | `<table>` | Tiny inline parser (handles quoted/escaped fields); caps at 200 rows with "+N more" footer |
| `text/html` | Sandboxed `<iframe>` | Same treatment as apps |
| `text/*` (fallback) | `<pre>` | Monospace text view |
| Anything else | Download card | `FileText` icon + filename + "Download" button |

Text-based renderers fetch content client-side via `fetch` with an `AbortController` bound to the component lifetime. A 2 MB size cap applies (checked via `Content-Length` first, then the fetched text length); over-cap files show a "Preview too large — open raw file" fallback.

**DOCX and XLSX previews** use a different data path: the companion HTML is generated server-side at artifact creation time by `create_docx` / `create_xlsx` in cr0n-a and stored at `s3://…/artifacts/{id}/preview.html`. The UI fetches it from `/api/artifacts/{id}/preview` and feeds it to a sandboxed iframe via `srcDoc` with `sandbox="allow-scripts"` (no `allow-same-origin`, so the iframe is in an opaque origin with no cookie/fetch access). If the preview fetch 404s (e.g. for a docx uploaded directly through `deploy_object` without a companion preview), the renderer falls back to the download card.

The `inline` variant caps at `max-h-[400px]` with `overflow-hidden` and an absolute click-catcher so the inline tile is a teaser that promotes to the side panel on click. The `panel` variant is full-height and scrollable.

### 1.6 SSE Streaming Protocol

1. Client opens `EventSource` to `/api/runs/{run_id}/events` (proxied to cr0n-a)
2. cr0n-a replays all historical events immediately (reconnection resilience)
3. Live events stream as they arrive
4. Client tracks `afterIndex` cursor for deduplication
5. Keepalive ping every 30s
6. `event: done` sentinel when run terminates
7. Client uses `requestAnimationFrame` batching for smooth UI updates

### 1.7 cr0n-a API Mapping

| UI Action | cr0n-a Endpoint | Method | Status |
|-----------|----------------|--------|--------|
| List tasks | `/runs/user/list` | GET | Implemented |
| Load conversation | `/runs/history/list?session_id={id}` | GET | Implemented |
| Start run | `/runs/create` | POST | Implemented |
| Run info | `/runs/{run_id}/info` | GET | Implemented |
| Stream events | `/runs/{run_id}/events/sse` | GET (SSE) | Implemented |
| Event history | `/runs/{run_id}/events/history` | GET | Implemented |
| Cancel run | `/runs/{run_id}/interrupt` | POST | Implemented |

### 1.8 StreamEvent Structure

```typescript
type StreamEvent = {
  index: number;      // Sequential event number (0, 1, 2, ...)
  type: string;       // Dot-notation type (e.g., "assistant", "result.success", "system.init")
  timestamp: string;  // ISO 8601
  raw: Record<string, unknown> | unknown[] | string;
};
```

**Event types** (from cr0n-a `serialization.py`):

| Agent SDK Message | StreamEvent `type` |
|---|---|
| `UserMessage` | `user` |
| `AssistantMessage` | `assistant` |
| `ResultMessage` | `result.success` or `result.error` |
| `SystemMessage` | `system.init`, `system.worker_started`, `system.task_started`, `system.task_progress`, `system.task_notification` |
| `RateLimitEvent` | `rate_limit_event` |

**Content blocks** in `raw.content` (for `assistant` and `user` events):
- `text` — `{ type: "text", text: string }`
- `thinking` — `{ type: "thinking", thinking: string, signature: string }`
- `tool_use` — `{ type: "tool_use", id: string, name: string, input: object }`
- `tool_result` — `{ type: "tool_result", tool_use_id: string, content: string | array }`

**Tool result metadata** in `raw.tool_use_result`:
- WebSearch: `{ results: [{ content: [{ title, url }] }], query: string }`
- ToolSearch: `{ matches: string[], query: string }`
- Bash: `{ stdout: string, stderr: string }`

**Result event fields** in `raw`:
- `session_id` — For conversation continuity
- `total_cost_usd`, `duration_ms` — For cost/time display
- `is_error` — Boolean error flag

### 1.9 Performance

Loading a conversation with N runs makes exactly **1 + N API calls** (1 for run list + N for events, all in parallel via `Promise.all`). No duplicate fetches.

The `useChat` hook calls cr0n-a API directly — no compatibility/translation layer. The `loadMessages` function sets `activeConversationId` synchronously before async work and uses a `loadingRef` guard to prevent re-entry from React effects.

---

## 2-7. Future Features

### 2. Agents (implemented)

The page at `/agents` lists per-user sub-agent definitions and edits them in place. Each sub-agent carries a `name`, `description`, system `prompt`, `tools` allowlist, and optional `model` override. The UI is a thin CRUD surface over cr0n-a's `/agents/*` endpoints — no UI-side storage. The list is fetched on mount and on window focus so changes made elsewhere (e.g. another tab) show up without a manual reload.

Rows expand into a detail Dialog with three modes: `view` (read-only), `edit` (editable), and `create` (fresh). `name` is immutable after creation, enforced client-side by a `^[a-z0-9-]+$` validator and server-side by cr0n-a's unique `(name, user_id)` constraint. The tool allowlist is entered via a colocated `TagInput` with chip-style removable tags plus a preset chip row (`Read Write Edit Bash Grep Glob WebSearch WebFetch`) that users can one-click toggle. The model field is a fixed-option `<select>` — `Inherit default`, `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5`. Selecting "Inherit default" persists as `null`. Delete uses an inline two-click confirmation (the "Delete" button swaps to "Really delete?" on first click) rather than a second modal.

#### API mapping

| UI action    | UI proxy route                    | cr0n-a route                  |
|--------------|-----------------------------------|-------------------------------|
| List         | `GET    /api/agents`              | `GET    /agents/list`         |
| Create       | `POST   /api/agents`              | `POST   /agents/create`       |
| Fetch one    | `GET    /api/agents/[id]`         | `GET    /agents/{id}/info`    |
| Edit         | `PUT    /api/agents/[id]`         | `PUT    /agents/{id}/edit`    |
| Delete       | `DELETE /api/agents/[id]`         | `DELETE /agents/{id}/delete`  |

All proxies use the standard `cronProxy` helper in `lib/server/cron-proxy.ts`, which injects `user_id` from the better-auth session so cr0n-a can scope the query and enforce per-user ownership.

#### Worker integration

Sub-agent definitions are stored as rows in cr0n-a's SQLite `agents` table. At run start the K8s worker calls `/internal/config/{user_id}`, and cr0n-a's `build_user_config()` returns the full list as `UserConfig.agents`. The worker converts each entry into a Claude Agent SDK `AgentDefinition` and passes them in memory via `options.agents` — nothing is written to disk. Creating or editing a sub-agent therefore takes effect on the **next** run kicked off by that user; in-flight runs keep their original config.

### 3. Skills (implemented)

The page at `/skills` lists per-user SKILL.md files and edits them in place. Each skill carries a `name`, `description`, and `content` — the full SKILL.md including YAML frontmatter. Same CRUD surface shape as Agents: list + detail Dialog with `view`/`edit`/`create` modes, focus-driven refetch, immutable name post-creation, inline two-click delete confirmation, and `^[a-z0-9-]+$` name validation.

The content field is a plain monospace `<textarea>` — users paste/edit raw markdown; there is no structured frontmatter editor. The placeholder seeds the expected frontmatter shape.

#### API mapping

| UI action    | UI proxy route                    | cr0n-a route                  |
|--------------|-----------------------------------|-------------------------------|
| List         | `GET    /api/skills`              | `GET    /skills/list`         |
| Create       | `POST   /api/skills`              | `POST   /skills/create`       |
| Fetch one    | `GET    /api/skills/[id]`         | `GET    /skills/{id}/info`    |
| Edit         | `PUT    /api/skills/[id]`         | `PUT    /skills/{id}/edit`    |
| Delete       | `DELETE /api/skills/[id]`         | `DELETE /skills/{id}/delete`  |

All proxies use `cronProxy`; user scoping is handled exactly like Agents.

#### Worker integration

Skill rows live in cr0n-a's SQLite `skills` table. At run start `build_user_config()` returns them as `UserConfig.skills`, and the worker **materialises each one to disk** at `.claude/skills/{name}/SKILL.md` in the workspace before starting the Claude agent. The worker also adds `"Skill"` to the allowed tool list when any skill is present. As with Agents, edits apply on the next run; in-flight runs are frozen.

### 4. Connectors (implemented)

Wires the cr0n-a connector catalog into the UI. The page at `/connectors` is a client component that fetches `/api/connectors/catalog` on mount and again whenever the window regains focus, so returning from an OAuth handoff in another tab refreshes the connected state without a manual reload. Connector state is lifted to the page root so the sidebar count and the `ConnectorsView` grid share a single fetch.

The catalog endpoint returns **every** entry in `connector_catalog.py`, not just provisioned ones. Entries with `configured=false` (no admin credentials in OpenBao) render as greyed-out, disabled cards under the "All" tab and are hidden from the "Connected" / "Available" tabs. Configured entries behave normally.

#### API mapping

| UI action        | UI proxy route                                                    | cr0n-a route                              |
|------------------|-------------------------------------------------------------------|-------------------------------------------|
| Load catalog     | `GET  /api/connectors/catalog`                                    | `GET  /connectors/catalog`                |
| Connect (OAuth)  | `GET  /api/connectors/[name]/authorize` (302 → OAuth provider)    | `GET  /connectors/{name}/authorize`       |
| Save PAT         | `POST /api/connectors/[name]/token`                               | `POST /connectors/{name}/token`           |
| Disconnect       | `POST /api/connectors/[name]/disconnect`                          | `POST /connectors/{name}/disconnect`      |

The catalog, token, and disconnect proxies use the standard `cronProxy` helper. The authorize proxy is handled separately: it `fetch`es cr0n-a with `redirect: "manual"`, captures the upstream `Location` header, and re-emits a 302 from the UI origin so the browser follows the chain (UI proxy → OpenBao → OAuth provider) cross-origin to the provider domain. The Connect button does a full-page navigation (not a `fetch`) so the redirect chain is honored end-to-end.

#### Authentication types

`ConnectorCatalogEntry.auth_type` drives the Connect UX:

- **`"oauth"`** — the Connect button does a full-page navigation to `/api/connectors/{name}/authorize`, which kicks off the OAuth handshake via OpenBao.
- **`"static_token"`** — no Connect button. The detail modal renders an "Access token" section with a textarea; on Save the UI POSTs the raw token to `/api/connectors/{name}/token`, which forwards it to cr0n-a, which writes it to OpenBao KV at `cr0n/tokens/{user_id}/{name}`. Used today for MotherDuck (PAT-based).

Disconnect uses the same endpoint for both auth types; cr0n-a picks the right teardown path (OAuth cred vs KV delete) based on the catalog entry.

#### OAuth callback target

cr0n-a's `/connectors/callback` currently redirects to its own `/test?connected={name}` page after exchanging the code and storing the token in OpenBao. The user manually navigates back to the UI `/connectors` page; on mount the page refetches, and the connector flips to connected. Wiring the callback to land back in the UI is a follow-up.

#### Display metadata

cr0n-a's `ConnectorCatalogEntry` carries `{id, name, description, configured, connected, mcp_url, auth_type}`. The page enriches each entry with `{iconUrl, category, developer}` from a local `CONNECTOR_DISPLAY` lookup keyed on connector `name`, covering every catalog provider. Unknown connectors fall back to a generic Plug icon and "Other" category, and `ConnectorIcon` swaps to the Plug fallback on `<img>` `onError` so any missing icon (e.g. simpleicons.org 404s for plaid/smartsheet/zoominfo) still renders cleanly.

#### Run-time integration

Once a user has connected a connector, cr0n-a's `build_user_config()` automatically picks up the token — either from the OpenBao `oauthapp` plugin (`{user_id}-{connector_name}`) or from KV (`cr0n/tokens/{user_id}/{connector_name}`) depending on `auth_type` — and includes the connector in `mcp_servers` + `allowed_mcp_tools` for every new run that user kicks off. The UI does nothing extra to make this work — the agent gets the new tools on the next run.
### 5. Artifacts (implemented)

The page at `/artifacts` is the user's library of everything their agents have produced — apps deployed via `deploy_app` and object uploads via `deploy_object`, unified into one grid. Backed by cr0n-a's `/artifacts/*` endpoints; list is fetched on mount and on window focus so new runs that produce artifacts populate without a manual reload.

**Filtering**: a category chip row (`All / Apps / Docs / Images / Data / Code / Other`) with per-chip counts. Chips are derived from `type` + `content_type` via `categorizeArtifact()` in `lib/artifacts.ts`:

| Chip    | Matches                                                                   |
|---------|---------------------------------------------------------------------------|
| Apps    | `type === "app"`                                                          |
| Images  | `content_type` starts with `image/`                                       |
| Data    | `text/csv`, `application/csv`, `application/json`, `.xlsx` MIME           |
| Docs    | `application/pdf`, `text/markdown`, `text/plain`, `.docx`/`.pptx` MIME    |
| Code    | `text/html`, remaining `text/*`, XML                                      |
| Other   | everything else                                                           |

A text search input filters on `name` and `description` client-side.

**Open UX**: clicking a card opens a fullscreen `Dialog` (95vw × 92vh) with a header bar and a content area:
- Apps: `<iframe>` loaded via `resolveAppUrl(app_url)` (private apps pass through the `/api/artifacts/{id}/view` proxy; public apps hit `https://{id}-app-cr0n.ns-apps.com` directly). Sandboxed with `allow-scripts allow-same-origin allow-popups allow-forms allow-downloads`.
- Objects: `<ObjectViewer variant="panel">` — the same MIME-dispatching component the chat stream uses for inline previews.

**Header actions** (in order): inline-editable name (Enter/blur saves, Escape cancels), click-toggle Public/Private badge, "Open in new tab" link, `MoreVertical` dropdown, X close.

**Menu actions**:
- **Rename** — `PUT /api/artifacts/{id}` with the new `name`.
- **Make public / Make private** — `PUT /api/artifacts/{id}` with `public: bool`. When an **app** flips from private to public, cr0n-a reconciles the K8s ingress so the external `https://{id}-app-cr0n.ns-apps.com` hostname becomes reachable (and the inverse tears it down).
- **Copy link** — for public apps copies the absolute `app_url`; for everything else copies the UI-origin URL that renders the viewer.
- **Download** — objects only; `GET /api/artifacts/{id}/download` streams through `cronBinaryProxy` so `Content-Disposition` is preserved.
- **Delete** — `DELETE /api/artifacts/{id}`, two-click inline confirm (same pattern as Skills/Agents).

**Sharing (v1 scope)**: the Public/Private toggle is the only sharing surface today. Per-user grants (`POST /artifacts/{id}/share`, `DELETE /artifacts/{id}/share/{user_id}`) have UI proxies wired but are not yet exposed in the dialog — cr0n-a does not yet expose a list-grants endpoint (`GET /artifacts/{id}/shares`), which the UI would need to render current recipients. That endpoint plus the grants panel is a v1.1 follow-up.

#### API mapping

| UI action          | UI proxy route                                  | cr0n-a route                                |
|--------------------|-------------------------------------------------|---------------------------------------------|
| List               | `GET    /api/artifacts`                         | `GET    /artifacts/list`                    |
| Fetch one          | `GET    /api/artifacts/[id]/info`               | `GET    /artifacts/{id}/info`               |
| View (binary)      | `GET    /api/artifacts/[id]/view`               | `GET    /artifacts/{id}/view`               |
| Preview (HTML)     | `GET    /api/artifacts/[id]/preview`            | `GET    /artifacts/{id}/preview`            |
| Download           | `GET    /api/artifacts/[id]/download`           | `GET    /artifacts/{id}/download`           |
| Rename / visibility| `PUT    /api/artifacts/[id]`                    | `PUT    /artifacts/{id}/edit`               |
| Delete             | `DELETE /api/artifacts/[id]`                    | `DELETE /artifacts/{id}/delete`             |
| Share with user    | `POST   /api/artifacts/[id]/share`              | `POST   /artifacts/{id}/share`              |
| Unshare user       | `DELETE /api/artifacts/[id]/share/[userId]`     | `DELETE /artifacts/{id}/share/{user_id}`    |

The view route uses `cronBinaryProxy` (preserving `Content-Type`/`Content-Length`/`Content-Disposition`/`Cache-Control`); everything else goes through the standard `cronProxy` which injects `user_id` from the better-auth session.
### 6. Schedules (implemented)

The page at `/schedule` is a two-pane surface: the left pane lists the user's cron-scheduled prompts; the right pane is one of `idle` (no selection), `create`, `view`, or `edit`. A schedule is a `{prompt, cron, timezone, enabled}` tuple — when its cron ticks, cr0n-a's K8s CronJob POSTs to `/internal/runs/create` with the prompt and the user's default config, producing a run that appears in the Tasks list.

Row display: enabled/paused dot, truncated first line of the prompt, humanised cron (`cronstrue`) + IANA timezone, created date. Search filters client-side on prompt/cron.

**Detail view**: enabled badge, created date, humanised cron, the raw cron + timezone in monospace, and a read-only pre-formatted view of the prompt. Below the prompt, a **Runs** section aggregates every session this schedule has fired (backed by `GET /schedule/{id}/runs`, refetched on window focus). Each row shows a status dot, the prompt's first line, the last-activity timestamp, and the run count if >1. Clicking a row navigates to `/tasks/{session_id}` — the same route the Tasks list uses — so users land on the full event stream of that specific fire. The list is client-side capped at 50 rows with a footer indicating how many older fires are hidden. Header actions are **Pause/Resume**, **Edit**, and **Delete** (two-click inline confirm).

**Form**: `Prompt` textarea, `Runs` editor — a **sentence-builder** (Every N minutes / Every hour / Every day / Every week / Every month) with a day-of-week picker + Weekdays/Weekends/Every-day presets for the weekly case, and month-day + time-of-day pickers for the others — plus an **Advanced** toggle exposing the raw 5-field cron input with `cronstrue` live preview + validity indicator. Existing schedules rehydrate into the sentence-builder when their cron fits one of the canonical patterns; anything else opens in Advanced. An IANA `Timezone` `<select>` (populated from `Intl.supportedValuesOf("timeZone")` with UTC prepended so the default matches a visible option), and an Enabled/Paused toggle round out the form.

**Run invoker tracking**: every run now carries `invoker_type` + `invoker_id`. `invoker_type="user"` means a user submitted the run (`invoker_id = user.id`); `invoker_type="schedule"` means a K8s CronJob fired it (`invoker_id = schedule.id`). The CronJob payload stamps these when it `POST`s to `/internal/runs/create`, the public `/runs/create` stamps them server-side from the auth'd user, and the runs-list detail query joins on `(invoker_type="schedule", invoker_id=scheduleId, user_id)`.

**Defers for v1** (require cr0n-a follow-ups):
- `agent_ids` / `skill_ids` / `connector_ids` selectors — schedules persist empty arrays, inheriting whatever `build_user_config()` gives the user on a manual run.
- Next-run / last-fired display — `ScheduleInfo` doesn't carry these fields.
- Manual-trigger button — no `POST /schedule/{id}/trigger` endpoint yet.

#### Worker-callable MCP tool: `mcp__cr0n__create_schedule`

An agent can schedule follow-up runs during a task by calling `mcp__cr0n__create_schedule` (wired alongside the artifact tools in `cr0n-a/artifact_tools.py`, sharing the `cr0n` MCP server namespace). Tool input: `{prompt: str, cron: str, timezone?: str = "UTC", enabled?: bool = true}`. The handler `POST`s to `/internal/schedules/create` with the caller's `user_id` + `run_id` + worker secret; the internal route delegates to `scheduler.create()` so K8s CronJob provisioning is shared with the UI path — no duplicated logic. Returns the new schedule id + humanised cadence in the tool result so the agent can report back.

#### API mapping

| UI action          | UI proxy route                        | cr0n-a route                     |
|--------------------|---------------------------------------|----------------------------------|
| List               | `GET    /api/schedule`                | `GET    /schedule/list`          |
| Create             | `POST   /api/schedule`                | `POST   /schedule/create`        |
| Fetch one          | `GET    /api/schedule/[id]`           | `GET    /schedule/{id}/info`     |
| Edit (rename/cron/timezone/pause) | `PUT    /api/schedule/[id]` | `PUT    /schedule/{id}/edit`     |
| Delete             | `DELETE /api/schedule/[id]`           | `DELETE /schedule/{id}/delete`   |
| List runs fired    | `GET    /api/schedule/[id]/runs`      | `GET    /schedule/{id}/runs`     |

All proxies use `cronProxy` which injects `user_id` from the better-auth session. cr0n-a's routes enforce per-user ownership (`Depends(get_current_user)`).
### 7. Billing — Not yet implemented.

---

## 8. Navigation & Route Structure

### 8.1 Sidebar

| Icon | Label | Route | Status |
|------|-------|-------|--------|
| MessageSquare | Tasks | `/tasks` | Implemented |
| CalendarClock | Schedule | `/schedule` | Implemented |
| FileText | Artifacts | `/artifacts` | Implemented |
| Zap | Skills | `/skills` | Implemented |
| Bot | Agents | `/agents` | Implemented |
| Settings | Customize | `/connectors` | Implemented |

### 8.2 Routes

```
app/
├── layout.tsx                              # Root (html/body)
├── login/page.tsx                          # Login/signup (outside auth guard)
└── (dashboard)/
    ├── layout.tsx                          # Auth guard + DashboardShell
    ├── page.tsx                            # Redirect → /tasks
    ├── error.tsx                           # Error boundary
    ├── tasks/
    │   ├── page.tsx                        # ChatLayout (new conversation)
    │   └── [conversationId]/page.tsx       # ChatLayout (existing conversation)
    ├── schedule/page.tsx                   # Two-pane list + create/view/edit
    ├── artifacts/page.tsx                  # Artifact gallery
    ├── connectors/page.tsx                 # Connector gallery
    ├── agents/page.tsx                     # Sub-agent CRUD
    └── skills/page.tsx                     # SKILL.md CRUD
```

### 8.3 API Proxy Routes

```
app/api/
├── auth/[...all]/route.ts                  # better-auth handler
├── agents/
│   ├── route.ts                            # GET → /agents/list, POST → /agents/create
│   └── [id]/route.ts                       # GET/PUT/DELETE → /agents/{id}/{info,edit,delete}
├── skills/
│   ├── route.ts                            # GET → /skills/list, POST → /skills/create
│   └── [id]/route.ts                       # GET/PUT/DELETE → /skills/{id}/{info,edit,delete}
├── schedule/
│   ├── route.ts                            # GET → /schedule/list, POST → /schedule/create
│   └── [id]/
│       ├── route.ts                        # GET/PUT/DELETE → /schedule/{id}/{info,edit,delete}
│       └── runs/route.ts                   # GET → /schedule/{id}/runs
├── artifacts/
│   ├── route.ts                            # GET → /artifacts/list
│   └── [id]/
│       ├── route.ts                        # PUT/DELETE → /artifacts/{id}/{edit,delete}
│       ├── view/route.ts                   # GET → /artifacts/{id}/view (binary proxy)
│       ├── preview/route.ts                # GET → /artifacts/{id}/preview (HTML preview companion)
│       ├── info/route.ts                   # GET → /artifacts/{id}/info
│       ├── download/route.ts               # GET → /artifacts/{id}/download (binary proxy)
│       └── share/
│           ├── route.ts                    # POST → /artifacts/{id}/share
│           └── [userId]/route.ts           # DELETE → /artifacts/{id}/share/{user_id}
└── runs/
    ├── create/route.ts                     # POST → /runs/create
    ├── list/route.ts                       # GET → /runs/user/list
    ├── history/route.ts                    # GET → /runs/history/list
    └── [runId]/
        ├── info/route.ts                   # GET → /runs/{id}/info
        ├── interrupt/route.ts              # POST → /runs/{id}/interrupt
        └── events/
            ├── route.ts                    # GET → /runs/{id}/events/sse (SSE proxy)
            └── history/route.ts            # GET → /runs/{id}/events/history
```

The `/api/artifacts/[id]/view` route uses `cronBinaryProxy` (in `lib/server/cron-proxy.ts`) which preserves `Content-Type`, `Content-Length`, `Content-Disposition`, and `Cache-Control` from upstream — needed so the browser can render images, PDFs, and honor inline vs attachment download semantics.

---

## 9. State Management (implemented)

### 9.1 useChat — Primary Hook (`lib/hooks/useChat.tsx`)

Calls cr0n-a API directly (no compatibility layer). Split into state + actions contexts.

**State**: `conversations`, `messages`, `activeConversationId`, `isStreaming`, `loaded`

**Actions**: `fetchConversations`, `loadMessages`, `deleteConversation`, `sendMessage`, `setActiveConversationId`

**Key patterns**:
- `fetchConversations` → `api.listSessions()` → maps `SessionSummary` to `Conversation`
- `loadMessages(sessionId)` → `api.listSessionRuns()` + `Promise.all(runs.map(r => api.getRunEvents(r.id)))` — parallel fetch
- `sendMessage` → `api.createRun()` → subscribes SSE → RAF-batched event flush
- Loading guard via `loadingRef` + immediate `setActiveConversationId` prevents duplicate fetches
- `!isStreaming` guard in ThreadPanel prevents `loadMessages` from overwriting in-flight SSE data

### 9.2 useTasks — Schedule/Task Hook (`lib/hooks/useTasks.tsx`)

Used by schedule components. Calls `api.fetchTasks()` (compat wrapper over `listSessions`). Polls every 60s.

### 9.3 Shared Utilities (`lib/events/parse.ts`)

- `parseEventType(type)` — Split dot-notation: `"result.success"` → `{ base: "result", sub: "success" }`
- `raw(event)` — Safe extraction of `raw` dict from StreamEvent
- `formatDuration(ms)` — Human-readable duration
- `isResultError(event)` — Check `is_error` or `result.error` type
- `truncate(s, max)` — String truncation with ellipsis
- `titleFromPrompt(prompt)` — Derive title from first line of prompt

---

## 10. Component Architecture (implemented)

### Chat Components (`components/chat/`)

| Component | Purpose |
|-----------|---------|
| `chat-layout.tsx` | Two/three-panel layout: task list + thread + optional artifact viewer |
| `chat-message.tsx` | User (bubble) and Assistant (EventStream) message rendering |
| `chat-input.tsx` | Follow-up prompt input in right panel |
| `chat-welcome.tsx` | Welcome screen with suggestion chips when no conversation selected |
| `event-stream.tsx` | Chain-of-thought event renderer using `@ai-elements/chain-of-thought` |
| `artifact-panel.tsx` | Resizable iframe viewer for deployed artifacts |
| `context-menu.tsx` | Attachment/connector context menu |
| `model-selector.tsx` | Model/agent selector dropdown |

### AI Elements (`components/ai-elements/`)

| Component | Purpose |
|-----------|---------|
| `message.tsx` | Streamdown markdown renderer (`MessageResponse`) |
| `prompt-input.tsx` | Composable prompt input with file attachments |
| `chain-of-thought.tsx` | Collapsible activity step rendering (from `@ai-elements`) |

### Run Components (`components/run/`)

| Component | Purpose |
|-----------|---------|
| `event-renderer.tsx` | Low-level event rendering + `extractAssistantText()` |
| `run-detail.tsx` | Run history viewer with chat-style thread |

---

## 11. Styling

### Themes

| Theme | Background | Accent |
|-------|-----------|--------|
| dark-cool (default) | #171717 | #d7ff64 (lime) |
| dark-warm | #1c1a18 | #d7ff64 |
| light-cool | #fafafa | #059669 |
| light-warm | #faf6f1 | #059669 |
| dracula | #282a36 | #50fa7b |
| nord | #2e3440 | #a3be8c |

### Typography

- **Sans**: Work Sans (body)
- **Mono**: JetBrains Mono (code, terminal)
- **Pixel**: Geist Pixel (branding)

### Status Colors

pending → yellow/amber, running → blue, completed → green, error → red, interrupted → orange

---

## 12. UI Database (SQLite — auth only)

| Table | Purpose |
|-------|---------|
| `user` | User records (better-auth) |
| `session` | Active sessions |
| `account` | Auth provider accounts |
| `verification` | Email verification tokens |

---

## 13. cr0n-a Endpoints (implemented)

### `GET /runs/user/list`

Lists the user's sessions (task list). Groups runs by `session_id`, returns `SessionSummary` with `first_prompt`, `run_count`, `latest_status`, `last_activity`. Sorted by most recent activity.

### `GET /runs/history/list?session_id=X`

Lists all runs in a session, ordered chronologically. Returns `list[RunInfo]` with full cost/token/usage data.

### Schema

`runs` table includes `user_id TEXT NOT NULL DEFAULT 'dev-user'` column for per-user scoping.
