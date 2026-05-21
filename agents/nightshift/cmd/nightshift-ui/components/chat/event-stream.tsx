"use client";

import { memo, useMemo, useState } from "react";
import { cn } from "@/lib/utils";
import type { StreamEvent } from "@/lib/api";
import { raw, parseEventType, formatDuration } from "@/lib/events/parse";
import { MessageResponse } from "@/components/ai-elements/message";
import { ObjectViewer } from "@/components/chat/object-viewer";
import type { ArtifactView } from "@/components/chat/artifact-panel";
import { resolveAppUrl } from "@/lib/artifacts";
import {
  ChainOfThought,
  ChainOfThoughtHeader,
  ChainOfThoughtContent,
  ChainOfThoughtStep,
} from "@/components/ai-elements/chain-of-thought";
import {
  Bot,
  Terminal,
  FileText,
  Search,
  Globe,
  Package,
  FileUp,
  ExternalLink,
  Loader2,
  ListTree,
  Monitor,
  Pencil,
  Eye,
  FolderSearch,
  Wrench,
  Share2,
  Shuffle,
  type LucideIcon,
} from "lucide-react";

// ── Types ───────────────────────────────────────────────────────

type Block = Record<string, unknown>;

type SearchResult = { title: string; domain: string; url?: string };
type ArtifactInfo = { id?: string; name: string; url: string; isPublic: boolean };
type InlineArtifact =
  | { kind: "app"; id: string; name: string; url: string }
  | { kind: "object"; id: string; name: string; contentType: string };

type Step = {
  icon: LucideIcon;
  label: string;
  description?: string;
  status: "complete" | "active" | "pending";
  searchResults?: SearchResult[];
  artifact?: ArtifactInfo;
};

type Segment =
  | { kind: "text"; text: string }
  | { kind: "activity"; label: string; icon: LucideIcon; steps: Step[] }
  | { kind: "inline_preview"; artifact: InlineArtifact; caption?: string }
  | { kind: "result"; cost: string | null; duration: string | null; isError: boolean };

// ── Helpers ─────────────────────────────────────────────────────

function iconFor(name: string): LucideIcon {
  if (name === "Agent" || name === "Task") return Bot;
  if (name === "Bash" || name === "bash") return Terminal;
  if (name === "Read") return Eye;
  if (name === "Edit") return Pencil;
  if (name === "Write") return FileText;
  if (name === "Grep") return Search;
  if (name === "Glob") return FolderSearch;
  if (name === "WebSearch") return Globe;
  if (name === "WebFetch") return Globe;
  if (name === "ToolSearch") return Search;
  if (name === "mcp__nightshift__deploy_app") return Globe;
  if (name === "mcp__nightshift__deploy_object") return FileUp;
  if (name === "mcp__nightshift__list_artifacts") return ListTree;
  if (name === "mcp__nightshift__update_artifact") return Pencil;
  if (name === "mcp__nightshift__share_artifact") return Share2;
  if (name === "mcp__nightshift__show_preview_artifact") return Monitor;
  if (name.startsWith("mcp__")) return Wrench;
  return Wrench;
}

function basename(val: unknown): string {
  if (typeof val !== "string") return "file";
  return val.split("/").pop() || val;
}

function domainFrom(url: string): string {
  try { return new URL(url).hostname.replace(/^www\./, ""); } catch { return ""; }
}

function labelFor(name: string, input: Block | undefined): string {
  if (name === "Agent" || name === "Task") return String(input?.description ?? input?.subagent_type ?? "Running subagent");
  if (name === "Bash" || name === "bash") {
    if (typeof input?.description === "string" && input.description) return input.description;
    return "Running command";
  }
  if (name === "Read") return `Reading ${basename(input?.file_path)}`;
  if (name === "Edit") return `Editing ${basename(input?.file_path)}`;
  if (name === "Write") return `Writing to ${basename(input?.file_path)}`;
  if (name === "Grep") return `Searching for "${String(input?.pattern ?? "")}"`;
  if (name === "Glob") return `Finding files matching ${String(input?.pattern ?? "")}`;
  if (name === "WebSearch") {
    const q = String(input?.query ?? "");
    return q ? q : "Searching the web";
  }
  if (name === "WebFetch") {
    const url = String(input?.url ?? "");
    const d = domainFrom(url);
    return d ? `Fetching ${d}` : "Fetching web page";
  }
  if (name === "ToolSearch") return "Looking up tools";
  if (name === "mcp__nightshift__deploy_app") return `Deploying app: ${String(input?.name ?? "app")}`;
  if (name === "mcp__nightshift__deploy_object") return `Uploading ${String(input?.filename ?? "file")}`;
  if (name === "mcp__nightshift__list_artifacts") return "Listing artifacts";
  if (name === "mcp__nightshift__update_artifact") return "Updating artifact";
  if (name === "mcp__nightshift__share_artifact") return "Sharing artifact";
  if (name === "mcp__nightshift__show_preview_artifact") return `Preview: ${String(input?.name ?? "artifact")}`;
  if (name.startsWith("mcp__")) return name.replace(/^mcp__\w+__/, "");
  return name;
}

