"use client";

import { memo, useEffect, useMemo, useState } from "react";
import { cn } from "@/lib/utils";
import { MessageResponse } from "@/components/ai-elements/message";
import { FileText, Download, AlertCircle, Loader2 } from "lucide-react";

// Maximum size for client-side fetch + inline render of text-based content.
// Larger files fall back to a "too large" message with an "open raw" link.
const TEXT_FETCH_MAX_BYTES = 2 * 1024 * 1024;

// Inline previews cap at this max height so they stay "teaser"-sized.
const INLINE_MAX_HEIGHT = "max-h-[400px]";

// Office Open XML MIME constants for the content-type dispatch.
const DOCX_MIME =
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document";
const XLSX_MIME =
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
const PPTX_MIME =
  "application/vnd.openxmlformats-officedocument.presentationml.presentation";

export type ObjectVariant = "inline" | "panel";

export type ObjectViewerProps = {
  id: string;
  name: string;
  contentType: string;
  variant?: ObjectVariant;
};

// ── Dispatcher ──────────────────────────────────────────────────

export const ObjectViewer = memo(function ObjectViewer({
  id,
  name,
  contentType,
  variant = "panel",
}: ObjectViewerProps) {
  const url = `/api/artifacts/${id}/view`;
  const ct = (contentType || "").toLowerCase();

  if (ct.startsWith("image/")) return <ImageView url={url} name={name} variant={variant} />;
  if (ct === "application/pdf") return <PdfView url={url} name={name} variant={variant} />;
  if (ct === DOCX_MIME) return <DocxView id={id} name={name} variant={variant} />;
  if (ct === XLSX_MIME) return <XlsxView id={id} name={name} variant={variant} />;
  if (ct === PPTX_MIME) return <PptxView id={id} name={name} variant={variant} />;
  if (ct === "text/markdown" || ct === "text/x-markdown") return <MarkdownView url={url} variant={variant} />;
  if (ct === "application/json") return <JsonView url={url} variant={variant} />;
  if (ct === "text/csv" || ct === "application/csv") return <CsvView url={url} variant={variant} />;
  if (ct === "text/html") return <HtmlView url={url} name={name} variant={variant} />;
  if (ct.startsWith("text/")) return <TextView url={url} variant={variant} />;
  return <DownloadFallback url={url} name={name} contentType={contentType} />;
});

// ── Shared hook ─────────────────────────────────────────────────

type FetchedText = { text: string; error: string | null; loading: boolean };

function useFetchedText(url: string): FetchedText {
  const [state, setState] = useState<FetchedText>({ text: "", error: null, loading: true });
  useEffect(() => {
    const ac = new AbortController();
    setState({ text: "", error: null, loading: true });
    (async () => {
      try {
        const res = await fetch(url, { signal: ac.signal });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const len = Number(res.headers.get("content-length") || "0");
        if (len > TEXT_FETCH_MAX_BYTES) throw new Error("Preview too large");
        const text = await res.text();
        if (text.length > TEXT_FETCH_MAX_BYTES) throw new Error("Preview too large");
        setState({ text, error: null, loading: false });
      } catch (err) {
        if ((err as Error).name === "AbortError") return;
        setState({ text: "", error: (err as Error).message || "fetch failed", loading: false });
      }
    })();
    return () => ac.abort();
  }, [url]);
  return state;
}

// ── Scroll container helper ─────────────────────────────────────

function ViewerShell({
  variant,
  children,
}: {
  variant: ObjectVariant;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "w-full overflow-auto terminal-scroll",
        variant === "inline" ? INLINE_MAX_HEIGHT : "h-full",
      )}
    >
      {children}
    </div>
  );
}

// ── Loading / error rows ────────────────────────────────────────

function LoadingRow() {
  return (
    <div className="flex items-center gap-2 px-4 py-6 text-muted text-sm">
      <Loader2 size={14} className="animate-spin" />
      <span>Loading preview…</span>
    </div>
  );
}

function ErrorRow({ error, url }: { error: string; url: string }) {
  return (
    <div className="flex flex-col gap-2 px-4 py-4 text-sm">
      <div className="flex items-center gap-2 text-error">
        <AlertCircle size={14} className="shrink-0" />
        <span>{error}</span>
      </div>
      <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="text-xs text-lime hover:underline"
      >
        Open raw file →
      </a>
    </div>
  );
}

// ── Image ────────────────────────────────────────────────────────

function ImageView({ url, name, variant }: { url: string; name: string; variant: ObjectVariant }) {
  return (
    <div
      className={cn(
        "flex items-center justify-center w-full bg-night",
        variant === "inline" ? INLINE_MAX_HEIGHT : "h-full",
      )}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={url}
        alt={name}
        className="max-w-full max-h-full object-contain"
        loading="lazy"
      />
    </div>
  );
}

