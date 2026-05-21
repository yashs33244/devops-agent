"use client";

import { useEffect, useMemo, useState } from "react";
import { Loader2, Search, X } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  getMe,
  listArtifactPermissions,
  listUsers,
  shareArtifact,
  unshareArtifact,
  type ArtifactPermissionInfo,
  type MeInfo,
  type UserSummary,
} from "@/lib/api";

type Props = {
  artifactId: string;
  artifactName: string;
  /** owner_id of the artifact — compared to the current session to gate edit UX. */
  ownerId: string;
  open: boolean;
  onClose: () => void;
};

type Role = "viewer" | "editor";

export function ShareDialog({
  artifactId,
  artifactName,
  ownerId,
  open,
  onClose,
}: Props) {
  const [me, setMe] = useState<MeInfo | null>(null);
  const myId = me?.id ?? "";
  const isOwner = !!myId && myId === ownerId;

  const [users, setUsers] = useState<UserSummary[] | null>(null);
  const [perms, setPerms] = useState<ArtifactPermissionInfo[] | null>(null);
  const [loadError, setLoadError] = useState("");

  const [query, setQuery] = useState("");
  const [pendingRole, setPendingRole] = useState<Role>("viewer");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState("");

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoadError("");
    setSaveError("");
    Promise.all([getMe(), listUsers(), listArtifactPermissions(artifactId)])
      .then(([m, u, p]) => {
        if (cancelled) return;
        setMe(m);
        setUsers(u);
        setPerms(p);
      })
      .catch((e: Error) => {
        if (cancelled) return;
        setLoadError(e.message || "Failed to load share data");
      });
    return () => {
      cancelled = true;
    };
  }, [artifactId, open]);

  const permsByUser = useMemo(() => {
    const m = new Map<string, ArtifactPermissionInfo>();
    for (const p of perms ?? []) m.set(p.user_id, p);
    return m;
  }, [perms]);

  const pickable = useMemo(() => {
    if (!users) return [];
    const q = query.trim().toLowerCase();
    return users.filter((u) => {
      if (u.id === myId) return false;
      if (u.id === ownerId) return false;
      if (permsByUser.has(u.id)) return false;
      return !q || u.name.toLowerCase().includes(q);
    });
  }, [users, query, myId, ownerId, permsByUser]);

  async function doGrant(userId: string) {
    if (!isOwner) return;
    setSaving(true);
    setSaveError("");
    try {
      await shareArtifact(artifactId, { user_id: userId, role: pendingRole });
      const next = await listArtifactPermissions(artifactId);
      setPerms(next);
      setQuery("");
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : "Grant failed");
    } finally {
      setSaving(false);
    }
  }

  async function doRevoke(userId: string) {
    if (!isOwner) return;
    setSaving(true);
    setSaveError("");
    try {
      await unshareArtifact(artifactId, userId);
      setPerms((cur) => (cur ?? []).filter((p) => p.user_id !== userId));
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : "Revoke failed");
    } finally {
      setSaving(false);
    }
  }

  const nameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const u of users ?? []) m.set(u.id, u.name);
    return m;
  }, [users]);

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="max-w-md">
        <DialogTitle>Share "{artifactName}"</DialogTitle>
        <DialogDescription>
          {isOwner
            ? "Grant viewer or editor access to other people."
            : "Only the artifact owner can change shares."}
        </DialogDescription>

        {loadError && (
          <div className="rounded-lg border border-error/30 bg-error/10 px-3 py-2 text-sm text-error">
            {loadError}
          </div>
        )}

        {/* Existing shares */}
        <div className="space-y-2">
          <div className="text-xs font-medium text-secondary">People with access</div>
          <div className="flex items-center justify-between rounded-lg border border-night-border px-3 py-2 text-sm">
            <span className="text-primary">
              {nameById.get(ownerId) ?? ownerId.slice(0, 8)}
            </span>
            <span className="text-xs text-muted">Owner</span>
          </div>
          {(perms ?? []).map((p) => (
            <div
              key={p.user_id}
              className="flex items-center justify-between rounded-lg border border-night-border px-3 py-2 text-sm"
            >
              <span className="text-primary">
                {nameById.get(p.user_id) ?? p.user_id.slice(0, 8)}
              </span>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted capitalize">{p.role}</span>
                {isOwner && (
                  <button
                    aria-label="Revoke"
                    onClick={() => doRevoke(p.user_id)}
                    disabled={saving}
                    className="size-6 flex items-center justify-center rounded text-muted hover:text-error hover:bg-error/10 disabled:opacity-50"
                  >
                    <X size={14} />
                  </button>
                )}
              </div>
            </div>
          ))}
          {(perms?.length ?? 0) === 0 && (
            <div className="text-xs text-muted">No one else has access yet.</div>
          )}
        </div>

        {/* Add people */}
        {isOwner && (
          <div className="space-y-2 border-t border-night-border pt-4">
            <div className="text-xs font-medium text-secondary">Add people</div>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Search
                  size={14}
                  className="absolute left-2 top-1/2 -translate-y-1/2 text-muted"
                />
                <input
                  type="text"
                  placeholder="Search by name"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  className="form-input text-sm pl-8 w-full"
                />
              </div>
              <select
                value={pendingRole}
                onChange={(e) => setPendingRole(e.target.value as Role)}
                className="form-input text-sm"
              >
                <option value="viewer">Viewer</option>
                <option value="editor">Editor</option>
              </select>
            </div>
            {users === null ? (
              <div className="flex items-center gap-2 text-xs text-muted">
                <Loader2 size={12} className="animate-spin" /> Loading users…
              </div>
            ) : pickable.length === 0 ? (
              <div className="text-xs text-muted">
                {query ? "No matches." : "Everyone's already on the list."}
              </div>
            ) : (
              <ul className="max-h-48 overflow-y-auto space-y-1">
                {pickable.map((u) => (
                  <li key={u.id}>
                    <button
                      onClick={() => doGrant(u.id)}
                      disabled={saving}
                      className="w-full text-left px-3 py-2 rounded-lg hover:bg-night-hover text-sm text-primary disabled:opacity-50"
                    >
                      {u.name}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {saveError && (
              <div className="rounded-lg border border-error/30 bg-error/10 px-3 py-2 text-xs text-error">
                {saveError}
              </div>
            )}
          </div>
        )}

        <div className="flex justify-end pt-4 border-t border-night-border">
          <Button onClick={onClose}>Done</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
