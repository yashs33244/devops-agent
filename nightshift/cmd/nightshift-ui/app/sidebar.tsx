"use client";

import { useState, useEffect, memo } from "react";
import Link from "next/link";
import Image from "next/image";
import { usePathname, useRouter } from "next/navigation";
import { cn, getInitials } from "@/lib/utils";
import { TasksProvider, useTasksActions } from "@/lib/hooks/useTasks";
import { ChatProvider, useChatState } from "@/lib/hooks/useChat";
import { TaskPanelProvider, useTaskPanel } from "@/lib/hooks/useTaskPanel";
import { useTheme, type ThemeId } from "@/lib/hooks/useTheme";
import {
  LogOut, Menu, Loader2, Sparkles,
  MessageSquare, CalendarClock, FileText, Settings, Bot, Zap,
  ChevronLeft, ChevronRight, ChevronsRight, Sun, Moon,
} from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

type UserInfo = { name: string; email: string; orgName: string; role?: string };

type SidebarProps = {
  user: UserInfo;
  theme: ThemeId;
  setTheme: (id: ThemeId) => void;
  collapsed: boolean;
  onToggle: () => void;
  className?: string;
};

// ── User-footer actions ──────────────────────────────────────────

function UserFooterActions({ theme, setTheme }: { theme: ThemeId; setTheme: (id: ThemeId) => void }) {
  const isLight = theme === "light";
  const toggleTheme = () => setTheme(isLight ? "dark" : "light");
  return (
    <div className="flex items-center gap-0.5 shrink-0">
      <button
        onClick={toggleTheme}
        aria-label={isLight ? "Switch to dark mode" : "Switch to light mode"}
        title={isLight ? "Dark mode" : "Light mode"}
        className="size-7 flex items-center justify-center rounded text-secondary hover:text-primary hover:bg-night-hover transition-colors"
      >
        {isLight ? <Moon size={14} /> : <Sun size={14} />}
      </button>
      <form action="/api/logout" method="POST">
        <button
          type="submit"
          aria-label="Log out"
          title="Log out"
          className="size-7 flex items-center justify-center rounded text-secondary hover:text-primary hover:bg-night-hover transition-colors"
        >
          <LogOut size={14} />
        </button>
      </form>
    </div>
  );
}

// ── Nav items ───────────────────────────────────────────────────

const NAV_ITEMS = [
  { href: "/tasks", label: "Tasks", icon: MessageSquare },
  { href: "/schedule", label: "Schedule", icon: CalendarClock },
  { href: "/artifacts", label: "Artifacts", icon: FileText },
  { href: "/skills", label: "Skills", icon: Zap },
  { href: "/agents", label: "Agents", icon: Bot },
  { href: "/connectors", label: "Connectors", icon: Settings },
] as const;

// ── Sidebar ──────────────────────────────────────────────────────

function TaskRouteSync() {
  const { refetch } = useTasksActions();
  const pathname = usePathname();
  const section = pathname.split("/").slice(0, 3).join("/");
  useEffect(() => { refetch(); }, [section, refetch]);
  return null;
}

