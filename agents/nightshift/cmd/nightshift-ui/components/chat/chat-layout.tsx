"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { cn, timeAgo } from "@/lib/utils";
import { useChatState, useChatActions, type Conversation } from "@/lib/hooks/useChat";
import { createSession, type ArtifactInfo } from "@/lib/api";
import { ChatMessageItem } from "@/components/chat/chat-message";
import { ChatInput, uploadStagedFiles } from "@/components/chat/chat-input";
import { ChatWelcome } from "@/components/chat/chat-welcome";
import { RenameDialog } from "@/components/shared/rename-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { ArtifactPanel, type ArtifactView } from "@/components/chat/artifact-panel";
import { useTaskPanel } from "@/lib/hooks/useTaskPanel";
import { LoadingCenter, PanelHeader } from "@/lib/ui";
import * as api from "@/lib/api";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Search,
  Clock,
  MoreHorizontal,
  Sparkles,
  Loader2,
  ArrowLeft,
  Pencil,
  Trash2,
  CalendarClock,
  MessageSquarePlus,
  ChevronsLeft,
  Check,
  X,
} from "lucide-react";

// Rewraps upload failures so the composer's staging strip shows a
// user-facing message instead of the raw fetch error.
async function uploadOrThrow(
  sessionId: string,
  files: File[],
): Promise<ArtifactInfo[]> {
  if (!files.length) return [];
  try {
    return await uploadStagedFiles(sessionId, files);
  } catch (e) {
    console.error("[chat-layout] upload failed:", e);
    throw new Error(
      e instanceof Error ? `Upload failed: ${e.message}` : "Upload failed — try again.",
    );
  }
}

// ── Conversation item ───────────────────────────────────────────

