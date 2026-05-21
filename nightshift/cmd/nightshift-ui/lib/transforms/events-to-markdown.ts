import type { StreamEvent } from "@/lib/api";
import { raw, parseEventType, formatDuration, isResultError } from "@/lib/events/parse";

// ── Content blocks → markdown ────────────────────────────────────

function toolUseToMarkdown(b: Record<string, unknown>): string {
  const name = String(b.name ?? "tool");
  const input = b.input as Record<string, unknown> | undefined;

  // MCP artifact tools
  if (name === "mcp__nightshift__deploy_app" && input) {
    return `> **Deploying app:** ${String(input.name ?? "app")}\n`;
  }
  if (name === "mcp__nightshift__deploy_object" && input) {
    return `> **Uploading:** ${String(input.filename ?? "file")}\n`;
  }
  if (name.startsWith("mcp__nightshift__")) {
    const short = name.replace("mcp__nightshift__", "");
    return `> **${short}**\n`;
  }
  if (name.startsWith("mcp__")) {
    const short = name.replace(/^mcp__\w+__/, "");
    return `> MCP: **${short}**\n`;
  }

  if (name === "Agent" && input) {
    const desc = String(input.description ?? input.subagent_type ?? "subagent");
    return `> **Agent:** ${desc}\n`;
  }
  if ((name === "Bash" || name === "bash") && input?.command) {
    return `<details>\n<summary>**Bash**</summary>\n\n\`\`\`bash\n${String(input.command)}\n\`\`\`\n\n</details>\n`;
  }
  if ((name === "Edit" || name === "Write" || name === "Read") && input?.file_path) {
    return `> **${name}** \`${String(input.file_path)}\`\n`;
  }
  if ((name === "Grep" || name === "Glob") && input?.pattern) {
    return `> **${name}** \`${String(input.pattern)}\`\n`;
  }
  if (name === "ToolSearch" && input?.query) {
    return `> **ToolSearch** \`${String(input.query)}\`\n`;
  }
  return `> **${name}**\n`;
}

function toolResultToMarkdown(b: Record<string, unknown>): string {
  const content = b.content;
  let text: string | null = null;
  if (typeof content === "string") {
    text = content;
  } else if (Array.isArray(content)) {
    const tb = content.find((c: Record<string, unknown>) => c?.type === "text");
    if (tb && typeof tb.text === "string") text = tb.text;
    // Skip tool_reference blocks
    if (content.some((c: Record<string, unknown>) => c?.type === "tool_reference")) return "";
  }
  if (text) {
    // Detect artifact deployment result
    if (text.includes("App deployed successfully") && text.includes("URL:")) {
      const url = text.match(/URL:\s*(\S+)/)?.[1] ?? "";
      const name = text.match(/Name:\s*(.+)/)?.[1]?.trim() ?? "App";
      return url ? `\n> **${name}** — [Open App](${url})\n` : `\`\`\`\n${text}\n\`\`\`\n`;
    }
    return `\`\`\`\n${text}\n\`\`\`\n`;
  }
  return "";
}

// ── Activity grouping ────────────────────────────────────────────

type Segment =
  | { kind: "text"; md: string }
  | { kind: "activity"; lines: string[]; tools: string[] };

