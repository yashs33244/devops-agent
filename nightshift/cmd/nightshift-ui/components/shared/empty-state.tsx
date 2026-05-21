"use client";

import { cn } from "@/lib/utils";

type EmptyStateProps = {
  icon?: React.ReactNode;
  message: string;
  action?: React.ReactNode;
  className?: string;
};

export function EmptyState({ icon, message, action, className }: EmptyStateProps) {
  return (
    <div
      data-slot="empty-state"
      className={cn(
        "flex flex-col items-center justify-center gap-3 py-16 text-center",
        className,
      )}
    >
      {icon && <div className="text-muted">{icon}</div>}
      <p className="text-sm text-muted">{message}</p>
      {action}
    </div>
  );
}
