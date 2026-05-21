"use client";

import {
  useState,
  useEffect,
  useRef,
  useCallback,
  useMemo,
  createContext,
  useContext,
  type ReactNode,
} from "react";
import type {
  ArtifactInfo,
  RunInfo,
  SessionSummary,
  StreamEvent,
} from "../api";
import * as api from "../api";
import { raw as rawEvent, titleFromPrompt, parseEventType } from "@/lib/events/parse";

// ── Types ────────────────────────────────────────────────────────

export type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  runId?: string;
  status: "sent" | "streaming" | "completed" | "error" | "interrupted";
  events: StreamEvent[];
  // Set on role=user messages that uploaded files. Persisted server-side
  // as session-scoped artifacts; this is the per-message slice.
  attachments?: ArtifactInfo[];
};

export type Conversation = {
  id: string;
  title: string;
  agentId: string | null;
  sessionId: string | null;
  createdAt: string;
  updatedAt: string;
  lastMessage?: { content: string; role: string; createdAt: string; status?: string } | null;
};

// ── Helpers ──────────────────────────────────────────────────────

// The `listSessions` endpoint actually returns flat runs (one row per
// run) — there's no separate ListSessions RPC on the API. We aggregate
// client-side: group by session_id, pick the FIRST run's prompt as the
// canonical title, and the LATEST run's timestamp for ordering.
type RawRun = {
  id: string;
  prompt: string;
  status: string;
  session_id: string;
  created_at: string;
  started_at?: string;
  ended_at?: string;
  invoker_type?: string;
};

function runsToConversations(runs: RawRun[]): Conversation[] {
  // Identify sessions whose origin run was a schedule fire. Those
  // belong on /schedule, not /tasks — the user shouldn't see their
  // cron-driven runs cluttering the conversation list. A session that
  // STARTED as user-created and was later joined by schedule fires
  // (rare — only if a schedule's session_id targets a chat) stays
  // visible because its earliest run is user-typed.
  const earliestBySession = new Map<string, RawRun>();
  for (const r of runs) {
    if (!r.session_id) continue;
    const t = (a?: string) => (a ? Date.parse(a) : 0);
    const cur = earliestBySession.get(r.session_id);
    if (!cur || t(r.created_at) < t(cur.created_at)) {
      earliestBySession.set(r.session_id, r);
    }
  }
  const scheduleOriginSessions = new Set<string>();
  for (const [sid, first] of earliestBySession) {
    if (first.invoker_type === "INVOKER_TYPE_SCHEDULE") scheduleOriginSessions.add(sid);
  }

  const bySession = new Map<string, RawRun>();
  for (const r of runs) {
    if (!r.session_id) continue;
    if (scheduleOriginSessions.has(r.session_id)) continue;
    const prev = bySession.get(r.session_id);
    const t = (a?: string) => (a ? Date.parse(a) : 0);
    if (!prev || t(r.started_at ?? r.created_at) >= t(prev.started_at ?? prev.created_at)) {
      bySession.set(r.session_id, r);
    }
  }
  const firstPromptBySession = new Map<string, string>();
  for (const r of runs) {
    if (!r.session_id) continue;
    const t = (a?: string) => (a ? Date.parse(a) : 0);
    const existing = firstPromptBySession.get(r.session_id);
    if (existing === undefined) {
      firstPromptBySession.set(r.session_id, r.prompt);
      continue;
    }
    const prevRun = runs.find((x) => x.session_id === r.session_id && x.prompt === existing);
    if (prevRun && t(r.created_at) < t(prevRun.created_at)) {
      firstPromptBySession.set(r.session_id, r.prompt);
    }
  }

  const out: Conversation[] = [];
  for (const [sid, latest] of bySession) {
    const firstPrompt = firstPromptBySession.get(sid) ?? latest.prompt;
    const ts = latest.started_at ?? latest.created_at;
    out.push({
      id: sid,
      title: titleFromPrompt(firstPrompt, "New task"),
      agentId: null,
      sessionId: sid,
      createdAt: ts,
      updatedAt: ts,
      lastMessage: { content: firstPrompt, role: "user", createdAt: ts },
    });
  }
  out.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
  return out;
}

