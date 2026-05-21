/**
 * Resolve an app artifact URL into something the browser can actually load
 * from the UI origin.
 *
 * Three classes of input:
 *   1. Public absolute URLs (`https://{id}-app.ns-apps.com`) — pass through.
 *   2. In-cluster service URLs (`http://ns-app-….svc[…]`) — the browser
 *      can't resolve those; route via the UI proxy at
 *      `/api/artifacts/{id}/view` which the API server reverse-proxies.
 *   3. Relative cr0n-a backend paths (`/artifacts/{id}/view`) — rewrite
 *      to `/api/artifacts/{id}/view`.
 *
 * Idempotent: URLs that are already `/api/artifacts/…` pass through
 * unchanged, so running this in multiple render layers is safe.
 *
 * `artifactId` is required to resolve case (2). Pass it whenever the
 * caller has it; without it, an in-cluster URL passes through and the
 * iframe will fail.
 */
export function resolveAppUrl(url: string, artifactId?: string): string {
  if (!url) return url;
  if (url.startsWith("/api/artifacts/")) return url;
  if (url.startsWith("/artifacts/")) return `/api${url}`;
  if (url.startsWith("http://") || url.startsWith("https://")) {
    try {
      const host = new URL(url).hostname;
      // Kubernetes in-cluster service DNS — unreachable from the browser.
      // Fall back to the UI proxy when we can identify the artifact.
      if (artifactId && (host.endsWith(".svc") || host.endsWith(".cluster.local"))) {
        return `/api/artifacts/${encodeURIComponent(artifactId)}/view`;
      }
    } catch {
      // Malformed URL — leave as-is.
    }
    return url;
  }
  return url;
}

import type { ArtifactInfo } from "@/lib/api";

export type ArtifactCategory = "apps" | "docs" | "images" | "data" | "code" | "other";

/**
 * Bucket an artifact into one of the gallery's category chips. Apps always
 * land in "apps"; objects get sorted by their MIME type.
 */
export function categorizeArtifact(a: {
  type: ArtifactInfo["type"];
  content_type: string;
}): ArtifactCategory {
  if (a.type === "app") return "apps";
  const ct = (a.content_type || "").toLowerCase();
  if (ct.startsWith("image/")) return "images";
  if (
    ct === "text/csv" ||
    ct === "application/csv" ||
    ct === "application/json" ||
    ct.includes("spreadsheetml") ||
    ct.includes("officedocument.spreadsheet")
  ) return "data";
  if (
    ct === "application/pdf" ||
    ct === "text/markdown" ||
    ct === "text/x-markdown" ||
    ct === "text/plain" ||
    ct.includes("wordprocessingml") ||
    ct.includes("presentationml")
  ) return "docs";
  if (ct === "text/html" || ct.startsWith("text/") || ct.includes("xml")) return "code";
  return "other";
}

/**
 * Whether an artifact's content_type can be safely rendered as text in a
 * thumbnail. Excludes binary formats (PDF, Office, images) — those need
 * native renderers, not a `<pre>` slice.
 */
export function isTextPreviewable(contentType: string): boolean {
  const ct = (contentType || "").toLowerCase();
  if (!ct) return false;
  if (ct.startsWith("text/")) return true;
  if (
    ct === "application/json" ||
    ct === "application/xml" ||
    ct === "application/yaml" ||
    ct === "application/x-yaml" ||
    ct === "application/javascript" ||
    ct === "application/typescript" ||
    ct === "application/x-sh"
  ) return true;
  return false;
}

/**
 * Fetch the leading bytes of an artifact's content for an inline preview.
 * Sends a `Range` header so the upstream object store returns just the
 * slice we need (S3 honors this on the presigned URL behind /view).
 */
export async function fetchArtifactHead(
  artifactId: string,
  maxBytes = 4096,
  signal?: AbortSignal,
): Promise<string> {
  const res = await fetch(
    `/api/artifacts/${encodeURIComponent(artifactId)}/view`,
    {
      headers: { Range: `bytes=0-${maxBytes - 1}` },
      credentials: "same-origin",
      signal,
    },
  );
  if (!res.ok && res.status !== 206) {
    throw new Error(`HTTP ${res.status}`);
  }
  const text = await res.text();
  return text.slice(0, maxBytes);
}

export function formatArtifactSize(bytes: number): string {
  if (!bytes || bytes <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i += 1;
  }
  return `${n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}
