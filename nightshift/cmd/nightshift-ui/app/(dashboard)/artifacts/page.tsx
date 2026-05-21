"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AppWindow,
  ArrowDownAZ,
  ArrowDownUp,
  ArrowUpAZ,
  Check,
  Clock,
  Copy,
  Database,
  Download,
  ExternalLink,
  FileCode,
  FileText,
  Globe,
  Grid3X3,
  Image as ImageIcon,
  List,
  Loader2,
  Lock,
  MoreVertical,
  Package,
  Pencil,
  Search,
  Trash2,
  UserPlus,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { PanelHeader } from "@/lib/ui";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  deleteArtifact,
  listArtifacts,
  updateArtifact,
  type ArtifactInfo,
} from "@/lib/api";
import {
  categorizeArtifact,
  fetchArtifactHead,
  formatArtifactSize,
  isTextPreviewable,
  resolveAppUrl,
  type ArtifactCategory,
} from "@/lib/artifacts";
import { ObjectViewer } from "@/components/chat/object-viewer";
import { ShareDialog } from "@/components/artifacts/share-dialog";

// ── Category metadata ───────────────────────────────────────────

type ChipId = "all" | ArtifactCategory;

const CHIPS: { id: ChipId; label: string }[] = [
  { id: "all", label: "All" },
  { id: "apps", label: "Apps" },
  { id: "docs", label: "Docs" },
  { id: "images", label: "Images" },
  { id: "data", label: "Data" },
  { id: "code", label: "Code" },
  { id: "other", label: "Other" },
];

const CATEGORY_META: Record<
  ArtifactCategory,
  { label: string; icon: typeof FileText }
> = {
  apps: { label: "App", icon: AppWindow },
  docs: { label: "Document", icon: FileText },
  images: { label: "Image", icon: ImageIcon },
  data: { label: "Data", icon: Database },
  code: { label: "Code", icon: FileCode },
  other: { label: "File", icon: Package },
};

function categoryFor(a: ArtifactInfo): ArtifactCategory {
  return categorizeArtifact({ type: a.type, content_type: a.content_type });
}

type ViewMode = "grid" | "list";

type SortMode = "newest" | "oldest" | "name-asc" | "name-desc";

const SORT_OPTIONS: {
  id: SortMode;
  label: string;
  icon: typeof Clock;
}[] = [
  { id: "newest", label: "Newest first", icon: Clock },
  { id: "oldest", label: "Oldest first", icon: Clock },
  { id: "name-asc", label: "Name (A–Z)", icon: ArrowDownAZ },
  { id: "name-desc", label: "Name (Z–A)", icon: ArrowUpAZ },
];

function compareArtifacts(a: ArtifactInfo, b: ArtifactInfo, mode: SortMode): number {
  switch (mode) {
    case "newest":
      return b.created_at.localeCompare(a.created_at);
    case "oldest":
      return a.created_at.localeCompare(b.created_at);
    case "name-asc":
      return (a.name || "").localeCompare(b.name || "", undefined, { sensitivity: "base" });
    case "name-desc":
      return (b.name || "").localeCompare(a.name || "", undefined, { sensitivity: "base" });
  }
}

// ── Page ────────────────────────────────────────────────────────

