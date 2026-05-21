"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Plus, Search, X, Zap } from "lucide-react";
import { cn, timeAgo } from "@/lib/utils";
import { PanelHeader } from "@/lib/ui";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  createSkill,
  deleteSkill,
  listSkills,
  updateSkill,
  type SkillInfo,
} from "@/lib/api";

const NAME_RE = /^[a-z0-9-]+$/;
const NAME_HINT = "Lowercase letters, digits, and hyphens only";

const PLACEHOLDER_BODY = `# My skill

Write the skill body in Markdown.
`;

// YAML frontmatter block at the top of SKILL.md. Matches an opening `---`
// on its own line, any body, a closing `---` on its own line, and the
// blank line that typically follows.
const FRONTMATTER_RE = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?\r?\n?/;

function stripFrontmatter(content: string): string {
  return content.replace(FRONTMATTER_RE, "");
}

/**
 * Build the full SKILL.md: a frontmatter block derived from the form's
 * name/description, followed by the user-authored body. Keeps the two
 * fields authoritative — the body textarea never owns those keys.
 */
function composeSkillContent(
  name: string,
  description: string,
  body: string,
): string {
  const safeDesc = description.replace(/\r?\n/g, " ").trim();
  const trimmedBody = body.replace(/^\s+/, "");
  return `---\nname: ${name}\ndescription: ${safeDesc}\n---\n\n${trimmedBody}`;
}

type DialogMode = "create" | "view" | "edit";