function ConversationItem({
  conv,
  isActive,
  isRunning,
  selectionMode,
  selected,
  onSelect,
  onToggleSelect,
  onRename,
  onDelete,
}: {
  conv: Conversation;
  isActive: boolean;
  isRunning: boolean;
  selectionMode: boolean;
  selected: boolean;
  onSelect: () => void;
  onToggleSelect: () => void;
  onRename: () => void;
  onDelete: () => void;
}) {
  // In selection mode, tapping anywhere on the row toggles selection. Outside
  // selection mode, the row still navigates to the conversation.
  const rowClick = selectionMode ? onToggleSelect : onSelect;

  return (
    <button
      onClick={rowClick}
      className={cn(
        "w-full flex items-center gap-2.5 px-4 py-2.5 text-left transition-colors group",
        selected
          ? "bg-lime/5 hover:bg-lime/10"
          : isActive
            ? "bg-night-hover"
            : "hover:bg-night-hover",
      )}
    >
      {/* Leading slot: cross-faded checkbox + status icon. Both occupy the
          same 16px square; in selection mode (or on hover) the checkbox
          shows and the icon fades. */}
      <span className="relative shrink-0 size-4 flex items-center justify-center">
        <span
          aria-hidden
          className={cn(
            "absolute inset-0 flex items-center justify-center transition-opacity",
            selectionMode || selected
              ? "opacity-0"
              : "opacity-100 group-hover:opacity-0",
          )}
        >
          {isRunning ? (
            <Loader2 size={13} className="text-lime animate-spin" />
          ) : (
            <Sparkles size={13} className="text-muted/40" />
          )}
        </span>
        <span
          role="checkbox"
          aria-checked={selected}
          aria-label={selected ? `Deselect ${conv.title}` : `Select ${conv.title}`}
          tabIndex={0}
          onClick={(e) => { e.stopPropagation(); onToggleSelect(); }}
          onKeyDown={(e) => {
            if (e.key === " " || e.key === "Enter") {
              e.preventDefault();
              e.stopPropagation();
              onToggleSelect();
            }
          }}
          className={cn(
            "absolute inset-0 rounded flex items-center justify-center border transition-all",
            selected
              ? "bg-lime border-lime text-night opacity-100"
              : "border-muted/50 text-transparent",
            selectionMode || selected
              ? "opacity-100"
              : "opacity-0 group-hover:opacity-100",
          )}
        >
          {selected && <Check size={11} strokeWidth={3} />}
        </span>
      </span>
      <p className={cn(
        "text-[13px] truncate flex-1",
        selected || isActive ? "text-primary" : "text-secondary",
      )}>
        {conv.title}
      </p>
      <span
        className={cn(
          "text-[11px] text-muted tabular-nums shrink-0 transition-opacity",
          selectionMode ? "opacity-0" : "group-hover:opacity-0",
        )}
      >
        {timeAgo(conv.updatedAt)}
      </span>
      {!selectionMode && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <span
              role="button"
              tabIndex={0}
              onClick={(e) => e.stopPropagation()}
              className="shrink-0 size-6 flex items-center justify-center rounded hover:bg-night-active transition-colors opacity-0 group-hover:opacity-100"
            >
              <MoreHorizontal size={14} className="text-muted" />
            </span>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-40">
            <DropdownMenuItem onClick={(e) => { e.stopPropagation(); onRename(); }}>
              <Pencil size={14} />
              Rename
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={(e) => { e.stopPropagation(); onDelete(); }}>
              <Trash2 size={14} />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </button>
  );
}

// ── Left panel ───────────────────────────────────────────────────

function TaskListPanel({
  conversations,
  activeId,
  runningConvs,
  onSelect,
  onRename,
  onDelete,
  onBulkDelete,
  panelRef,
  width,
}: {
  conversations: Conversation[];
  activeId: string | null;
  runningConvs: Set<string>;
  onSelect: (id: string) => void;
  onRename: (conv: Conversation) => void;
  onDelete: (conv: Conversation) => void;
  onBulkDelete: (ids: string[]) => void;
  panelRef: React.RefObject<HTMLDivElement | null>;
  width: number;
}) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const selectionMode = selectedIds.size > 0;
  const [query, setQuery] = useState("");
  const { setCollapsed: setPanelCollapsed } = useTaskPanel();

  const filtered = query
    ? conversations.filter((c) => c.title?.toLowerCase().includes(query.toLowerCase()))
    : conversations;

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);
  const clearSelection = useCallback(() => setSelectedIds(new Set()), []);
  const selectAll = useCallback(
    () => setSelectedIds(new Set(conversations.map((c) => c.id))),
    [conversations],
  );

  // Drop selected ids that no longer exist (e.g. after an external delete).
  useEffect(() => {
    if (selectedIds.size === 0) return;
    const live = new Set(conversations.map((c) => c.id));
    let changed = false;
    for (const id of selectedIds) {
      if (!live.has(id)) { changed = true; break; }
    }
    if (changed) {
      setSelectedIds((prev) => {
        const next = new Set<string>();
        for (const id of prev) if (live.has(id)) next.add(id);
        return next;
      });
    }
  }, [conversations, selectedIds]);

  const allSelected = selectionMode && selectedIds.size === conversations.length;

  const handleBulkDelete = () => {
    onBulkDelete(Array.from(selectedIds));
    clearSelection();
  };

  return (
    <div
      ref={panelRef}
      style={{ width }}
      className="@container shrink-0 flex flex-col h-full bg-night"
    >
      {/* Toolbar header — swaps into a selection bar when any row is selected. */}
      <PanelHeader>
        {selectionMode ? (
          <>
            <div className="flex items-center gap-3">
              <button
                onClick={allSelected ? clearSelection : selectAll}
                className="size-7 flex items-center justify-center rounded text-muted hover:text-secondary transition-colors"
                title={allSelected ? "Deselect all" : "Select all"}
              >
                <span
                  className={cn(
                    "size-4 rounded flex items-center justify-center border",
                    allSelected
                      ? "bg-lime border-lime text-night"
                      : "border-muted/60",
                  )}
                >
                  {allSelected && <Check size={11} strokeWidth={3} />}
                </span>
              </button>
              <span className="text-sm font-medium text-primary tabular-nums">
                {selectedIds.size} selected
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <button
                onClick={handleBulkDelete}
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs text-error hover:bg-error/10 transition-colors"
              >
                <Trash2 size={12} />
                Delete
              </button>
              <button
                onClick={clearSelection}
                className="size-7 flex items-center justify-center rounded text-muted hover:text-secondary transition-colors"
                title="Cancel selection"
              >
                <X size={14} />
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => setPanelCollapsed(true)}
                aria-label="Hide tasks panel"
                title="Hide tasks panel"
                className="size-6 flex items-center justify-center rounded text-muted hover:text-primary hover:bg-night-hover transition-colors"
              >
                <ChevronsLeft size={14} />
              </button>
              <span className="text-sm font-medium text-primary">Tasks</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Link
                href="/tasks"
                className="flex items-center justify-center gap-1.5 h-7 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors w-7 @sm:w-auto @sm:pl-2 @sm:pr-2.5"
                title="New task"
              >
                <MessageSquarePlus size={14} />
                <span className="hidden @sm:inline">New task</span>
              </Link>
              <Link
                href="/schedule"
                className="flex items-center justify-center gap-1.5 h-7 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors w-7 @sm:w-auto @sm:pl-2 @sm:pr-2.5"
                title="Scheduled tasks"
              >
                <Clock size={14} />
                <span className="hidden @sm:inline">Schedule</span>
              </Link>
            </div>
          </>
        )}
      </PanelHeader>

      {/* Search */}
      <div className="shrink-0 border-b border-night-border px-3 py-2">
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search tasks"
            className="w-full h-8 pl-9 pr-3 rounded-md border border-night-border bg-transparent text-[13px] text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/10 transition-colors"
          />
        </div>
      </div>

      {/* Task list */}
      <div className="flex-1 min-h-0 overflow-y-auto terminal-scroll">
        {filtered.map((conv) => (
          <ConversationItem
            key={conv.id}
            conv={conv}
            isActive={conv.id === activeId}
            isRunning={runningConvs.has(conv.id)}
            selectionMode={selectionMode}
            selected={selectedIds.has(conv.id)}
            onSelect={() => onSelect(conv.id)}
            onToggleSelect={() => toggleSelect(conv.id)}
            onRename={() => onRename(conv)}
            onDelete={() => onDelete(conv)}
          />
        ))}
        {filtered.length === 0 && (
          <div className="px-4 py-12 text-center text-sm text-muted">
            {query ? "No tasks match your search." : "No tasks yet."}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Right panel ──────────────────────────────────────────────────

// Pill that surfaces "Triggered by schedule" when the open session
// has at least one run with `invoker_type=SCHEDULE`. Best-effort: a
// fetch failure stays silent rather than showing a broken affordance.
function ScheduleAttribution({ sessionId }: { sessionId: string }) {
  const [scheduleId, setScheduleId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setScheduleId(null);
    api.listSessionRuns(sessionId)
      .then((runs) => {
        if (cancelled) return;
        const fired = runs.find((r) => r.invoker_type === "INVOKER_TYPE_SCHEDULE" && r.invoker_id);
        setScheduleId(fired?.invoker_id ?? null);
      })
      .catch(() => { /* silent */ });
    return () => { cancelled = true; };
  }, [sessionId]);

  if (!scheduleId) return null;

  return (
    <Link
      href={`/schedule?id=${encodeURIComponent(scheduleId)}`}
      className="shrink-0 inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[11px] text-muted hover:text-secondary hover:bg-night-hover border border-night-border transition-colors"
      title="Open schedule"
    >
      <CalendarClock size={11} />
      Triggered by schedule
    </Link>
  );
}

function ThreadPanel({
  conversationId,
  onStartFromPrompt,
  onBack,
  onOpenArtifact,
}: {
  conversationId: string | null;
  onStartFromPrompt?: (text: string, files: File[]) => void;
  onBack?: () => void;
  onOpenArtifact?: (view: ArtifactView) => void;
}) {
  const { conversations, messages, activeConversationId, isStreaming, runningConvs } = useChatState();
  const { loadMessages, sendMessage, cancelRun } = useChatActions();
  const scrollRef = useRef<HTMLDivElement>(null);

  const conv = conversationId ? conversations.find((c) => c.id === conversationId) : null;

  // The runId of this thread's currently-streaming assistant turn, if any.
  // The Stop button calls `cancelRun` against this id; null disables it.
  const streamingRunId =
    messages.findLast((m) => m.role === "assistant" && m.status === "streaming")?.runId ?? null;

  useEffect(() => {
    // Switching conversations must load even if the previous one is still
    // streaming — each run now owns its own EventSource, so loads don't
    // interfere with in-flight runs elsewhere.
    if (conversationId && conversationId !== activeConversationId) {
      loadMessages(conversationId);
    }
  }, [conversationId, activeConversationId, loadMessages]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 150;
    if (nearBottom) el.scrollTop = el.scrollHeight;
  }, [messages]);

  const handleSend = async (text: string, files: File[]) => {
    if (!conversationId || isStreaming) return;
    const attachments = await uploadOrThrow(conversationId, files);
    sendMessage(conversationId, text, attachments);
  };

  if (!conversationId) {
    return <ChatWelcome onSend={onStartFromPrompt ?? (() => {})} />;
  }

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Header */}
      <PanelHeader>
        <button
          onClick={onBack}
          className="size-7 flex items-center justify-center rounded text-muted hover:text-primary hover:bg-night-hover transition-colors shrink-0"
          title="Back to new task"
        >
          <ArrowLeft size={15} />
        </button>
        <h2 className="text-sm font-medium text-primary truncate flex-1">
          {conv?.title || "Chat"}
        </h2>
        <ScheduleAttribution sessionId={conversationId} />
      </PanelHeader>

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto terminal-scroll">
        <div className="px-8 md:px-12 lg:px-16 py-6 space-y-6">
          {activeConversationId !== conversationId ? (
            <LoadingCenter />
          ) : messages.length === 0 ? (
            <div className="flex items-center justify-center gap-2 h-40 text-muted text-sm">
              {runningConvs.has(conversationId) ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  <span>Sending your message…</span>
                </>
              ) : (
                <span>Send a message to start the conversation.</span>
              )}
            </div>
          ) : (
            messages.map((msg) => (
              <ChatMessageItem key={msg.id} message={msg} onOpenArtifact={onOpenArtifact} />
            ))
          )}
        </div>
      </div>

      <ChatInput
        onSend={handleSend}
        onStop={streamingRunId ? () => cancelRun(streamingRunId) : undefined}
        streaming={isStreaming}
        seed={`${conversationId}:${messages.filter((m) => m.role === "user").length}`}
        conversationId={conversationId}
      />
    </div>
  );
}

