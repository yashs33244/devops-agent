import type { StreamEvent } from "@/lib/api";

/** Split dot-notation event type: "result.success" → { base: "result", sub: "success" } */
export function parseEventType(type: string): { base: string; sub: string } {
  const dot = type.indexOf(".");
  return dot === -1 ? { base: type, sub: "" } : { base: type.slice(0, dot), sub: type.slice(dot + 1) };
}

/** Safely extract the raw dict from a StreamEvent. */
export function raw(event: StreamEvent): Record<string, unknown> {
  return (typeof event.raw === "object" && !Array.isArray(event.raw) ? event.raw : {}) as Record<string, unknown>;
}

/** Format milliseconds as human-readable duration. */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

/** Check whether a result event represents an error. */
export function isResultError(event: StreamEvent): boolean {
  const r = raw(event);
  return r.is_error === true || event.type === "result.error";
}

/** Truncate a string with ellipsis. */
export function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + "\n..." : s;
}

/** Derive a short title from a prompt string. Tolerant of undefined/empty
 *  inputs — callers passing a missing API field shouldn't crash the UI. */
export function titleFromPrompt(prompt: string | null | undefined, fallback = "Untitled"): string {
  if (typeof prompt !== "string" || prompt.length === 0) return fallback;
  const firstLine = prompt.split("\n")[0] ?? prompt;
  const cleaned = firstLine.replace(/^<[^>]+>\s*/, "").replace(/^#{1,6}\s+/, "").trim();
  if (!cleaned) return fallback;
  return cleaned.length <= 60 ? cleaned : cleaned.slice(0, 57) + "...";
}