function descriptionFor(name: string, input: Block | undefined): string | undefined {
  if (name === "Bash" || name === "bash") {
    const cmd = String(input?.command ?? "");
    return cmd || undefined;
  }
  if (name === "WebFetch") {
    return typeof input?.url === "string" ? input.url : undefined;
  }
  if (name === "Read" || name === "Edit" || name === "Write") {
    return typeof input?.file_path === "string" ? input.file_path : undefined;
  }
  return undefined;
}

/** Derive a short activity header from the tools used in a group */
function activityLabel(steps: Step[]): { label: string; icon: LucideIcon } {
  const names = steps.map((s) => s.label);
  const hasSearch = names.some((n) => n.startsWith("Searching:"));
  const hasFetch = names.some((n) => n.startsWith("Fetching"));
  const hasAgent = steps.some((s) => s.icon === Bot);
  const hasDeploy = names.some((n) => n.startsWith("Deploying"));
  const hasWrite = names.some((n) => n.startsWith("Writing"));

  if (hasAgent) return { label: "Running tasks in parallel", icon: Shuffle };
  if (hasSearch && steps.length > 1) return { label: "Searching", icon: Globe };
  if (hasSearch) return { label: names[0] ?? "Searching", icon: Globe };
  if (hasFetch && steps.length > 1) return { label: "Researching", icon: Globe };
  if (hasFetch) return { label: names[0] ?? "Fetching", icon: Globe };
  if (hasDeploy) return { label: names[0] ?? "Deploying", icon: Globe };
  if (hasWrite) return { label: names[0] ?? "Writing", icon: FileText };
  if (steps.length === 1) return { label: names[0] ?? "Working", icon: steps[0]!.icon };
  return { label: "Working", icon: Wrench };
}

/** Extract search results from a tool_use_result */
function extractSearchResults(r: Block): SearchResult[] {
  const tur = r.tool_use_result;
  if (!tur || typeof tur !== "object") return [];

  // WebSearch results: {results: [{content: [{title, url}]}]}
  const results = (tur as Block).results;
  if (Array.isArray(results)) {
    const out: SearchResult[] = [];
    for (const item of results) {
      if (!item || typeof item !== "object") continue;
      const content = (item as Block).content;
      if (Array.isArray(content)) {
        for (const c of content) {
          if (c && typeof c === "object" && typeof (c as Block).title === "string") {
            const title = String((c as Block).title);
            const url = String((c as Block).url ?? "");
            out.push({ title, domain: domainFrom(url), url });
          }
        }
      }
    }
    return out;
  }
  return [];
}

function parseArtifactFromText(text: string): ArtifactInfo | null {
  if (!text.includes("Artifact ID:")) return null;
  const id = text.match(/Artifact ID:\s*(\S+)/)?.[1] ?? "";
  const name = text.match(/Name:\s*(.+)/)?.[1]?.trim() ?? "";
  const url = text.match(/URL:\s*(\S+)/)?.[1] ?? "";
  const isPublic = /Public:\s*True/i.test(text);
  return name ? { id, name, url, isPublic } : null;
}

// ── Segment builder ─────────────────────────────────────────────

