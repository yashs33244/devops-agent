"use client";

import { Globe, FileText, ExternalLink, X } from "lucide-react";
import { ObjectViewer } from "@/components/chat/object-viewer";
import { resolveAppUrl } from "@/lib/artifacts";

export type ArtifactView =
  | { kind: "app"; id: string; name: string; url: string }
  | { kind: "object"; id: string; name: string; contentType: string };

export function ArtifactPanel({
  artifact,
  onClose,
}: {
  artifact: ArtifactView;
  onClose: () => void;
}) {
  const openUrl =
    artifact.kind === "app"
      ? resolveAppUrl(artifact.url, artifact.id)
      : `/api/artifacts/${artifact.id}/view`;
  const HeaderIcon = artifact.kind === "app" ? Globe : FileText;

  return (
    <div className="flex flex-col h-full border-l border-night-border bg-night">
      {/* Header */}
      <div className="h-12 flex items-center gap-2 px-4 border-b border-night-border shrink-0">
        <HeaderIcon size={14} className="text-muted shrink-0" />
        <span className="text-sm font-medium text-primary truncate flex-1">
          {artifact.name}
        </span>
        {artifact.kind === "object" && (
          <span className="text-[10px] text-muted/60 shrink-0">
            {artifact.contentType}
          </span>
        )}
        <a
          href={openUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-night-border text-xs text-muted hover:text-secondary transition-colors"
        >
          <ExternalLink size={12} />
          Open
        </a>
        <button
          onClick={onClose}
          className="size-7 flex items-center justify-center rounded text-muted hover:text-primary hover:bg-night-hover transition-colors"
          title="Close"
        >
          <X size={14} />
        </button>
      </div>

      {/* Body — iframe for apps, ObjectViewer for objects */}
      <div className="flex-1 min-h-0">
        {artifact.kind === "app" ? (
          <iframe
            src={resolveAppUrl(artifact.url, artifact.id)}
            title={artifact.name}
            className="w-full h-full border-0"
            sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
          />
        ) : (
          <ObjectViewer
            id={artifact.id}
            name={artifact.name}
            contentType={artifact.contentType}
            variant="panel"
          />
        )}
      </div>
    </div>
  );
}
