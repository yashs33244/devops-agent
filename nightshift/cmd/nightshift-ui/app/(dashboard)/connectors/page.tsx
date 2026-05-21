"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { cn } from "@/lib/utils";
import { iconUrlFor, developerFor, titleFor } from "@/lib/connectors-display";
import { PanelHeader } from "@/lib/ui";
import { AlertCircle, Check, X, Search, Plug, Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  authorizeConnectorUrl,
  disconnectConnector,
  listConnectorCatalog,
  setConnectorToken,
  type ConnectorCatalogEntry,
} from "@/lib/api";

type Connector = {
  id: string;
  name: string;          // catalog identifier — used for API calls and keys
  title: string;         // human-friendly product name shown on the card
  description: string;
  configured: boolean;   // admin has provisioned OpenBao credentials
  connected: boolean;
  mcpUrl: string;
  authType: "oauth" | "static_token";
  developer: string;
  iconUrl: string;
};

function decorate(c: ConnectorCatalogEntry): Connector {
  return {
    id: c.id,
    name: c.name,
    title: titleFor(c.name),
    description: c.description,
    configured: c.configured,
    connected: c.connected,
    mcpUrl: c.mcp_url,
    authType: c.auth_type,
    developer: developerFor(c.name),
    iconUrl: iconUrlFor(c.name),
  };
}

type Tab = "all" | "connected" | "available";

function ConnectorIcon({ connector, size = "md" }: { connector: Connector; size?: "sm" | "md" }) {
  const [errored, setErrored] = useState(false);
  const s = size === "sm" ? "size-8" : "size-10";
  const imgS = size === "sm" ? 18 : 26;
  const showImg = connector.iconUrl && !errored;
  return (
    <div className={cn("rounded-xl flex items-center justify-center bg-night-hover shrink-0", s)}>
      {showImg ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={connector.iconUrl}
          alt={connector.name}
          width={imgS}
          height={imgS}
          onError={() => setErrored(true)}
        />
      ) : (
        <Plug size={imgS} className="text-muted" />
      )}
    </div>
  );
}