function buildSegments(events: StreamEvent[], isStreaming: boolean): Segment[] {
  const segments: Segment[] = [];
  let pendingSteps: Step[] = [];
  // Map tool_use IDs to their step index for pairing with results
  const toolIdToStep = new Map<string, number>();

  function flushSteps() {
    if (pendingSteps.length === 0) return;
    const { label, icon } = activityLabel(pendingSteps);
    segments.push({ kind: "activity", label, icon, steps: [...pendingSteps] });
    // After the activity, surface any deployed app as a top-level inline
    // preview so it doesn't get buried inside a collapsed activity drawer.
    for (const step of pendingSteps) {
      const a = step.artifact;
      if (a && a.id && a.url) {
        segments.push({
          kind: "inline_preview",
          artifact: { kind: "app", id: a.id, name: a.name, url: a.url },
        });
      }
    }
    pendingSteps = [];
    toolIdToStep.clear();
  }

  for (const event of events) {
    const r = raw(event);
    const { base, sub } = parseEventType(event.type);

    if (base === "system") {
      if (sub === "task_started") {
        const data = (r.data ?? r) as Block;
        pendingSteps.push({
          icon: Bot,
          label: String(data.description ?? "Subagent"),
          status: "active",
        });
      } else if (sub === "task_notification") {
        const data = (r.data ?? r) as Block;
        const summary = String(data.summary ?? data.description ?? "");
        if (summary) {
          pendingSteps.push({
            icon: ListTree,
            label: summary,
            status: "complete",
          });
        }
      }
      continue;
    }

    if (base === "assistant") {
      const content = r.content;
      if (!Array.isArray(content)) continue;

      for (const block of content as Block[]) {
        if (block.type === "text" && typeof block.text === "string") {
          flushSteps();
          segments.push({ kind: "text", text: block.text });
        } else if (block.type === "tool_use") {
          const name = String(block.name ?? "tool");
          const input = block.input as Block | undefined;
          const toolId = String(block.id ?? "");

          // show_preview_artifact is a UI-only signal: flush any in-flight
          // activity group and push an inline preview segment instead of a
          // chain-of-thought step. The tool_use.input carries everything
          // the UI needs — url for apps, content_type for objects.
          if (name === "mcp__nightshift__show_preview_artifact") {
            const id = String(input?.artifact_id ?? "");
            const nm = String(input?.name ?? "Preview");
            const rawCap = input?.caption;
            const cap = typeof rawCap === "string" ? rawCap : undefined;
            const artifactKind =
              String(input?.type ?? "app") === "object" ? "object" : "app";

            if (id && artifactKind === "app") {
              const u = String(input?.url ?? "");
              if (u) {
                flushSteps();
                segments.push({
                  kind: "inline_preview",
                  artifact: { kind: "app", id, name: nm, url: u },
                  caption: cap,
                });
              }
            } else if (id && artifactKind === "object") {
              const ct = String(input?.content_type ?? "");
              if (ct) {
                flushSteps();
                segments.push({
                  kind: "inline_preview",
                  artifact: { kind: "object", id, name: nm, contentType: ct },
                  caption: cap,
                });
              }
            }

            // Register a sentinel so the matching tool_result is a no-op.
            if (toolId) toolIdToStep.set(toolId, -1);
            continue;
          }

          const stepIdx = pendingSteps.length;
          pendingSteps.push({
            icon: iconFor(name),
            label: labelFor(name, input),
            description: descriptionFor(name, input),
            status: isStreaming ? "active" : "complete",
          });
          if (toolId) toolIdToStep.set(toolId, stepIdx);
        }
      }
      continue;
    }

    if (base === "user") {
      const content = r.content;
      if (!Array.isArray(content)) continue;

      for (const block of content as Block[]) {
        if (block.type === "tool_result") {
          const toolId = String(block.tool_use_id ?? "");
          const stepIdx = toolIdToStep.get(toolId);
          const hasStep = stepIdx !== undefined && stepIdx >= 0 && !!pendingSteps[stepIdx];

          // Extract search results from tool_use_result (WebSearch)
          const searchResults = extractSearchResults(r);
          if (searchResults.length > 0 && hasStep) {
            pendingSteps[stepIdx!]!.searchResults = searchResults;
            pendingSteps[stepIdx!]!.status = "complete";
          }

          // ToolSearch result — show what tools were discovered
          const tur = r.tool_use_result;
          if (tur && typeof tur === "object" && !Array.isArray(tur) && Array.isArray((tur as Block).matches)) {
            const matches = (tur as Block).matches as string[];
            if (matches.length > 0 && hasStep) {
              pendingSteps[stepIdx!]!.description = `Found: ${matches.join(", ")}`;
            }
          }

          // Check for artifact in text content
          let text = "";
          if (typeof block.content === "string") text = block.content;
          else if (Array.isArray(block.content)) {
            const tb = (block.content as Block[]).find((c) => c.type === "text");
            if (tb && typeof tb.text === "string") text = tb.text;
          }
          const artifact = text ? parseArtifactFromText(text) : null;
          if (artifact && hasStep) {
            pendingSteps[stepIdx!]!.artifact = artifact;
            pendingSteps[stepIdx!]!.status = "complete";
          }

          // Mark step complete
          if (hasStep) {
            pendingSteps[stepIdx!]!.status = "complete";
          }
        }
      }
      continue;
    }

    if (base === "result") {
      flushSteps();
      const cost = typeof r.total_cost_usd === "number" ? `$${r.total_cost_usd.toFixed(4)}` : null;
      const duration = typeof r.duration_ms === "number" ? formatDuration(r.duration_ms) : null;
      const isError = r.is_error === true || sub === "error";
      segments.push({ kind: "result", cost, duration, isError });
      continue;
    }
  }

  // Flush remaining
  if (isStreaming && pendingSteps.length > 0) {
    const last = pendingSteps[pendingSteps.length - 1];
    if (last) last.status = "active";
  }
  flushSteps();
  return segments;
}