// ── Main layout ──────────────────────────────────────────────────

export function ChatLayout({ initialConversationId }: { initialConversationId?: string }) {
  const { conversations, runningConvs } = useChatState();
  const { sendMessage, deleteConversation, beginPendingConv, clearPendingConv } = useChatActions();
  const router = useRouter();
  const { collapsed: taskPanelCollapsed } = useTaskPanel();
  const [activeId, setActiveId] = useState<string | null>(initialConversationId ?? null);
  const [sending, setSending] = useState(false);

  // Dialog state
  const [renameTarget, setRenameTarget] = useState<Conversation | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Conversation | null>(null);
  const [bulkDeleteIds, setBulkDeleteIds] = useState<string[] | null>(null);
  const [artifact, setArtifact] = useState<ArtifactView | null>(null);
  const [splitPercent, setSplitPercent] = useState(45);
  const [isDragging, setIsDragging] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Task panel width — resizable via drag handle. Persisted across reloads.
  // The committed state seeds initial render and persistence; during a drag
  // we write directly to the panel's DOM node (`taskPanelRef.current.style`)
  // so React doesn't re-render on every mousemove.
  const TASK_PANEL_KEY = "nightshift-task-panel-width";
  const TASK_PANEL_MIN = 280;
  const TASK_PANEL_MAX = 720;
  const TASK_PANEL_DEFAULT = 480;
  const [taskPanelWidth, setTaskPanelWidth] = useState(TASK_PANEL_DEFAULT);
  const taskPanelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const saved = localStorage.getItem(TASK_PANEL_KEY);
    if (!saved) return;
    const n = parseInt(saved, 10);
    if (!Number.isFinite(n)) return;
    setTaskPanelWidth(Math.min(TASK_PANEL_MAX, Math.max(TASK_PANEL_MIN, n)));
  }, []);

  // Sync active ID from initial prop
  useEffect(() => {
    if (initialConversationId) setActiveId(initialConversationId);
  }, [initialConversationId]);

  const handleNewTask = useCallback(
    async (text: string, files: File[]) => {
      if (sending) return;
      setSending(true);
      try {
        // Uploads need a session_id scope, so mint one up front and
        // seed the sidebar row + spinner before awaiting the upload.
        // Otherwise let CreateRun mint the id.
        let sessionId: string | null = null;
        let attachments: ArtifactInfo[] = [];
        if (files.length > 0) {
          sessionId = (await createSession()).session_id;
          beginPendingConv(sessionId, text.trim());
          setActiveId(sessionId);
          router.replace(`/tasks/${sessionId}`, { scroll: false });
          try {
            attachments = await uploadOrThrow(sessionId, files);
          } catch (e) {
            clearPendingConv(sessionId);
            throw e;
          }
        }
        const sid = await sendMessage(sessionId, text.trim(), attachments);
        setActiveId(sid);
        router.replace(`/tasks/${sid}`, { scroll: false });
      } finally {
        setSending(false);
      }
    },
    [sending, sendMessage, beginPendingConv, clearPendingConv, router],
  );

  const handleSelect = useCallback((id: string) => {
    setActiveId(id);
    router.replace(`/tasks/${id}`, { scroll: false });
  }, [router]);

  const handleRenameConfirm = useCallback(async (_newName: string) => {
    // Sessions can't be renamed in cr0n-a — no-op for now
    setRenameTarget(null);
  }, []);

  const handleDeleteConfirm = useCallback(async () => {
    if (!deleteTarget) return;
    await deleteConversation(deleteTarget.id);
    if (activeId === deleteTarget.id) {
      setActiveId(null);
      router.replace("/tasks", { scroll: false });
    }
    setDeleteTarget(null);
  }, [deleteTarget, deleteConversation, activeId, router]);

  const handleBulkDeleteConfirm = useCallback(async () => {
    if (!bulkDeleteIds || bulkDeleteIds.length === 0) {
      setBulkDeleteIds(null);
      return;
    }
    const ids = bulkDeleteIds;
    setBulkDeleteIds(null);
    // Fire all deletes in parallel; allSettled so a single 409 (running run)
    // doesn't cancel the rest. Individual errors are logged but not shown —
    // the failed ones will remain in the list after the optimistic
    // deleteConversation rolls back and useChat re-fetches.
    const results = await Promise.allSettled(
      ids.map((id) => deleteConversation(id)),
    );
    const failed = results.filter((r) => r.status === "rejected").length;
    if (failed > 0) {
      console.warn(`[chat-layout] bulk delete: ${failed}/${ids.length} failed`);
    }
    if (activeId && ids.includes(activeId)) {
      setActiveId(null);
      router.replace("/tasks", { scroll: false });
    }
  }, [bulkDeleteIds, deleteConversation, activeId, router]);


  const handleOpenArtifact = useCallback((view: ArtifactView) => {
    setArtifact(view);
    setSplitPercent(45);
  }, []);

  const handleDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setIsDragging(true);
    document.body.style.cursor = "col-resize";

    const onMove = (ev: MouseEvent) => {
      if (!containerRef.current) return;
      const rect = containerRef.current.getBoundingClientRect();
      const pct = ((ev.clientX - rect.left) / rect.width) * 100;
      setSplitPercent(Math.min(80, Math.max(25, pct)));
    };
    const onUp = () => {
      setIsDragging(false);
      document.body.style.cursor = "";
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, []);

  // Tasks panel resize. Drag bypasses React state — we write `style.width`
  // directly to the panel node on every mousemove. React only re-renders
  // once on mouseup, when we commit the final width to state + localStorage.
  const handleTaskDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const node = taskPanelRef.current;
    if (!node) return;
    const startX = e.clientX;
    const startWidth = node.getBoundingClientRect().width;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";

    const onMove = (ev: MouseEvent) => {
      const next = Math.min(
        TASK_PANEL_MAX,
        Math.max(TASK_PANEL_MIN, startWidth + (ev.clientX - startX)),
      );
      node.style.width = `${next}px`;
    };
    const onUp = () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
      const finalWidth = Math.round(node.getBoundingClientRect().width);
      setTaskPanelWidth(finalWidth);
      try { localStorage.setItem(TASK_PANEL_KEY, String(finalWidth)); } catch {}
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, []);

  return (
    <div className="flex-1 flex min-h-0">
      {!taskPanelCollapsed && (
        <>
          <TaskListPanel
            conversations={conversations}
            activeId={activeId}
            runningConvs={runningConvs}
            onSelect={handleSelect}
            onRename={setRenameTarget}
            onDelete={setDeleteTarget}
            onBulkDelete={setBulkDeleteIds}
            panelRef={taskPanelRef}
            width={taskPanelWidth}
          />
          <div
            onMouseDown={handleTaskDragStart}
            role="separator"
            aria-orientation="vertical"
            aria-label="Resize tasks panel"
            className="group relative shrink-0 w-1.5 cursor-col-resize"
          >
            <div className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-night-border group-hover:bg-night-active group-active:bg-night-hover transition-colors" />
          </div>
        </>
      )}
      <div ref={containerRef} className="flex flex-1 min-w-0">
        <div className="flex flex-col min-w-0" style={artifact ? { width: `${splitPercent}%` } : { flex: 1 }}>
          <ThreadPanel
            conversationId={activeId}
            onStartFromPrompt={handleNewTask}
            onOpenArtifact={handleOpenArtifact}
            onBack={() => {
              setActiveId(null);
              setArtifact(null);
              router.replace("/tasks", { scroll: false });
            }}
          />
        </div>
        {artifact && (
          <>
            {/* Drag handle */}
            <div
              onMouseDown={handleDragStart}
              className="w-1 shrink-0 cursor-col-resize bg-night-border hover:bg-lime/30 active:bg-lime/50 transition-colors"
            />
            <div className="flex-1 min-w-0 relative">
              <ArtifactPanel artifact={artifact} onClose={() => setArtifact(null)} />
              {isDragging && <div className="absolute inset-0 z-10" />}
            </div>
          </>
        )}
      </div>

      {/* Rename dialog */}
      <RenameDialog
        open={!!renameTarget}
        onOpenChange={(open) => { if (!open) setRenameTarget(null); }}
        currentName={renameTarget?.title ?? ""}
        onConfirm={handleRenameConfirm}
      />

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
        title="Delete task"
        description={
          <>
            Permanently delete <span className="text-secondary font-medium">{deleteTarget?.title}</span>? This cannot be undone.
          </>
        }
        onConfirm={handleDeleteConfirm}
        confirmLabel="Delete"
      />

      {/* Bulk delete confirmation */}
      <ConfirmDialog
        open={!!bulkDeleteIds && bulkDeleteIds.length > 0}
        onOpenChange={(open) => { if (!open) setBulkDeleteIds(null); }}
        title={`Delete ${bulkDeleteIds?.length ?? 0} ${bulkDeleteIds?.length === 1 ? "task" : "tasks"}`}
        description={
          <>
            Permanently delete <span className="text-secondary font-medium">{bulkDeleteIds?.length ?? 0}</span>{" "}
            {bulkDeleteIds?.length === 1 ? "task" : "tasks"}? This cannot be undone.
          </>
        }
        onConfirm={handleBulkDeleteConfirm}
        confirmLabel="Delete all"
      />

    </div>
  );
}