export default function SkillsPage() {
  const [skills, setSkills] = useState<SkillInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const list = await listSkills();
      list.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
      setSkills(list);
    } catch (e) {
      setError((e as Error).message);
      setSkills([]);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    const onFocus = () => { refresh(); };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refresh]);

  const filtered = useMemo(() => {
    if (!skills) return [];
    const q = query.trim().toLowerCase();
    if (!q) return skills;
    return skills.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q),
    );
  }, [skills, query]);

  const selected = selectedId
    ? skills?.find((s) => s.id === selectedId) ?? null
    : null;

  return (
    <div className="flex-1 flex flex-col min-h-0 bg-night">
      <PanelHeader>
        <span className="text-[15px] font-semibold text-primary">Skills</span>
        <button
          onClick={() => setCreating(true)}
          className="flex items-center gap-1.5 pl-2 pr-2.5 h-7 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
        >
          <Plus size={14} />
          Create skill
        </button>
      </PanelHeader>

      <div className="flex-1 overflow-y-auto terminal-scroll">
        <div className="px-8 py-6">
          <div className="flex items-center justify-end gap-4 mb-6">
            <div className="relative w-72">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search skills"
                className="w-full h-8 pl-9 pr-8 rounded-md border border-night-border bg-transparent text-[13px] text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/10 transition-colors"
              />
              {query && (
                <button
                  onClick={() => setQuery("")}
                  className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted hover:text-secondary"
                >
                  <X size={12} />
                </button>
              )}
            </div>
          </div>

          {error && (
            <div className="mb-4 rounded-lg border border-error/40 bg-error/5 px-4 py-3 text-sm text-error">
              Failed to load skills: {error}
            </div>
          )}

          {skills === null ? (
            <div className="flex items-center gap-2 py-12 justify-center text-sm text-muted">
              <Loader2 size={14} className="animate-spin" /> Loading skills…
            </div>
          ) : skills.length === 0 ? (
            <EmptyState onCreate={() => setCreating(true)} />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {filtered.map((skill) => (
                <button
                  key={skill.id}
                  onClick={() => setSelectedId(skill.id)}
                  className="group flex flex-col gap-2 p-4 rounded-xl border border-night-border hover:bg-night-hover/40 transition-colors text-left"
                >
                  <div className="min-w-0">
                    <p className="text-[15px] font-medium text-primary truncate">{skill.name}</p>
                  </div>
                  <p className="text-[13px] text-secondary leading-relaxed line-clamp-2">
                    {skill.description || (
                      <span className="italic text-muted">No description</span>
                    )}
                  </p>
                  <div className="text-[12px] text-muted mt-1">
                    Updated {timeAgo(skill.updated_at)}
                  </div>
                </button>
              ))}
              {filtered.length === 0 && (
                <div className="col-span-full text-center py-16">
                  <p className="text-sm text-secondary">No skills match your search.</p>
                  <button
                    onClick={() => setCreating(true)}
                    className="mt-4 inline-flex items-center gap-1.5 pl-2 pr-2.5 h-8 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
                  >
                    <Plus size={14} />
                    Create skill
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {creating && (
        <SkillDialog
          mode="create"
          onClose={() => setCreating(false)}
          onSaved={async () => {
            await refresh();
            setCreating(false);
          }}
        />
      )}
      {selected && (
        <SkillDialog
          key={selected.id}
          mode="view"
          skill={selected}
          onClose={() => setSelectedId(null)}
          onSaved={refresh}
          onDeleted={async () => {
            setSelectedId(null);
            await refresh();
          }}
        />
      )}
    </div>
  );
}

function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center gap-3 py-16 text-center">
      <div className="size-12 rounded-2xl bg-night-hover flex items-center justify-center">
        <Zap size={20} className="text-muted" />
      </div>
      <div>
        <p className="text-sm font-medium text-primary">No skills yet</p>
        <p className="text-xs text-muted mt-1 max-w-sm">
          Create a SKILL.md file that your agents can apply automatically.
        </p>
      </div>
      <button
        onClick={onCreate}
        className="inline-flex items-center gap-1.5 pl-2 pr-2.5 h-8 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
      >
        <Plus size={14} />
        New skill
      </button>
    </div>
  );
}

// ── Dialog (create/view/edit) ────────────────────────────────────

function SkillDialog({
  mode: initialMode,
  skill,
  onClose,
  onSaved,
  onDeleted,
}: {
  mode: DialogMode;
  skill?: SkillInfo;
  onClose: () => void;
  onSaved: () => void | Promise<void>;
  onDeleted?: () => void | Promise<void>;
}) {
  const [mode, setMode] = useState<DialogMode>(initialMode);
  const [name, setName] = useState(skill?.name ?? "");
  const [description, setDescription] = useState(skill?.description ?? "");
  // `body` holds the markdown *after* any frontmatter. The full SKILL.md
  // sent to cr0n-a is recomposed at save time so the frontmatter always
  // reflects the current name/description form fields.
  const [body, setBody] = useState(stripFrontmatter(skill?.content ?? ""));
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const editable = mode === "create" || mode === "edit";
  const nameError =
    mode === "create" && name && !NAME_RE.test(name) ? NAME_HINT : null;
  const canSubmit =
    !busy &&
    name.trim() &&
    description.trim() &&
    body.trim() &&
    !nameError;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setBusy(true);
    setFormError(null);
    const content = composeSkillContent(name.trim(), description, body);
    try {
      if (mode === "create") {
        await createSkill({ name: name.trim(), description, content });
      } else if (mode === "edit" && skill) {
        await updateSkill(skill.id, { name: name.trim(), description, content });
      }
      await onSaved();
      if (mode === "edit") setMode("view");
    } catch (e) {
      setFormError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async () => {
    if (!skill) return;
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setBusy(true);
    setFormError(null);
    try {
      await deleteSkill(skill.id);
      await onDeleted?.();
    } catch (e) {
      setFormError((e as Error).message);
      setBusy(false);
    }
  };

  const resetAndClose = () => {
    if (mode === "edit" && skill) {
      setName(skill.name);
      setDescription(skill.description);
      setBody(stripFrontmatter(skill.content));
      setMode("view");
      setFormError(null);
      return;
    }
    onClose();
  };

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent
        showCloseButton={false}
        className="max-w-2xl max-h-[82vh] rounded-2xl sm:top-[8vh] sm:translate-y-0 overflow-hidden flex flex-col"
      >
        <DialogHeader>
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0 flex-1">
              <DialogTitle className="text-base font-semibold text-primary truncate">
                {mode === "create" ? "New skill" : name || skill?.name}
              </DialogTitle>
              <DialogDescription className="text-xs text-muted mt-0.5">
                SKILL.md
              </DialogDescription>
            </div>
            <button
              onClick={onClose}
              className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-secondary transition-colors"
            >
              <X size={16} />
            </button>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto terminal-scroll space-y-4 pr-1">
          <Field label="Name">
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!editable || mode === "edit"}
              placeholder="my-skill"
              autoFocus={mode === "create"}
              spellCheck={false}
              className={cn(
                "w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors",
                !editable && "opacity-70",
              )}
            />
            {nameError && (
              <p className="mt-1 text-[11px] text-error">{nameError}</p>
            )}
            {mode === "edit" && (
              <p className="mt-1 text-[11px] text-muted">
                Name is immutable after creation.
              </p>
            )}
          </Field>

          <Field label="Description">
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              disabled={!editable}
              placeholder="What this skill does and when to use it"
              className={cn(
                "w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors",
                !editable && "opacity-70",
              )}
            />
          </Field>

          <Field label="Body (markdown)">
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              disabled={!editable}
              placeholder={PLACEHOLDER_BODY}
              spellCheck={false}
              rows={18}
              className={cn(
                "w-full px-3 py-2 text-[13px] rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors resize-y font-mono leading-relaxed terminal-scroll min-h-[300px]",
                !editable && "opacity-70",
              )}
            />
            <p className="mt-1 text-[11px] text-muted">
              Frontmatter (<span className="font-mono">name</span>,{" "}
              <span className="font-mono">description</span>) is generated
              from the fields above. Written to{" "}
              <span className="font-mono">
                .claude/skills/{name || "{name}"}/SKILL.md
              </span>{" "}
              on every run.
            </p>
          </Field>

          {formError && (
            <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error">
              {formError}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-2 pt-4 border-t border-night-border">
          <div className="flex items-center gap-2">
            {mode === "view" && skill && (
              <Button
                variant="outline"
                size="sm"
                onClick={handleDelete}
                disabled={busy}
                className={cn(
                  confirmDelete && "text-error border-error/40 hover:bg-error/10",
                )}
              >
                {busy ? (
                  <Loader2 size={14} className="mr-1.5 animate-spin" />
                ) : null}
                {confirmDelete ? "Really delete?" : "Delete"}
              </Button>
            )}
          </div>
          <div className="flex items-center gap-2">
            {mode === "view" && skill && (
              <>
                <Button variant="outline" size="sm" onClick={onClose}>
                  Close
                </Button>
                <Button size="sm" onClick={() => setMode("edit")}>
                  Edit
                </Button>
              </>
            )}
            {(mode === "create" || mode === "edit") && (
              <>
                <Button variant="outline" size="sm" onClick={resetAndClose}>
                  Cancel
                </Button>
                <Button size="sm" onClick={handleSubmit} disabled={!canSubmit}>
                  {busy ? (
                    <Loader2 size={14} className="mr-1.5 animate-spin" />
                  ) : null}
                  {mode === "create" ? "Create" : "Save"}
                </Button>
              </>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="block text-xs font-medium text-muted uppercase tracking-wider mb-1.5">
        {label}
      </label>
      {children}
    </div>
  );
}
