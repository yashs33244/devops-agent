// Proxy + adapter layer that translates cr0n-shape REST paths to
// nightshift's /v1/* proto-canonical surface. The UI was ported from
// cr0n verbatim; every app/api/*/route.ts handler still calls
// cr0n-shape paths like `/runs/user/list`, `/artifacts/{id}/info`.
// This file is the single point where those get rewritten before
// they hit nightshift-api.
//
// Per the chunk-15 cr0n-port body-shape memory:
//   1. URL paths       — rewritten by ADAPTERS below
//   2. Request bodies  — transformBody applies a per-endpoint shape
//   3. Response bodies — transformResponse + camelToSnake unwrap
//   4. Query params    — proto enums need full names (INVOKER_TYPE_USER)
//
// Streaming has its own helper (nightshiftSSEProxy) that re-frames
// grpc-gateway's JSON-lines response as text/event-stream so the UI's
// existing EventSource consumer Just Works.

import { env } from "./env";
import { getSession } from "./auth";
import { getOidcIdToken, getOidcSubject } from "./oidc-token";

type ProxyOptions = {
  method?: string;
  body?: unknown;
  query?: Record<string, string>;
  timeout?: number;
};

// Adapter declares how a cr0n-shape path translates to nightshift /v1/*.
type Adapter = {
  match: RegExp;
  rewrite: (m: RegExpMatchArray, opts: AdapterContext) => { path: string; method?: string };
  transformBody?: (body: unknown, ctx: AdapterContext) => unknown;
  transformResponse?: (resp: unknown) => unknown;
  // When set, skip the upstream API call entirely and return this
  // payload as JSON. Use for cr0n endpoints that have no nightshift
  // equivalent yet — saves a guaranteed-to-404 round-trip + lets the
  // UI degrade gracefully instead of seeing a 404.
  synthetic?: (m: RegExpMatchArray, ctx: AdapterContext) => unknown;
};

type AdapterContext = {
  method: string;
  userId: string;
  query: Record<string, string>;
};

