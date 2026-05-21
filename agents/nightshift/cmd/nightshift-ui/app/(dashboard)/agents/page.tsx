"use client";

import { useCallback, useEffect, useMemo, useState, type KeyboardEvent } from "react";
import { Bot, Loader2, Plus, Search, X } from "lucide-react";
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
  createAgent,
  deleteAgent,
  listAgents,
  updateAgent,
  type AgentInfo,
} from "@/lib/api";

const NAME_RE = /^[a-z0-9-]+$/;
const NAME_HINT = "Lowercase letters, digits, and hyphens only";

const PRESET_TOOLS = [
  "Read",
  "Write",
  "Edit",
  "Bash",
  "Grep",
  "Glob",
  "WebSearch",
  "WebFetch",
];

const MODEL_OPTIONS = [
  { value: "", label: "Inherit default" },
  { value: "claude-opus-4-6", label: "Claude Opus 4.6" },
  { value: "claude-sonnet-4-6", label: "Claude Sonnet 4.6" },
  { value: "claude-haiku-4-5", label: "Claude Haiku 4.5" },
] as const;

const PLACEHOLDER_PROMPT = `You are a specialized sub-agent for …

Responsibilities:
- …
- …
`;

type DialogMode = "create" | "view" | "edit";

export default function AgentsPage() {
  const [agents, setAgents] = useState<AgentInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const list = await listAgents();
      list.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
      setAgents(list);
    } catch (e) {
      setError((e as Error).message);
      setAgents([]);
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
    if (!agents) return [];
    const q = query.trim().toLowerCase();
    if (!q) return agents;
    return agents.filter(
      (a) =>
        a.name.toLowerCase().includes(q) ||
        a.description.toLowerCase().includes(q),
    );
  }, [agents, query]);

  const selected = selectedId
    ? agents?.find((a) => a.id === selectedId) ?? null
    : null;

  return (
    <div className="flex-1 flex flex-col min-h-0 bg-night">
      <PanelHeader>
        <span className="text-[15px] font-semibold text-primary">Agents</span>
        <button
          onClick={() => setCreating(true)}
          className="flex items-center gap-1.5 pl-2 pr-2.5 h-7 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
        >
          <Plus size={14} />
          Create agent
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
                placeholder="Search agents"
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
              Failed to load agents: {error}
            </div>
          )}

          {agents === null ? (
            <div className="flex items-center gap-2 py-12 justify-center text-sm text-muted">
              <Loader2 size={14} className="animate-spin" /> Loading agents…
            </div>
          ) : agents.length === 0 ? (
            <EmptyState onCreate={() => setCreating(true)} />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {filtered.map((agent) => (
                <button
                  key={agent.id}
                  onClick={() => setSelectedId(agent.id)}
                  className="group flex flex-col gap-2 p-4 rounded-xl border border-night-border hover:bg-night-hover/40 transition-colors text-left"
                >
                  <div className="flex items-start justify-between gap-3 w-full">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="text-[15px] font-medium text-primary truncate">{agent.name}</p>
                        {agent.model && (
                          <span className="text-[11px] font-mono text-muted shrink-0">{agent.model}</span>
                        )}
                      </div>
                    </div>
                  </div>
                  <p className="text-[13px] text-secondary leading-relaxed line-clamp-2">
                    {agent.description || (
                      <span className="italic text-muted">No description</span>
                    )}
                  </p>
                  {agent.tools.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {agent.tools.slice(0, 6).map((t) => (
                        <span
                          key={t}
                          className="text-[10px] font-mono text-muted px-1.5 py-0.5 rounded bg-night-hover"
                        >
                          {t}
                        </span>
                      ))}
                      {agent.tools.length > 6 && (
                        <span className="text-[10px] text-muted">
                          +{agent.tools.length - 6}
                        </span>
                      )}
                    </div>
                  )}
                  <div className="text-[12px] text-muted mt-1">
                    Updated {timeAgo(agent.updated_at)}
                  </div>
                </button>
              ))}
              {filtered.length === 0 && (
                <div className="col-span-full text-center py-16">
                  <p className="text-sm text-secondary">No agents match your search.</p>
                  <button
                    onClick={() => setCreating(true)}
                    className="mt-4 inline-flex items-center gap-1.5 pl-2 pr-2.5 h-8 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
                  >
                    <Plus size={14} />
                    Create agent
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {creating && (
        <AgentDialog
          mode="create"
          onClose={() => setCreating(false)}
          onSaved={async () => {
            await refresh();
            setCreating(false);
          }}
        />
      )}
      {selected && (
        <AgentDialog
          key={selected.id}
          mode="view"
          agent={selected}
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
        <Bot size={20} className="text-muted" />
      </div>
      <div>
        <p className="text-sm font-medium text-primary">No agents yet</p>
        <p className="text-xs text-muted mt-1 max-w-sm">
          Define a sub-agent with its own prompt and tool allowlist so your
          main agent can delegate to it.
        </p>
      </div>
      <button
        onClick={onCreate}
        className="inline-flex items-center gap-1.5 pl-2 pr-2.5 h-8 rounded-md text-[13px] font-medium text-primary bg-night-elevated hover:bg-night-active transition-colors"
      >
        <Plus size={14} />
        New agent
      </button>
    </div>
  );
}

// ── Dialog (create/view/edit) ────────────────────────────────────

function AgentDialog({
  mode: initialMode,
  agent,
  onClose,
  onSaved,
  onDeleted,
}: {
  mode: DialogMode;
  agent?: AgentInfo;
  onClose: () => void;
  onSaved: () => void | Promise<void>;
  onDeleted?: () => void | Promise<void>;
}) {
  const [mode, setMode] = useState<DialogMode>(initialMode);
  const [name, setName] = useState(agent?.name ?? "");
  const [description, setDescription] = useState(agent?.description ?? "");
  const [prompt, setPrompt] = useState(agent?.prompt ?? "");
  const [tools, setTools] = useState<string[]>(agent?.tools ?? []);
  const [model, setModel] = useState(agent?.model ?? "");
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const editable = mode === "create" || mode === "edit";
  const nameError =
    mode === "create" && name && !NAME_RE.test(name) ? NAME_HINT : null;
  const canSubmit =
    !busy && name.trim() && description.trim() && prompt.trim() && !nameError;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setBusy(true);
    setFormError(null);
    const body = {
      name: name.trim(),
      description,
      prompt,
      tools,
      model: model || null,
    };
    try {
      if (mode === "create") {
        await createAgent(body);
      } else if (mode === "edit" && agent) {
        await updateAgent(agent.id, body);
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
    if (!agent) return;
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setBusy(true);
    setFormError(null);
    try {
      await deleteAgent(agent.id);
      await onDeleted?.();
    } catch (e) {
      setFormError((e as Error).message);
      setBusy(false);
    }
  };

  const resetAndClose = () => {
    if (mode === "edit" && agent) {
      setName(agent.name);
      setDescription(agent.description);
      setPrompt(agent.prompt);
      setTools(agent.tools);
      setModel(agent.model ?? "");
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
        className="max-w-2xl max-h-[85vh] rounded-2xl sm:top-[8vh] sm:translate-y-0 overflow-hidden flex flex-col"
      >
        <DialogHeader>
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0 flex-1">
              <DialogTitle className="text-base font-semibold text-primary truncate">
                {mode === "create" ? "New agent" : name || agent?.name}
              </DialogTitle>
              <DialogDescription className="text-xs text-muted mt-0.5">
                Sub-agent definition
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
              placeholder="my-agent"
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
              placeholder="When the main agent should delegate to this one"
              className={cn(
                "w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors",
                !editable && "opacity-70",
              )}
            />
          </Field>

          <Field label="System prompt">
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              disabled={!editable}
              placeholder={PLACEHOLDER_PROMPT}
              spellCheck={false}
              rows={10}
              className={cn(
                "w-full px-3 py-2 text-[13px] rounded-lg border border-night-border bg-night-hover text-primary placeholder:text-muted focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors resize-y font-mono leading-relaxed terminal-scroll min-h-[200px]",
                !editable && "opacity-70",
              )}
            />
          </Field>

          <Field label="Tools">
            <TagInput
              values={tools}
              onChange={setTools}
              disabled={!editable}
              placeholder={editable ? "Type a tool name, Enter to add" : ""}
            />
            {editable && (
              <div className="mt-2 flex flex-wrap gap-1.5">
                <span className="text-[11px] text-muted self-center mr-1">
                  Common:
                </span>
                {PRESET_TOOLS.map((t) => {
                  const active = tools.includes(t);
                  return (
                    <button
                      key={t}
                      type="button"
                      onClick={() =>
                        active
                          ? setTools(tools.filter((x) => x !== t))
                          : setTools([...tools, t])
                      }
                      className={cn(
                        "text-[11px] font-mono px-2 py-0.5 rounded border transition-colors",
                        active
                          ? "bg-lime/10 border-lime/40 text-lime"
                          : "border-night-border text-muted hover:text-secondary hover:bg-night-hover",
                      )}
                    >
                      {t}
                    </button>
                  );
                })}
              </div>
            )}
          </Field>

          <Field label="Model">
            <select
              value={model}
              onChange={(e) => setModel(e.target.value)}
              disabled={!editable}
              className={cn(
                "w-full px-3 py-2 text-sm rounded-lg border border-night-border bg-night-hover text-primary focus:outline-none focus:border-primary/40 focus:ring-2 focus:ring-primary/10 transition-colors",
                !editable && "opacity-70",
              )}
            >
              {MODEL_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </Field>

          {formError && (
            <div className="rounded-lg border border-error/40 bg-error/5 px-3 py-2 text-xs text-error">
              {formError}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-2 pt-4 border-t border-night-border">
          <div className="flex items-center gap-2">
            {mode === "view" && agent && (
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
            {mode === "view" && agent && (
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

// ── TagInput ─────────────────────────────────────────────────────

function TagInput({
  values,
  onChange,
  disabled,
  placeholder,
}: {
  values: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const [draft, setDraft] = useState("");

  const commit = (raw: string) => {
    const v = raw.trim();
    if (!v) return;
    if (values.includes(v)) {
      setDraft("");
      return;
    }
    onChange([...values, v]);
    setDraft("");
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit(draft);
    } else if (e.key === "Backspace" && !draft && values.length > 0) {
      onChange(values.slice(0, -1));
    }
  };

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-1.5 px-2 py-1.5 rounded-lg border border-night-border bg-night-hover min-h-[2.5rem]",
        disabled && "opacity-70",
      )}
    >
      {values.map((v) => (
        <span
          key={v}
          className="inline-flex items-center gap-1 text-[11px] font-mono px-1.5 py-0.5 rounded bg-night-hover text-primary"
        >
          {v}
          {!disabled && (
            <button
              type="button"
              onClick={() => onChange(values.filter((x) => x !== v))}
              className="text-muted hover:text-secondary"
              aria-label={`Remove ${v}`}
            >
              <X size={10} />
            </button>
          )}
        </span>
      ))}
      {!disabled && (
        <input
          type="text"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          onBlur={() => commit(draft)}
          placeholder={values.length === 0 ? placeholder : ""}
          spellCheck={false}
          className="flex-1 min-w-[10ch] bg-transparent text-sm text-primary placeholder:text-muted focus:outline-none font-mono px-1 py-0.5"
        />
      )}
    </div>
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
