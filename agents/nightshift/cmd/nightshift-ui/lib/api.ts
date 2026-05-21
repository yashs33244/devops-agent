import { titleFromPrompt } from "@/lib/events/parse";
import { authClient } from "@/auth/client";

// ── Types (matching cr0n-a models) ──────────────────────────────

export type RunStatus = "pending" | "running" | "completed" | "error" | "interrupted";

export type RunUsage = {
  ai_usage: number;
  vcpu: number;
  memory: number;
  network_flows: number;
  network_dns: number;
  file_ops: number;
};

export type RunInfo = {
  id: string;
  prompt: string;
  status: RunStatus;
  session_id: string;
  created_at: string;
  started_at?: string;
  ended_at: string;
  error: string;
  event_count: number;
  total_cost_usd: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  run_usage: RunUsage;
  invoker_type?: "INVOKER_TYPE_USER" | "INVOKER_TYPE_SCHEDULE" | "INVOKER_TYPE_UNSPECIFIED";
  invoker_id?: string;
  // "auto" (cron fire), "manual" (operator-fired via TriggerSchedule),
  // or empty (direct user-created run).
  trigger_mode?: "auto" | "manual" | "";
};

export type CreateRunResponse = {
  run_id: string;
  session_id: string;
  status: RunStatus;
  created_at: string;
};

export type StreamEvent = {
  index: number;
  type: string;
  timestamp: string;
  raw: Record<string, unknown> | unknown[] | string;
};

export type SessionSummary = {
  session_id: string;
  first_prompt: string;
  run_count: number;
  latest_status: RunStatus;
  last_activity: string;
};

export type AgentInfo = {
  id: string;
  user_id: string;
  name: string;
  description: string;
  prompt: string;
  tools: string[];
  model: string | null;
  created_at: string;
  updated_at: string;
};

export type CreateAgentRequest = {
  name: string;
  description: string;
  prompt: string;
  tools: string[];
  model?: string | null;
};

export type UpdateAgentRequest = Partial<CreateAgentRequest>;

export type SkillInfo = {
  id: string;
  user_id: string;
  name: string;
  description: string;
  content: string;
  created_at: string;
  updated_at: string;
};

export type CreateSkillRequest = {
  name: string;
  description: string;
  content: string;
};

export type UpdateSkillRequest = Partial<CreateSkillRequest>;

export type ArtifactType = "app" | "object";

export type ArtifactInfo = {
  id: string;
  type: ArtifactType;
  name: string;
  description: string;
  owner_id: string;
  run_id: string;
  session_id: string;
  public: boolean;
  s3_key: string;
  content_type: string;
  size_bytes: number;
  app_url: string;
  app_status: string;
  created_at: string;
  updated_at: string;
};

export type UpdateArtifactRequest = Partial<{
  name: string;
  description: string;
  public: boolean;
}>;

export type ArtifactPermissionInfo = {
  artifact_id: string;
  user_id: string;
  role: string;
  granted_at: string;
};

// ── Fetch helpers ───────────────────────────────────────────────

// Avoid a reauth storm when ~everything fires at once on page load.
let reauthInFlight = false;

function triggerReauth(): void {
  if (reauthInFlight) return;
  reauthInFlight = true;
  if (typeof window === "undefined") return;
  // Better-Auth's genericOAuth entry point is a POST; authClient.signIn.oauth2
  // handles the request + redirect. OpenBao's session cookie is still valid
  // (24h) so the user sees a brief redirect chain, not a login form.
  const back = window.location.pathname + window.location.search;
  void authClient.signIn.oauth2({
    providerId: "openbao",
    callbackURL: back,
  });
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    signal: AbortSignal.timeout(30_000),
    ...init,
  });
  if (res.status === 401 && res.headers.get("X-Reauth-Required") === "openbao") {
    triggerReauth();
    // Throw a descriptive error for any code paths that catch and render —
    // the browser will navigate away before most of them see it.
    throw new Error("OIDC token expired — re-authenticating");
  }
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText);
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json();
}

async function send(path: string, init?: RequestInit): Promise<void> {
  await request<unknown>(path, init);
}