// ── PDF ──────────────────────────────────────────────────────────

// Chrome's built-in PDF viewer refuses to render inside a sandboxed
// <iframe>, showing "This page has been blocked by Chrome" instead.
// <object type="application/pdf"> is the correct HTML element for
// embedding documents and works reliably with the browser's native
// PDF plugin. Falls back to a download link if the browser has no
// PDF viewer at all.
function PdfView({ url, name, variant }: { url: string; name: string; variant: ObjectVariant }) {
  return (
    <object
      data={url}
      type="application/pdf"
      className={cn(
        "w-full bg-night",
        variant === "inline" ? "h-[400px]" : "h-full",
      )}
      aria-label={name}
    >
      <div className="flex h-full items-center justify-center p-6 text-sm text-muted">
        <a href={url} className="text-lime hover:underline">
          Download {name}
        </a>
      </div>
    </object>
  );
}

// ── HTML (rare — same sandboxing as apps) ───────────────────────

function HtmlView({ url, name, variant }: { url: string; name: string; variant: ObjectVariant }) {
  return (
    <iframe
      src={url}
      title={name}
      sandbox="allow-scripts allow-same-origin"
      className={cn(
        "w-full border-0 bg-night",
        variant === "inline" ? "h-[400px]" : "h-full",
      )}
      loading="lazy"
    />
  );
}

// ── DOCX / XLSX (server-generated HTML preview via srcDoc) ─────

/**
 * Shared sub-renderer for docx/xlsx. Fetches the stored HTML preview
 * from `/api/artifacts/{id}/preview` and feeds it into a sandboxed
 * iframe via srcDoc. Without `allow-same-origin` in the sandbox the
 * iframe is in an opaque origin — no cookies, no fetch, no parent
 * access — so even if the preview HTML contained untrusted markup it
 * can't escape. `allow-scripts` is kept on so the xlsx preview's
 * tab-switching helper can run.
 *
 * On fetch error (including the 404 returned when a preview wasn't
 * generated for an artifact), falls back to `DownloadFallback`.
 */
function PreviewIframeView({
  id,
  name,
  variant,
  contentType,
}: {
  id: string;
  name: string;
  variant: ObjectVariant;
  contentType: string;
}) {
  const previewUrl = `/api/artifacts/${id}/preview`;
  const downloadUrl = `/api/artifacts/${id}/view`;
  const [state, setState] = useState<{ html: string; error: string | null; loading: boolean }>({
    html: "",
    error: null,
    loading: true,
  });

  useEffect(() => {
    const ac = new AbortController();
    setState({ html: "", error: null, loading: true });
    (async () => {
      try {
        const res = await fetch(previewUrl, { signal: ac.signal });
        if (!res.ok) {
          throw new Error(res.status === 404 ? "No inline preview" : `HTTP ${res.status}`);
        }
        const len = Number(res.headers.get("content-length") || "0");
        if (len > TEXT_FETCH_MAX_BYTES) throw new Error("Preview too large");
        const text = await res.text();
        if (text.length > TEXT_FETCH_MAX_BYTES) throw new Error("Preview too large");
        setState({ html: text, error: null, loading: false });
      } catch (err) {
        if ((err as Error).name === "AbortError") return;
        setState({ html: "", error: (err as Error).message || "fetch failed", loading: false });
      }
    })();
    return () => ac.abort();
  }, [previewUrl]);

  if (state.loading) return <LoadingRow />;
  if (state.error) {
    return <DownloadFallback url={downloadUrl} name={name} contentType={contentType} />;
  }
  return (
    <iframe
      srcDoc={state.html}
      title={name}
      sandbox="allow-scripts"
      className={cn(
        "w-full border-0 bg-white",
        variant === "inline" ? "h-[400px]" : "h-full",
      )}
      loading="lazy"
    />
  );
}

function DocxView({ id, name, variant }: { id: string; name: string; variant: ObjectVariant }) {
  return <PreviewIframeView id={id} name={name} variant={variant} contentType={DOCX_MIME} />;
}

function XlsxView({ id, name, variant }: { id: string; name: string; variant: ObjectVariant }) {
  return <PreviewIframeView id={id} name={name} variant={variant} contentType={XLSX_MIME} />;
}

function PptxView({ id, name, variant }: { id: string; name: string; variant: ObjectVariant }) {
  return <PreviewIframeView id={id} name={name} variant={variant} contentType={PPTX_MIME} />;
}

// ── Markdown ────────────────────────────────────────────────────

function MarkdownView({ url, variant }: { url: string; variant: ObjectVariant }) {
  const { text, error, loading } = useFetchedText(url);
  if (loading) return <LoadingRow />;
  if (error) return <ErrorRow error={error} url={url} />;
  return (
    <ViewerShell variant={variant}>
      <div className="px-4 py-3">
        <MessageResponse>{text}</MessageResponse>
      </div>
    </ViewerShell>
  );
}

