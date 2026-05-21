"use client";

import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";

// Mascot family registry. Each family is a list of filenames under
// src/ui/public/animations/ which Next.js serves at /animations/*.
//
// Add a new family by:
//   1. Dropping its SVGs into public/animations/ (with a unique prefix)
//   2. Adding the entry here
//   3. Setting CR0N_MASCOT to the family key
//
// Currently only "clawd" has assets. "gemini" and "codex" are valid env
// values reserved for future asset sets — they render nothing for now.
//
// Clawd assets sourced from https://github.com/marciogranzotto/clawd-tank
// with explicit permission from the author — see public/animations/NOTICE.md.
const MASCOT_FAMILIES: Record<string, readonly string[]> = {
  clawd: [
    "clawd-crab-walking.svg",
    "clawd-dizzy.svg",
    "clawd-going-away.svg",
    "clawd-happy.svg",
    "clawd-idle-living.svg",
    "clawd-mini-clawd.svg",
    "clawd-notification.svg",
    "clawd-sleeping.svg",
    "clawd-working-beacon.svg",
    "clawd-working-building.svg",
    "clawd-working-carrying.svg",
    "clawd-working-conducting.svg",
    "clawd-working-confused.svg",
    "clawd-working-debugger.svg",
    "clawd-working-juggling.svg",
    "clawd-working-overheated.svg",
    "clawd-working-pushing.svg",
    "clawd-working-sweeping.svg",
    "clawd-working-thinking.svg",
    "clawd-working-typing.svg",
    "clawd-working-wizard.svg",
    "mini-crab-typing.svg",
  ],
  gemini: [],
  codex: [],
};

// Read at module load — Next.js inlines `process.env.CR0N_MASCOT` at build
// time via the `env` block in next.config.mjs. Empty/unset/unknown → no
// mascot rendered.
const ACTIVE_FAMILY = (process.env.CR0N_MASCOT ?? "").trim().toLowerCase();
const ACTIVE_MANIFEST: readonly string[] = MASCOT_FAMILIES[ACTIVE_FAMILY] ?? [];

function pickRandom(): string | null {
  if (ACTIVE_MANIFEST.length === 0) return null;
  const idx = Math.floor(Math.random() * ACTIVE_MANIFEST.length);
  return ACTIVE_MANIFEST[idx] ?? ACTIVE_MANIFEST[0]!;
}

/**
 * Tiny animated mascot that hovers just above the right-hand prompt input.
 * Picks a random animation on mount; rerolls whenever `seed` changes
 * (pass the conversationId from a follow-up ChatInput so each task gets
 * its own mascot).
 *
 * The parent MUST be `position: relative` so this absolute-positioned tile
 * anchors to the prompt input's top-right corner.
 *
 * Hydration note: the pick is deferred to a client-side useEffect after
 * mount, so the server renders nothing and the client fills it in. This
 * avoids an SSR/client mismatch that would otherwise fire on every render
 * because Math.random() returns different values server-side and
 * client-side.
 */
export function PromptMascot({
  seed,
  className,
}: {
  seed?: string | null;
  className?: string;
}) {
  const [file, setFile] = useState<string | null>(null);
  useEffect(() => {
    setFile(pickRandom());
  }, [seed]);

  // No active family (env var unset, unknown, or family has no assets) →
  // render nothing. The early return also covers the SSR/first-paint window
  // where `file` hasn't been picked yet.
  if (!file) return null;

  return (
    <div
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute bottom-[calc(100%+4px)] right-6 z-10 select-none",
        className,
      )}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={`/animations/${file}`}
        alt=""
        width={128}
        height={128}
        className="size-32 drop-shadow-[0_6px_12px_rgba(0,0,0,0.55)]"
        draggable={false}
      />
    </div>
  );
}
