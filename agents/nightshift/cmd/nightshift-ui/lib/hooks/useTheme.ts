"use client";

import { useState, useCallback, useEffect } from "react";

const THEME_IDS = ["light", "dark"] as const;

export type ThemeId = (typeof THEME_IDS)[number];

const STORAGE_KEY = "nightshift-theme";
const DEFAULT_THEME: ThemeId = "light";
const VALID: ReadonlySet<string> = new Set<string>(THEME_IDS);

export function useTheme(): { theme: ThemeId; setTheme: (id: ThemeId) => void } {
  const [theme, setThemeState] = useState<ThemeId>(DEFAULT_THEME);

  // Hydrate from localStorage after mount to avoid SSR mismatch
  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (saved && VALID.has(saved)) setThemeState(saved as ThemeId);
  }, []);

  const setTheme = useCallback((id: ThemeId) => {
    setThemeState(id);
    localStorage.setItem(STORAGE_KEY, id);
  }, []);

  return { theme, setTheme };
}
