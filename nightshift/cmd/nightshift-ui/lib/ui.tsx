import Image from "next/image";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

export function LoadingCenter({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center justify-center", className ?? "flex-1")}>
      <Loader2 size={20} className="animate-spin text-muted" />
    </div>
  );
}

export function CenteredMessage({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex-1 flex items-center justify-center text-muted">
      {children}
    </div>
  );
}

const STATUS: Record<string, { text: string; badge: string; dot: string; border: string }> = {
  pending:     { text: "text-blue-400",   badge: "text-blue-400 bg-blue-400/10",     dot: "bg-blue-400/10",     border: "border-l-blue-400" },
  running:     { text: "text-lime",       badge: "text-lime bg-lime/10",             dot: "bg-lime/10",         border: "border-l-lime" },
  completed:   { text: "text-success",    badge: "text-success bg-success/10",       dot: "bg-success/10",      border: "border-l-success" },
  error:       { text: "text-error",      badge: "text-error bg-error/10",           dot: "bg-error/10",        border: "border-l-error" },
  interrupted: { text: "text-yellow-400", badge: "text-yellow-400 bg-yellow-400/10", dot: "bg-yellow-400/10",   border: "border-l-yellow-400" },
};

export const statusTextClass = (s: string) => STATUS[s]?.text ?? "";
export const statusBadgeClass = (s: string) => STATUS[s]?.badge ?? "";
export const statusBorderClass = (s: string) => STATUS[s]?.border ?? "border-l-white/20";
export const statusDotColor = (s: string) => STATUS[s]?.dot ?? "bg-white/20";

export function diffLineColor(line: string) {
  if (line.startsWith("+")) return "text-success";
  if (line.startsWith("-")) return "text-error";
  return "text-muted";
}

export function PageHeader({ children }: { children: React.ReactNode }) {
  return (
    <header className="h-12 flex items-center gap-3 px-6 border-b border-night-border shrink-0">
      {children}
    </header>
  );
}

export function PanelHeader({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-12 flex items-center justify-between px-4 border-b border-night-border shrink-0">
      {children}
    </div>
  );
}

export function ShortcutBar({ children }: { children: React.ReactNode }) {
  return (
    <footer className="hidden md:flex items-center gap-4 px-5 py-3 border-t border-night-border bg-night-surface shrink-0 font-mono text-sm">
      {children}
      <span className="ml-auto flex items-center opacity-40">
        <Image
          src="/nightshift-text.png"
          alt="Nightshift"
          width={512}
          height={96}
          className="nightshift-logo-dark h-5 w-auto"
        />
        <Image
          src="/nightshift-text-black.png"
          alt="Nightshift"
          width={512}
          height={96}
          className="nightshift-logo-light h-5 w-auto"
        />
      </span>
    </footer>
  );
}

export function Shortcut({ k, label }: { k: string; label: string }) {
  return (
    <span className="text-muted">
      <kbd className="inline-block min-w-[1.5rem] text-center rounded border border-night-border bg-night-hover px-1.5 py-0.5 text-xs text-primary font-medium">
        {k}
      </kbd>
      <span className="ml-1.5">{label}</span>
    </span>
  );
}