// Mapping table — first match wins. Ordered most-specific → least.
// Add new entries here when the UI grows new pages.
const ADAPTERS: Adapter[] = [
  // ── Runs ────────────────────────────────────────────────────────────
  {
    match: /^\/runs\/user\/list$/,
    rewrite: (_m, ctx) => ({ path: `/v1/runs?user_id=${encodeURIComponent(ctx.userId)}`, method: "GET" }),
    transformResponse: (r) => unwrapList(r, "runs"),
  },
  {
    match: /^\/runs\/history\/list$/,
    rewrite: (_m, ctx) => {
      const sessionID = ctx.query.session_id || "";
      // order_by=created_at asc → render the conversation oldest→newest.
      // The records-store default is updated_at DESC, which would
      // reverse follow-up messages on refresh.
      return {
        path:
          `/v1/runs?user_id=${encodeURIComponent(ctx.userId)}` +
          `&session_id=${encodeURIComponent(sessionID)}` +
          `&order_by=${encodeURIComponent("created_at asc")}`,
        method: "GET",
      };
    },
    transformResponse: (r) => unwrapList(r, "runs"),
  },
  {
    // Mint a session id before any run exists, so staged-file uploads
    // can be attached to it.
    match: /^\/sessions\/create$/,
    rewrite: () => ({ path: "/v1/sessions", method: "POST" }),
    transformResponse: (r) => {
      const body = (r ?? {}) as Record<string, unknown>;
      return { session_id: body.sessionId ?? body.session_id };
    },
  },
  {
    match: /^\/runs\/create$/,
    rewrite: () => ({ path: "/v1/runs", method: "POST" }),
    transformBody: (b, ctx) => {
      const body = (b ?? {}) as Record<string, unknown>;
      return {
        prompt: body.prompt,
        user_id: ctx.userId,
        session_id: body.session_id || "",
        invoker_type: "INVOKER_TYPE_USER",
        invoker_id: ctx.userId,
        agent_ids: body.agent_ids || [],
        skill_ids: body.skill_ids || [],
        connector_ids: body.connector_ids || [],
      };
    },
    // cr0n's POST /runs/create response shape was
    // {run_id, session_id, status, created_at} — the chunk-19 UI's
    // CreateRunResponse type still expects that shape. nightshift's
    // POST /v1/runs returns {run: {id, ...}}. Unwrap + remap so the
    // UI's `res.run_id` reads the right field. Without this, the chat
    // hook's SSE subscription sends `/api/runs/undefined/events`.
    transformResponse: (r) => {
      const run = (unwrapSingle(r, "run") ?? {}) as Record<string, unknown>;
      return {
        run_id: run.id,
        session_id: run.sessionId ?? run.session_id,
        status: run.status,
        created_at: run.createdAt ?? run.created_at,
      };
    },
  },
  {
    match: /^\/runs\/session\/([^/]+)$/,
    rewrite: (m) => ({ path: `/v1/runs/session/${m[1]}`, method: "DELETE" }),
  },
  {
    match: /^\/runs\/([^/]+)\/info$/,
    rewrite: (m) => ({ path: `/v1/runs/${m[1]}`, method: "GET" }),
    transformResponse: (r) => unwrapSingle(r, "run"),
  },
  {
    match: /^\/runs\/([^/]+)\/interrupt$/,
    rewrite: (m) => ({ path: `/v1/runs/${m[1]}:interrupt`, method: "POST" }),
  },
  {
    match: /^\/runs\/([^/]+)\/events\/history$/,
    rewrite: (m) => ({ path: `/v1/runs/${m[1]}/events`, method: "GET" }),
    transformResponse: (r) => unwrapList(r, "events"),
  },
  // /runs/{id}/events/sse handled by nightshiftSSEProxy below.

  // ── Artifacts ───────────────────────────────────────────────────────
  {
    match: /^\/artifacts\/list$/,
    rewrite: (_m, ctx) => ({ path: `/v1/artifacts?owner_id=${encodeURIComponent(ctx.userId)}`, method: "GET" }),
    transformResponse: (r) => normalizeArtifactList(unwrapList(r, "artifacts")),
  },
  {
    match: /^\/artifacts\/session\/([^/]+)$/,
    rewrite: (m, ctx) => ({
      path: `/v1/artifacts?owner_id=${encodeURIComponent(ctx.userId)}&session_id=${encodeURIComponent(m[1]!)}`,
      method: "GET",
    }),
    transformResponse: (r) => normalizeArtifactList(unwrapList(r, "artifacts")),
  },
  {
    // content_base64 → content: grpc-gateway accepts base64 for proto bytes fields.
    match: /^\/artifacts\/upload$/,
    rewrite: () => ({ path: "/v1/artifacts/objects", method: "POST" }),
    transformBody: (b, ctx) => {
      const { content_base64, ...rest } = (b ?? {}) as Record<string, unknown>;
      return { ...rest, content: content_base64, owner_id: ctx.userId };
    },
    transformResponse: (r) => normalizeArtifact(unwrapSingle(r, "artifact")),
  },
  {
    match: /^\/artifacts\/([^/]+)\/info$/,
    rewrite: (m) => ({ path: `/v1/artifacts/${m[1]}`, method: "GET" }),
    transformResponse: (r) => normalizeArtifact(unwrapSingle(r, "artifact")),
  },
  {
    match: /^\/artifacts\/([^/]+)\/edit$/,
    rewrite: (m) => ({ path: `/v1/artifacts/${m[1]}`, method: "PATCH" }),
    transformBody: (b) => {
      const body = (b ?? {}) as Record<string, unknown>;
      const out: Record<string, unknown> = {};
      if ("name" in body) out.name = body.name;
      if ("description" in body) out.description = body.description;
      if ("public" in body) out.public = body.public;
      if ("content_base64" in body) {
        out.content_bytes = body.content_base64;
        if ("content_type" in body) out.content_type = body.content_type;
      }
      if ("content" in body) out.html_content = body.content;
      return out;
    },
    transformResponse: (r) => normalizeArtifact(unwrapSingle(r, "artifact")),
  },
  {
    match: /^\/artifacts\/([^/]+)\/delete$/,
    rewrite: (m) => ({ path: `/v1/artifacts/${m[1]}`, method: "DELETE" }),
  },
  {
    // POST /artifacts/{id}/share → POST /v1/artifacts/{id}/permissions.
    // The UI's shareArtifact() (lib/api.ts:288) sends body
    // `{user_id, role}` already in the proto-canonical shape — pass
    // user_id through and only widen `role` to the proto enum form.
    // Revoke is its own DELETE adapter below; this one is POST-only.
    match: /^\/artifacts\/([^/]+)\/share$/,
    rewrite: (m) => ({
      path: `/v1/artifacts/${m[1]}/permissions`,
      method: "POST",
    }),
    transformBody: (b) => {
      const body = (b ?? {}) as Record<string, unknown>;
      return {
        user_id: body.user_id,
        role: body.role === "editor" ? "ARTIFACT_ROLE_EDITOR" : "ARTIFACT_ROLE_VIEWER",
      };
    },
    transformResponse: (r) => unwrapSingle(r, "permission"),
  },
  {
    // Revoke a single grant. cr0n uses DELETE /artifacts/{id}/share/{userId};
    // nightshift uses DELETE /v1/artifacts/{id}/permissions/{userId}.
    match: /^\/artifacts\/([^/]+)\/share\/([^/]+)$/,
    rewrite: (m) => ({
      path: `/v1/artifacts/${m[1]}/permissions/${encodeURIComponent(m[2]!)}`,
      method: "DELETE",
    }),
  },
  {
    match: /^\/artifacts\/([^/]+)\/permissions$/,
    rewrite: (m) => ({ path: `/v1/artifacts/${m[1]}/permissions`, method: "GET" }),
    // cr0n's UI renders role as `<span className="capitalize">` over a
    // bare lowercase string ("viewer" / "editor"). nightshift's proto
    // serializes the enum as "ARTIFACT_ROLE_VIEWER" — strip the prefix
    // + lowercase so the dialog shows "Viewer" not "ARTIFACT_ROLE_VIEWER".
    transformResponse: (r) => {
      const list = unwrapList(r, "permissions") as
        | Array<Record<string, unknown>>
        | undefined;
      if (!Array.isArray(list)) return list;
      return list.map((p) => {
        const role = typeof p.role === "string" ? p.role : "";
        return {
          ...p,
          role: role.replace(/^ARTIFACT_ROLE_/, "").toLowerCase(),
        };
      });
    },
  },
  // /artifacts/{id}/view + /preview + /download → handled via
  // nightshiftBinaryProxy below; the proxy URL maps to nightshift's
  // chunk-16 /v1/artifacts/{id}/view (binary passthrough). For
  // download/preview, the UI route fetches a presigned URL first and
  // 302-redirects.

  // ── Schedules ──────────────────────────────────────────────────────
  {
    match: /^\/schedule\/list$/,
    rewrite: (_m, ctx) => ({ path: `/v1/schedules?user_id=${encodeURIComponent(ctx.userId)}`, method: "GET" }),
    transformResponse: (r) => unwrapList(r, "schedules"),
  },
  {
    match: /^\/schedule\/create$/,
    rewrite: () => ({ path: "/v1/schedules", method: "POST" }),
    transformBody: (b, ctx) => ({ ...((b ?? {}) as object), user_id: ctx.userId }),
    transformResponse: (r) => unwrapSingle(r, "schedule"),
  },
  {
    match: /^\/schedule\/([^/]+)\/info$/,
    rewrite: (m) => ({ path: `/v1/schedules/${m[1]}`, method: "GET" }),
    transformResponse: (r) => unwrapSingle(r, "schedule"),
  },
  {
    match: /^\/schedule\/([^/]+)\/edit$/,
    rewrite: (m) => ({ path: `/v1/schedules/${m[1]}`, method: "PATCH" }),
    transformResponse: (r) => unwrapSingle(r, "schedule"),
  },
  {
    match: /^\/schedule\/([^/]+)\/delete$/,
    rewrite: (m) => ({ path: `/v1/schedules/${m[1]}`, method: "DELETE" }),
  },
  {
    match: /^\/schedule\/([^/]+)\/runs$/,
    rewrite: (m, ctx) => ({
      path: `/v1/runs?user_id=${encodeURIComponent(ctx.userId)}&invoker_id=${m[1]}`,
      method: "GET",
    }),
    transformResponse: (r) => unwrapList(r, "runs"),
  },
  {
    match: /^\/schedule\/([^/]+)\/trigger$/,
    rewrite: (m) => ({ path: `/v1/schedules/${m[1]}:trigger`, method: "POST" }),
    transformResponse: (r) => unwrapSingle(r, "run"),
  },

  // ── Agents (chunk 11) ──────────────────────────────────────────────
  {
    match: /^\/agents\/list$/,
    rewrite: (_m, ctx) => ({ path: `/v1/agents?user_id=${encodeURIComponent(ctx.userId)}`, method: "GET" }),
    transformResponse: (r) => unwrapList(r, "agents"),
  },
  {
    match: /^\/agents\/create$/,
    rewrite: () => ({ path: "/v1/agents", method: "POST" }),
    transformBody: (b, ctx) => ({ ...((b ?? {}) as object), user_id: ctx.userId }),
    transformResponse: (r) => unwrapSingle(r, "agent"),
  },
  {
    match: /^\/agents\/([^/]+)\/info$/,
    rewrite: (m) => ({ path: `/v1/agents/${m[1]}`, method: "GET" }),
    transformResponse: (r) => unwrapSingle(r, "agent"),
  },
  {
    match: /^\/agents\/([^/]+)\/edit$/,
    rewrite: (m) => ({ path: `/v1/agents/${m[1]}`, method: "PATCH" }),
    transformBody: (b) => {
      // UpdateAgentRequest needs explicit set_tools=true to replace
      // tools; otherwise the API leaves them untouched.
      const body = (b ?? {}) as Record<string, unknown>;
      if ("tools" in body) return { ...body, set_tools: true };
      return body;
    },
    transformResponse: (r) => unwrapSingle(r, "agent"),
  },
  {
    match: /^\/agents\/([^/]+)\/delete$/,
    rewrite: (m) => ({ path: `/v1/agents/${m[1]}`, method: "DELETE" }),
  },

  // ── Skills (chunk 11) ──────────────────────────────────────────────
  {
    match: /^\/skills\/list$/,
    rewrite: (_m, ctx) => ({ path: `/v1/skills?user_id=${encodeURIComponent(ctx.userId)}`, method: "GET" }),
    transformResponse: (r) => unwrapList(r, "skills"),
  },
  {
    match: /^\/skills\/create$/,
    rewrite: () => ({ path: "/v1/skills", method: "POST" }),
    transformBody: (b, ctx) => ({ ...((b ?? {}) as object), user_id: ctx.userId }),
    transformResponse: (r) => unwrapSingle(r, "skill"),
  },
  {
    match: /^\/skills\/([^/]+)\/info$/,
    rewrite: (m) => ({ path: `/v1/skills/${m[1]}`, method: "GET" }),
    transformResponse: (r) => unwrapSingle(r, "skill"),
  },
  {
    match: /^\/skills\/([^/]+)\/edit$/,
    rewrite: (m) => ({ path: `/v1/skills/${m[1]}`, method: "PATCH" }),
    transformResponse: (r) => unwrapSingle(r, "skill"),
  },
  {
    match: /^\/skills\/([^/]+)\/delete$/,
    rewrite: (m) => ({ path: `/v1/skills/${m[1]}`, method: "DELETE" }),
  },

  // ── Connectors ─────────────────────────────────────────────────────
  // user_id on list calls populates per-user `connected` state.
  {
    match: /^\/connectors\/(?:catalog|list)$/,
    rewrite: (_m, ctx) => ({
      path: `/v1/connectors?user_id=${encodeURIComponent(ctx.userId)}`,
      method: "GET",
    }),
    transformResponse: (r) => normalizeConnectorEntries(unwrapList(r, "entries")),
  },
  {
    match: /^\/connectors\/([^/]+)\/token$/,
    rewrite: (m) => ({ path: `/v1/connectors/${m[1]}:setStaticToken`, method: "POST" }),
    transformBody: (b, ctx) => ({ ...((b ?? {}) as object), user_id: ctx.userId }),
  },
  {
    match: /^\/connectors\/([^/]+)\/disconnect$/,
    rewrite: (m) => ({ path: `/v1/connectors/${m[1]}:disconnect`, method: "POST" }),
    transformBody: (_b, ctx) => ({ user_id: ctx.userId }),
  },

  // ── Users (chunk-19 share-dialog directory) ────────────────────────
  {
    // Forwards to nightshift-api's HTTP-only /v1/users handler, which
    // resolves the OpenBao identity-group `user`'s members. When
    // OpenBao is disabled in the chart, /v1/users 404s and the share
    // dialog falls back to "Everyone's already on the list" — same
    // behavior as the previous synthetic [] but powered by real data
    // when available.
    match: /^\/users\/list$/,
    rewrite: () => ({ path: "/v1/users", method: "GET" }),
    transformResponse: (r) => unwrapList(r, "users"),
  },
];