export const Sidebar = memo(function Sidebar({
  user,
  theme,
  setTheme,
  collapsed,
  onToggle,
  className,
}: SidebarProps) {
  const pathname = usePathname();

  return (
    <TooltipProvider delayDuration={0}>
      <aside
        className={cn(
          "group/sidebar theme-scope-dark shrink-0 bg-night-surface border-r border-night-border flex-col h-dvh transition-[width] duration-200 ease-out",
          collapsed ? "w-16" : "w-64",
          className,
        )}
      >
        {/* Logo + collapse toggle (chevron pinned to top-right of sidebar in both states) */}
        <div className={cn("shrink-0 relative flex items-center justify-center pt-5 pb-4", collapsed ? "px-2" : "px-3")}>
          <button
            onClick={onToggle}
            className={cn(
              "size-6 shrink-0 flex items-center justify-center rounded text-secondary hover:text-primary hover:bg-night-hover transition-colors",
              collapsed
                ? ""
                : "absolute right-3 top-1/2 -translate-y-1/2 opacity-0 group-hover/sidebar:opacity-100 transition-opacity",
            )}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {collapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
          </button>
          {!collapsed && (
            <Link href="/tasks" className="flex items-center justify-center">
              <Image
                src="/nightshift-text.png"
                alt="Nightshift"
                width={512}
                height={96}
                priority
                className="nightshift-logo-dark h-10 w-auto"
              />
              <Image
                src="/nightshift-text-black.png"
                alt="Nightshift"
                width={512}
                height={96}
                priority
                className="nightshift-logo-light h-10 w-auto"
              />
            </Link>
          )}
        </div>

        {/* Navigation — static menu items at the top. */}
        <nav className={cn("shrink-0 space-y-0.5", collapsed ? "px-2 pb-3" : "px-3 pb-3")}>
          {NAV_ITEMS.map((item) => {
            const active = pathname.startsWith(item.href);
            const Icon = item.icon;
            const isTasks = item.href === "/tasks";
            const link = (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-3 text-[15px] font-medium focus-ring rounded-md transition-colors",
                  collapsed ? "justify-center px-2 py-2.5" : "px-2.5 py-2.5",
                  active ? "text-lime bg-night-hover" : "text-secondary hover:text-primary hover:bg-night-hover",
                )}
              >
                <Icon size={17} className="opacity-80 shrink-0" />
                {!collapsed && (
                  <span className="flex-1 truncate">{item.label}</span>
                )}
                {!collapsed && isTasks && <TaskPanelExpandButton />}
              </Link>
            );

            if (collapsed) {
              return (
                <Tooltip key={item.href}>
                  <TooltipTrigger asChild>{link}</TooltipTrigger>
                  <TooltipContent side="right" sideOffset={8}>{item.label}</TooltipContent>
                </Tooltip>
              );
            }
            return link;
          })}
        </nav>

        {/* Below the static menu: docked task list when the panel is
            collapsed (and the sidebar itself is expanded), otherwise an
            empty spacer that pushes the user footer to the bottom. */}
        <DockedTasksRegion sidebarCollapsed={collapsed} />

        {/* User footer */}
        <div className={cn("border-t border-night-border shrink-0", collapsed ? "px-2 py-3" : "px-3 py-4")}>
          {collapsed ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="flex justify-center">
                  <div className="size-9 shrink-0 rounded-full bg-lime/10 flex items-center justify-center">
                    <span className="text-xs font-semibold text-lime">{getInitials(user.name)}</span>
                  </div>
                </div>
              </TooltipTrigger>
              <TooltipContent side="right" sideOffset={8}>
                <p className="font-medium">{user.name}</p>
                <p className="text-muted-foreground">{user.orgName}</p>
              </TooltipContent>
            </Tooltip>
          ) : (
            <div className="flex items-center gap-3 px-1">
              <div className="size-10 shrink-0 rounded-full bg-lime/10 flex items-center justify-center">
                <span className="text-sm font-semibold text-lime">{getInitials(user.name)}</span>
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-[15px] font-medium text-primary truncate">{user.name}</p>
                <p className="text-[13px] text-secondary truncate">{user.orgName}</p>
              </div>
              <UserFooterActions theme={theme} setTheme={setTheme} />
            </div>
          )}
        </div>
      </aside>
    </TooltipProvider>
  );
});

// ── Docked task list ────────────────────────────────────────────
//
// When the user collapses the Tasks panel (chevron in chat-layout's
// TaskListPanel header), the conversation list "zaps" into the main
// sidebar, nested directly under the Tasks nav row. Clicking the
// expand chevron there pops it back out into the panel.

function TaskPanelExpandButton() {
  const { collapsed, setCollapsed } = useTaskPanel();
  if (!collapsed) return null;
  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        setCollapsed(false);
      }}
      aria-label="Show tasks panel"
      title="Show tasks panel"
      className="size-5 ml-auto flex items-center justify-center rounded text-muted hover:text-primary hover:bg-night-active transition-colors shrink-0"
    >
      <ChevronsRight size={13} />
    </button>
  );
}