// ── Sub-components ──────────────────────────────────────────────

const MAX_RESULTS = 5;

function Favicon({ domain }: { domain: string }) {
  if (!domain) return <Search className="size-3.5 shrink-0 text-muted" />;
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={`https://www.google.com/s2/favicons?sz=32&domain=${domain}`}
      alt=""
      width={14}
      height={14}
      className="size-3.5 shrink-0 rounded-sm"
      loading="lazy"
    />
  );
}

function SearchResultList({ results }: { results: SearchResult[] }) {
  const visible = results.slice(0, MAX_RESULTS);
  const remaining = results.length - MAX_RESULTS;

  return (
    <div className="mt-1 space-y-0.5">
      {visible.map((r, i) => (
        <div key={i} className="flex items-center gap-2 py-0.5 text-xs text-muted">
          <Favicon domain={r.domain} />
          <span className="truncate">{r.title}</span>
          {r.domain && <span className="shrink-0 opacity-50">{r.domain}</span>}
        </div>
      ))}
      {remaining > 0 && (
        <div className="text-xs text-muted/50 pl-5.5">+{remaining} more</div>
      )}
    </div>
  );
}

function ArtifactBadge({
  artifact,
  onOpen,
}: {
  artifact: ArtifactInfo;
  onOpen?: (view: ArtifactView) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => {
        if (!artifact.url) return;
        onOpen?.({
          kind: "app",
          id: artifact.id ?? "",
          name: artifact.name,
          url: resolveAppUrl(artifact.url, artifact.id),
        });
      }}
      className="mt-1.5 inline-flex items-center gap-2 rounded-lg border border-night-border bg-night-surface/50 px-3 py-2 hover:bg-night-hover transition-colors cursor-pointer text-left"
    >
      <FileText size={14} className="text-muted shrink-0" />
      <span className="text-sm">{artifact.name}</span>
      {artifact.url && (
        <span className="flex items-center gap-1 text-xs text-lime ml-1">
          <ExternalLink size={10} /> View
        </span>
      )}
    </button>
  );
}

function InlinePreview({
  artifact,
  caption,
  onOpen,
}: {
  artifact: InlineArtifact;
  caption?: string;
  onOpen?: (view: ArtifactView) => void;
}) {
  const HeaderIcon = artifact.kind === "app" ? Monitor : FileText;
  const handleOpen = () => onOpen?.(artifact);
  // NOTE: wrapper must be a <div role="button">, NOT a <button>. Object
  // previews render streamdown code blocks and other interactive elements
  // that already contain <button> descendants — nesting them inside a
  // <button> is invalid HTML and causes a React hydration error.
  return (
    <div className="my-2">
      {caption && <div className="text-xs text-muted mb-1.5">{caption}</div>}
      <div
        role="button"
        tabIndex={0}
        onClick={handleOpen}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            handleOpen();
          }
        }}
        className="group relative block w-full overflow-hidden rounded-xl border border-night-border bg-night-surface hover:border-lime/40 transition-colors text-left cursor-pointer focus:outline-none focus:border-lime/60"
      >
        <div className="flex items-center gap-2 h-8 px-3 border-b border-night-border">
          <HeaderIcon size={12} className="text-muted shrink-0" />
          <span className="text-xs text-secondary truncate flex-1">{artifact.name}</span>
          {artifact.kind === "object" && (
            <span className="text-[10px] text-muted/60 shrink-0">{artifact.contentType}</span>
          )}
          <span className="flex items-center gap-1 text-[11px] text-lime opacity-0 group-hover:opacity-100 transition-opacity">
            <ExternalLink size={10} /> Open
          </span>
        </div>
        <div className="relative bg-black">
          {artifact.kind === "app" ? (
            <div className="relative aspect-[16/10]">
              <iframe
                src={resolveAppUrl(artifact.url, artifact.id)}
                title={artifact.name}
                className="absolute inset-0 w-full h-full border-0 pointer-events-none"
                sandbox="allow-scripts allow-same-origin"
                loading="lazy"
              />
            </div>
          ) : (
            <ObjectViewer
              id={artifact.id}
              name={artifact.name}
              contentType={artifact.contentType}
              variant="inline"
            />
          )}
          {/* Transparent click-catcher so the wrapper receives clicks. For
              scrollable object renderers this intentionally blocks inline
              interaction — users expand to the side panel for full access. */}
          <div className="absolute inset-0" />
        </div>
      </div>
    </div>
  );
}