export type ProxyContext = {
  userId: string;
  idToken: string | null;
};

/**
 * Authenticated proxy to nightshift-api with cr0n→/v1 URL/body
 * translation. Drop-in replacement for the original cr0n-proxy.ts;
 * every app/api Next.js route that called cronProxy(path) now calls
 * nightshiftProxy(path) and gets the same surface back.
 */
export async function nightshiftProxy(
  path: string,
  options: ProxyOptions = {},
): Promise<Response> {
  const session = await getSession();
  if (!session) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { method = "GET", body, query = {}, timeout = 30_000 } = options;

  // The API derives identity from the JWT `sub`; better-auth's
  // session.user.id is its own local primary key and does NOT match
  // sub, so adapters must use the OIDC sub for any owner_id /
  // user_id parameter that round-trips through the API. Otherwise
  // ListArtifacts (and any other endpoint enforcing the cross-tenant
  // collapse rule) returns empty even for the caller's own data.
  const sub = await getOidcSubject(session.user.id);
  if (!sub) return sessionExpiredResponse();
  const ctx: AdapterContext = { method, userId: sub, query };

  const resolved = resolveAdapter(path, ctx);
  if (!resolved) {
    return Response.json(
      { error: "no_nightshift_adapter", path, method },
      { status: 404 },
    );
  }
  const { adapter, rewrite, match: matchArr } = resolved;

  // Synthetic adapters short-circuit before any upstream call. Used
  // for cr0n endpoints with no nightshift equivalent (e.g. user
  // listing) so the UI gets a stable, well-typed response.
  if (adapter.synthetic) {
    return Response.json(adapter.synthetic(matchArr, ctx));
  }

  const upstreamMethod = rewrite.method ?? method;
  const finalBody = adapter.transformBody ? adapter.transformBody(body, ctx) : body;

  const idToken = await getOidcIdToken(session.user.id);
  const headers: Record<string, string> = {};
  if (idToken) headers.Authorization = `Bearer ${idToken}`;
  else return sessionExpiredResponse();

  const init: RequestInit = {
    method: upstreamMethod,
    headers,
    signal: AbortSignal.timeout(timeout),
  };
  if (finalBody !== undefined && upstreamMethod !== "GET" && upstreamMethod !== "DELETE") {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(finalBody);
  }

  const url = `${env.NIGHTSHIFT_API_URL}${rewrite.path}`;
  const upstream = await fetch(url, init);

  // Non-2xx: pass through with the upstream body so the UI's existing
  // error handling (which reads `error` field) keeps working.
  if (!upstream.ok) {
    const text = await upstream.text();
    return new Response(text, {
      status: upstream.status,
      headers: { "Content-Type": upstream.headers.get("Content-Type") || "application/json" },
    });
  }

  // Success: parse + apply response transform + camelCase→snake_case
  // unwrap, since the cr0n UI's components expect snake_case fields.
  const text = await upstream.text();
  let parsed: unknown;
  try {
    parsed = text ? JSON.parse(text) : {};
  } catch {
    // Non-JSON response (rare for /v1/*) — pass through verbatim.
    return new Response(text, {
      status: upstream.status,
      headers: { "Content-Type": upstream.headers.get("Content-Type") || "application/json" },
    });
  }

  const transformed = adapter.transformResponse
    ? adapter.transformResponse(parsed)
    : parsed;
  const snakeCased = camelToSnakeDeep(transformed);

  return Response.json(snakeCased, { status: upstream.status });
}