function DockedTasksRegion({ sidebarCollapsed }: { sidebarCollapsed: boolean }) {
  const { collapsed: panelCollapsed } = useTaskPanel();
  // Only dock conversations when the task panel is collapsed AND the
  // main sidebar is expanded — there is no useful icon-only treatment
  // for a list of conversations. Otherwise this region acts as the
  // flex-1 spacer that pushes the user footer to the bottom.
  const showDocked = panelCollapsed && !sidebarCollapsed;
  if (!showDocked) return <div className="flex-1" />;
  return <DockedConversationList />;
}

function DockedConversationList() {
  const { conversations, runningConvs } = useChatState();
  const pathname = usePathname();
  return (
    <div className="flex-1 min-h-0 overflow-y-auto terminal-scroll px-3 pb-3">
      {conversations.length === 0 ? (
        <p className="px-2 py-2 text-[12px] text-muted">No tasks yet.</p>
      ) : (
        <ul className="space-y-0.5">
          {conversations.map((conv) => {
            const href = `/tasks/${conv.id}`;
            const isActive = pathname === href;
            const isRunning = runningConvs.has(conv.id);
            return (
              <li key={conv.id}>
                <Link
                  href={href}
                  className={cn(
                    "flex items-center gap-2 px-2 py-1.5 rounded text-[13px] transition-colors",
                    isActive
                      ? "text-primary bg-night-hover"
                      : "text-secondary hover:text-primary hover:bg-night-hover",
                  )}
                >
                  {isRunning ? (
                    <Loader2 size={11} className="text-success shrink-0 animate-spin" />
                  ) : (
                    <Sparkles size={11} className="text-secondary/60 shrink-0" />
                  )}
                  <span className="truncate flex-1">{conv.title}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

// ── Dashboard Shell ──────────────────────────────────────────────

const SIDEBAR_KEY = "nightshift-sidebar-collapsed";

export function DashboardShell({
  children,
  user,
}: {
  children: React.ReactNode;
  user: UserInfo;
}) {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const pathname = usePathname();
  const router = useRouter();

  // Load collapsed state from localStorage
  useEffect(() => {
    const saved = localStorage.getItem(SIDEBAR_KEY);
    if (saved === "true") setCollapsed(true);
  }, []);

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem(SIDEBAR_KEY, String(next));
      return next;
    });
  };

  useEffect(() => {
    setDrawerOpen(false);
    if (pathname === "/") router.replace("/tasks");
  }, [pathname, router]);

  useEffect(() => {
    if (!drawerOpen) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setDrawerOpen(false); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [drawerOpen]);

  const { theme, setTheme } = useTheme();

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  return (
    <ChatProvider>
    <TasksProvider userName={user.name}>
    <TaskPanelProvider>
      <TaskRouteSync />
      <div className="flex flex-col md:flex-row h-dvh">
        <header className="theme-scope-dark md:hidden flex items-center gap-3 px-4 h-14 border-b border-night-border bg-night-surface shrink-0">
          <button onClick={() => setDrawerOpen(true)} className="size-10 flex items-center justify-center text-secondary" aria-label="Open menu">
            <Menu size={20} />
          </button>
          <Image
            src="/nightshift-text.png"
            alt="Nightshift"
            width={512}
            height={96}
            priority
            className="nightshift-logo-dark h-8 w-auto"
          />
          <Image
            src="/nightshift-text-black.png"
            alt="Nightshift"
            width={512}
            height={96}
            priority
            className="nightshift-logo-light h-8 w-auto"
          />
        </header>

        {drawerOpen && (
          <div className="fixed inset-0 z-40 bg-black/60 md:hidden" onClick={() => setDrawerOpen(false)} />
        )}

        <Sidebar
          user={user}
          theme={theme}
          setTheme={setTheme}
          collapsed={collapsed}
          onToggle={toggleCollapsed}
          className={cn(
            drawerOpen ? "flex fixed inset-y-0 left-0 z-50" : "hidden",
            "md:flex md:relative md:z-auto",
          )}
        />

        <main className="flex-1 flex flex-col min-w-0 min-h-0 bg-night">
          {children}
        </main>
      </div>
    </TaskPanelProvider>
    </TasksProvider>
    </ChatProvider>
  );
}