function runToMessages(
  run: RunInfo,
  events: StreamEvent[],
  attachments?: ArtifactInfo[],
): ChatMessage[] {
  const msgs: ChatMessage[] = [];
  msgs.push({
    id: `${run.id}-user`,
    role: "user",
    content: run.prompt,
    runId: run.id,
    status: "sent",
    events: [],
    attachments,
  });
  const status: ChatMessage["status"] =
    run.status === "running" || run.status === "pending"
      ? "streaming"
      : run.status === "error"
        ? "error"
        : run.status === "interrupted"
          ? "interrupted"
          : "completed";
  msgs.push({
    id: `${run.id}-assistant`,
    role: "assistant",
    content: "",
    runId: run.id,
    status,
    events,
  });
  return msgs;
}

// ── Hook ─────────────────────────────────────────────────────────

type Sub = {
  es: EventSource;
  msgId: string;
  buffer: StreamEvent[];
  rafId: number;
  // Highest event index we've forwarded to the renderer. Mutates per
  // event. On reconnect the server replays from index 0; this lets us
  // skip everything we already handled, regardless of how many times
  // the connection drops + recovers.
  latestIndex: number;
  // Bounded reconnect counter. Resets on any successful event so a
  // healthy run that hiccups once isn't penalized later in the stream.
  retries: number;
  // Pending reconnect timer — cleared on terminal / resubscribe so we
  // don't fire a retry against a sub that's already been replaced.
  retryTimer: number | null;
  // Watchdog for silently-stalled streams (EventSource.onerror only
  // fires on explicit network/close, not idle). Reset on every event;
  // on fire we fall back to finalizeFromHistory.
  idleTimer: number | null;
};

const SSE_MAX_RETRIES = 5;
const SSE_RETRY_DELAYS_MS = [500, 1500, 3000, 5000, 8000];
const SSE_IDLE_MS = 45_000;