function ActivitySegment({
  label,
  icon: Icon,
  steps,
  isLive = false,
  onOpenArtifact,
}: Segment & { kind: "activity" } & { isLive?: boolean; onOpenArtifact?: (view: ArtifactView) => void }) {
  const [manualOpen, setManualOpen] = useState<boolean | null>(null);
  const open = manualOpen !== null ? manualOpen : isLive;

  return (
    <ChainOfThought open={open} onOpenChange={setManualOpen}>
      <ChainOfThoughtHeader icon={Icon}>
        {label}
      </ChainOfThoughtHeader>
      <ChainOfThoughtContent>
        {steps.map((step, i) => (
          <ChainOfThoughtStep
            key={i}
            icon={step.icon}
            label={step.label}
            description={step.description}
            status={step.status}
          >
            {step.searchResults && step.searchResults.length > 0 && (
              <SearchResultList results={step.searchResults} />
            )}
            {step.artifact && <ArtifactBadge artifact={step.artifact} onOpen={onOpenArtifact} />}
          </ChainOfThoughtStep>
        ))}
      </ChainOfThoughtContent>
    </ChainOfThought>
  );
}

function ResultFooter({ cost, duration, isError }: { cost: string | null; duration: string | null; isError: boolean }) {
  const stats = [cost, duration].filter(Boolean).join(", ");
  if (!stats) return null;
  return (
    <div className={cn("pt-3 mt-3 border-t border-night-border text-sm", isError ? "text-error" : "text-success")}>
      <span className="font-medium">{isError ? "Error" : "Completed"}</span>
      <span className="text-muted ml-1">— {stats}</span>
    </div>
  );
}

// ── Main ────────────────────────────────────────────────────────

export const EventStream = memo(function EventStream({
  events,
  isStreaming,
  onOpenArtifact,
}: {
  events: StreamEvent[];
  isStreaming: boolean;
  onOpenArtifact?: (view: ArtifactView) => void;
}) {
  const segments = useMemo(() => buildSegments(events, isStreaming), [events, isStreaming]);

  if (segments.length === 0 && isStreaming) {
    return (
      <div className="flex items-center gap-2 text-muted text-sm py-2">
        <Loader2 size={14} className="animate-spin" />
        <span>Thinking...</span>
      </div>
    );
  }

  // Find the last activity segment index for auto-open during streaming
  const lastActivityIdx = isStreaming
    ? segments.reduce((acc, seg, i) => (seg.kind === "activity" ? i : acc), -1)
    : -1;

  return (
    <div className="space-y-4">
      {segments.map((seg, i) => {
        if (seg.kind === "text") {
          return (
            <div key={i}>
              <MessageResponse isAnimating={isStreaming && i === segments.length - 1}>
                {seg.text}
              </MessageResponse>
            </div>
          );
        }
        if (seg.kind === "activity") {
          return <ActivitySegment key={i} {...seg} isLive={i === lastActivityIdx} onOpenArtifact={onOpenArtifact} />;
        }
        if (seg.kind === "inline_preview") {
          return (
            <InlinePreview
              key={i}
              artifact={seg.artifact}
              caption={seg.caption}
              onOpen={onOpenArtifact}
            />
          );
        }
        if (seg.kind === "result") {
          return <ResultFooter key={i} {...seg} />;
        }
        return null;
      })}
      {isStreaming && segments.length > 0 && segments[segments.length - 1]?.kind !== "text" && (
        <div className="flex items-center gap-2 text-muted text-xs mt-2">
          <Loader2 size={12} className="animate-spin" />
          <span>Working...</span>
        </div>
      )}
    </div>
  );
});