function buildSegments(events: StreamEvent[]): Segment[] {
  const segments: Segment[] = [];
  let currentActivity: { lines: string[]; tools: string[] } | null = null;

  function flushActivity() {
    if (currentActivity && currentActivity.lines.length > 0) {
      segments.push({ kind: "activity", ...currentActivity });
    }
    currentActivity = null;
  }

  function ensureActivity() {
    if (!currentActivity) currentActivity = { lines: [], tools: [] };
  }

  function pushText(md: string) {
    if (!md.trim()) return;
    flushActivity();
    segments.push({ kind: "text", md });
  }

  function pushActivity(line: string, toolName?: string) {
    ensureActivity();
    currentActivity!.lines.push(line);
    if (toolName) currentActivity!.tools.push(toolName);
  }

  for (const event of events) {
    const r = raw(event);
    const { base, sub } = parseEventType(event.type);

    switch (base) {
      case "system": {
        if (sub === "init" || sub === "") {
          const data = r.data as Record<string, unknown> | undefined;
          const model = typeof data?.model === "string" ? ` (${data.model})` : "";
          pushActivity(`> Session started${model}\n`);
        } else if (sub === "worker_started") {
          pushActivity("> Worker started\n");
        } else if (sub === "task_started") {
          const data = (r.data ?? r) as Record<string, unknown>;
          const desc = String(data.description ?? data.task_id ?? "");
          pushActivity(`> **Subagent started:** ${desc}\n`, "Agent");
        } else if (sub === "task_progress") {
          const data = (r.data ?? r) as Record<string, unknown>;
          const desc = String(data.description ?? "working...");
          const lastTool = typeof data.last_tool_name === "string" ? ` (${data.last_tool_name})` : "";
          const usage = data.usage as Record<string, unknown> | undefined;
          const dur = typeof usage?.duration_ms === "number" ? ` — ${formatDuration(usage.duration_ms)}` : "";
          pushActivity(`> ${desc}${lastTool}${dur}\n`);
        } else if (sub === "task_notification") {
          const data = (r.data ?? r) as Record<string, unknown>;
          const status = String(data.status ?? "done");
          const summary = typeof data.summary === "string" ? ` ${data.summary}` : "";
          const usage = data.usage as Record<string, unknown> | undefined;
          const dur = typeof usage?.duration_ms === "number" ? `, ${formatDuration(usage.duration_ms)}` : "";
          const tools = typeof usage?.tool_uses === "number" ? `, ${usage.tool_uses} tools` : "";
          const icon = status === "completed" ? "done" : "failed";
          pushActivity(`> **Subagent ${icon}**${summary}${dur}${tools}\n`);
        } else {
          pushActivity(`> [system.${sub}]\n`);
        }
        break;
      }

      case "assistant": {
        const content = r.content;
        if (!Array.isArray(content)) break;

        for (const block of content) {
          if (typeof block === "string") { pushText(block); continue; }
          if (!block || typeof block !== "object") continue;
          const b = block as Record<string, unknown>;

          if (b.type === "text" && typeof b.text === "string") {
            pushText(b.text as string);
          } else if (b.type === "thinking" && typeof b.thinking === "string") {
            pushActivity(`<details>\n<summary>Thinking...</summary>\n\n${b.thinking as string}\n\n</details>\n`);
          } else if (b.type === "tool_use") {
            const name = String(b.name ?? "tool");
            pushActivity(toolUseToMarkdown(b), name);
          }
        }
        break;
      }

      case "user": {
        const content = r.content;
        if (!Array.isArray(content)) break;
        for (const block of content) {
          if (!block || typeof block !== "object") continue;
          const b = block as Record<string, unknown>;
          if (b.type === "tool_result") {
            pushActivity(toolResultToMarkdown(b));
          }
        }
        break;
      }

      case "result": {
        flushActivity();
        const isError = isResultError(event);
        const cost = typeof r.total_cost_usd === "number" ? `$${r.total_cost_usd.toFixed(4)}` : null;
        const durMs = typeof r.duration_ms === "number" ? formatDuration(r.duration_ms) : null;
        const stats = [cost, durMs].filter(Boolean).join(", ");
        const md = `\n---\n**${isError ? "Error" : "Completed"}**${stats ? ` — ${stats}` : ""}\n`;
        segments.push({ kind: "text", md });
        break;
      }

      case "rate_limit_event":
        break;

      default:
        pushActivity(`> [${event.type}]\n`);
        break;
    }
  }

  flushActivity();
  return segments;
}

// ── Main conversion ──────────────────────────────────────────────

export function eventsToMarkdown(events: StreamEvent[]): string {
  const seen = new Set<number>();
  const deduped = events.filter(e => {
    if (typeof e.index !== "number") return true;
    if (seen.has(e.index)) return false;
    seen.add(e.index);
    return true;
  });
  const segments = buildSegments(deduped);
  const parts: string[] = [];

  for (const seg of segments) {
    if (seg.kind === "text") {
      parts.push(seg.md);
    } else {
      const unique = [...new Set(seg.tools)];
      const label = unique.length > 0
        ? `Activity — ${unique.join(", ")}`
        : "Activity";
      parts.push(`\n<details>\n<summary>${label}</summary>\n\n${seg.lines.join("\n")}\n</details>\n`);
    }
  }

  return parts.join("\n");
}
