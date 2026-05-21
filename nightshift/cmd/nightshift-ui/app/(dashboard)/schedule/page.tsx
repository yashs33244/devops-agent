"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import cronstrue from "cronstrue";
import { Cron } from "croner";
import {
  ArrowLeft,
  CalendarClock,
  ChevronRight,
  Clock,
  Loader2,
  Pencil,
  Play,
  Plus,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { PanelHeader, statusDotColor } from "@/lib/ui";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import * as api from "@/lib/api";
import {
  createSchedule,
  deleteSchedule,
  listSchedules,
  listScheduleRuns,
  triggerSchedule,
  updateSchedule,
  type CreateScheduleRequest,
  type RunInfo,
  type ScheduleInfo,
  type StreamEvent,
} from "@/lib/api";
import { parseEventType } from "@/lib/events/parse";
import { ChatMessageItem } from "@/components/chat/chat-message";
import type { ChatMessage } from "@/lib/hooks/useChat";

// ── Timezones ───────────────────────────────────────────────────

const TIMEZONES: string[] = (() => {
  const intl = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf;
  if (typeof intl === "function") {
    try {
      const rest = intl("timeZone").filter((tz) => tz !== "UTC");
      return ["UTC", ...rest];
    } catch { /* fall through */ }
  }
  return [
    "UTC",
    "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
    "Europe/London", "Europe/Berlin", "Asia/Tokyo",
  ];
})();

// ── Cron helpers ────────────────────────────────────────────────

function describeCron(cron: string): string {
  try { return cronstrue.toString(cron, { use24HourTimeFormat: true }); }
  catch { return cron; }
}

function isValidCron(cron: string): boolean {
  try { cronstrue.toString(cron); return true; } catch { return false; }
}

function firstLine(s: string): string {
  const line = s.split("\n", 1)[0] ?? "";
  return line.length > 80 ? `${line.slice(0, 80)}…` : line;
}

function nextRunAt(cron: string, timezone: string): Date | null {
  try {
    const c = new Cron(cron, { timezone: timezone || "UTC" });
    return c.nextRun() ?? null;
  } catch {
    return null;
  }
}

function fmtRelative(date: Date | null): string {
  if (!date) return "—";
  const now = Date.now();
  const diff = date.getTime() - now;
  const abs = Math.abs(diff);
  const past = diff < 0;
  const m = Math.round(abs / 60_000);
  if (m < 1) return past ? "just now" : "in <1m";
  if (m < 60) return past ? `${m}m ago` : `in ${m}m`;
  const h = Math.round(m / 60);
  if (h < 24) return past ? `${h}h ago` : `in ${h}h`;
  const d = Math.round(h / 24);
  if (d < 7) return past ? `${d}d ago` : `in ${d}d`;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function fmtDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s - m * 60;
  return rs ? `${m}m ${rs}s` : `${m}m`;
}

// ── Friendly cron model (preserved from prior page) ─────────────

type Friendly =
  | { kind: "every_minutes"; n: number }
  | { kind: "hourly"; min: number }
  | { kind: "daily"; hour: number; min: number }
  | { kind: "weekly"; dows: number[]; hour: number; min: number }
  | { kind: "monthly"; day: number; hour: number; min: number };

function parseCron(cron: string): Friendly | null {
  const parts = cron.trim().split(/\s+/);
  if (parts.length !== 5) return null;
  const m = parts[0]!, h = parts[1]!, dom = parts[2]!, mon = parts[3]!, dow = parts[4]!;

  const stepMin = /^\*\/(\d+)$/.exec(m);
  if (stepMin && h === "*" && dom === "*" && mon === "*" && dow === "*") {
    const n = Number(stepMin[1]!);
    if (n >= 1 && n <= 59) return { kind: "every_minutes", n };
  }

  const min = /^\d+$/.test(m) ? Number(m) : NaN;
  if (!Number.isFinite(min) || min < 0 || min > 59) return null;

  if (h === "*" && dom === "*" && mon === "*" && dow === "*") return { kind: "hourly", min };

  const hour = /^\d+$/.test(h) ? Number(h) : NaN;
  if (!Number.isFinite(hour) || hour < 0 || hour > 23) return null;

  if (dom === "*" && mon === "*" && dow === "*") return { kind: "daily", hour, min };
  if (dom === "*" && mon === "*" && dow !== "*") {
    const dows = parseDowList(dow);
    if (dows) return { kind: "weekly", dows, hour, min };
  }
  if (dom !== "*" && mon === "*" && dow === "*" && /^\d+$/.test(dom)) {
    const day = Number(dom);
    if (day >= 1 && day <= 31) return { kind: "monthly", day, hour, min };
  }
  return null;
}

function parseDowList(s: string): number[] | null {
  const set = new Set<number>();
  for (const piece of s.split(",")) {
    const range = /^(\d)-(\d)$/.exec(piece);
    if (range) {
      const lo = Number(range[1]!), hi = Number(range[2]!);
      if (lo < 0 || lo > 6 || hi < 0 || hi > 6 || lo > hi) return null;
      for (let i = lo; i <= hi; i++) set.add(i);
    } else if (/^\d$/.test(piece)) {
      const n = Number(piece);
      if (n < 0 || n > 6) return null;
      set.add(n);
    } else return null;
  }
  return Array.from(set).sort((a, b) => a - b);
}

function buildCron(f: Friendly): string {
  switch (f.kind) {
    case "every_minutes": return `*/${f.n} * * * *`;
    case "hourly":        return `${f.min} * * * *`;
    case "daily":         return `${f.min} ${f.hour} * * *`;
    case "weekly":        return `${f.min} ${f.hour} * * ${compactDowList(f.dows)}`;
    case "monthly":       return `${f.min} ${f.hour} ${f.day} * *`;
  }
}

function compactDowList(dows: number[]): string {
  if (dows.length === 0) return "0";
  const sorted = [...dows].sort((a, b) => a - b);
  const out: string[] = [];
  let i = 0;
  while (i < sorted.length) {
    let j = i;
    while (j + 1 < sorted.length && sorted[j + 1]! === sorted[j]! + 1) j++;
    if (j - i >= 2) out.push(`${sorted[i]!}-${sorted[j]!}`);
    else for (let k = i; k <= j; k++) out.push(String(sorted[k]!));
    i = j + 1;
  }
  return out.join(",");
}

const DOWS = [
  { n: 0, short: "Sun" }, { n: 1, short: "Mon" }, { n: 2, short: "Tue" }, { n: 3, short: "Wed" },
  { n: 4, short: "Thu" }, { n: 5, short: "Fri" }, { n: 6, short: "Sat" },
];

const DEFAULTS: Record<Friendly["kind"], Friendly> = {
  every_minutes: { kind: "every_minutes", n: 15 },
  hourly:        { kind: "hourly", min: 0 },
  daily:         { kind: "daily", hour: 9, min: 0 },
  weekly:        { kind: "weekly", dows: [1], hour: 9, min: 0 },
  monthly:       { kind: "monthly", day: 1, hour: 9, min: 0 },
};

const KIND_LABEL: Record<Friendly["kind"] | "advanced", string> = {
  every_minutes: "Every N minutes",
  hourly:        "Every hour",
  daily:         "Every day",
  weekly:        "Every week",
  monthly:       "Every month",
  advanced:      "Custom…",
};

// ── Page ────────────────────────────────────────────────────────

type Mode = { kind: "idle" } | { kind: "view"; id: string } | { kind: "create" };

export default function SchedulePage() {
  const [schedules, setSchedules] = useState<ScheduleInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [mode, setMode] = useState<Mode>({ kind: "idle" });

  // ── Live run tracking ──
  // Per-page SSE bookkeeping for in-flight schedule fires. The reactive
  // `runningSchedules` Set drives sidebar spinners + the Run-now button
  // disabled state; `runsBumper` increments whenever a tracked run
  // changes terminal state so the run-history list refetches without
  // requiring focus.
  const subsRef = useRef<Map<string, EventSource>>(new Map());
  const runScheduleMap = useRef<Map<string, string>>(new Map());
  const [runningSchedules, setRunningSchedules] = useState<Set<string>>(new Set());
  const [runsBumper, setRunsBumper] = useState(0);

  const refreshRunningSet = useCallback(() => {
    setRunningSchedules(new Set(runScheduleMap.current.values()));
  }, []);

  const trackRun = useCallback((runId: string, scheduleId: string) => {
    if (subsRef.current.has(runId)) return;
    runScheduleMap.current.set(runId, scheduleId);
    refreshRunningSet();
    const es = api.subscribeToRunEvents(runId, (event: StreamEvent) => {
      if (parseEventType(event.type).base === "result") {
        es.close();
        subsRef.current.delete(runId);
        runScheduleMap.current.delete(runId);
        refreshRunningSet();
        setRunsBumper((b) => b + 1);
      }
    }, -1);
    es.onerror = () => {
      // The server closes the stream once a run is terminal. We don't
      // reconnect — a stale "running" indicator is corrected when the
      // user reopens the schedule (initial fetch reseeds the map).
      es.close();
      if (subsRef.current.get(runId) === es) {
        subsRef.current.delete(runId);
        runScheduleMap.current.delete(runId);
        refreshRunningSet();
        setRunsBumper((b) => b + 1);
      }
    };
    subsRef.current.set(runId, es);
  }, [refreshRunningSet]);

  // Cleanup all SSE on unmount.
  useEffect(() => {
    return () => {
      for (const es of subsRef.current.values()) es.close();
      subsRef.current.clear();
      runScheduleMap.current.clear();
    };
  }, []);

  const PANEL_KEY = "nightshift-schedule-panel-width";
  const PANEL_MIN = 280, PANEL_MAX = 600, PANEL_DEFAULT = 340;
  const [panelWidth, setPanelWidth] = useState(PANEL_DEFAULT);
  const panelRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const saved = localStorage.getItem(PANEL_KEY);
    if (!saved) return;
    const n = parseInt(saved, 10);
    if (!Number.isFinite(n)) return;
    setPanelWidth(Math.min(PANEL_MAX, Math.max(PANEL_MIN, n)));
  }, []);

  const handlePanelDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const node = panelRef.current;
    if (!node) return;
    const startX = e.clientX;
    const startWidth = node.getBoundingClientRect().width;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";

    const onMove = (ev: MouseEvent) => {
      const next = Math.min(PANEL_MAX, Math.max(PANEL_MIN, startWidth + (ev.clientX - startX)));
      node.style.width = `${next}px`;
    };
    const onUp = () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
      const final = Math.round(node.getBoundingClientRect().width);
      setPanelWidth(final);
      try { localStorage.setItem(PANEL_KEY, String(final)); } catch {}
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, []);

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const list = await listSchedules();
      list.sort((a, b) => b.created_at.localeCompare(a.created_at));
      setSchedules(list);
    } catch (e) {
      setError((e as Error).message);
      setSchedules([]);
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);
  useEffect(() => {
    const onFocus = () => { refresh(); };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refresh]);

  // Deep-link: /schedule?id=<schedule_id> auto-opens that schedule.
  // Used by the chat-view "Triggered by schedule" pill so an
  // attribution click lands directly on the right detail pane.
  const searchParams = useSearchParams();
  useEffect(() => {
    const id = searchParams?.get("id");
    if (!id || !schedules) return;
    if (!schedules.some((s) => s.id === id)) return;
    setMode((cur) => (cur.kind === "view" && cur.id === id ? cur : { kind: "view", id }));
  }, [searchParams, schedules]);

  const filtered = useMemo(() => {
    if (!schedules) return [];
    const q = query.trim().toLowerCase();
    if (!q) return schedules;
    return schedules.filter((s) => s.prompt.toLowerCase().includes(q) || s.cron.toLowerCase().includes(q));
  }, [schedules, query]);

  const selectedId = mode.kind === "view" ? mode.id : null;
  const selected = selectedId ? schedules?.find((s) => s.id === selectedId) ?? null : null;

  return (
    <div className="flex-1 flex min-h-0">
      <aside
        ref={panelRef}
        style={{ width: panelWidth }}
        className="@container shrink-0 flex flex-col h-full bg-night"
      >
        <PanelHeader>
          <span className="text-[15px] font-semibold text-primary">Schedules</span>
          <button
            onClick={() => setMode({ kind: "create" })}
            title="New schedule"
            className="flex items-center justify-center gap-1.5 h-7 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors w-7 @sm:w-auto @sm:pl-2 @sm:pr-2.5"
          >
            <Plus size={14} />
            <span className="hidden @sm:inline">New schedule</span>
          </button>
        </PanelHeader>

        <div className="p-3 shrink-0 border-b border-night-border">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search scheduled tasks"
              className="pl-9 pr-3 py-1.5 text-sm w-full rounded-lg border border-night-border bg-transparent text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors"
            />
            {query && (
              <button
                onClick={() => setQuery("")}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted hover:text-secondary"
              >
                <X size={12} />
              </button>
            )}
          </div>
        </div>

        {error && (
          <div className="mx-3 mt-3 rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error shrink-0">
            Failed to load: {error}
          </div>
        )}

        <div className="flex-1 overflow-y-auto terminal-scroll">
          {schedules === null ? (
            <div className="flex items-center gap-2 py-12 justify-center text-sm text-muted">
              <Loader2 size={14} className="animate-spin" /> Loading…
            </div>
          ) : schedules.length === 0 ? (
            <EmptyList onCreate={() => setMode({ kind: "create" })} />
          ) : filtered.length === 0 ? (
            <p className="text-sm text-muted text-center py-12">No matches</p>
          ) : (
            filtered.map((s) => (
              <ScheduleRow
                key={s.id}
                schedule={s}
                active={selectedId === s.id}
                running={runningSchedules.has(s.id)}
                onClick={() => setMode({ kind: "view", id: s.id })}
              />
            ))
          )}
        </div>
      </aside>

      <div
        onMouseDown={handlePanelDragStart}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize schedules panel"
        className="group relative shrink-0 w-1.5 cursor-col-resize"
      >
        <div className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-night-border group-hover:bg-night-active group-active:bg-night-hover transition-colors" />
      </div>

      <section className="flex-1 min-w-0">
        {mode.kind === "idle" && <IdlePane />}
        {mode.kind === "create" && (
          <CreatePane
            onCancel={() => setMode({ kind: "idle" })}
            onSaved={async (info) => {
              await refresh();
              setMode({ kind: "view", id: info.id });
            }}
          />
        )}
        {mode.kind === "view" && selected && (
          <DetailPane
            schedule={selected}
            running={runningSchedules.has(selected.id)}
            runsBumper={runsBumper}
            trackRun={trackRun}
            onChanged={refresh}
            onDeleted={async () => {
              setMode({ kind: "idle" });
              await refresh();
            }}
          />
        )}
      </section>
    </div>
  );
}

