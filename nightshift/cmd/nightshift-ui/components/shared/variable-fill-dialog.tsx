"use client";

import { useState, useCallback } from "react";
import { Play } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

// ── Variable extraction & interpolation ──────────────────────────

export const VAR_RE = /\{\{([\w ]+)\}\}/g;

export function extractVariables(prompt: string): string[] {
  const seen = new Set<string>();
  const vars: string[] = [];
  for (const match of prompt.matchAll(VAR_RE)) {
    const name = match[1];
    if (name && !seen.has(name)) { seen.add(name); vars.push(name); }
  }
  return vars;
}

function interpolate(prompt: string, values: Record<string, string>): string {
  return prompt.replace(VAR_RE, (match, name) => values[name] ?? match);
}

// ── Hook: wraps runTask to show variable dialog when needed ──────

type VarTarget = { id: string; name: string; prompt: string; vars: string[] };

export function useVariableRun(
  tasks: { id: string; name: string; prompt: string }[],
  runTask: (taskId: string, promptOverride?: string) => void,
): {
  handleRun: (taskId: string) => void;
  variableDialog: React.ReactNode;
} {
  const [target, setTarget] = useState<VarTarget | null>(null);

  const handleRun = useCallback((taskId: string) => {
    const task = tasks.find((t) => t.id === taskId);
    if (!task) return;
    const vars = extractVariables(task.prompt);
    if (vars.length > 0) {
      setTarget({ id: task.id, name: task.name, prompt: task.prompt, vars });
    } else {
      runTask(task.id);
    }
  }, [tasks, runTask]);

  const variableDialog = (
    <VariableFillDialog
      open={!!target}
      onOpenChange={(o) => { if (!o) setTarget(null); }}
      taskName={target?.name ?? ""}
      variables={target?.vars ?? []}
      onRun={(values) => { if (target) { runTask(target.id, interpolate(target.prompt, values)); setTarget(null); } }}
    />
  );

  return { handleRun, variableDialog };
}

// ── Dialog component ─────────────────────────────────────────────

function VariableFillDialog({
  open,
  onOpenChange,
  taskName,
  variables,
  onRun,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  taskName: string;
  variables: string[];
  onRun: (values: Record<string, string>) => void;
}) {
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(variables.map((v) => [v, ""])),
  );

  // Reset values when variables change (new dialog opened)
  const variablesKey = variables.join(",");
  const [prevKey, setPrevKey] = useState(variablesKey);
  if (variablesKey !== prevKey) {
    setPrevKey(variablesKey);
    setValues(Object.fromEntries(variables.map((v) => [v, ""])));
  }

  const allFilled = variables.every((v) => values[v]?.trim());

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (allFilled) onRun(values);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="max-w-md rounded-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle className="text-base font-semibold text-primary">Fill variables</DialogTitle>
            <DialogDescription className="text-sm text-muted mt-1">
              Set values for <span className="text-secondary font-medium">{taskName}</span>
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 mt-4">
            {variables.map((name, i) => (
              <div key={name} className="space-y-1">
                <label className="text-xs uppercase tracking-wider text-muted capitalize">
                  {name.replace(/_/g, " ")}
                </label>
                <textarea
                  autoFocus={i === 0}
                  rows={1}
                  value={values[name]}
                  onChange={(e) => setValues((prev) => ({ ...prev, [name]: e.target.value }))}
                  className="form-input w-full text-sm resize-none"
                  placeholder={`{{${name}}}`}
                />
              </div>
            ))}
          </div>

          <DialogFooter className="flex gap-3 pt-5">
            <Button
              type="button"
              variant="outline"
              className="flex-1"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={!allFilled}
              className="flex-1"
            >
              <Play size={14} />
              Run
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