/**
 * Binary-safe proxy. Used for /artifacts/{id}/view + /preview + /download.
 * The cr0n-shape path maps directly to nightshift's chunk-16 proxy URL
 * for view; download/preview return signed URLs the UI then fetches.
 *
 * Anonymous callers are forwarded without an Authorization header so
 * the upstream policy (which short-circuits to allow public artifacts
 * on /view) can run. Public artifact links must work for logged-out
 * viewers; the API is the single source of truth for the public flag.
 */
export async function nightshiftBinaryProxy(path: string): Promise<Response> {
  // Map cr0n binary paths → nightshift /v1/* shape.
  let target: string;
  const m = path.match(/^\/artifacts\/([^/]+)\/(view|preview|download)$/);
  if (!m) {
    return Response.json({ error: "no_nightshift_adapter", path }, { status: 404 });
  }
  const [, id, kind] = m;

  // Attach a bearer when the caller has a session; forward anonymously
  // otherwise so the API can serve public artifacts to logged-out
  // viewers. A partial session (cookie present, no OIDC token cached)
  // still needs reauth — that path is distinct from "no session at all".
  const session = await getSession();
  const headers: Record<string, string> = {};
  if (session) {
    const idToken = await getOidcIdToken(session.user.id);
    if (!idToken) return sessionExpiredResponse();
    headers.Authorization = `Bearer ${idToken}`;
  }

  if (kind === "view") {
    // chunk-16 reverse proxy — passes bytes through directly.
    target = `${env.NIGHTSHIFT_API_URL}/v1/artifacts/${id}/view`;
  } else if (kind === "download") {
    // Two-step: fetch signed URL, then 302 the user to it.
    const urlResp = await fetch(`${env.NIGHTSHIFT_API_URL}/v1/artifacts/${id}:downloadUrl`, { headers });
    if (!urlResp.ok) {
      return new Response(await urlResp.text(), { status: urlResp.status });
    }
    const { downloadUrl } = await urlResp.json();
    return Response.redirect(downloadUrl, 302);
  } else {
    // preview
    const urlResp = await fetch(`${env.NIGHTSHIFT_API_URL}/v1/artifacts/${id}:previewUrl`, { headers });
    if (!urlResp.ok) {
      return new Response(await urlResp.text(), { status: urlResp.status });
    }
    const { previewUrl } = await urlResp.json();
    return Response.redirect(previewUrl, 302);
  }

  const upstream = await fetch(target, { headers, signal: AbortSignal.timeout(30_000) });
  const outHeaders = new Headers();
  for (const h of ["content-type", "content-length", "content-disposition", "cache-control"]) {
    const v = upstream.headers.get(h);
    if (v) outHeaders.set(h, v);
  }
  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: outHeaders,
  });
}

