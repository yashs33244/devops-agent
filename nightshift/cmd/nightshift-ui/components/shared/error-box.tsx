"use client";

import { AlertTriangle } from "lucide-react";
import { cn } from "@/lib/utils";

type ErrorBoxProps = {
  children: React.ReactNode;
  className?: string;
};

export function ErrorBox({ children, className }: ErrorBoxProps) {
  return (
    <div
      data-slot="error-box"
      role="alert"
      className={cn(
        "rounded-lg border border-error/30 bg-error/10 px-4 py-2 text-sm text-error",
        className,
      )}
    >
      <AlertTriangle size={14} className="inline mr-1.5 -mt-0.5" />
      {children}
    </div>
  );
}
