"use client";

import { Copy, Check, Loader2 } from "lucide-react";
import { useState } from "react";
import type { ChatMessage as ChatMessageType } from "@/lib/hooks/useChat";
import { EventStream } from "@/components/chat/event-stream";
import type { ArtifactView } from "@/components/chat/artifact-panel";
import { AttachmentChip } from "@/components/chat/file-staging";
import { extractAssistantText } from "@/components/run/event-renderer";

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={() => {
        navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }}
      className="flex items-center gap-1 px-2 py-1 rounded-md text-xs text-muted hover:text-secondary hover:bg-night-hover transition-colors"
      aria-label="Copy"
    >
      {copied ? <Check size={12} /> : <Copy size={12} />}
      {copied ? "Copied" : "Copy"}
    </button>
  );
}

export function UserMessage({ message }: { message: ChatMessageType }) {
  const attachments = message.attachments ?? [];
  return (
    <div className="flex flex-col items-end gap-1.5">
      <div className="max-w-[85%] rounded-2xl bg-night-elevated px-4 py-3">
        <p className="text-sm text-primary whitespace-pre-wrap leading-relaxed">{message.content}</p>
      </div>
      {attachments.length > 0 && (
        <div className="flex flex-wrap justify-end gap-1.5 max-w-[85%]">
          {attachments.map((a) => (
            <AttachmentChip key={a.id} name={a.name} />
          ))}
        </div>
      )}
    </div>
  );
}

export function AssistantMessage({
  message,
  onOpenArtifact,
}: {
  message: ChatMessageType;
  onOpenArtifact?: (view: ArtifactView) => void;
}) {
  const isStreaming = message.status === "streaming";
  const isInterrupted = message.status === "interrupted";
  const hasEvents = message.events.length > 0;

  if (!hasEvents && isStreaming) {
    return (
      <div className="flex items-center gap-2 text-muted text-sm py-2">
        <Loader2 size={14} className="animate-spin" />
        <span>Thinking...</span>
      </div>
    );
  }

  const plainText = extractAssistantText(message.events);

  return (
    <div className="group">
      <EventStream events={message.events} isStreaming={isStreaming} onOpenArtifact={onOpenArtifact} />
      {isInterrupted && (
        <div className="text-xs text-muted mt-2 italic">Stopped by you</div>
      )}
      {!isStreaming && plainText && (
        <div className="flex items-center gap-1 mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
          <CopyButton text={plainText} />
        </div>
      )}
    </div>
  );
}

export function ChatMessageItem({
  message,
  onOpenArtifact,
}: {
  message: ChatMessageType;
  onOpenArtifact?: (view: ArtifactView) => void;
}) {
  if (message.role === "user") return <UserMessage message={message} />;
  return <AssistantMessage message={message} onOpenArtifact={onOpenArtifact} />;
}