function useChat() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);
  const [pendingSendCount, setPendingSendCount] = useState(0);
  const [loaded, setLoaded] = useState(false);

  // Reactive view of in-flight runs grouped by conversation. Drives the
  // sidebar spinner and per-conversation `isStreaming`. Mirrors what
  // `subsRef` knows, but as state so the UI re-renders on changes.
  const [runningByConv, setRunningByConv] = useState<Map<string, Set<string>>>(new Map());

  // Client-side-minted sessions still waiting on their first CreateRun
  // (attachments uploading). Folded into `runningConvs`.
  const [pendingConvs, setPendingConvs] = useState<Set<string>>(new Set());

  const activeConvRef = useRef(activeConversationId);
  activeConvRef.current = activeConversationId;

  const conversationsRef = useRef(conversations);
  conversationsRef.current = conversations;

  // runId → convId. Lets terminal/cancel handlers update `runningByConv`
  // without threading convId through every callback.
  const runConvIdRef = useRef<Map<string, string>>(new Map());

  // Runs the user explicitly cancelled. The terminal `result` SSE event
  // (sent by the worker after `client.interrupt()`) would otherwise mark
  // the message "completed"; this set lets dispatch label it "interrupted"
  // even if a late event arrives after we tore the subscription down.
  const cancelledRunsRef = useRef<Set<string>>(new Set());

  // One SSE subscription per in-flight run, keyed by runId. Each subscription
  // owns its own RAF-batched event buffer so concurrent runs don't clobber
  // each other, and starting a new run never closes a sibling stream.
  const subsRef = useRef<Map<string, Sub>>(new Map());

  const addRunning = useCallback((convId: string, runId: string) => {
    runConvIdRef.current.set(runId, convId);
    setRunningByConv((prev) => {
      const cur = prev.get(convId);
      if (cur && cur.has(runId)) return prev;
      const next = new Map(prev);
      const set = new Set(cur ?? []);
      set.add(runId);
      next.set(convId, set);
      return next;
    });
  }, []);

  const removeRunning = useCallback((runId: string) => {
    const convId = runConvIdRef.current.get(runId);
    runConvIdRef.current.delete(runId);
    if (!convId) return;
    setRunningByConv((prev) => {
      const cur = prev.get(convId);
      if (!cur || !cur.has(runId)) return prev;
      const next = new Map(prev);
      const set = new Set(cur);
      set.delete(runId);
      if (set.size === 0) next.delete(convId);
      else next.set(convId, set);
      return next;
    });
  }, []);

  useEffect(() => {
    return () => {
      for (const sub of subsRef.current.values()) {
        sub.es.close();
        if (sub.rafId) cancelAnimationFrame(sub.rafId);
        if (sub.retryTimer) clearTimeout(sub.retryTimer);
        if (sub.idleTimer) clearTimeout(sub.idleTimer);
      }
      subsRef.current.clear();
    };
  }, []);

  // Seeds a sidebar row + spinner before CreateRun fires, so the user
  // sees an instant response while attachments upload. sendMessage
  // clears the entry once subscribeToRun takes over.
  const beginPendingConv = useCallback((sessionId: string, prompt: string) => {
    const now = new Date().toISOString();
    setConversations((prev) => {
      if (prev.some((c) => c.id === sessionId)) return prev;
      const conv: Conversation = {
        id: sessionId,
        title: titleFromPrompt(prompt, "New task"),
        agentId: null,
        sessionId,
        createdAt: now,
        updatedAt: now,
        lastMessage: { content: prompt, role: "user", createdAt: now },
      };
      return [conv, ...prev];
    });
    setActiveConversationId(sessionId);
    // Clear the previous thread so the user lands on a blank canvas
    // immediately instead of staring at the prior chat.
    setMessages([]);
    setPendingConvs((prev) => {
      if (prev.has(sessionId)) return prev;
      const next = new Set(prev);
      next.add(sessionId);
      return next;
    });
  }, []);

  const clearPendingConv = useCallback((sessionId: string) => {
    setPendingConvs((prev) => {
      if (!prev.has(sessionId)) return prev;
      const next = new Set(prev);
      next.delete(sessionId);
      return next;
    });
  }, []);

  // ── Flush one run's RAF-batched events into message state ─────

  const flushSub = useCallback((runId: string) => {
    const sub = subsRef.current.get(runId);
    if (!sub) return;
    sub.rafId = 0;
    const batch = sub.buffer.splice(0);
    if (batch.length === 0) return;
    const msgId = sub.msgId;
    setMessages((prev) =>
      prev.map((m) => (m.id === msgId ? { ...m, events: [...m.events, ...batch] } : m)),
    );
  }, []);

  // ── Fetch conversation list ───────────────────────────────────

  const fetchConversations = useCallback(async () => {
    try {
      const sessions = await api.listSessions();
      setConversations(runsToConversations(sessions as unknown as RawRun[]));
    } catch (e) {
      console.error("[useChat] fetchConversations:", e);
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => { fetchConversations(); }, [fetchConversations]);

  // ── SSE subscription — one EventSource per run ────────────────

  const subscribeToRun = useCallback((
    runId: string,
    msgId: string,
    convId: string,
    afterIndex = -1,
  ) => {
    // Replace any stale subscription for this run (e.g. after a re-navigation
    // that triggers loadMessages while the prior ES is still open).
    const prior = subsRef.current.get(runId);
    if (prior) {
      prior.es.close();
      if (prior.rafId) cancelAnimationFrame(prior.rafId);
      if (prior.retryTimer) clearTimeout(prior.retryTimer);
      if (prior.idleTimer) clearTimeout(prior.idleTimer);
      subsRef.current.delete(runId);
    }

    const sub: Sub = {
      es: null as unknown as EventSource,
      msgId,
      buffer: [],
      rafId: 0,
      latestIndex: afterIndex,
      retries: 0,
      retryTimer: null,
      idleTimer: null,
    };
    subsRef.current.set(runId, sub);
    addRunning(convId, runId);

    const armIdleTimer = () => {
      if (sub.idleTimer) clearTimeout(sub.idleTimer);
      sub.idleTimer = window.setTimeout(() => {
        sub.idleTimer = null;
        // Dropped/replaced subs shouldn't escalate.
        if (subsRef.current.get(runId) !== sub) return;
        sub.es.close();
        if (sub.retryTimer) { clearTimeout(sub.retryTimer); sub.retryTimer = null; }
        void finalizeFromHistory();
      }, SSE_IDLE_MS);
    };

    // Single dispatch path — used on initial connect AND every retry.
    // Dedupes against `sub.latestIndex` (mutable) so we tolerate the
    // server's full-replay-on-reconnect.
    const dispatch = (event: StreamEvent) => {
      if (typeof event.index === "number") {
        if (event.index <= sub.latestIndex) return;
        sub.latestIndex = event.index;
      }
      // Any successful event means we have a healthy connection; reset
      // the retry budget so a later transient glitch gets a fresh five
      // attempts rather than starting where the earlier one left off.
      sub.retries = 0;
      armIdleTimer();

      const { base } = parseEventType(event.type);

      if (base === "result") {
        const buffered = sub.buffer.splice(0);
        if (sub.rafId) { cancelAnimationFrame(sub.rafId); sub.rafId = 0; }
        const r = rawEvent(event);
        const isError = r.is_error === true || event.type === "result.error";
        const wasCancelled = cancelledRunsRef.current.delete(runId);

        setMessages((prev) =>
          prev.map((m) =>
            m.id === msgId
              ? {
                  ...m,
                  status: wasCancelled ? "interrupted" : isError ? "error" : "completed",
                  events: [...m.events, ...buffered, event],
                }
              : m
          ),
        );
        if (sub.retryTimer) { clearTimeout(sub.retryTimer); sub.retryTimer = null; }
        if (sub.idleTimer) { clearTimeout(sub.idleTimer); sub.idleTimer = null; }
        sub.es.close();
        subsRef.current.delete(runId);
        removeRunning(runId);
        return;
      }

      sub.buffer.push(event);
      if (!sub.rafId) sub.rafId = requestAnimationFrame(() => flushSub(runId));
    };

    // Recovery path: REST history backfill. Used only after we've
    // exhausted reconnect attempts — events live durably server-side,
    // so if the run already finished we can still render it cleanly.
    const finalizeFromHistory = async () => {
      let history: StreamEvent[] | null = null;
      try { history = await api.getRunEvents(runId); } catch {}
      setMessages((prev) =>
        prev.map((m) => {
          if (m.id !== msgId) return m;
          const events = history ?? [...m.events, ...sub.buffer.splice(0)];
          const last = events[events.length - 1];
          const finished = !!last && parseEventType(last.type).base === "result";
          const errored = finished && (rawEvent(last).is_error === true || last.type === "result.error");
          const status: ChatMessage["status"] = finished
            ? (errored ? "error" : "completed")
            : (events.length > 0 ? "completed" : "error");
          return { ...m, events, status };
        }),
      );
      subsRef.current.delete(runId);
      removeRunning(runId);
    };

    const open = () => {
      // Pass afterIndex=-1 to the api helper — we do all dedupe in
      // `dispatch` against the mutable `sub.latestIndex` so reconnects
      // don't have to negotiate with the helper's closure.
      const es = api.subscribeToRunEvents(runId, dispatch, -1);
      // Arm so a never-delivers connection still escalates.
      armIdleTimer();
      es.onerror = () => {
        es.close();
        // The sub may have been replaced (resubscribe) or torn down
        // (terminal in another path). Bail cleanly in either case.
        if (subsRef.current.get(runId) !== sub) return;

        if (sub.retries >= SSE_MAX_RETRIES) {
          if (sub.retryTimer) { clearTimeout(sub.retryTimer); sub.retryTimer = null; }
          if (sub.idleTimer) { clearTimeout(sub.idleTimer); sub.idleTimer = null; }
          void finalizeFromHistory();
          return;
        }
        const delay = SSE_RETRY_DELAYS_MS[sub.retries] ?? 8000;
        sub.retries += 1;
        sub.retryTimer = window.setTimeout(() => {
          sub.retryTimer = null;
          if (subsRef.current.get(runId) !== sub) return;
          open();
        }, delay);
      };
      sub.es = es;
    };

    open();
  }, [flushSub, addRunning, removeRunning]);

  // ── Load messages for a session (parallel event fetch) ────────

  const loadingRef = useRef<string | null>(null);

  const loadMessages = useCallback(async (sessionId: string) => {
    // Prevent duplicate concurrent loads for the same session
    if (loadingRef.current === sessionId) return;
    loadingRef.current = sessionId;
    // Set active immediately to prevent the effect from re-triggering
    setActiveConversationId(sessionId);

    try {
      const [runs, sessionAttachments] = await Promise.all([
        api.listSessionRuns(sessionId),
        api.listSessionAttachments(sessionId).catch(() => [] as ArtifactInfo[]),
      ]);

      const eventsByRun = await Promise.all(
        runs.map((r) => api.getRunEvents(r.id).catch(() => [] as StreamEvent[]))
      );

      // Pair each upload with the next run that followed it (user
      // attached, then sent). Falls back to the first run on clock skew.
      const userUploads = sessionAttachments
        .filter((a) => !a.run_id)
        .sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
      const sortedRuns = [...runs].sort(
        (a, b) => Date.parse(a.created_at) - Date.parse(b.created_at),
      );
      const attachmentsByRunId = new Map<string, ArtifactInfo[]>();
      for (const art of userUploads) {
        const uploadedAt = Date.parse(art.created_at);
        const match =
          sortedRuns.find((r) => Date.parse(r.created_at) >= uploadedAt) ??
          sortedRuns[0];
        if (!match) continue;
        const list = attachmentsByRunId.get(match.id) ?? [];
        list.push(art);
        attachmentsByRunId.set(match.id, list);
      }

      const msgs: ChatMessage[] = [];
      const resumeRuns: { run: RunInfo; afterIndex: number }[] = [];
      runs.forEach((run, i) => {
        const events = eventsByRun[i] ?? [];
        msgs.push(...runToMessages(run, events, attachmentsByRunId.get(run.id)));
        if (run.status === "running" || run.status === "pending") {
          // Client-side dedup anchor: SSE will replay everything in the
          // backend's in-memory event list; drop anything we already have.
          const afterIndex = events.reduce(
            (acc, e) => (typeof e.index === "number" && e.index > acc ? e.index : acc),
            -1,
          );
          resumeRuns.push({ run, afterIndex });
        }
      });

      setMessages(msgs);

      // Resume every still-running run, not just the last.
      for (const { run, afterIndex } of resumeRuns) {
        subscribeToRun(run.id, `${run.id}-assistant`, sessionId, afterIndex);
      }
    } catch (e) {
      console.error("[useChat] loadMessages:", e);
    } finally {
      loadingRef.current = null;
    }
  }, [subscribeToRun]);

  // ── Delete conversation ───────────────────────────────────────

  const deleteConversation = useCallback(async (id: string) => {
    // Optimistic removal so the user sees it disappear immediately. If the
    // API call fails (e.g. running run → 409) we re-fetch the canonical list
    // and surface the error.
    const snapshot = conversationsRef.current;
    setConversations((prev) => prev.filter((c) => c.id !== id));
    if (activeConvRef.current === id) {
      setActiveConversationId(null);
      setMessages([]);
    }
    try {
      await api.deleteSession(id);
    } catch (e) {
      console.error("[useChat] deleteSession failed:", e);
      setConversations(snapshot);
      throw e;
    }
  }, []);

  // ── Send message ──────────────────────────────────────────────

  // Pass `conversationId = null` to start a brand-new conversation. The
  // backend generates the session_id and returns it synchronously; we use it
  // to insert the conversation row and route immediately, so there's no
  // pending-<ts> placeholder state.
  const sendMessage = useCallback(async (
    conversationId: string | null,
    prompt: string,
    attachments?: ArtifactInfo[],
  ): Promise<string> => {
    setPendingSendCount((n) => n + 1);
    // About to take ownership of the spinner via subscribeToRun.
    if (conversationId) clearPendingConv(conversationId);
    try {
      const res = await api.createRun(prompt, conversationId ?? undefined);
      const sid = res.session_id;

      const userMsg: ChatMessage = {
        id: `${res.run_id}-user`,
        role: "user",
        content: prompt,
        status: "sent",
        events: [],
        attachments,
      };
      const assistantMsg: ChatMessage = {
        id: `${res.run_id}-assistant`,
        role: "assistant",
        content: "",
        runId: res.run_id,
        status: "streaming",
        events: [],
      };

      if (conversationId === null) {
        // Fresh conversation — replace messages and seed a sidebar entry.
        setMessages([userMsg, assistantMsg]);
        setConversations((prev) => {
          const now = new Date().toISOString();
          const conv: Conversation = {
            id: sid,
            title: titleFromPrompt(prompt, "New task"),
            agentId: null,
            sessionId: sid,
            createdAt: now,
            updatedAt: now,
            lastMessage: { content: prompt, role: "user", createdAt: now },
          };
          return [conv, ...prev.filter((c) => c.id !== sid)];
        });
        setActiveConversationId(sid);
      } else {
        // Follow-up — append to existing messages, bump the sidebar row.
        setMessages((prev) => [...prev, userMsg, assistantMsg]);
        setConversations((prev) =>
          prev.map((c) => {
            if (c.id !== conversationId) return c;
            const now = new Date().toISOString();
            const title = c.title === "New task" ? titleFromPrompt(prompt, "New task") : c.title;
            return {
              ...c,
              title,
              updatedAt: now,
              lastMessage: { content: prompt, role: "user", createdAt: now },
            };
          }),
        );
      }

      subscribeToRun(res.run_id, assistantMsg.id, sid);
      return sid;
    } catch (e) {
      console.error("[useChat] sendMessage:", e);
      throw e;
    } finally {
      setPendingSendCount((n) => Math.max(0, n - 1));
    }
  }, [subscribeToRun, clearPendingConv]);

  // ── Cancellation ──────────────────────────────────────────────

  // Tear down the local subscription, mark the message as interrupted,
  // and signal the API. The server replies with a terminal `result`
  // event after the worker drains; if it lands before this completes
  // the cancelledRunsRef guard in dispatch keeps the status correct.
  const cancelRun = useCallback(async (runId: string) => {
    cancelledRunsRef.current.add(runId);

    const sub = subsRef.current.get(runId);
    const msgId = sub?.msgId;
    if (sub) {
      sub.es.close();
      if (sub.rafId) cancelAnimationFrame(sub.rafId);
      if (sub.retryTimer) clearTimeout(sub.retryTimer);
      subsRef.current.delete(runId);
    }
    removeRunning(runId);

    if (msgId) {
      setMessages((prev) =>
        prev.map((m) => (m.id === msgId ? { ...m, status: "interrupted" as const } : m)),
      );
    }

    try {
      await api.interruptRun(runId);
    } catch (e) {
      // Benign race: the run reached a terminal state server-side
      // between the Stop click and this RPC landing. The local SSE
      // already finalized the message, so the click "succeeded" from
      // the user's perspective — swallow.
      const msg = e instanceof Error ? e.message : String(e);
      if (/run already RUN_STATUS_/.test(msg)) return;
      console.error("[useChat] cancelRun:", e);
    }
  }, [removeRunning]);

  // ── Memoized context values ───────────────────────────────────

  // Conversations with at least one in-flight run OR an upload-in-progress
  // that hasn't yet dispatched CreateRun. Sidebar reads this directly;
  // per-conv `isStreaming` is derived from it.
  const runningConvs = useMemo(() => {
    const s = new Set<string>(pendingConvs);
    for (const [k, set] of runningByConv) if (set.size > 0) s.add(k);
    return s;
  }, [runningByConv, pendingConvs]);

  // The currently-viewed conversation is "streaming" iff it has a live run
  // or we're between the user clicking Send and createRun returning (the
  // latter only matters for new threads — pendingSendCount covers that).
  const isStreaming = useMemo(
    () =>
      pendingSendCount > 0 ||
      (activeConversationId !== null && runningConvs.has(activeConversationId)),
    [pendingSendCount, activeConversationId, runningConvs],
  );

  const state = useMemo(() => ({
    conversations, messages, activeConversationId, isStreaming, loaded, runningConvs,
  }), [conversations, messages, activeConversationId, isStreaming, loaded, runningConvs]);

  const actions = useMemo(() => ({
    fetchConversations, loadMessages, deleteConversation, sendMessage, cancelRun,
    beginPendingConv, clearPendingConv, setActiveConversationId,
  }), [fetchConversations, loadMessages, deleteConversation, sendMessage, cancelRun, beginPendingConv, clearPendingConv]);

  return { state, actions };
}

// ── Context ──────────────────────────────────────────────────────

type ChatState = ReturnType<typeof useChat>["state"];
type ChatActions = ReturnType<typeof useChat>["actions"];

const ChatStateContext = createContext<ChatState | null>(null);
const ChatActionsContext = createContext<ChatActions | null>(null);

export function ChatProvider({ children }: { children: ReactNode }) {
  const { state, actions } = useChat();
  return (
    <ChatActionsContext.Provider value={actions}>
      <ChatStateContext.Provider value={state}>{children}</ChatStateContext.Provider>
    </ChatActionsContext.Provider>
  );
}

export function useChatState(): ChatState {
  const ctx = useContext(ChatStateContext);
  if (!ctx) throw new Error("useChatState must be used within ChatProvider");
  return ctx;
}

export function useChatActions(): ChatActions {
  const ctx = useContext(ChatActionsContext);
  if (!ctx) throw new Error("useChatActions must be used within ChatProvider");
  return ctx;
}

export function useChatContext(): ChatState & ChatActions {
  return { ...useChatState(), ...useChatActions() };
}
