import { cn } from "@/lib/utils";

const AGENT_COLORS: Record<string, string> = {
  purchasing: "bg-blue-500/15 text-blue-400",
  accounting: "bg-amber-500/15 text-amber-400",
  warehouse: "bg-emerald-500/15 text-emerald-400",
};

export function AgentBadge({
  agent,
  className,
}: {
  agent: string | null | undefined;
  className?: string;
}) {
  if (!agent) return null;
  const colors = AGENT_COLORS[agent] ?? "bg-white/10 text-muted";
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none capitalize",
        colors,
        className,
      )}
    >
      {agent}
    </span>
  );
}
