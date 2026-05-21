"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { ChevronLeft, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { statusBadgeClass } from "@/lib/ui";
import { useTasksState, useTasksActions, isActiveRun } from "@/lib/hooks/useTasks";
import { useChatState, useChatActions } from "@/lib/hooks/useChat";
import { ChatMessageItem } from "@/components/chat/chat-message";
import { ChatInput } from "@/components/chat/chat-input";
import { eventsToMessages } from "@/lib/transforms/run-to-chat";

// ── Run detail — renders as a chat thread ────────────────────────

export function RunDetail({ taskId, runId }: { taskId: string; runId: string }) {
  const { tasks } = useTasksState();
  const { loadStreamEvents, subscribeToRun } = useTasksActions();
  const { messages: chatMessages, isStreaming } = useChatState();
  const { sendMessage, loadMessages } = useChatActions();
  const router = useRouter();

  const task = tasks.find((t) => t.id === taskId);
  const run = task?.runs.find((r) => r.id === runId);
  const scrollRef = useRef<HTMLDivElement>(null);

  // State for run-to-chat continuation
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [continuing, setContinuing] = useState(false);

  const isRunActive = run?.status === "running" || run?.status === "pending";

  // Subscribe to run events (existing logic)
  useEffect(() => {
    if (!run) return;
    let cleanup: (() => void) | undefined;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let stopped = false;

    function tryConnect() {
      if (stopped) return;
      cleanup = subscribeToRun(taskId, runId, () => {
        if (!stopped && isRunActive) {
          retryTimer = setTimeout(tryConnect, 3000);
        } else if (!stopped) {
          loadStreamEvents(taskId, runId);
        }
      });
    }
    tryConnect();

    return () => {
      stopped = true;
      cleanup?.();
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, [taskId, runId, run, isRunActive, subscribeToRun, loadStreamEvents]);

  // Auto-scroll
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 150;
    if (nearBottom) el.scrollTop = el.scrollHeight;
  }, [run?.events.length, chatMessages.length]);

  // Transform run events into chat messages
  const runMessages = run
    ? eventsToMessages(run.events, task?.prompt ?? "", runId)
    : [];

  // Handle reply — creates conversation on first reply, then sends message
  const handleReply = useCallback(async (text: string) => {
    if (continuing || isStreaming) return;

    // First reply: create conversation from run
    if (!conversationId) {
      setContinuing(true);
      try {
        const res = await fetch(`/api/runs/${runId}/continue`, { method: "POST" });
        if (!res.ok) throw new Error("Failed to continue run");
        const { conversationId: convId } = await res.json();
        setConversationId(convId);

        // Load the synthetic messages, then send the new reply
        await loadMessages(convId);
        await sendMessage(convId, text);
      } catch (e) {
        console.error("[run-detail] continue failed:", e);
      } finally {
        setContinuing(false);
      }
      return;
    }

    // Subsequent replies: use normal chat
    await sendMessage(conversationId, text);
  }, [conversationId, continuing, isStreaming, runId, loadMessages, sendMessage]);

  if (!run || !task) return null;

  // Combine: run messages first, then any live chat messages (after continuation)
  const allMessages = conversationId
    ? [...runMessages, ...chatMessages.filter((m) => m.id !== runMessages[0]?.id && m.id !== runMessages[1]?.id)]
    : runMessages;

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Header */}
      <header className="h-12 flex items-center gap-2 px-4 border-b border-night-border shrink-0">
        <button
          onClick={() => router.push(`/schedule/${taskId}`)}
          className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-primary hover:bg-night-hover transition-colors"
          aria-label="Back to task"
        >
          <ChevronLeft size={16} />
        </button>
        <h2 className="text-sm font-medium text-secondary truncate">
          &ldquo;{task.name}&rdquo; run
        </h2>
        <span className={cn(
          "text-[10px] font-medium capitalize px-2 py-0.5 rounded-full ml-1",
          statusBadgeClass(run.status),
        )}>
          {run.status}
        </span>
      </header>

      {/* Messages — chat-style */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto terminal-scroll">
        <div className="max-w-3xl mx-auto px-4 py-6 space-y-6">
          {allMessages.map((msg) => (
            <ChatMessageItem key={msg.id} message={msg} />
          ))}
          {continuing && (
            <div className="flex items-center gap-2 text-muted text-xs justify-center py-4">
              <Loader2 size={12} className="animate-spin" />
              Continuing session...
            </div>
          )}
        </div>
      </div>

      {/* Reply input */}
      <ChatInput
        onSend={handleReply}
        disabled={isRunActive || continuing || isStreaming}
      />
    </div>
  );
}