// ── JSON ─────────────────────────────────────────────────────────

function JsonView({ url, variant }: { url: string; variant: ObjectVariant }) {
  const { text, error, loading } = useFetchedText(url);
  const rendered = useMemo(() => {
    if (!text) return "";
    try {
      return "```json\n" + JSON.stringify(JSON.parse(text), null, 2) + "\n```";
    } catch {
      // Not valid JSON — show as a plain code block so streamdown still
      // renders monospace without syntax colors.
      return "```\n" + text + "\n```";
    }
  }, [text]);
  if (loading) return <LoadingRow />;
  if (error) return <ErrorRow error={error} url={url} />;
  return (
    <ViewerShell variant={variant}>
      <div className="px-4 py-3">
        <MessageResponse>{rendered}</MessageResponse>
      </div>
    </ViewerShell>
  );
}

// ── CSV ──────────────────────────────────────────────────────────

const CSV_MAX_ROWS = 200;

/** Tiny CSV parser. Handles quoted fields with embedded commas, newlines,
 * and escaped double quotes (""). Doesn't try to be RFC 4180 perfect. */
function parseCsv(input: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let inQuotes = false;
  for (let i = 0; i < input.length; i++) {
    const c = input[i];
    if (inQuotes) {
      if (c === '"') {
        if (input[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += c;
      }
      continue;
    }
    if (c === '"') {
      inQuotes = true;
    } else if (c === ",") {
      row.push(field);
      field = "";
    } else if (c === "\n" || c === "\r") {
      row.push(field);
      field = "";
      if (row.length > 1 || row[0] !== "") rows.push(row);
      row = [];
      if (c === "\r" && input[i + 1] === "\n") i++;
    } else {
      field += c;
    }
  }
  if (field !== "" || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows;
}

function CsvView({ url, variant }: { url: string; variant: ObjectVariant }) {
  const { text, error, loading } = useFetchedText(url);
  const parsed = useMemo(() => (text ? parseCsv(text) : []), [text]);
  if (loading) return <LoadingRow />;
  if (error) return <ErrorRow error={error} url={url} />;
  if (parsed.length === 0) {
    return (
      <ViewerShell variant={variant}>
        <div className="px-4 py-4 text-muted text-sm">Empty CSV</div>
      </ViewerShell>
    );
  }
  const [header, ...body] = parsed;
  const visibleRows = body.slice(0, CSV_MAX_ROWS);
  const remaining = body.length - visibleRows.length;
  return (
    <ViewerShell variant={variant}>
      <div className="px-4 py-3">
        <table className="w-full text-xs border-collapse font-mono">
          <thead>
            <tr className="border-b border-night-border">
              {(header ?? []).map((cell, i) => (
                <th
                  key={i}
                  className="text-left font-medium text-secondary px-2 py-1.5 whitespace-nowrap"
                >
                  {cell}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visibleRows.map((row, ri) => (
              <tr key={ri} className="border-b border-night-border/40 hover:bg-night-hover/30">
                {row.map((cell, ci) => (
                  <td key={ci} className="text-muted px-2 py-1 whitespace-nowrap">
                    {cell}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {remaining > 0 && (
          <div className="mt-2 text-[11px] text-muted/60">
            +{remaining} more row{remaining === 1 ? "" : "s"} — open the raw file to see all
          </div>
        )}
      </div>
    </ViewerShell>
  );
}

// ── Plain text ──────────────────────────────────────────────────

function TextView({ url, variant }: { url: string; variant: ObjectVariant }) {
  const { text, error, loading } = useFetchedText(url);
  if (loading) return <LoadingRow />;
  if (error) return <ErrorRow error={error} url={url} />;
  return (
    <ViewerShell variant={variant}>
      <pre className="px-4 py-3 font-mono text-xs whitespace-pre-wrap text-secondary">{text}</pre>
    </ViewerShell>
  );
}

// ── Download fallback ───────────────────────────────────────────

function DownloadFallback({
  url,
  name,
  contentType,
}: {
  url: string;
  name: string;
  contentType: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 w-full p-6 bg-night min-h-[160px]">
      <FileText size={32} className="text-muted/50" />
      <div className="text-center">
        <div className="text-sm text-primary">{name}</div>
        <div className="text-xs text-muted mt-0.5">{contentType || "unknown type"}</div>
      </div>
      <a
        href={url}
        download={name}
        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-night-border text-xs text-muted hover:text-secondary hover:bg-night-hover transition-colors"
      >
        <Download size={12} />
        Download
      </a>
    </div>
  );
}
