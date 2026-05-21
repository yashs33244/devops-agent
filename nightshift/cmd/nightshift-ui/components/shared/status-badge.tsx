"use client";

import { cn } from "@/lib/utils";

const STATUS: Record<string, { text: string; bg: string }> = {
  pending:     { text: "text-blue-400",   bg: "bg-blue-400/10" },
  running:     { text: "text-lime",       bg: "bg-lime/10" },
  completed:   { text: "text-success",    bg: "bg-success/10" },
  error:       { text: "text-error",      bg: "bg-error/10" },
  interrupted: { text: "text-yellow-400", bg: "bg-yellow-400/10" },
};

type StatusBadgeProps = {
  status: string;
  label?: string;
  className?: string;
};

export function StatusBadge({ status, label, className }: StatusBadgeProps) {
  const s = STATUS[status];
  return (
    <span
      data-slot="status-badge"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium capitalize",
        s?.text,
        s?.bg,
        className,
      )}
    >
      {status === "running" && (
        <span className="size-1.5 rounded-full bg-current animate-pulse" />
      )}
      {label ?? status}
    </span>
  );
}
