"use client";

import { useEffect, useRef, useState } from "react";
import { Paperclip, X } from "lucide-react";

// gRPC server caps messages at 64 MiB (internal/server/grpc.go); base64
// inflates the wire payload ~33%, so a 25 MB raw file lands at ~33 MB at
// the gateway and 25 MB at the handler — both under the ceiling.
export const MAX_FILE_BYTES = 25 * 1024 * 1024;

export type StagedFile = { id: string; file: File };

export function useFileStaging(resetKey?: unknown) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [staged, setStaged] = useState<StagedFile[]>([]);
  const [stagingError, setStagingError] = useState<string>("");

  useEffect(() => {
    setStaged([]);
    setStagingError("");
  }, [resetKey]);

  const stageFiles = (files: FileList | null) => {
    if (!files) return;
    setStagingError("");
    const next: StagedFile[] = [];
    for (const f of Array.from(files)) {
      if (f.size > MAX_FILE_BYTES) {
        setStagingError(
          `"${f.name}" is ${(f.size / (1024 * 1024)).toFixed(1)} MB — over the 25 MB upload cap. Try a smaller file or split it.`,
        );
        continue;
      }
      next.push({ id: crypto.randomUUID(), file: f });
    }
    if (next.length) setStaged((cur) => [...next, ...cur]);
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  const removeStaged = (id: string) => {
    setStaged((cur) => cur.filter((s) => s.id !== id));
  };

  const submit = async (
    message: { text?: string },
    onSend: (text: string, files: File[]) => void | Promise<void>,
  ) => {
    const text = message.text?.trim();
    if (!text) return;
    setStagingError("");
    try {
      await onSend(text, staged.map((s) => s.file));
      setStaged([]);
    } catch (e) {
      setStagingError(e instanceof Error ? e.message : "Send failed — try again.");
    }
  };

  return {
    fileInputRef,
    staged,
    stagingError,
    stageFiles,
    removeStaged,
    submit,
  };
}

export function AttachmentChip({
  name,
  onRemove,
}: {
  name: string;
  onRemove?: () => void;
}) {
  return (
    <span
      className={
        "inline-flex items-center gap-1.5 rounded-full bg-night-surface border border-night-border py-0.5 text-xs text-secondary max-w-[260px] " +
        (onRemove ? "pl-3 pr-1" : "px-3")
      }
      title={name}
    >
      <Paperclip size={12} className="shrink-0" />
      <span className="truncate">{name}</span>
      {onRemove && (
        <button
          type="button"
          aria-label={`Remove ${name}`}
          onClick={onRemove}
          className="size-5 flex items-center justify-center rounded text-muted hover:text-error hover:bg-error/10"
        >
          <X size={12} />
        </button>
      )}
    </span>
  );
}

export function StagedChips({
  staged,
  stagingError,
  onRemove,
}: {
  staged: StagedFile[];
  stagingError: string;
  onRemove: (id: string) => void;
}) {
  if (!staged.length && !stagingError) return null;
  return (
    <div className="mb-2 flex flex-wrap items-center gap-2">
      {staged.map((s) => (
        <AttachmentChip key={s.id} name={s.file.name} onRemove={() => onRemove(s.id)} />
      ))}
      {stagingError && (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-error/10 border border-error/30 px-3 py-1 text-xs text-error max-w-[360px]">
          <X size={12} className="shrink-0" />
          <span className="truncate">{stagingError}</span>
        </span>
      )}
    </div>
  );
}

export function HiddenFileInput({
  inputRef,
  onPick,
}: {
  inputRef: React.RefObject<HTMLInputElement | null>;
  onPick: (files: FileList | null) => void;
}) {
  return (
    <input
      ref={inputRef}
      type="file"
      multiple
      className="hidden"
      onChange={(e) => onPick(e.target.files)}
    />
  );
}
