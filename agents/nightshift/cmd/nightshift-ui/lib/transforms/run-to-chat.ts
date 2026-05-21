import type { StreamEvent } from "@/lib/api";
import type { ChatMessage } from "@/lib/hooks/useChat";
import { parseEventType, isResultError } from "@/lib/events/parse";
import { eventsToMarkdown } from "@/lib/transforms/events-to-markdown";

/**
 * Transform a run's events into chat-style messages.
 * The prompt becomes a user message, the agent's output becomes an assistant message.
 */
export function eventsToMessages(
  events: StreamEvent[],
  prompt: string,
  runId: string,
): ChatMessage[] {
  const messages: ChatMessage[] = [];

  messages.push({
    id: `run-${runId}-prompt`,
    role: "user",
    content: prompt,
    status: "sent",
    events: [],
  });

  if (events.length === 0) return messages;

  const markdown = eventsToMarkdown(events);
  const isComplete = events.some((e) => parseEventType(e.type).base === "result");
  const isError = events.some((e) => isResultError(e));

  messages.push({
    id: `run-${runId}-response`,
    role: "assistant",
    content: markdown,
    runId,
    status: isComplete ? (isError ? "error" : "completed") : "streaming",
    events,
  });

  return messages;
}