/**
 * SSE proxy. Re-frames nightshift's grpc-gateway streaming JSON-lines
 * response as text/event-stream so the UI's existing EventSource
 * consumer doesn't need to change.
 *
 * grpc-gateway emits one JSON object per line (`Transfer-Encoding:
 * chunked`, `Content-Type: application/json`). SSE expects
 * `data: <json>\n\n` blocks. The TransformStream below buffers
 * partial lines + emits one `data:` frame per complete JSON line.
 */
export async function nightshiftSSEProxy(path: string): Promise<Response> {
  const session = await getSession();
  if (!session) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }
  const idToken = await getOidcIdToken(session.user.id);
  const headers: Record<string, string> = {};
  if (idToken) headers.Authorization = `Bearer ${idToken}`;
  else return sessionExpiredResponse();

  const m = path.match(/^\/runs\/([^/]+)\/events\/sse$/);
  if (!m) {
    return Response.json({ error: "no_nightshift_adapter", path }, { status: 404 });
  }
  const url = `${env.NIGHTSHIFT_API_URL}/v1/runs/${m[1]}/events:stream`;
  // no-store: keep undici from buffering the streaming body for caching.
  const upstream = await fetch(url, { headers, cache: "no-store" });
  if (!upstream.ok || !upstream.body) {
    return new Response(upstream.body, { status: upstream.status });
  }

  const decoder = new TextDecoder();
  const encoder = new TextEncoder();
  let buffer = "";

  // Each grpc-gateway streaming line looks like {"result": <Item>}; the
  // UI's EventSource consumer expects the bare event shape (index/
  // type/timestamp/raw). Unwrap + coerce `index` from string to number
  // since proto int64 serializes as string in JSON.
  const reframeLine = (raw: string): string | null => {
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown>;
      const inner = (parsed.result ?? parsed) as Record<string, unknown>;
      const ev = ((inner as { event?: unknown }).event ?? inner) as Record<
        string,
        unknown
      >;
      if (typeof ev.index === "string") ev.index = Number(ev.index);
      return JSON.stringify(ev);
    } catch {
      return raw;
    }
  };

  // 15s pings keep the connection from being classified as idle while
  // the agent thinks between tool calls; a silent EventSource leaves
  // the UI stuck waiting.
  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      controller.enqueue(encoder.encode(": stream-open\n\n"));

      const ping = setInterval(() => {
        try {
          controller.enqueue(encoder.encode(": ping\n\n"));
        } catch {}
      }, 15_000);

      const reader = upstream.body!.getReader();
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";
          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed) continue;
            const out = reframeLine(trimmed);
            if (out) controller.enqueue(encoder.encode(`data: ${out}\n\n`));
          }
        }
        const tail = buffer.trim();
        if (tail) {
          const out = reframeLine(tail);
          if (out) controller.enqueue(encoder.encode(`data: ${out}\n\n`));
        }
      } catch {
      } finally {
        clearInterval(ping);
        try { controller.close(); } catch {}
      }
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      "X-Accel-Buffering": "no",
      Connection: "keep-alive",
    },
  });
}

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------

