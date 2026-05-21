"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

const STORAGE_KEY = "nightshift-task-panel-collapsed";

type TaskPanelCtx = {
  collapsed: boolean;
  setCollapsed: (next: boolean) => void;
  toggle: () => void;
};

const Ctx = createContext<TaskPanelCtx | null>(null);

export function TaskPanelProvider({ children }: { children: ReactNode }) {
  const [collapsed, setCollapsedState] = useState(false);

  // Hydrate from localStorage after mount to avoid SSR mismatch.
  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (saved === "true") setCollapsedState(true);
  }, []);

  const setCollapsed = useCallback((next: boolean) => {
    setCollapsedState(next);
    try { localStorage.setItem(STORAGE_KEY, String(next)); } catch {}
  }, []);

  const toggle = useCallback(() => {
    setCollapsedState((prev) => {
      const next = !prev;
      try { localStorage.setItem(STORAGE_KEY, String(next)); } catch {}
      return next;
    });
  }, []);

  const value = useMemo(
    () => ({ collapsed, setCollapsed, toggle }),
    [collapsed, setCollapsed, toggle],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useTaskPanel(): TaskPanelCtx {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useTaskPanel must be used within TaskPanelProvider");
  return ctx;
}