function ConnectorDetail({
  open,
  onOpenChange,
  connector,
  onChanged,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connector: Connector;
  onChanged: () => void | Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [tokenInput, setTokenInput] = useState("");
  const [tokenError, setTokenError] = useState<string | null>(null);
  const isStaticToken = connector.authType === "static_token";

  // Full-page navigate so the 302 chain (UI proxy → cr0n-a → OpenBao →
  // OAuth provider) is honored by the browser. Don't fetch this from JS.
  const handleConnect = () => {
    if (isStaticToken) return;
    window.location.href = authorizeConnectorUrl(connector.name);
  };

  const handleSaveToken = async () => {
    if (busy) return;
    const token = tokenInput.trim();
    if (!token) {
      setTokenError("Token must not be empty");
      return;
    }
    setBusy(true);
    setTokenError(null);
    try {
      await setConnectorToken(connector.name, token);
      setTokenInput("");
      await onChanged();
      onOpenChange(false);
    } catch (e) {
      setTokenError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleDisconnect = async () => {
    if (busy) return;
    setBusy(true);
    try {
      await disconnectConnector(connector.name);
      await onChanged();
      onOpenChange(false);
    } catch (e) {
      console.error("[connectors] disconnect failed:", e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="max-w-xl rounded-2xl sm:top-[10vh] sm:translate-y-0">
        <div className="flex items-center gap-4">
          <ConnectorIcon connector={connector} />
          <div className="flex-1 min-w-0">
            <DialogTitle className="text-base font-semibold text-primary">{connector.title}</DialogTitle>
            <DialogDescription className="text-sm text-muted mt-0.5">{connector.description}</DialogDescription>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {connector.connected ? (
              <Button variant="outline" size="sm" onClick={handleDisconnect} disabled={busy}>
                {busy ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : null}
                Disconnect
              </Button>
            ) : isStaticToken ? null : (
              <Button size="sm" onClick={handleConnect}>Connect</Button>
            )}
            <button onClick={() => onOpenChange(false)} className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-secondary transition-colors">
              <X size={16} />
            </button>
          </div>
        </div>
        <div className="space-y-6">
          {isStaticToken && !connector.connected && (
            <div>
              <h4 className="text-sm font-medium text-primary mb-2">Access token</h4>
              <p className="text-xs text-muted mb-3">
                This connector uses a long-lived bearer token. Paste your {connector.title} token below.
                It is stored in OpenBao and sent as an Authorization header to the MCP server.
              </p>
              <textarea
                value={tokenInput}
                onChange={(e) => setTokenInput(e.target.value)}
                placeholder="Paste token…"
                rows={3}
                autoFocus
                spellCheck={false}
                className="w-full px-3 py-2 text-xs rounded-lg border border-night-border bg-transparent text-primary placeholder:text-muted focus:outline-none focus:border-lime/40 transition-colors resize-none font-mono break-all"
              />
              {tokenError && (
                <p className="mt-2 text-xs text-error">{tokenError}</p>
              )}
              <div className="mt-3 flex justify-end">
                <Button size="sm" onClick={handleSaveToken} disabled={busy || !tokenInput.trim()}>
                  {busy ? <Loader2 size={14} className="mr-1.5 animate-spin" /> : null}
                  Save token
                </Button>
              </div>
            </div>
          )}
          <div>
            <h4 className="text-sm font-medium text-primary mb-3">Features</h4>
            <div className="space-y-3">
              <div className="flex items-start gap-3 p-3 rounded-xl border border-night-border">
                <Search size={16} className="text-muted shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm text-primary">Standard Search</p>
                  <p className="text-xs text-muted mt-0.5">Search your entire {connector.title}. Files are retrieved at query time.</p>
                </div>
                <Check size={16} className="text-muted shrink-0" />
              </div>
              <div className="flex items-start gap-3 p-3 rounded-xl border border-night-border">
                <Search size={16} className="text-muted shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm text-primary">High-Precision Search</p>
                  <p className="text-xs text-muted mt-0.5">Select up to 5,000 of your most important files to catalog and keep in sync.</p>
                </div>
                <Check size={16} className="text-muted shrink-0" />
              </div>
            </div>
            {connector.connected && (
              <button className="mt-3 px-4 py-2 rounded-lg border border-night-border text-sm text-secondary hover:text-primary hover:bg-night-hover transition-colors">
                Upload files
              </button>
            )}
          </div>
          <div>
            <h4 className="text-sm font-medium text-primary mb-2">Overview</h4>
            <ul className="text-sm text-muted space-y-1.5">
              <li className="flex items-start gap-2"><span className="size-1 rounded-full bg-muted mt-2 shrink-0" />File and folder selection is based on your existing {connector.title} permissions</li>
              <li className="flex items-start gap-2"><span className="size-1 rounded-full bg-muted mt-2 shrink-0" />Opt into High-Precision Search for even more comprehensive answers</li>
            </ul>
          </div>
          <div className="flex gap-12">
            {connector.developer && connector.developer !== "—" && (
              <div>
                <h4 className="text-sm font-medium text-primary mb-2">Developed by</h4>
                <p className="text-sm text-muted">{connector.developer}</p>
              </div>
            )}
            {connector.mcpUrl && (
              <div>
                <h4 className="text-sm font-medium text-primary mb-2">MCP server</h4>
                <p className="text-xs text-muted font-mono break-all">{connector.mcpUrl}</p>
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ConnectorsView({
  connectors,
  loading,
  error,
  onRefresh,
}: {
  connectors: Connector[];
  loading: boolean;
  error: string | null;
  onRefresh: () => Promise<void>;
}) {
  const [tab, setTab] = useState<Tab>("all");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Connector | null>(null);

  // Connected / Available tabs only list provisioned entries.
  // Unconfigured entries stay visible in "All" as greyed-out cards.
  const q = query.toLowerCase();
  const filtered = connectors.filter((c) => {
    if (tab === "connected" && (!c.configured || !c.connected)) return false;
    if (tab === "available" && (!c.configured || c.connected)) return false;
    if (!q) return true;
    return (
      c.name.toLowerCase().includes(q) ||
      c.title.toLowerCase().includes(q) ||
      c.description.toLowerCase().includes(q)
    );
  });

  const TABS: { id: Tab; label: string }[] = [
    { id: "all", label: "All" },
    { id: "connected", label: "Connected" },
    { id: "available", label: "Available" },
  ];

  // Re-resolve from the current list so the modal reflects post-refresh state.
  const selectedLive = selected
    ? connectors.find((c) => c.id === selected.id) ?? selected
    : null;

  return (
    <div className="px-8 py-6">
      <div className="flex items-center justify-between gap-4 mb-6">
        <div className="flex items-center gap-1">
          {TABS.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={cn(
                "px-3 h-8 rounded-md text-[13px] font-medium transition-colors",
                tab === t.id
                  ? "bg-night-hover text-primary"
                  : "text-secondary hover:text-primary hover:bg-night-hover/60",
              )}
            >
              {t.label}
            </button>
          ))}
        </div>
        <div className="relative w-72">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search connectors"
            className="w-full h-8 pl-9 pr-8 rounded-md border border-night-border bg-transparent text-[13px] text-primary placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/10 transition-colors"
          />
          {query && (
            <button onClick={() => setQuery("")} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted hover:text-secondary"><X size={12} /></button>
          )}
        </div>
      </div>
      {error && (
        <div className="mb-4 rounded-lg border border-error/40 bg-error/5 px-4 py-3 text-sm text-error">
          Failed to load connectors: {error}
        </div>
      )}
      {loading && connectors.length === 0 ? (
        <div className="flex items-center gap-2 py-12 justify-center text-sm text-muted">
          <Loader2 size={14} className="animate-spin" /> Loading connectors…
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {filtered.map((c) => (
            <button
              key={c.name}
              onClick={() => c.configured && setSelected(c)}
              disabled={!c.configured}
              title={c.configured ? undefined : "Not set — credentials haven't been provisioned"}
              className={cn(
                "group relative flex items-center gap-3.5 p-4 text-left transition-colors rounded-xl border border-night-border",
                c.configured ? "hover:bg-night-hover/40" : "opacity-40 cursor-not-allowed",
              )}
            >
              <ConnectorIcon connector={c} />
              <div className="flex-1 min-w-0">
                <p className="text-[15px] font-medium text-primary truncate">{c.title}</p>
                <p className="text-[12px] text-muted mt-0.5 truncate">
                  {c.configured ? c.description : "Not set"}
                </p>
              </div>
              {c.connected && (
                // pointer-events-none so the indicator doesn't intercept the card click.
                <div
                  aria-label="Connected"
                  className="pointer-events-none absolute top-3 right-3 flex items-center overflow-hidden text-lime"
                >
                  <span className="text-[12px] font-medium whitespace-nowrap max-w-0 opacity-0 group-hover:max-w-[80px] group-hover:opacity-100 group-hover:mr-1 transition-all duration-200 ease-out">
                    Connected
                  </span>
                  <Check size={14} className="shrink-0" />
                </div>
              )}
            </button>
          ))}
          {filtered.length === 0 && !loading && (
            <div className="col-span-full py-16 text-center text-sm text-secondary">
              {connectors.length === 0
                ? "No connectors are configured. Provision OpenBao credentials for at least one provider to populate the catalog."
                : "No connectors match your search."}
            </div>
          )}
        </div>
      )}
      {selectedLive && (
        <ConnectorDetail
          open={!!selected}
          onOpenChange={(o) => { if (!o) setSelected(null); }}
          connector={selectedLive}
          onChanged={onRefresh}
        />
      )}
    </div>
  );
}

type Flash =
  | { kind: "success"; connector: string }
  | { kind: "error"; message: string };

export default function ConnectorsPage() {
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [flash, setFlash] = useState<Flash | null>(null);

  const router = useRouter();
  const searchParams = useSearchParams();

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const list = await listConnectorCatalog();
      setConnectors(list.map(decorate));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  // Refetch on mount and on window focus, so returning from an OAuth
  // handoff in another tab refreshes the connected state.
  useEffect(() => {
    refresh();
    const onFocus = () => { refresh(); };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refresh]);

  // Handle the OAuth callback's `?connected=<name>` / `?error=<desc>`
  // query params: flash a banner, strip the params, refresh.
  useEffect(() => {
    const connected = searchParams.get("connected");
    const oauthErr = searchParams.get("error");
    if (!connected && !oauthErr) return;
    if (connected) {
      setFlash({ kind: "success", connector: connected });
      refresh();
    } else if (oauthErr) {
      setFlash({ kind: "error", message: oauthErr });
    }
    router.replace("/connectors", { scroll: false });
  }, [searchParams, router, refresh]);

  return (
    <div className="flex-1 flex flex-col min-h-0 bg-night">
      <PanelHeader>
        <span className="text-[15px] font-semibold text-primary">Connectors</span>
      </PanelHeader>
      {flash && (
        <FlashBanner flash={flash} onDismiss={() => setFlash(null)} />
      )}
      <div className="flex-1 overflow-y-auto terminal-scroll">
        <ConnectorsView
          connectors={connectors}
          loading={loading}
          error={error}
          onRefresh={refresh}
        />
      </div>
    </div>
  );
}

// Auto-dismisses success after 4s; errors stick until closed.
function FlashBanner({
  flash,
  onDismiss,
}: {
  flash: Flash;
  onDismiss: () => void;
}) {
  useEffect(() => {
    if (flash.kind === "success") {
      const id = setTimeout(onDismiss, 4000);
      return () => clearTimeout(id);
    }
  }, [flash, onDismiss]);

  const success = flash.kind === "success";
  return (
    <div
      role={success ? "status" : "alert"}
      className={cn(
        "mx-8 mt-4 flex items-start gap-3 rounded-lg border px-4 py-3 text-sm",
        success
          ? "border-lime/40 bg-lime/10 text-lime"
          : "border-error/40 bg-error/10 text-error",
      )}
    >
      {success ? (
        <Check size={16} className="shrink-0 mt-0.5" />
      ) : (
        <AlertCircle size={16} className="shrink-0 mt-0.5" />
      )}
      <div className="flex-1 min-w-0">
        {success
          ? <>Connected to <span className="font-medium">{titleFor(flash.connector)}</span>.</>
          : <>OAuth failed: {flash.message}</>}
      </div>
      <button
        onClick={onDismiss}
        aria-label="Dismiss"
        className="size-5 flex items-center justify-center rounded text-muted hover:text-secondary transition-colors"
      >
        <X size={14} />
      </button>
    </div>
  );
}