function json<T>(method: string, path: string, body: unknown): Promise<T> {
  return request<T>(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

function post<T>(path: string, body: unknown): Promise<T> {
  return json<T>("POST", path, body);
}

// ── Runs ────────────────────────────────────────────────────────

export const createRun = (
  prompt: string,
  session_id?: string,
): Promise<CreateRunResponse> =>
  post("/api/runs/create", { prompt, session_id: session_id || undefined });

export const listSessions = (): Promise<SessionSummary[]> =>
  request("/api/runs/list");

/** Permanently delete a session and all its runs + events. */
export const deleteSession = (sessionId: string): Promise<void> =>
  send(`/api/runs/session/${encodeURIComponent(sessionId)}`, {
    method: "DELETE",
  });

export const listSessionRuns = (sessionId: string): Promise<RunInfo[]> =>
  request(`/api/runs/history?session_id=${encodeURIComponent(sessionId)}`);

export const getRunInfo = (runId: string): Promise<RunInfo> =>
  request(`/api/runs/${runId}/info`);

export const interruptRun = (runId: string): Promise<void> =>
  send(`/api/runs/${runId}/interrupt`, { method: "POST" });

export const getRunEvents = (runId: string): Promise<StreamEvent[]> =>
  request(`/api/runs/${runId}/events/history`);

/**
 * Subscribe to live SSE events for a run.
 * cr0n-a replays all historical events first, then streams live.
 * Pass `afterIndex` to skip events the client already has.
 */
export function subscribeToRunEvents(
  runId: string,
  onEvent: (event: StreamEvent) => void,
  afterIndex = -1,
): EventSource {
  const es = new EventSource(`/api/runs/${runId}/events`);
  es.onmessage = (e: MessageEvent) => {
    try {
      const event = JSON.parse(e.data) as StreamEvent;
      if (typeof event.index === "number" && event.index <= afterIndex) return;
      onEvent(event);
    } catch (err) {
      console.warn("[sse] failed to parse event:", err);
    }
  };
  return es;
}

// ── Agents ──────────────────────────────────────────────────────

export const listAgents = (): Promise<AgentInfo[]> =>
  request("/api/agents");

export const getAgent = (id: string): Promise<AgentInfo> =>
  request(`/api/agents/${encodeURIComponent(id)}`);

export const createAgent = (body: CreateAgentRequest): Promise<AgentInfo> =>
  post("/api/agents", body);

export const updateAgent = (
  id: string,
  body: UpdateAgentRequest,
): Promise<AgentInfo> => json("PUT", `/api/agents/${encodeURIComponent(id)}`, body);

export const deleteAgent = (id: string): Promise<void> =>
  send(`/api/agents/${encodeURIComponent(id)}`, { method: "DELETE" });

// ── Skills ──────────────────────────────────────────────────────

export const listSkills = (): Promise<SkillInfo[]> =>
  request("/api/skills");

export const getSkill = (id: string): Promise<SkillInfo> =>
  request(`/api/skills/${encodeURIComponent(id)}`);

export const createSkill = (body: CreateSkillRequest): Promise<SkillInfo> =>
  post("/api/skills", body);

export const updateSkill = (
  id: string,
  body: UpdateSkillRequest,
): Promise<SkillInfo> => json("PUT", `/api/skills/${encodeURIComponent(id)}`, body);

export const deleteSkill = (id: string): Promise<void> =>
  send(`/api/skills/${encodeURIComponent(id)}`, { method: "DELETE" });

// ── Artifacts ───────────────────────────────────────────────────

export const listArtifacts = (): Promise<ArtifactInfo[]> =>
  request("/api/artifacts");

export const getArtifact = (id: string): Promise<ArtifactInfo> =>
  request(`/api/artifacts/${encodeURIComponent(id)}/info`);

export const updateArtifact = (
  id: string,
  body: UpdateArtifactRequest,
): Promise<ArtifactInfo> =>
  json("PUT", `/api/artifacts/${encodeURIComponent(id)}`, body);

export const deleteArtifact = (id: string): Promise<void> =>
  send(`/api/artifacts/${encodeURIComponent(id)}`, { method: "DELETE" });

export const shareArtifact = (
  id: string,
  body: { user_id: string; role?: string },
): Promise<ArtifactPermissionInfo> =>
  post(`/api/artifacts/${encodeURIComponent(id)}/share`, body);

export const unshareArtifact = (id: string, userId: string): Promise<void> =>
  send(
    `/api/artifacts/${encodeURIComponent(id)}/share/${encodeURIComponent(userId)}`,
    { method: "DELETE" },
  );

export const listArtifactPermissions = (
  id: string,
): Promise<ArtifactPermissionInfo[]> =>
  request(`/api/artifacts/${encodeURIComponent(id)}/permissions`);

// ── Sessions ────────────────────────────────────────────────────

/** The server is the exclusive minter of session ids; the UI never
 * generates them. */
export const createSession = (): Promise<{ session_id: string }> =>
  post<{ session_id: string }>("/api/sessions/create", {});

// ── File uploads attached to a chat session ─────────────────────

export const listSessionAttachments = (
  sessionId: string,
): Promise<ArtifactInfo[]> =>
  request(`/api/artifacts/session/${encodeURIComponent(sessionId)}`);

/** One pass over the bytes for both base64 encoding and SHA-256 hash —
 * matters at multi-MB file sizes. */
async function fileToBase64AndHash(
  file: File,
): Promise<{ base64: string; sha256Hex: string }> {
  const buf = await file.arrayBuffer();
  const bytes = new Uint8Array(buf);
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", buf));
  let sha256Hex = "";
  for (let i = 0; i < digest.length; i++) {
    sha256Hex += digest[i]!.toString(16).padStart(2, "0");
  }
  let binary = "";
  const CHUNK = 0x8000;
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return { base64: btoa(binary), sha256Hex };
}

/** Idempotency key scopes content hash to session id, so re-uploading
 * the same file in a different thread produces a separate artifact. */
export async function uploadSessionFile(
  sessionId: string,
  file: File,
): Promise<ArtifactInfo> {
  const { base64, sha256Hex } = await fileToBase64AndHash(file);
  return post<ArtifactInfo>("/api/artifacts/upload", {
    name: file.name,
    content_type: file.type || "application/octet-stream",
    content_base64: base64,
    session_id: sessionId,
    idempotency_key: `${sessionId}:sha256-${sha256Hex}`,
  });
}

// ── Users (directory) ───────────────────────────────────────────

export type UserSummary = { id: string; name: string };

/** Everyone in the OpenBao identity directory (sub + display name). Used by
 * the artifact share picker. */
export const listUsers = (): Promise<UserSummary[]> => request("/api/users");

export type MeInfo = {
  id: string; // OIDC sub — matches owner_id / invoker_id / permission user_id
  name: string;
  email: string;
  role: string;
};

/** The current user's OIDC sub + profile. Use this, not session.user.id,
 * whenever you need to compare against cr0n-a rows. */
export const getMe = (): Promise<MeInfo> => request("/api/me");

// ── Connectors ──────────────────────────────────────────────────

/** Per-user connector status returned by cr0n-a's GET /connectors/list. */
export type ConnectorStatus = {
  id: string;
  name: string;
  description: string;
  connected: boolean;
  mcp_url: string;
};

export const listConnectors = (): Promise<ConnectorStatus[]> =>
  request("/api/connectors/list");

/**
 * Catalog entry returned by cr0n-a's GET /connectors/catalog. Covers every
 * connector in the static catalog, with `configured` indicating whether the
 * admin has provisioned OpenBao credentials. Unconfigured entries are shown
 * in the UI greyed out and disabled.
 */
export type ConnectorCatalogEntry = {
  id: string;
  name: string;
  description: string;
  mcp_url: string;
  configured: boolean;
  connected: boolean;
  /** "oauth" — standard authorization-code flow via OpenBao.
   *  "static_token" — user pastes a long-lived PAT stored in OpenBao KV. */
  auth_type: "oauth" | "static_token";
};

export const listConnectorCatalog = (): Promise<ConnectorCatalogEntry[]> =>
  request("/api/connectors/catalog");

/** Save a static bearer token for a static_token connector (e.g. MotherDuck). */
export const setConnectorToken = (name: string, token: string): Promise<{ ok: boolean }> =>
  post(`/api/connectors/${encodeURIComponent(name)}/token`, { token });

/**
 * Returns the URL the Connect button should navigate to. The browser MUST
 * follow this as a full-page navigation (not a fetch) so the 302 chain
 * (UI proxy → cr0n-a → OpenBao → OAuth provider) works.
 */
export function authorizeConnectorUrl(name: string): string {
  return `/api/connectors/${encodeURIComponent(name)}/authorize`;
}

export const disconnectConnector = (name: string): Promise<void> =>
  send(`/api/connectors/${encodeURIComponent(name)}/disconnect`, {
    method: "POST",
  });

// ── Compatibility layer ─────────────────────────────────────────
// Maps the designer's UI types/functions to cr0n-a's run-based API.
// "Tasks" in the UI = "Runs" in cr0n-a. "Conversations" = sessions.

/** Alias: designer components use RunEvent, cr0n-a uses StreamEvent */
export type RunEvent = StreamEvent;

export type TaskResponse = {
  id: string;
  tenantId: string;
  agentId?: string | null;
  name?: string;
  prompt: string;
  status: "draft" | "running" | "completed" | "error" | "interrupted";
  runs?: { id: string; status: RunStatus; startedAt: string | null; completedAt: string | null; iterations: number | null; promptVersion: number | null; ranBy: string | null }[];
  createdBy: string;
  createdAt: string;
  updatedAt: string;
};

export type StartRunResponse = {
  id: string;
  status: string;
};

export type Agent = {
  id: string;
  name: string;
  url: string | null;
  userCount?: number;
  status: "online" | "offline" | "unreachable";
};

// ── Compat: Tasks (maps to sessions/runs in cr0n-a) ────────────

// `listSessions` returns flat runs (one row per run) — there is no
// ListSessions RPC on the API. Aggregate by session_id: oldest run's
// prompt becomes the title, newest run's status/timestamp drives state.
type RawRun = {
  id: string;
  prompt?: string;
  status?: string; // "RUN_STATUS_*"
  session_id?: string;
  created_at?: string;
  started_at?: string;
};

function rawRunStatusToTaskStatus(s: string | undefined): TaskResponse["status"] {
  if (s === "RUN_STATUS_RUNNING" || s === "RUN_STATUS_PENDING") return "running";
  if (s === "RUN_STATUS_ERROR" || s === "RUN_STATUS_FAILED") return "error";
  if (s === "RUN_STATUS_INTERRUPTED") return "interrupted";
  return "completed";
}

export async function fetchTasks(): Promise<TaskResponse[]> {
  const runs = (await listSessions()) as unknown as RawRun[];
  const t = (a?: string) => (a ? Date.parse(a) : 0);

  const latestBySession = new Map<string, RawRun>();
  const oldestBySession = new Map<string, RawRun>();
  for (const r of runs) {
    if (!r.session_id) continue;
    const ts = t(r.started_at ?? r.created_at);
    const latest = latestBySession.get(r.session_id);
    if (!latest || ts >= t(latest.started_at ?? latest.created_at)) {
      latestBySession.set(r.session_id, r);
    }
    const oldest = oldestBySession.get(r.session_id);
    if (!oldest || t(r.created_at) < t(oldest.created_at)) {
      oldestBySession.set(r.session_id, r);
    }
  }

  const out: TaskResponse[] = [];
  for (const [sid, latest] of latestBySession) {
    const first = oldestBySession.get(sid) ?? latest;
    const ts = latest.started_at ?? latest.created_at ?? "";
    out.push({
      id: sid,
      tenantId: "",
      name: titleFromPrompt(first.prompt),
      prompt: first.prompt ?? "",
      status: rawRunStatusToTaskStatus(latest.status),
      createdBy: "",
      createdAt: ts,
      updatedAt: ts,
    });
  }
  out.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
  return out;
}

export async function createTask(data: { name?: string; prompt: string }): Promise<TaskResponse> {
  const res = await createRun(data.prompt);
  return {
    id: res.run_id,
    tenantId: "",
    name: data.name || titleFromPrompt(data.prompt),
    prompt: data.prompt,
    status: "running",
    createdBy: "",
    createdAt: res.created_at,
    updatedAt: res.created_at,
  };
}

export async function updateTask(
  _taskId: string,
  _data: { name?: string; prompt?: string; restoredFromVersion?: number },
): Promise<TaskResponse> {
  // No-op: tasks are not editable in cr0n-a (they're runs)
  return { id: _taskId, tenantId: "", prompt: "", status: "completed", createdBy: "", createdAt: "", updatedAt: "" };
}

export async function deleteTask(_taskId: string): Promise<void> {
  // No-op: sessions can't be deleted from cr0n-a
}

export async function startRun(taskId: string, prompt: string, _agent?: string): Promise<StartRunResponse> {
  const res = await createRun(prompt, taskId); // taskId = session_id for continuity
  return { id: res.run_id, status: res.status };
}

export async function fetchRunEvents(runId: string): Promise<RunEvent[]> {
  return getRunEvents(runId);
}

export function subscribeToEvents(
  onEvent: (event: RunEvent) => void,
  runId: string,
  afterIndex = -1,
): EventSource {
  return subscribeToRunEvents(runId, onEvent, afterIndex);
}

export async function fetchAgents(): Promise<Agent[]> {
  try {
    const agents = await listAgents();
    return agents.map((a) => ({
      id: a.id,
      name: a.name,
      url: null,
      status: "online" as const,
    }));
  } catch {
    return [];
  }
}

// ── Schedules ───────────────────────────────────────────────────

export type ScheduleInfo = {
  id: string;
  user_id: string;
  prompt: string;
  cron: string;
  session_id: string;
  timezone: string;
  enabled: boolean;
  agent_ids: string[];
  skill_ids: string[];
  connector_ids: string[];
  created_at: string;
  updated_at: string;
};

export type CreateScheduleRequest = {
  prompt: string;
  cron: string;
  timezone?: string;
  enabled?: boolean;
  session_id?: string;
};

export type UpdateScheduleRequest = Partial<CreateScheduleRequest>;

export const listSchedules = (): Promise<ScheduleInfo[]> =>
  request("/api/schedule");

export const getSchedule = (id: string): Promise<ScheduleInfo> =>
  request(`/api/schedule/${encodeURIComponent(id)}`);

export const createSchedule = (
  body: CreateScheduleRequest,
): Promise<ScheduleInfo> => post("/api/schedule", body);

export const updateSchedule = (
  id: string,
  body: UpdateScheduleRequest,
): Promise<ScheduleInfo> =>
  json("PUT", `/api/schedule/${encodeURIComponent(id)}`, body);

export const deleteSchedule = (id: string): Promise<void> =>
  send(`/api/schedule/${encodeURIComponent(id)}`, { method: "DELETE" });

export const listScheduleRuns = (id: string): Promise<RunInfo[]> =>
  request(`/api/schedule/${encodeURIComponent(id)}/runs`);

export const triggerSchedule = (id: string): Promise<RunInfo> =>
  post(`/api/schedule/${encodeURIComponent(id)}/trigger`, {});

// ── Compat: Versions (stubs) ────────────────────────────────────

export type VersionSummary = { id: string; version: number; name: string; description: string; createdBy: string; authorName: string; createdAt: string };
export type VersionDetail = VersionSummary & { prompt: string };

export const fetchVersions = (_taskId: string): Promise<VersionSummary[]> =>
  Promise.resolve([]);

export const fetchVersion = (_taskId: string, _versionId: string): Promise<VersionDetail> =>
  Promise.resolve({} as VersionDetail);

export const fetchVersionByNumber = (_taskId: string, _version: number): Promise<VersionDetail> =>
  Promise.resolve({} as VersionDetail);