// ── List row ────────────────────────────────────────────────────

function ScheduleRow({
  schedule, active, running, onClick,
}: {
  schedule: ScheduleInfo;
  active: boolean;
  running: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "w-full flex items-start gap-2.5 px-4 py-3 text-left border-b border-night-border transition-colors",
        active ? "bg-night-hover" : "hover:bg-night-hover",
      )}
    >
      {running ? (
        <Loader2 size={11} className="text-lime shrink-0 mt-1 animate-spin" />
      ) : (
        <span
          className={cn("size-1.5 rounded-full shrink-0 mt-1.5", schedule.enabled ? "bg-lime" : "bg-muted")}
          title={schedule.enabled ? "Enabled" : "Paused"}
        />
      )}
      <div className="min-w-0 flex-1">
        <p className={cn("text-[13px] truncate", active ? "text-primary" : "text-secondary")}>
          {firstLine(schedule.prompt) || <span className="italic">Untitled</span>}
        </p>
        <p className="text-[11px] text-muted mt-1 truncate">
          {describeCron(schedule.cron)} · {schedule.timezone}
        </p>
      </div>
    </button>
  );
}

function EmptyList({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center gap-3 py-12 px-6 text-center">
      <div className="size-10 rounded-2xl bg-night-hover flex items-center justify-center">
        <CalendarClock size={16} className="text-muted" />
      </div>
      <p className="text-sm font-medium text-primary">No schedules</p>
      <p className="text-xs text-muted max-w-[220px]">Save a prompt + cadence so the agent auto-fires it.</p>
      <button
        onClick={onCreate}
        className="inline-flex items-center gap-1.5 pl-2 pr-2.5 h-8 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
      >
        <Plus size={14} />
        New schedule
      </button>
    </div>
  );
}