function resolveAdapter(
  path: string,
  ctx: AdapterContext,
): {
  adapter: Adapter;
  rewrite: { path: string; method?: string };
  match: RegExpMatchArray;
} | null {
  for (const adapter of ADAPTERS) {
    const m = path.match(adapter.match);
    if (m) {
      return { adapter, rewrite: adapter.rewrite(m, ctx), match: m };
    }
  }
  return null;
}

function unwrapList(resp: unknown, key: string): unknown {
  if (resp && typeof resp === "object" && key in resp) {
    return (resp as Record<string, unknown>)[key];
  }
  return resp;
}

function unwrapSingle(resp: unknown, key: string): unknown {
  if (resp && typeof resp === "object" && key in resp) {
    return (resp as Record<string, unknown>)[key];
  }
  return resp;
}

// grpc-gateway serializes proto enums as their full UPPER_SNAKE name
// (e.g. "ARTIFACT_TYPE_APP", "DEPLOYMENT_STATE_READY"). The UI compares
// against short lowercase forms ("app", "ready"); without this, every
// artifact silently mis-classifies as "object" and the iframe-app
// branch never fires.
function stripEnumPrefix(value: unknown, prefix: string): unknown {
  if (typeof value !== "string") return value;
  if (!value.startsWith(prefix)) return value;
  return value.slice(prefix.length).toLowerCase();
}

