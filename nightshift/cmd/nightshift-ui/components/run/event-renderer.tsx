"use client";

import { Bot, ExternalLink, FileUp, Globe, Loader2, Package } from "lucide-react";
import { cn } from "@/lib/utils";
import type { StreamEvent } from "@/lib/api";
import { raw, truncate, formatDuration, parseEventType, isResultError } from "@/lib/events/parse";

// ── Artifact result detection ───────────────────────────────────

type ArtifactResult = {
  id: string;
  name: string;
  url: string;
  isPublic: boolean;
};

function parseArtifactResult(text: string): ArtifactResult | null {
  if (!text.includes("Artifact ID:")) return null;
  const id = text.match(/Artifact ID:\s*(\S+)/)?.[1] ?? "";
  const name = text.match(/Name:\s*(.+)/)?.[1]?.trim() ?? "";
  const url = text.match(/URL:\s*(\S+)/)?.[1] ?? "";
  const isPublic = /Public:\s*True/i.test(text);
  return id ? { id, name, url, isPublic } : null;
}

// ── Content block renderer ───────────────────────────────────────

export function ContentBlocks({ content }: { content: unknown }) {
  if (!Array.isArray(content)) return null;
  return (
    <>
      {content.map((block, i) => {
        if (typeof block === "string") {
          return <span key={i} className="text-secondary whitespace-pre-wrap">{block}</span>;
        }
        if (!block || typeof block !== "object") return null;
        const b = block as Record<string, unknown>;

        // Thinking block
        if (b.type === "thinking" && typeof b.thinking === "string") {
          return (
            <details key={i} className="py-1">
              <summary className="text-muted text-xs cursor-pointer hover:text-secondary select-none">thinking...</summary>
              <pre className="text-muted text-xs mt-0.5 whitespace-pre-wrap max-h-40 overflow-y-auto terminal-scroll">{truncate(b.thinking, 3000)}</pre>
            </details>
          );
        }

        // Text block
        if (b.type === "text" && typeof b.text === "string") {
          return <span key={i} className="text-secondary whitespace-pre-wrap">{b.text}</span>;
        }

        // Tool use block
        if (b.type === "tool_use") {
          const name = typeof b.name === "string" ? b.name : "tool";
          const input = b.input as Record<string, unknown> | undefined;

          // MCP artifact tools
          if (name === "mcp__nightshift__deploy_app" && input) {
            return (
              <div key={i} className="py-1.5 pl-3 border-l-2 border-lime/30">
                <div className="flex items-center gap-1.5 text-lime text-xs font-medium">
                  <Globe size={12} />
                  Deploying app: {String(input.name ?? "app")}
                </div>
                {typeof input.description === "string" && (
                  <div className="text-muted text-xs mt-0.5">{input.description}</div>
                )}
              </div>
            );
          }

          if (name === "mcp__nightshift__deploy_object" && input) {
            return (
              <div key={i} className="py-1.5 pl-3 border-l-2 border-lime/30">
                <div className="flex items-center gap-1.5 text-lime text-xs font-medium">
                  <FileUp size={12} />
                  Uploading: {String(input.filename ?? "file")}
                </div>
              </div>
            );
          }

          if (name === "mcp__nightshift__list_artifacts") {
            return (
              <div key={i} className="py-0.5">
                <span className="text-muted text-xs">&#9654; Listing artifacts</span>
              </div>
            );
          }

          if (name === "mcp__nightshift__update_artifact" || name === "mcp__nightshift__share_artifact") {
            const label = name === "mcp__nightshift__update_artifact" ? "Updating artifact" : "Sharing artifact";
            return (
              <div key={i} className="py-0.5">
                <span className="text-muted text-xs">&#9654; {label}</span>
              </div>
            );
          }

          // Other MCP tools
          if (name.startsWith("mcp__")) {
            const short = name.replace(/^mcp__\w+__/, "");
            return (
              <div key={i} className="py-0.5">
                <span className="text-muted text-xs">&#9654; MCP: {short}</span>
              </div>
            );
          }

          // Agent dispatch
          if (name === "Agent" && input) {
            return (
              <div key={i} className="py-1.5 pl-3 border-l-2 border-lime/30">
                <div className="flex items-center gap-1.5 text-lime text-xs font-medium">
                  <Bot size={12} />
                  Agent: {String(input.subagent_type ?? input.description ?? "subagent")}
                </div>
                {input.description && input.subagent_type ? (
                  <div className="text-muted text-xs mt-0.5">{String(input.description)}</div>
                ) : null}
              </div>
            );
          }

          // Bash command
          if ((name === "Bash" || name === "bash") && input?.command) {
            return (
              <div key={i} className="py-1">
                <div className="text-muted text-xs">&#9654; {name}</div>
                <pre className="text-lime/80 text-xs mt-0.5 whitespace-pre-wrap">{truncate(String(input.command), 2000)}</pre>
              </div>
            );
          }

          // Edit/Write/Read with file path
          if ((name === "Edit" || name === "Write" || name === "Read") && input?.file_path) {
            return (
              <div key={i} className="py-1">
                <div className="text-muted text-xs">&#9654; {name} <span className="text-secondary font-mono">{String(input.file_path)}</span></div>
              </div>
            );
          }

          // Grep/Glob
          if ((name === "Grep" || name === "Glob") && input?.pattern) {
            return (
              <div key={i} className="py-1">
                <div className="text-muted text-xs">&#9654; {name} <span className="text-secondary font-mono">{String(input.pattern)}</span></div>
              </div>
            );
          }

          // ToolSearch
          if (name === "ToolSearch" && input?.query) {
            return (
              <div key={i} className="py-0.5">
                <span className="text-muted text-xs">&#9654; ToolSearch: {String(input.query)}</span>
              </div>
            );
          }

          // Generic tool
          return (
            <div key={i} className="py-0.5">
              <span className="text-muted text-xs">&#9654; {name}</span>
            </div>
          );
        }

        // Tool result block
        if (b.type === "tool_result") {
          const resultContent = b.content;
          let text: string | null = null;

          if (typeof resultContent === "string") {
            text = resultContent;
          } else if (Array.isArray(resultContent)) {
            const textBlock = resultContent.find((c: Record<string, unknown>) => c?.type === "text");
            if (textBlock && typeof textBlock.text === "string") text = textBlock.text;
            // tool_reference blocks (from ToolSearch) — skip rendering
            if (resultContent.some((c: Record<string, unknown>) => c?.type === "tool_reference")) return null;
          }

          if (text) {
            // Check for artifact deployment result
            const artifact = parseArtifactResult(text);
            if (artifact) {
              return (
                <div key={i} className="my-2 p-3 rounded-lg border border-night-border bg-night-surface">
                  <div className="flex items-center gap-2">
                    <Package size={14} className="text-lime" />
                    <span className="text-sm font-medium text-primary">{artifact.name}</span>
                    {artifact.isPublic && <span className="text-xs text-muted bg-night-elevated px-1.5 py-0.5 rounded">Public</span>}
                  </div>
                  {artifact.url && (
                    <a href={artifact.url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1 text-xs text-lime hover:underline mt-1.5">
                      <ExternalLink size={10} />
                      Open App
                    </a>
                  )}
                </div>
              );
            }

            return (
              <div key={i} className="py-1">
                <pre className="text-secondary/70 text-xs whitespace-pre-wrap max-h-40 overflow-y-auto terminal-scroll">{truncate(text, 2000)}</pre>
              </div>
            );
          }
          return null;
        }

        return null;
      })}
    </>
  );
}

// ── Event line renderer ──────────────────────────────────────────

export function EventLine({ event }: { event: StreamEvent }) {
  const r = raw(event);
  const { base, sub } = parseEventType(event.type);

  switch (base) {
    case "system": {
      if (sub === "init" || sub === "") {
        const data = r.data as Record<string, unknown> | undefined;
        const model = typeof data?.model === "string" ? data.model : null;
        return (
          <div className="text-muted text-xs py-1">
            session started{model ? <span className="ml-1 opacity-60">({model})</span> : null}
          </div>
        );
      }

      if (sub === "worker_started") {
        return <div className="text-muted text-xs py-0.5">worker started</div>;
      }

      if (sub === "task_started") {
        return (
          <div className="py-1.5 flex items-center gap-1.5 text-lime text-xs">
            <Bot size={12} />
            <span className="font-medium">subagent started:</span>
            <span className="text-secondary">{String(r.description ?? (r.data as Record<string, unknown>)?.task_id ?? "")}</span>
          </div>
        );
      }

      if (sub === "task_progress") {
        const data = (r.data ?? r) as Record<string, unknown>;
        const lastTool = typeof data.last_tool_name === "string" ? data.last_tool_name : null;
        const usage = data.usage as Record<string, unknown> | undefined;
        const dur = typeof usage?.duration_ms === "number" ? formatDuration(usage.duration_ms) : null;
        return (
          <div className="py-0.5 text-muted text-xs flex items-center gap-1.5">
            <Loader2 size={10} className="animate-spin" />
            <span>{String(data.description ?? "working...")}</span>
            {lastTool && <span className="opacity-60">({lastTool})</span>}
            {dur && <span className="opacity-40">{dur}</span>}
          </div>
        );
      }

      if (sub === "task_notification") {
        const data = (r.data ?? r) as Record<string, unknown>;
        const status = String(data.status ?? "done");
        const usage = data.usage as Record<string, unknown> | undefined;
        const dur = typeof usage?.duration_ms === "number" ? formatDuration(usage.duration_ms) : null;
        const tools = typeof usage?.tool_uses === "number" ? `${usage.tool_uses} tools` : null;
        const isOk = status === "completed";
        return (
          <div className={cn("py-1.5 text-xs flex items-center gap-1.5", isOk ? "text-success" : "text-error")}>
            <Bot size={12} />
            <span className="font-medium">subagent {status}</span>
            <span className="text-muted">{String(data.summary ?? "")}</span>
            {dur && <span className="text-muted opacity-60">{dur}</span>}
            {tools && <span className="text-muted opacity-60">{tools}</span>}
          </div>
        );
      }

      return <div className="text-muted text-xs py-0.5">[system.{sub}]</div>;
    }

    case "assistant": {
      const content = r.content;
      if (Array.isArray(content)) return <ContentBlocks content={content} />;
      return null;
    }

    case "user": {
      const content = r.content;
      if (Array.isArray(content)) return <ContentBlocks content={content} />;
      return null;
    }

    case "result": {
      const isError = isResultError(event);
      const cost = typeof r.total_cost_usd === "number" ? `$${r.total_cost_usd.toFixed(4)}` : null;
      const durMs = typeof r.duration_ms === "number" ? r.duration_ms : null;
      const result = typeof r.result === "string" ? r.result : null;

      return (
        <div className={cn("py-2 mt-2 border-t border-night-border", isError ? "text-error" : "text-success")}>
          <div className="font-semibold text-sm">
            {isError ? "Error" : "Completed"}
            {cost && <span className="text-muted ml-2 font-normal text-xs">{cost}</span>}
            {durMs !== null && <span className="text-muted ml-2 font-normal text-xs">{formatDuration(durMs)}</span>}
          </div>
          {isError && result && (
            <pre className="text-error/80 text-xs mt-1 whitespace-pre-wrap max-h-60 overflow-y-auto terminal-scroll">
              {truncate(result, 3000)}
            </pre>
          )}
        </div>
      );
    }

    case "rate_limit_event":
      return null;

    default:
      return <div className="text-muted/50 text-xs py-0.5">[{event.type}]</div>;
  }
}

// ── Text extraction utility ──────────────────────────────────────

/** Extract plain text from assistant events for storing as message content. */
export function extractAssistantText(events: StreamEvent[]): string {
  const parts: string[] = [];
  for (const event of events) {
    const { base } = parseEventType(event.type);
    if (base !== "assistant") continue;
    const content = raw(event).content;
    if (!Array.isArray(content)) continue;
    for (const block of content) {
      if (block && typeof block === "object" && (block as Record<string, unknown>).type === "text") {
        const text = (block as Record<string, unknown>).text;
        if (typeof text === "string") parts.push(text);
      }
    }
  }
  return parts.join("");
}