function IdlePane() {
  return (
    <div className="h-full flex flex-col items-center justify-center gap-2 text-center px-6">
      <CalendarClock size={20} className="text-muted" />
      <p className="text-sm text-muted">Select a schedule or create a new one.</p>
    </div>
  );
}

// ── Detail pane: instructions + repeats + run history ───────────

function DetailPane({
  schedule, running, runsBumper, trackRun, onChanged, onDeleted,
}: {
  schedule: ScheduleInfo;
  running: boolean;
  runsBumper: number;
  trackRun: (runId: string, scheduleId: string) => void;
  onChanged: () => Promise<void> | void;
  onDeleted: () => Promise<void> | void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [editingPrompt, setEditingPrompt] = useState(false);
  const [showCronModal, setShowCronModal] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);

  // Reset transient state when switching schedules.
  useEffect(() => {
    setEditingPrompt(false);
    setShowCronModal(false);
    setConfirmDelete(false);
    setFormError(null);
    setSelectedRunId(null);
  }, [schedule.id]);

  const next = useMemo(
    () => (schedule.enabled ? nextRunAt(schedule.cron, schedule.timezone) : null),
    [schedule.cron, schedule.timezone, schedule.enabled],
  );

  const toggleEnabled = async () => {
    if (busy) return;
    setBusy("toggle");
    setFormError(null);
    try {
      await updateSchedule(schedule.id, { enabled: !schedule.enabled });
      await onChanged();
    } catch (e) {
      setFormError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const runNow = async () => {
    if (busy) return;
    setBusy("run");
    setFormError(null);
    try {
      const run = await triggerSchedule(schedule.id);
      // Subscribe so the sidebar spinner clears and the run-history
      // refetches the moment the worker finishes — without a refresh.
      if (run?.id) trackRun(run.id, schedule.id);
      await onChanged();
    } catch (e) {
      setFormError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const doDelete = async () => {
    if (!confirmDelete) { setConfirmDelete(true); return; }
    setBusy("delete");
    setFormError(null);
    try {
      await deleteSchedule(schedule.id);
      await onDeleted();
    } catch (e) {
      setFormError((e as Error).message);
      setBusy(null);
    }
  };

  return (
    <div className="h-full flex flex-col min-h-0">
      <div className="flex-1 overflow-y-auto terminal-scroll">
        <div className="px-8 md:px-12 lg:px-16 pt-8 pb-10 max-w-[1200px]">
          {/* Header */}
          <header className="flex items-start gap-6 mb-3">
            <div className="min-w-0 flex-1">
              <h1 className="text-2xl font-semibold text-primary tracking-tight leading-tight line-clamp-2">
                {firstLine(schedule.prompt) || "Untitled schedule"}
              </h1>
              <div className="flex items-center gap-2 mt-2 text-[13px]">
                {running ? (
                  <span className="inline-flex items-center gap-1.5 text-lime">
                    <Loader2 size={12} className="animate-spin" />
                    Running
                  </span>
                ) : schedule.enabled ? (
                  <span className="inline-flex items-center gap-1.5 text-lime">
                    <span className="size-1.5 rounded-full bg-lime" />
                    Active
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 text-muted">
                    <span className="size-1.5 rounded-full bg-muted" />
                    Paused
                  </span>
                )}
                <span className="text-muted">·</span>
                <span className="text-muted">
                  {schedule.enabled
                    ? next ? `Next ${fmtRelative(next)}` : "Next run unknown"
                    : "Resume to schedule next run"}
                </span>
              </div>
            </div>
            <div className="flex items-center gap-1.5 shrink-0 pt-1">
              <button
                onClick={doDelete}
                disabled={busy === "delete"}
                className={cn(
                  "size-9 flex items-center justify-center rounded-lg transition-colors",
                  confirmDelete
                    ? "text-error bg-error/5 hover:bg-error/10"
                    : "text-muted hover:text-error hover:bg-night-hover",
                )}
                title={confirmDelete ? "Click again to confirm" : "Delete"}
              >
                {busy === "delete" ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
              </button>
              <button
                onClick={runNow}
                disabled={busy === "run" || running}
                className="flex items-center gap-2 ml-1 px-4 py-2 rounded-lg border border-night-border text-sm font-medium text-primary hover:bg-night-hover transition-colors disabled:opacity-50"
              >
                {busy === "run" || running ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}
                Run now
              </button>
            </div>
          </header>

          {formError && (
            <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error mb-4">
              {formError}
            </div>
          )}

          {/* Body — swaps to inline run view when a row is selected. */}
          {selectedRunId ? (
            <RunView
              runId={selectedRunId}
              prompt={schedule.prompt}
              onClose={() => setSelectedRunId(null)}
            />
          ) : (
            <div className="grid grid-cols-1 lg:grid-cols-[1fr_1px_22rem] gap-0 mt-6">
              <div className="space-y-7 pr-0 lg:pr-8">
                <InstructionsBlock
                  schedule={schedule}
                  editing={editingPrompt}
                  onEdit={() => setEditingPrompt(true)}
                  onCancel={() => setEditingPrompt(false)}
                  onSaved={async () => { setEditingPrompt(false); await onChanged(); }}
                />

                <div className="border-t border-night-border pt-6">
                  <RepeatsBlock
                    schedule={schedule}
                    busy={busy === "toggle"}
                    onToggle={toggleEnabled}
                    onEdit={() => setShowCronModal(true)}
                  />
                </div>
              </div>

              <div className="hidden lg:block bg-night-border" />

              <div className="mt-8 lg:mt-0 lg:pl-8">
                <ScheduleRuns
                  scheduleId={schedule.id}
                  bumper={runsBumper}
                  onSelectRun={setSelectedRunId}
                  onRunsLoaded={(runs) => {
                    for (const r of runs) {
                      if (r.status === "running" || r.status === "pending") {
                        trackRun(r.id, schedule.id);
                      }
                    }
                  }}
                />
              </div>
            </div>
          )}
        </div>
      </div>

      <ScheduleEditDialog
        open={showCronModal}
        onOpenChange={setShowCronModal}
        schedule={schedule}
        onSaved={async () => { setShowCronModal(false); await onChanged(); }}
      />
    </div>
  );
}

// ── Instructions block ──────────────────────────────────────────

function InstructionsBlock({
  schedule, editing, onEdit, onCancel, onSaved,
}: {
  schedule: ScheduleInfo;
  editing: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onSaved: () => Promise<void> | void;
}) {
  const [draft, setDraft] = useState(schedule.prompt);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => { setDraft(schedule.prompt); setErr(null); }, [schedule.id, schedule.prompt, editing]);

  const save = async () => {
    if (busy) return;
    const trimmed = draft.trim();
    if (!trimmed || trimmed === schedule.prompt) { onCancel(); return; }
    setBusy(true);
    setErr(null);
    try {
      await updateSchedule(schedule.id, { prompt: trimmed });
      await onSaved();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-xs font-medium uppercase tracking-wider text-muted">Instructions</h3>
        {!editing && (
          <button
            onClick={onEdit}
            className="size-6 flex items-center justify-center rounded text-muted hover:text-secondary hover:bg-night-hover transition-colors"
            aria-label="Edit instructions"
          >
            <Pencil size={12} />
          </button>
        )}
      </div>
      {editing ? (
        <div className="space-y-2">
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            rows={6}
            className="w-full px-3 py-2 text-[13px] rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors resize-y leading-relaxed terminal-scroll min-h-[140px]"
          />
          {err && <p className="text-xs text-error">{err}</p>}
          <div className="flex items-center justify-end gap-2">
            <Button variant="outline" size="sm" onClick={onCancel}>Cancel</Button>
            <Button size="sm" onClick={save} disabled={busy || !draft.trim()}>
              {busy && <Loader2 size={14} className="mr-1.5 animate-spin" />}
              Save
            </Button>
          </div>
        </div>
      ) : (
        <p className="text-[13px] text-secondary leading-relaxed whitespace-pre-wrap">
          {schedule.prompt}
        </p>
      )}
    </section>
  );
}

// ── Repeats block ───────────────────────────────────────────────

function RepeatsBlock({
  schedule, busy, onToggle, onEdit,
}: {
  schedule: ScheduleInfo;
  busy: boolean;
  onToggle: () => void;
  onEdit: () => void;
}) {
  return (
    <section>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-xs font-medium uppercase tracking-wider text-muted">Repeats</h3>
        <button
          onClick={onToggle}
          disabled={busy}
          className={cn(
            "relative inline-flex h-[22px] w-[40px] items-center rounded-full transition-colors shrink-0",
            schedule.enabled ? "bg-lime" : "bg-night-active",
            busy && "opacity-60",
          )}
          title={schedule.enabled ? "Pause schedule" : "Resume schedule"}
        >
          <span className={cn(
            "inline-block size-4 rounded-full bg-white transition-transform",
            schedule.enabled ? "translate-x-[20px]" : "translate-x-[3px]",
          )} />
        </button>
      </div>
      <button
        onClick={onEdit}
        className="w-full text-left rounded-xl border border-night-border bg-night-surface px-4 py-3 hover:bg-night-hover transition-colors group"
      >
        <p className="text-[13px] text-primary">
          {describeCron(schedule.cron)}
        </p>
        <p className="text-[11px] text-muted mt-1 font-mono">
          {schedule.cron} · {schedule.timezone}
        </p>
      </button>
    </section>
  );
}

// ── Schedule edit dialog ────────────────────────────────────────

function ScheduleEditDialog({
  open, onOpenChange, schedule, onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  schedule: ScheduleInfo;
  onSaved: () => Promise<void> | void;
}) {
  const [cron, setCron] = useState(schedule.cron);
  const [timezone, setTimezone] = useState(schedule.timezone);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setCron(schedule.cron);
    setTimezone(schedule.timezone);
    setErr(null);
  }, [open, schedule.cron, schedule.timezone]);

  const valid = isValidCron(cron);
  const dirty = cron !== schedule.cron || timezone !== schedule.timezone;

  const save = async () => {
    if (busy || !valid || !dirty) return;
    setBusy(true);
    setErr(null);
    try {
      await updateSchedule(schedule.id, { cron, timezone });
      await onSaved();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Edit schedule</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <Field label="Runs">
            <CronEditor cron={cron} onChange={setCron} />
          </Field>
          <Field label="Timezone">
            <select
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              className="w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors"
            >
              {TIMEZONES.map((tz) => <option key={tz} value={tz}>{tz}</option>)}
            </select>
          </Field>
          {err && (
            <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error">{err}</div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button size="sm" onClick={save} disabled={busy || !valid || !dirty}>
            {busy && <Loader2 size={14} className="mr-1.5 animate-spin" />}
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ── Run history ─────────────────────────────────────────────────

const RUNS_CAP = 50;

function ScheduleRuns({
  scheduleId, bumper, onSelectRun, onRunsLoaded,
}: {
  scheduleId: string;
  bumper: number;
  onSelectRun: (runId: string) => void;
  onRunsLoaded: (runs: RunInfo[]) => void;
}) {
  const [runs, setRuns] = useState<RunInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // The latest onRunsLoaded callback. Refs so the refresh closure
  // doesn't have to be re-created (and re-fired) every render.
  const onRunsLoadedRef = useRef(onRunsLoaded);
  onRunsLoadedRef.current = onRunsLoaded;

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const list = await listScheduleRuns(scheduleId);
      list.sort((a, b) => (b.created_at ?? "").localeCompare(a.created_at ?? ""));
      setRuns(list);
      onRunsLoadedRef.current(list);
    } catch (e) {
      setError((e as Error).message);
      setRuns([]);
    }
  }, [scheduleId]);

  // Refetch on schedule change AND whenever the parent bumps after a
  // tracked run terminates or a new fire is triggered.
  useEffect(() => { refresh(); }, [refresh, bumper]);
  useEffect(() => {
    const onFocus = () => { refresh(); };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refresh]);

  // Total fire count is what we use to compute per-row fire numbers.
  // Older rows further down the list get smaller numbers.
  const total = runs?.length ?? 0;
  const capped = runs ? runs.slice(0, RUNS_CAP) : [];
  const overflow = runs ? Math.max(0, runs.length - RUNS_CAP) : 0;

  return (
    <section>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-xs font-medium uppercase tracking-wider text-muted">Run history</h3>
        {runs !== null && (
          <span className="text-[11px] text-muted tabular-nums">{runs.length}</span>
        )}
      </div>

      {error && (
        <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error mb-2">
          Failed to load: {error}
        </div>
      )}

      {runs === null ? (
        <div className="flex items-center gap-2 py-6 justify-center text-xs text-muted">
          <Loader2 size={12} className="animate-spin" /> Loading…
        </div>
      ) : runs.length === 0 ? (
        <div className="rounded-xl border border-night-border border-dashed px-4 py-8 text-center">
          <Clock size={14} className="mx-auto text-muted mb-2" />
          <p className="text-xs text-muted">
            No runs yet — the first fire will appear here.
          </p>
        </div>
      ) : (
        <div className="space-y-1">
          {capped.map((r, i) => (
            <RunRow key={r.id} run={r} fireNumber={total - i} onSelect={() => onSelectRun(r.id)} />
          ))}
          {overflow > 0 && (
            <p className="px-2 pt-2 text-[11px] text-muted">
              {RUNS_CAP} most-recent shown — {overflow} older hidden.
            </p>
          )}
        </div>
      )}
    </section>
  );
}

function RunRow({
  run, fireNumber, onSelect,
}: {
  run: RunInfo;
  fireNumber: number;
  onSelect: () => void;
}) {
  const isManual = run.trigger_mode === "manual";
  const isLive = run.status === "running" || run.status === "pending";
  const duration = (() => {
    if (!run.started_at || !run.ended_at) return null;
    const ms = new Date(run.ended_at).getTime() - new Date(run.started_at).getTime();
    return Number.isFinite(ms) && ms > 0 ? fmtDuration(ms) : null;
  })();
  const when = run.started_at ?? run.created_at;
  const whenLabel = when ? new Date(when).toLocaleString() : "—";

  return (
    <button
      onClick={onSelect}
      className="w-full group flex items-center gap-3 px-2 py-2 -mx-2 rounded-lg hover:bg-night-hover transition-colors text-left"
    >
      {isLive ? (
        <Loader2 size={11} className="text-lime shrink-0 animate-spin" />
      ) : (
        <span className={cn("size-1.5 rounded-full shrink-0", statusDotColor(run.status))} title={run.status} />
      )}
      <span className="text-[11px] font-mono text-muted tabular-nums shrink-0">#{fireNumber}</span>
      <div className="min-w-0 flex-1">
        <p className="text-[13px] text-secondary truncate group-hover:text-primary transition-colors">
          {whenLabel}
        </p>
        {duration && (
          <p className="text-[11px] text-muted tabular-nums">{duration}</p>
        )}
      </div>
      <span
        className={cn(
          "text-[10px] uppercase tracking-wider px-1.5 py-0.5 rounded shrink-0",
          isManual ? "text-lime bg-lime/10" : "text-muted bg-night-active",
        )}
      >
        {isManual ? "manual" : "scheduled"}
      </span>
      <ChevronRight size={12} className="text-muted opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
    </button>
  );
}

// ── Inline run view ─────────────────────────────────────────────

// Loads a run's events + subscribes to live updates, renders them as
// chat messages. Mirrors cr0n-demo's pattern of opening a run inline
// inside the schedule pane rather than navigating to /tasks/<sid>.
function RunView({
  runId, prompt, onClose,
}: {
  runId: string;
  prompt: string;
  onClose: () => void;
}) {
  const [events, setEvents] = useState<StreamEvent[]>([]);
  const [status, setStatus] = useState<ChatMessage["status"]>("streaming");
  const [error, setError] = useState<string | null>(null);
  const latestIndex = useRef(-1);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    let es: EventSource | null = null;
    setEvents([]);
    setError(null);
    setStatus("streaming");
    latestIndex.current = -1;

    (async () => {
      try {
        const history = await api.getRunEvents(runId);
        if (cancelled) return;
        setEvents(history);
        for (const e of history) {
          if (typeof e.index === "number" && e.index > latestIndex.current) {
            latestIndex.current = e.index;
          }
        }
        const last = history[history.length - 1];
        const finished = last && parseEventType(last.type).base === "result";
        if (finished) {
          const isErr = last.type === "result.error";
          setStatus(isErr ? "error" : "completed");
          return;
        }
        // Still running — open SSE for the tail.
        es = api.subscribeToRunEvents(runId, (event) => {
          if (typeof event.index === "number") {
            if (event.index <= latestIndex.current) return;
            latestIndex.current = event.index;
          }
          setEvents((prev) => [...prev, event]);
          if (parseEventType(event.type).base === "result") {
            const isErr = event.type === "result.error";
            setStatus(isErr ? "error" : "completed");
            es?.close();
          }
        }, latestIndex.current);
      } catch (e) {
        if (!cancelled) setError((e as Error).message);
      }
    })();

    return () => {
      cancelled = true;
      es?.close();
    };
  }, [runId]);

  // Auto-scroll on new events.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 150;
    if (nearBottom) el.scrollTop = el.scrollHeight;
  }, [events.length]);

  const messages: ChatMessage[] = useMemo(() => [
    { id: `${runId}-user`,      role: "user",      content: prompt, runId, status: "sent",  events: [] },
    { id: `${runId}-assistant`, role: "assistant", content: "",     runId, status,          events },
  ], [runId, prompt, status, events]);

  return (
    <div className="mt-6 flex flex-col min-h-0">
      <div className="flex items-center justify-between border-y border-night-border py-2.5">
        <button
          onClick={onClose}
          className="flex items-center gap-1.5 text-[13px] text-muted hover:text-secondary transition-colors"
        >
          <ArrowLeft size={13} />
          Back to schedule
        </button>
        <span className="text-[11px] text-muted">
          {status === "streaming" ? "Live" : status === "error" ? "Failed" : "Completed"}
        </span>
      </div>

      {error && (
        <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error mt-3">
          {error}
        </div>
      )}

      <div ref={scrollRef} className="flex-1 overflow-y-auto terminal-scroll mt-4">
        <div className="space-y-6 pb-6">
          {messages.map((m) => (
            <ChatMessageItem key={m.id} message={m} />
          ))}
        </div>
      </div>
    </div>
  );
}

// ── Create flow ─────────────────────────────────────────────────

const DEFAULT_FORM: CreateScheduleRequest = {
  prompt: "",
  cron: "0 9 * * *",
  timezone: "UTC",
  enabled: true,
};

function CreatePane({
  onCancel, onSaved,
}: {
  onCancel: () => void;
  onSaved: (info: ScheduleInfo) => Promise<void> | void;
}) {
  const [value, setValue] = useState<CreateScheduleRequest>(DEFAULT_FORM);
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const cronValid = isValidCron(value.cron);
  const canSubmit = !busy && value.prompt.trim() && cronValid;

  const submit = async () => {
    if (!canSubmit) return;
    setBusy(true);
    setFormError(null);
    try {
      const info = await createSchedule({
        prompt: value.prompt.trim(),
        cron: value.cron.trim(),
        timezone: value.timezone || "UTC",
        enabled: value.enabled ?? true,
      });
      await onSaved(info);
    } catch (e) {
      setFormError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <div className="h-full overflow-y-auto terminal-scroll">
      <div className="max-w-2xl px-8 md:px-12 lg:px-16 py-8 space-y-5">
        <header className="flex items-center justify-between gap-3">
          <h1 className="text-2xl font-semibold text-primary tracking-tight">New schedule</h1>
          <button
            onClick={onCancel}
            aria-label="Close"
            className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-secondary hover:bg-night-hover transition-colors shrink-0"
          >
            <X size={16} />
          </button>
        </header>

        <Field label="Instructions">
          <textarea
            value={value.prompt}
            onChange={(e) => setValue({ ...value, prompt: e.target.value })}
            rows={8}
            placeholder="What should the agent do when this fires?"
            className="w-full px-3 py-2 text-[13px] rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors resize-y leading-relaxed terminal-scroll min-h-[160px]"
          />
        </Field>

        <Field label="Runs">
          <CronEditor cron={value.cron} onChange={(cron) => setValue({ ...value, cron })} />
        </Field>

        <Field label="Timezone">
          <select
            value={value.timezone}
            onChange={(e) => setValue({ ...value, timezone: e.target.value })}
            className="w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors"
          >
            {TIMEZONES.map((tz) => <option key={tz} value={tz}>{tz}</option>)}
          </select>
        </Field>

        {formError && (
          <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error">{formError}</div>
        )}

        <div className="flex items-center justify-end gap-2 pt-2 border-t border-night-border">
          <Button variant="outline" size="sm" onClick={onCancel}>Cancel</Button>
          <Button size="sm" onClick={submit} disabled={!canSubmit}>
            {busy && <Loader2 size={14} className="mr-1.5 animate-spin" />}
            Create
          </Button>
        </div>
      </div>
    </div>
  );
}

// ── Cron editor + sub-fields (preserved from prior implementation) ─

function CronEditor({ cron, onChange }: { cron: string; onChange: (next: string) => void }) {
  const parsed = useMemo(() => parseCron(cron), [cron]);
  const [advanced, setAdvanced] = useState(() => parsed === null);
  const [friendly, setFriendly] = useState<Friendly>(() => parsed ?? DEFAULTS.daily);

  useEffect(() => { if (parsed) setFriendly(parsed); }, [parsed]);

  const emitFromFriendly = (next: Friendly) => {
    setFriendly(next);
    onChange(buildCron(next));
  };

  const valid = isValidCron(cron);
  const preview = valid ? describeCron(cron) : "Invalid cron expression";

  return (
    <div className="space-y-3">
      {!advanced ? (
        <SimpleFields value={friendly} onChange={emitFromFriendly} />
      ) : (
        <input
          type="text"
          value={cron}
          onChange={(e) => onChange(e.target.value)}
          spellCheck={false}
          placeholder="minute hour day-of-month month day-of-week"
          className={cn(
            "w-full px-3 py-2 text-sm rounded-lg border bg-night-hover text-primary font-mono focus:outline-none transition-colors",
            valid
              ? "border-night-border focus:border-primary/40 focus:ring-2 focus:ring-primary/10"
              : "border-error/40 focus:border-error/60",
          )}
        />
      )}

      <div className="flex items-center justify-between gap-3 text-[11px]">
        <span className={cn("flex-1 min-w-0 truncate", valid ? "text-muted" : "text-error")}>
          {preview}
          {valid && <span className="ml-2 font-mono opacity-60">({cron})</span>}
        </span>
        <button
          type="button"
          onClick={() => setAdvanced(!advanced)}
          className="shrink-0 text-muted hover:text-secondary underline underline-offset-2"
        >
          {advanced ? "Simple" : "Advanced"}
        </button>
      </div>
    </div>
  );
}

function SimpleFields({ value, onChange }: { value: Friendly; onChange: (next: Friendly) => void }) {
  const switchKind = (kind: Friendly["kind"]) => {
    if (kind === value.kind) return;
    const hour = "hour" in value ? value.hour : 9;
    const min = "min" in value ? value.min : 0;
    const base = DEFAULTS[kind];
    if (kind === "every_minutes") onChange(base);
    else if (kind === "hourly") onChange({ kind: "hourly", min });
    else if (kind === "daily") onChange({ kind: "daily", hour, min });
    else if (kind === "weekly") onChange({ kind: "weekly", dows: [1], hour, min });
    else if (kind === "monthly") onChange({ kind: "monthly", day: 1, hour, min });
  };

  return (
    <div className="space-y-3">
      <select
        value={value.kind}
        onChange={(e) => switchKind(e.target.value as Friendly["kind"])}
        className="w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors"
      >
        <option value="every_minutes">{KIND_LABEL.every_minutes}</option>
        <option value="hourly">{KIND_LABEL.hourly}</option>
        <option value="daily">{KIND_LABEL.daily}</option>
        <option value="weekly">{KIND_LABEL.weekly}</option>
        <option value="monthly">{KIND_LABEL.monthly}</option>
      </select>

      {value.kind === "every_minutes" && (
        <NumberRow prefix="Every" suffix="minutes" value={value.n} min={1} max={59}
          onChange={(n) => onChange({ ...value, n })} />
      )}
      {value.kind === "hourly" && (
        <NumberRow prefix="At" suffix="minutes past the hour" value={value.min} min={0} max={59}
          onChange={(min) => onChange({ ...value, min })} />
      )}
      {value.kind === "daily" && (
        <TimeOfDayRow hour={value.hour} min={value.min}
          onChange={(hour, min) => onChange({ ...value, hour, min })} />
      )}
      {value.kind === "weekly" && (
        <>
          <DowPicker selected={value.dows} onChange={(dows) => onChange({ ...value, dows })} />
          <TimeOfDayRow hour={value.hour} min={value.min}
            onChange={(hour, min) => onChange({ ...value, hour, min })} />
        </>
      )}
      {value.kind === "monthly" && (
        <>
          <NumberRow prefix="On day" suffix="of the month" value={value.day} min={1} max={31}
            onChange={(day) => onChange({ ...value, day })} />
          <TimeOfDayRow hour={value.hour} min={value.min}
            onChange={(hour, min) => onChange({ ...value, hour, min })} />
        </>
      )}
    </div>
  );
}

function NumberRow({
  prefix, suffix, value, min, max, onChange,
}: {
  prefix: string; suffix: string; value: number; min: number; max: number;
  onChange: (next: number) => void;
}) {
  return (
    <div className="flex items-center gap-2 text-sm text-secondary">
      <span className="shrink-0">{prefix}</span>
      <input
        type="number" value={value} min={min} max={max}
        onChange={(e) => {
          const n = Number(e.target.value);
          if (Number.isFinite(n)) onChange(Math.min(max, Math.max(min, Math.round(n))));
        }}
        className="w-20 px-2 py-1.5 rounded-lg border border-night-border bg-night-hover text-primary font-mono focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors"
      />
      <span className="text-muted">{suffix}</span>
    </div>
  );
}

function TimeOfDayRow({
  hour, min, onChange,
}: {
  hour: number; min: number;
  onChange: (hour: number, min: number) => void;
}) {
  return (
    <div className="flex items-center gap-2 text-sm text-secondary">
      <span className="shrink-0">At</span>
      <input
        type="number" value={hour} min={0} max={23}
        onChange={(e) => {
          const n = Number(e.target.value);
          if (Number.isFinite(n)) onChange(Math.min(23, Math.max(0, Math.round(n))), min);
        }}
        className="w-16 px-2 py-1.5 rounded-lg border border-night-border bg-night-hover text-primary font-mono focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors text-center"
      />
      <span className="text-muted">:</span>
      <input
        type="number" value={min.toString().padStart(2, "0")} min={0} max={59}
        onChange={(e) => {
          const n = Number(e.target.value);
          if (Number.isFinite(n)) onChange(hour, Math.min(59, Math.max(0, Math.round(n))));
        }}
        className="w-16 px-2 py-1.5 rounded-lg border border-night-border bg-night-hover text-primary font-mono focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors text-center"
      />
      <span className="text-[11px] text-muted">(24-hour)</span>
    </div>
  );
}

function DowPicker({
  selected, onChange,
}: {
  selected: number[];
  onChange: (next: number[]) => void;
}) {
  const toggle = (n: number) => {
    const next = selected.includes(n) ? selected.filter((x) => x !== n) : [...selected, n];
    onChange(next.length ? next : [n]);
  };
  const setPreset = (days: number[]) => onChange(days);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1">
        {DOWS.map((d) => {
          const active = selected.includes(d.n);
          return (
            <button
              key={d.n}
              type="button"
              onClick={() => toggle(d.n)}
              className={cn(
                "text-[11px] font-mono px-2.5 py-1 rounded border transition-colors",
                active
                  ? "bg-lime/10 border-lime/40 text-lime"
                  : "border-night-border text-muted hover:text-secondary hover:bg-night-hover",
              )}
            >
              {d.short}
            </button>
          );
        })}
      </div>
      <div className="flex items-center gap-1.5 text-[11px]">
        <span className="text-muted mr-1">Presets:</span>
        <button type="button" onClick={() => setPreset([1, 2, 3, 4, 5])}
          className="px-2 py-0.5 rounded border border-night-border text-muted hover:text-secondary hover:bg-night-hover transition-colors">
          Weekdays
        </button>
        <button type="button" onClick={() => setPreset([0, 6])}
          className="px-2 py-0.5 rounded border border-night-border text-muted hover:text-secondary hover:bg-night-hover transition-colors">
          Weekends
        </button>
        <button type="button" onClick={() => setPreset([0, 1, 2, 3, 4, 5, 6])}
          className="px-2 py-0.5 rounded border border-night-border text-muted hover:text-secondary hover:bg-night-hover transition-colors">
          Every day
        </button>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs font-medium text-muted uppercase tracking-wider mb-1.5">{label}</label>
      {children}
    </div>
  );
}