function normalizeArtifact(art: unknown): unknown {
  if (!art || typeof art !== "object") return art;
  const a = art as Record<string, unknown>;
  if ("type" in a) a.type = stripEnumPrefix(a.type, "ARTIFACT_TYPE_");
  if ("appStatus" in a) a.appStatus = stripEnumPrefix(a.appStatus, "DEPLOYMENT_STATE_");
  if ("app_status" in a) a.app_status = stripEnumPrefix(a.app_status, "DEPLOYMENT_STATE_");
  return a;
}

function normalizeArtifactList(list: unknown): unknown {
  if (!Array.isArray(list)) return list;
  return list.map(normalizeArtifact);
}

// Flatten ListConnectors entries: `{connector: {...}, configured,
// connected}` → flat shape the UI expects, with auth_type lowercased.
function normalizeConnectorEntries(list: unknown): unknown {
  if (!Array.isArray(list)) return list;
  return list.map((raw) => {
    if (!raw || typeof raw !== "object") return raw;
    const e = raw as Record<string, unknown>;
    const inner = (e.connector ?? {}) as Record<string, unknown>;
    return {
      ...inner,
      authType: stripEnumPrefix(inner.authType, "CONNECTOR_AUTH_TYPE_"),
      configured: e.configured,
      connected: e.connected,
    };
  });
}

// camelToSnakeDeep walks a value, converting object keys from camelCase
// to snake_case. grpc-gateway emits camelCase JSON by default; the
// cr0n-derived UI reads snake_case fields. Recursive but stable on
// strings, numbers, arrays, and primitives.
export function camelToSnakeDeep(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(camelToSnakeDeep);
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      out[camelToSnake(k)] = camelToSnakeDeep(v);
    }
    return out;
  }
  return value;
}

function camelToSnake(s: string): string {
  return s.replace(/[A-Z]/g, (ch, idx) => (idx === 0 ? ch.toLowerCase() : "_" + ch.toLowerCase()));
}

function sessionExpiredResponse(): Response {
  return Response.json(
    { error: "session_expired" },
    { status: 401, headers: { "X-Reauth-Required": "openbao" } },
  );
}

// -----------------------------------------------------------------------------
// Backward-compatible aliases
// -----------------------------------------------------------------------------
//
// Existing app/api/*/route.ts handlers import { cronProxy } from
// "@/lib/server/cron-proxy". We keep cron-proxy.ts as a re-export
// shim so the route handlers can stay verbatim and the URL-translation
// happens transparently.

export { nightshiftProxy as cronProxy };
export { nightshiftBinaryProxy as cronBinaryProxy };
export { nightshiftSSEProxy as cronSSEProxy };