export default function ArtifactsPage() {
  const [artifacts, setArtifacts] = useState<ArtifactInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [chip, setChip] = useState<ChipId>("all");
  const [query, setQuery] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>("grid");
  const [sortMode, setSortMode] = useState<SortMode>("newest");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const list = await listArtifacts();
      setArtifacts(list);
    } catch (e) {
      setError((e as Error).message);
      setArtifacts([]);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    const onFocus = () => { refresh(); };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refresh]);

  const categoryCounts = useMemo(() => {
    const counts: Record<ChipId, number> = {
      all: 0, apps: 0, docs: 0, images: 0, data: 0, code: 0, other: 0,
    };
    if (!artifacts) return counts;
    counts.all = artifacts.length;
    for (const a of artifacts) counts[categoryFor(a)] += 1;
    return counts;
  }, [artifacts]);

  const filtered = useMemo(() => {
    if (!artifacts) return [];
    const q = query.trim().toLowerCase();
    const result = artifacts.filter((a) => {
      if (chip !== "all" && categoryFor(a) !== chip) return false;
      if (!q) return true;
      return (
        a.name.toLowerCase().includes(q) ||
        a.description.toLowerCase().includes(q)
      );
    });
    result.sort((a, b) => compareArtifacts(a, b, sortMode));
    return result;
  }, [artifacts, chip, query, sortMode]);

  const groupByDateInList = sortMode === "newest" || sortMode === "oldest";
  const grouped = useMemo(
    () => (groupByDateInList ? groupByDate(filtered) : null),
    [filtered, groupByDateInList],
  );
  const selected = useDerivedSelected(artifacts, selectedId);

  return (
    <div className="flex-1 flex flex-col min-h-0 bg-night">
      <PanelHeader>
        <div className="flex items-center gap-3 shrink-0">
          <span className="text-[15px] font-semibold text-primary">Artifacts</span>
          {artifacts !== null && (
            <span className="text-sm text-muted tabular-nums">
              {filtered.length} {filtered.length === 1 ? "item" : "items"}
            </span>
          )}
        </div>
        <div className="flex-1 flex justify-center px-4">
          <div className="relative w-full max-w-md">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none"
            />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search artifacts…"
              className="pl-9 pr-3 py-1.5 text-sm w-full rounded-lg border border-night-border bg-transparent text-primary placeholder:text-muted focus:outline-none focus:border-lime/40 transition-colors"
            />
            {query && (
              <button
                onClick={() => setQuery("")}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted hover:text-secondary"
              >
                <X size={12} />
              </button>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                aria-label="Sort"
                className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-night-border text-xs text-muted hover:text-secondary hover:bg-night-hover transition-colors"
              >
                <ArrowDownUp size={12} />
                <span className="hidden sm:inline">
                  {SORT_OPTIONS.find((o) => o.id === sortMode)?.label}
                </span>
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
              {SORT_OPTIONS.map((opt) => (
                <DropdownMenuItem
                  key={opt.id}
                  onSelect={() => setSortMode(opt.id)}
                  className={cn(sortMode === opt.id && "bg-night-hover")}
                >
                  <opt.icon size={14} />
                  {opt.label}
                  {sortMode === opt.id && <Check size={12} className="ml-auto" />}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <div className="flex items-center rounded-lg border border-night-border overflow-hidden">
            <button
              onClick={() => setViewMode("grid")}
              aria-label="Grid view"
              className={cn(
                "p-1.5 transition-colors",
                viewMode === "grid"
                  ? "bg-night-hover text-primary"
                  : "text-muted hover:text-secondary",
              )}
            >
              <Grid3X3 size={14} />
            </button>
            <button
              onClick={() => setViewMode("list")}
              aria-label="List view"
              className={cn(
                "p-1.5 transition-colors",
                viewMode === "list"
                  ? "bg-night-hover text-primary"
                  : "text-muted hover:text-secondary",
              )}
            >
              <List size={14} />
            </button>
          </div>
        </div>
      </PanelHeader>

      <div className="flex-1 overflow-y-auto terminal-scroll p-6">
        <div className="mb-5 flex items-center gap-1 rounded-lg border border-night-border w-fit overflow-hidden">
          {CHIPS.map((c) => {
            const active = chip === c.id;
            const count = categoryCounts[c.id];
            return (
              <button
                key={c.id}
                onClick={() => setChip(c.id)}
                disabled={c.id !== "all" && count === 0}
                className={cn(
                  "px-3 py-1.5 text-sm font-medium transition-colors flex items-center gap-1.5 disabled:opacity-30 disabled:cursor-not-allowed",
                  active
                    ? "bg-night-hover text-primary"
                    : "text-muted hover:text-secondary",
                )}
              >
                {c.label}
                <span
                  className={cn(
                    "text-[10px] tabular-nums",
                    active ? "text-lime" : "text-muted",
                  )}
                >
                  {count}
                </span>
              </button>
            );
          })}
        </div>

        {error && (
          <div className="mb-4 rounded-lg border border-error/40 bg-error/5 px-4 py-3 text-sm text-error">
            Failed to load artifacts: {error}
          </div>
        )}

        {artifacts === null ? (
          <div className="flex items-center gap-2 py-12 justify-center text-sm text-muted">
            <Loader2 size={14} className="animate-spin" /> Loading artifacts…
          </div>
        ) : artifacts.length === 0 ? (
          <EmptyState />
        ) : filtered.length === 0 ? (
          <p className="text-sm text-muted text-center py-12">
            No artifacts match your filter
          </p>
        ) : viewMode === "grid" ? (
          <div
            className="grid gap-3"
            style={{ gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))" }}
          >
            {filtered.map((a) => (
              <ArtifactCard
                key={a.id}
                artifact={a}
                onOpen={() => setSelectedId(a.id)}
                onChanged={refresh}
                onDeleted={refresh}
              />
            ))}
          </div>
        ) : grouped ? (
          <div className="space-y-5">
            {grouped.map((g) => (
              <div key={g.label}>
                <h3 className="text-[10px] font-medium uppercase tracking-wider text-muted mb-1.5 px-3">
                  {g.label}
                </h3>
                <div className="rounded-lg border border-night-border overflow-hidden divide-y divide-night-border">
                  {g.items.map((a) => (
                    <ArtifactRow
                      key={a.id}
                      artifact={a}
                      onOpen={() => setSelectedId(a.id)}
                      onChanged={refresh}
                      onDeleted={refresh}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="rounded-lg border border-night-border overflow-hidden divide-y divide-night-border">
            {filtered.map((a) => (
              <ArtifactRow
                key={a.id}
                artifact={a}
                onOpen={() => setSelectedId(a.id)}
                onChanged={refresh}
                onDeleted={refresh}
              />
            ))}
          </div>
        )}
      </div>

      {selected && (
        <ArtifactDialog
          key={selected.id}
          artifact={selected}
          onClose={() => setSelectedId(null)}
          onChanged={refresh}
          onDeleted={async () => {
            setSelectedId(null);
            await refresh();
          }}
        />
      )}
    </div>
  );
}

// ── Date grouping ───────────────────────────────────────────────

function startOfDay(d: Date): number {
  const r = new Date(d);
  r.setHours(0, 0, 0, 0);
  return r.getTime();
}

function formatDateGroup(iso: string): string {
  const d = new Date(iso);
  const diffDays = Math.floor(
    (startOfDay(new Date()) - startOfDay(d)) / (1000 * 60 * 60 * 24),
  );
  if (diffDays <= 0) return "Today";
  if (diffDays === 1) return "Yesterday";
  if (diffDays < 7) return d.toLocaleDateString("en-US", { weekday: "long" });
  return d.toLocaleDateString("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
  });
}

function groupByDate(
  items: ArtifactInfo[],
): { label: string; items: ArtifactInfo[] }[] {
  const order: string[] = [];
  const groups = new Map<string, ArtifactInfo[]>();
  for (const item of items) {
    const label = formatDateGroup(item.created_at);
    if (!groups.has(label)) {
      groups.set(label, []);
      order.push(label);
    }
    groups.get(label)!.push(item);
  }
  return order.map((label) => ({ label, items: groups.get(label)! }));
}

// ── Selection helpers / dialog ──────────────────────────────────

function useDerivedSelected(
  artifacts: ArtifactInfo[] | null,
  selectedId: string | null,
): ArtifactInfo | null {
  return useMemo(() => {
    if (!selectedId || !artifacts) return null;
    return artifacts.find((a) => a.id === selectedId) ?? null;
  }, [artifacts, selectedId]);
}

// Shared rename / visibility / copy / share / delete state. Used by the
// dialog header *and* by every card and row, so per-instance state
// (busy, confirm-delete, share-dialog open) doesn't leak between them.
function useArtifactActions(
  artifact: ArtifactInfo,
  onChanged: () => Promise<void> | void,
  onDeleted: () => Promise<void> | void,
) {
  const [busy, setBusy] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sharing, setSharing] = useState(false);

  const isApp = artifact.type === "app";
  const openUrl = isApp
    ? resolveAppUrl(artifact.app_url, artifact.id)
    : `/api/artifacts/${artifact.id}/view`;
  const downloadHref = isApp
    ? null
    : `/api/artifacts/${artifact.id}/download`;

  const doUpdate = async (
    body: Parameters<typeof updateArtifact>[1],
    label: string,
  ) => {
    setBusy(label);
    setError(null);
    try {
      await updateArtifact(artifact.id, body);
      await onChanged();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const rename = async (next: string) => {
    const trimmed = next.trim();
    if (!trimmed || trimmed === artifact.name) return;
    await doUpdate({ name: trimmed }, "rename");
  };

  const togglePublic = () =>
    doUpdate({ public: !artifact.public }, "visibility");

  const copyLink = async () => {
    try {
      const url = isApp && artifact.public
        ? artifact.app_url
        : new URL(openUrl, window.location.origin).toString();
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("Failed to copy");
    }
  };

  const doDelete = async () => {
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setBusy("delete");
    setError(null);
    try {
      await deleteArtifact(artifact.id);
      await onDeleted();
    } catch (e) {
      setError((e as Error).message);
      setBusy(null);
    }
  };

  return {
    isApp,
    openUrl,
    downloadHref,
    busy,
    error,
    copied,
    confirmDelete,
    sharing,
    setSharing,
    rename,
    togglePublic,
    copyLink,
    doDelete,
  };
}

type ArtifactActions = ReturnType<typeof useArtifactActions>;

function ArtifactMenuItems({
  artifact,
  actions,
  onRename,
}: {
  artifact: ArtifactInfo;
  actions: ArtifactActions;
  onRename: () => void;
}) {
  const { isApp, busy, copied, confirmDelete, downloadHref } = actions;
  return (
    <>
      <DropdownMenuItem onSelect={onRename}>
        <Pencil size={14} /> Rename
      </DropdownMenuItem>
      <DropdownMenuItem
        onSelect={actions.togglePublic}
        disabled={busy === "visibility"}
      >
        {artifact.public ? <Lock size={14} /> : <Globe size={14} />}
        {artifact.public ? "Make private" : "Make public"}
      </DropdownMenuItem>
      <DropdownMenuItem onSelect={actions.copyLink}>
        {copied ? <Check size={14} /> : <Copy size={14} />}
        {copied
          ? "Copied"
          : isApp && artifact.public
          ? "Copy public URL"
          : "Copy link"}
      </DropdownMenuItem>
      <DropdownMenuItem onSelect={() => actions.setSharing(true)}>
        <UserPlus size={14} /> Share…
      </DropdownMenuItem>
      {downloadHref && (
        <DropdownMenuItem
          onSelect={() => {
            window.location.href = downloadHref;
          }}
        >
          <Download size={14} /> Download
        </DropdownMenuItem>
      )}
      <DropdownMenuSeparator />
      <DropdownMenuItem
        onSelect={(e) => {
          e.preventDefault();
          actions.doDelete();
        }}
        className={cn(
          "text-error focus:text-error",
          confirmDelete && "bg-error/10",
        )}
        disabled={busy === "delete"}
      >
        <Trash2 size={14} />
        {confirmDelete ? "Really delete?" : "Delete"}
      </DropdownMenuItem>
    </>
  );
}

function ArtifactDialog({
  artifact,
  onClose,
  onChanged,
  onDeleted,
}: {
  artifact: ArtifactInfo;
  onClose: () => void;
  onChanged: () => Promise<void> | void;
  onDeleted: () => Promise<void> | void;
}) {
  const [renaming, setRenaming] = useState(false);
  const [nameDraft, setNameDraft] = useState(artifact.name);
  const actions = useArtifactActions(artifact, onChanged, onDeleted);

  const startRename = () => {
    setNameDraft(artifact.name);
    setRenaming(true);
  };

  const commitRename = async () => {
    await actions.rename(nameDraft);
    setRenaming(false);
  };

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent
        showCloseButton={false}
        className="max-w-[95vw] w-[95vw] max-h-[92vh] h-[92vh] p-0 gap-0 rounded-2xl sm:top-[4vh] sm:translate-y-0 overflow-hidden flex flex-col"
      >
        <DialogTitle className="sr-only">{artifact.name}</DialogTitle>
        <DialogDescription className="sr-only">
          {artifact.type === "app" ? "Deployed app" : artifact.content_type}
        </DialogDescription>
        <header className="h-14 shrink-0 flex items-center gap-3 px-4 border-b border-night-border">
          <DialogIcon artifact={artifact} />
          {renaming ? (
            <InlineRenameInput
              value={nameDraft}
              onChange={setNameDraft}
              onCommit={commitRename}
              onCancel={() => {
                setNameDraft(artifact.name);
                setRenaming(false);
              }}
              className="flex-1 px-2 py-1"
            />
          ) : (
            <h2 className="flex-1 min-w-0 text-sm font-medium text-primary truncate">
              {artifact.name || <span className="italic">Untitled</span>}
            </h2>
          )}
          <VisibilityBadge
            artifact={artifact}
            busy={actions.busy === "visibility"}
            onToggle={actions.togglePublic}
          />
          <a
            href={actions.openUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg border border-night-border text-secondary hover:text-primary hover:bg-night-hover transition-colors"
          >
            <ExternalLink size={12} /> Open
          </a>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                aria-label="Actions"
                className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-secondary hover:bg-night-hover transition-colors"
              >
                <MoreVertical size={16} />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              className="w-56"
              onCloseAutoFocus={(e) => e.preventDefault()}
            >
              <ArtifactMenuItems
                artifact={artifact}
                actions={actions}
                onRename={startRename}
              />
            </DropdownMenuContent>
          </DropdownMenu>
          <button
            onClick={onClose}
            className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-secondary hover:bg-night-hover transition-colors"
          >
            <X size={16} />
          </button>
        </header>

        {actions.error && (
          <div className="shrink-0 border-b border-error/40 bg-error/5 px-4 py-2 text-xs text-error">
            {actions.error}
          </div>
        )}

        <div className="flex-1 overflow-hidden bg-night">
          {actions.isApp ? (
            artifact.app_url ? (
              <iframe
                key={artifact.id}
                src={resolveAppUrl(artifact.app_url, artifact.id)}
                className="w-full h-full bg-white"
                sandbox="allow-scripts allow-same-origin allow-popups allow-forms allow-downloads"
              />
            ) : (
              <div className="h-full flex items-center justify-center text-sm text-muted">
                App is still deploying…
              </div>
            )
          ) : (
            <div className="h-full overflow-auto terminal-scroll p-4">
              <ObjectViewer
                id={artifact.id}
                name={artifact.name}
                contentType={artifact.content_type}
                variant="panel"
              />
            </div>
          )}
        </div>
      </DialogContent>
      <ShareDialog
        artifactId={artifact.id}
        artifactName={artifact.name || "Untitled"}
        ownerId={artifact.owner_id}
        open={actions.sharing}
        onClose={() => actions.setSharing(false)}
      />
    </Dialog>
  );
}

function VisibilityBadge({
  artifact,
  busy,
  onToggle,
}: {
  artifact: ArtifactInfo;
  busy: boolean;
  onToggle: () => void;
}) {
  const isPublic = artifact.public;
  return (
    <button
      onClick={onToggle}
      disabled={busy}
      title={isPublic ? "Public — click to make private" : "Private — click to make public"}
      className={cn(
        "inline-flex items-center gap-1.5 px-2 py-1 text-[11px] rounded-md border transition-colors",
        isPublic
          ? "border-lime/40 bg-lime/10 text-lime hover:bg-lime/20"
          : "border-night-border text-muted hover:text-secondary hover:bg-night-hover",
      )}
    >
      {busy ? (
        <Loader2 size={10} className="animate-spin" />
      ) : isPublic ? (
        <Globe size={10} />
      ) : (
        <Lock size={10} />
      )}
      {isPublic ? "Public" : "Private"}
    </button>
  );
}

function DialogIcon({ artifact }: { artifact: ArtifactInfo }) {
  const cat = categoryFor(artifact);
  const Icon = CATEGORY_META[cat].icon;
  return (
    <div className="size-9 shrink-0 rounded-lg bg-night-hover flex items-center justify-center">
      <Icon size={16} className="text-muted" />
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center gap-3 py-16 text-center">
      <div className="size-12 rounded-2xl bg-night-hover flex items-center justify-center">
        <FileText size={20} className="text-muted" />
      </div>
      <div>
        <p className="text-sm font-medium text-primary">No artifacts yet</p>
        <p className="text-xs text-muted mt-1 max-w-sm">
          Start a task — anything the agent deploys or saves will show up here.
        </p>
      </div>
    </div>
  );
}

// ── Thumbnail ───────────────────────────────────────────────────
//
// Three rendering paths, all backed by real data:
//   1. images → native <img> at /api/artifacts/{id}/view (browser handles
//      lazy-load, decoding, caching).
//   2. text-y content (markdown, JSON, CSV, code, plain text) → fetch the
//      first 4KB and render it as a dim mono <pre>. Fetch is gated on
//      IntersectionObserver so off-screen cards don't fire requests.
//   3. anything else (apps, PDFs, Office docs, unknown binary) → identity
//      treatment: centered category icon. No fabricated content.

function ArtifactThumbnail({
  artifact,
  className,
}: {
  artifact: ArtifactInfo;
  className?: string;
}) {
  const cat = categoryFor(artifact);
  const Icon = CATEGORY_META[cat].icon;
  const isImage = (artifact.content_type || "").toLowerCase().startsWith("image/");
  const isText = artifact.type === "object" && isTextPreviewable(artifact.content_type);

  if (isImage) {
    return (
      <div
        className={cn(
          "relative w-full overflow-hidden bg-night-hover",
          className,
        )}
      >
        <img
          src={`/api/artifacts/${encodeURIComponent(artifact.id)}/view`}
          alt=""
          loading="lazy"
          decoding="async"
          className="absolute inset-0 w-full h-full object-cover"
        />
      </div>
    );
  }

  if (isText) {
    return (
      <div
        className={cn(
          "relative w-full overflow-hidden bg-night-hover",
          className,
        )}
      >
        <TextHeadPreview artifactId={artifact.id} />
      </div>
    );
  }

  return (
    <div
      className={cn(
        "relative w-full bg-night-hover flex items-center justify-center",
        className,
      )}
    >
      <Icon size={28} className="text-muted opacity-60" />
    </div>
  );
}

function TextHeadPreview({ artifactId }: { artifactId: string }) {
  const ref = useRef<HTMLDivElement | null>(null);
  const [text, setText] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    const controller = new AbortController();
    let started = false;
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting && !started) {
            started = true;
            observer.disconnect();
            fetchArtifactHead(artifactId, 4096, controller.signal)
              .then((t) => setText(t))
              .catch((e) => {
                if ((e as Error).name !== "AbortError") setFailed(true);
              });
            break;
          }
        }
      },
      { rootMargin: "200px" },
    );
    observer.observe(node);
    return () => {
      observer.disconnect();
      controller.abort();
    };
  }, [artifactId]);

  return (
    <div ref={ref} className="absolute inset-0">
      {text !== null ? (
        <pre className="absolute inset-0 px-3 py-2 text-[9px] leading-[1.35] font-mono text-secondary/70 whitespace-pre-wrap break-words overflow-hidden">
          {text}
        </pre>
      ) : failed ? (
        <div className="absolute inset-0 flex items-center justify-center">
          <FileText size={28} className="text-muted opacity-60" />
        </div>
      ) : null}
      <div className="absolute inset-x-0 bottom-0 h-10 bg-gradient-to-t from-night-hover to-transparent pointer-events-none" />
    </div>
  );
}

// ── Card / row ──────────────────────────────────────────────────

// Inline rename input shared by the card and the row. Stops click/keydown
// propagation so typing/clicking inside the input doesn't open the
// preview dialog the parent container is otherwise listening for.
//
// Focus ergonomics:
//   - We focus + select via a useLayoutEffect ref (not autoFocus), so we
//     control timing instead of racing Radix's menu close cycle.
//   - Blur within the first ~150ms of mount is ignored. Radix's
//     FocusScope teardown can momentarily strip focus from the
//     newly-mounted input even with onCloseAutoFocus prevented; without
//     this guard the input flashes in and immediately commits/exits.
function InlineRenameInput({
  value,
  onChange,
  onCommit,
  onCancel,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  onCommit: () => void | Promise<void>;
  onCancel: () => void;
  className?: string;
}) {
  const ref = useRef<HTMLInputElement>(null);
  const mountedAt = useRef(0);
  useEffect(() => {
    mountedAt.current = Date.now();
    const id = requestAnimationFrame(() => {
      ref.current?.focus();
      ref.current?.select();
    });
    return () => cancelAnimationFrame(id);
  }, []);
  return (
    <input
      ref={ref}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => {
        e.stopPropagation();
        if (e.key === "Enter") onCommit();
        else if (e.key === "Escape") onCancel();
      }}
      onBlur={() => {
        if (Date.now() - mountedAt.current < 150) return;
        onCommit();
      }}
      className={cn(
        "min-w-0 px-1.5 py-0.5 text-sm rounded border border-night-border bg-night text-primary focus:outline-none focus:border-lime/40 font-medium",
        className,
      )}
    />
  );
}

function CardActionsMenu({
  artifact,
  actions,
  onRename,
}: {
  artifact: ArtifactInfo;
  actions: ArtifactActions;
  onRename: () => void;
}) {
  return (
    <div
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            aria-label="Actions"
            className="size-7 flex items-center justify-center rounded-md bg-night/85 backdrop-blur border border-night-border text-muted hover:text-primary opacity-0 group-hover:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100 transition-opacity"
          >
            <MoreVertical size={14} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          className="w-56"
          // Don't yank focus back to the three-dots trigger on close —
          // when the user picks Rename, the input mounts and grabs focus
          // via autoFocus; without this preventDefault, Radix steals it
          // back, the blur fires, and rename exits before the user can
          // type a character.
          onCloseAutoFocus={(e) => e.preventDefault()}
        >
          <ArtifactMenuItems
            artifact={artifact}
            actions={actions}
            onRename={onRename}
          />
        </DropdownMenuContent>
      </DropdownMenu>
      <ShareDialog
        artifactId={artifact.id}
        artifactName={artifact.name || "Untitled"}
        ownerId={artifact.owner_id}
        open={actions.sharing}
        onClose={() => actions.setSharing(false)}
      />
    </div>
  );
}

function ArtifactCard({
  artifact,
  onOpen,
  onChanged,
  onDeleted,
}: {
  artifact: ArtifactInfo;
  onOpen: () => void;
  onChanged: () => Promise<void> | void;
  onDeleted: () => Promise<void> | void;
}) {
  const cat = categoryFor(artifact);
  const meta = CATEGORY_META[cat];
  const isApp = artifact.type === "app";
  const actions = useArtifactActions(artifact, onChanged, onDeleted);
  const [renaming, setRenaming] = useState(false);
  const [nameDraft, setNameDraft] = useState(artifact.name);

  const startRename = () => {
    setNameDraft(artifact.name);
    setRenaming(true);
  };
  const commitRename = async () => {
    if (!renaming) return;
    setRenaming(false);
    await actions.rename(nameDraft);
  };
  const cancelRename = () => {
    setNameDraft(artifact.name);
    setRenaming(false);
  };

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => { if (!renaming) onOpen(); }}
      onKeyDown={(e) => {
        if (renaming) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      className="group relative flex flex-col text-left rounded-xl border border-night-border hover:bg-night-hover/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-lime/40 transition-colors overflow-hidden cursor-pointer"
    >
      <div className="relative">
        <ArtifactThumbnail artifact={artifact} className="aspect-[16/10]" />
        <div className="absolute top-2 right-2">
          <CardActionsMenu
            artifact={artifact}
            actions={actions}
            onRename={startRename}
          />
        </div>
        {artifact.public && (
          <span
            title="Public — anyone with the link can view"
            className="absolute bottom-2 right-2 inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium rounded-md bg-lime/15 backdrop-blur border border-lime/40 text-lime"
          >
            <Globe size={10} />
            Public
          </span>
        )}
      </div>
      <div className="px-3 py-2.5 border-t border-night-border/60">
        <div className="flex items-center gap-1.5 min-w-0">
          {renaming ? (
            <InlineRenameInput
              value={nameDraft}
              onChange={setNameDraft}
              onCommit={commitRename}
              onCancel={cancelRename}
              className="flex-1"
            />
          ) : (
            <p className="text-sm font-medium text-primary truncate">
              {artifact.name || <span className="italic">Untitled</span>}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2 mt-1 text-[11px] text-muted">
          <span className="inline-flex items-center gap-1">
            <meta.icon size={11} />
            {meta.label}
          </span>
          <span className="opacity-40">·</span>
          <span className="tabular-nums">
            {new Date(artifact.created_at).toLocaleDateString()}
          </span>
          {!isApp && artifact.size_bytes > 0 && (
            <>
              <span className="opacity-40">·</span>
              <span className="tabular-nums">
                {formatArtifactSize(artifact.size_bytes)}
              </span>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function ArtifactRow({
  artifact,
  onOpen,
  onChanged,
  onDeleted,
}: {
  artifact: ArtifactInfo;
  onOpen: () => void;
  onChanged: () => Promise<void> | void;
  onDeleted: () => Promise<void> | void;
}) {
  const cat = categoryFor(artifact);
  const meta = CATEGORY_META[cat];
  const isApp = artifact.type === "app";
  const actions = useArtifactActions(artifact, onChanged, onDeleted);
  const [renaming, setRenaming] = useState(false);
  const [nameDraft, setNameDraft] = useState(artifact.name);

  const startRename = () => {
    setNameDraft(artifact.name);
    setRenaming(true);
  };
  const commitRename = async () => {
    if (!renaming) return;
    setRenaming(false);
    await actions.rename(nameDraft);
  };
  const cancelRename = () => {
    setNameDraft(artifact.name);
    setRenaming(false);
  };

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => { if (!renaming) onOpen(); }}
      onKeyDown={(e) => {
        if (renaming) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      className="group flex items-center gap-4 w-full px-3 py-2 text-left hover:bg-night-hover focus-visible:outline-none focus-visible:bg-night-hover transition-colors cursor-pointer"
    >
      <div className="size-9 shrink-0 rounded-md overflow-hidden border border-night-border/50">
        <ArtifactThumbnail artifact={artifact} className="h-full" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          {renaming ? (
            <InlineRenameInput
              value={nameDraft}
              onChange={setNameDraft}
              onCommit={commitRename}
              onCancel={cancelRename}
              className="flex-1 max-w-sm"
            />
          ) : (
            <>
              <p className="text-sm font-medium text-primary truncate">
                {artifact.name || <span className="italic">Untitled</span>}
              </p>
              {artifact.public && (
                <span
                  title="Public"
                  className="size-1.5 rounded-full bg-lime shrink-0"
                />
              )}
            </>
          )}
        </div>
        {artifact.description && !renaming && (
          <p className="text-xs text-muted truncate mt-0.5">
            {artifact.description}
          </p>
        )}
      </div>
      <span className="hidden sm:inline-flex items-center gap-1 text-xs text-muted shrink-0 w-20">
        <meta.icon size={11} />
        {meta.label}
      </span>
      <span className="hidden md:inline text-xs text-muted tabular-nums shrink-0 w-24 text-right">
        {new Date(artifact.created_at).toLocaleTimeString("en-US", {
          hour: "numeric",
          minute: "2-digit",
        })}
      </span>
      <span className="hidden md:inline text-xs text-muted tabular-nums shrink-0 w-16 text-right">
        {!isApp && artifact.size_bytes > 0
          ? formatArtifactSize(artifact.size_bytes)
          : ""}
      </span>
      <CardActionsMenu
        artifact={artifact}
        actions={actions}
        onRename={startRename}
      />
    </div>
  );
}
