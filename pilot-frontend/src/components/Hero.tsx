"use client";

import { useEffect, useState } from "react";

type LineKind = "cmd" | "blank" | "step" | "sub" | "success" | "detail";
type LogLine  = { id: number; text: string; kind: LineKind };

const SCRIPT: Array<{ at: number; text: string; kind: LineKind }> = [
  { at: 350,  kind: "cmd",     text: "$ pilot deploy payment-api --cloud aws --env prod" },
  { at: 1050, kind: "blank",   text: "" },
  { at: 1150, kind: "step",    text: "  ◆  scanning repository              ✓  187ms" },
  { at: 1500, kind: "step",    text: "  ◆  generating Dockerfile             ✓   91ms" },
  { at: 1800, kind: "step",    text: "  ◆  provisioning terraform (EKS)      ✓   2.1s" },
  { at: 2000, kind: "sub",     text: "     ├  vpc.tf" },
  { at: 2120, kind: "sub",     text: "     ├  eks-cluster.tf" },
  { at: 2240, kind: "sub",     text: "     └  iam-oidc.tf" },
  { at: 2440, kind: "step",    text: "  ◆  building helm chart               ✓   44ms" },
  { at: 2680, kind: "step",    text: "  ◆  writing ci/cd pipeline (OIDC)     ✓  122ms" },
  { at: 2920, kind: "step",    text: "  ◆  test suite · 7 checks             ✓   3.2s" },
  { at: 3150, kind: "blank",   text: "" },
  { at: 3350, kind: "success", text: "  ✓  DEPLOYED  payment-api → us-east-1" },
  { at: 3600, kind: "detail",  text: "     3 replicas  ·  $0.003/req  ·  healthy" },
];

function lineColor(kind: LineKind): string {
  if (kind === "cmd")     return "var(--text-on-dark)";
  if (kind === "success") return "var(--good)";
  if (kind === "step")    return "var(--text-on-dark-soft)";
  if (kind === "sub")     return "rgba(156,156,163,0.55)";
  if (kind === "detail")  return "rgba(156,156,163,0.7)";
  return "transparent";
}

function DeployConsole() {
  const [lines,  setLines]  = useState<LogLine[]>([]);
  const [cursor, setCursor] = useState(true);
  const [done,   setDone]   = useState(false);

  useEffect(() => {
    const timers = SCRIPT.map((s, i) =>
      setTimeout(() => {
        setLines(prev => [...prev, { id: i, text: s.text, kind: s.kind }]);
        if (i === SCRIPT.length - 1) setDone(true);
      }, s.at)
    );
    const blink = setInterval(() => setCursor(c => !c), 520);
    return () => { timers.forEach(clearTimeout); clearInterval(blink); };
  }, []);

  return (
    <div className="console-card">
      {/* title bar */}
      <div className="console-bar">
        <span className="cc-dot" style={{ background: "#ff5f56" }} />
        <span className="cc-dot" style={{ background: "#ffbd2e" }} />
        <span className="cc-dot" style={{ background: "#27c93f" }} />
        <span className="cc-title">pilot — deploy</span>
        {done && (
          <span className="cc-badge">
            <span className="cc-badge-dot" /> live
          </span>
        )}
      </div>

      {/* body */}
      <div className="console-body">
        {lines.map(l => (
          l.kind === "blank"
            ? <div key={l.id} style={{ height: "0.9em" }} />
            : (
              <div
                key={l.id}
                className={`cc-line ${l.kind === "success" ? "cc-success" : ""}`}
                style={{ color: lineColor(l.kind) }}
              >
                {l.text}
              </div>
            )
        ))}
        {!done && (
          <span
            className="cc-cursor"
            style={{ opacity: cursor ? 1 : 0 }}
          >
            ▌
          </span>
        )}
      </div>
    </div>
  );
}

export function Hero() {
  return (
    <header className="hero bg-dark">
      <style dangerouslySetInnerHTML={{ __html: `
        /* ── Hero layout ── */
        .hero { padding-top: 61px; }
        .hero-wrap {
          width: 100%;
          padding: 80px 5vw 96px;
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 48px;
          align-items: center;
          box-sizing: border-box;
        }
        @media (max-width: 900px) {
          .hero-wrap {
            grid-template-columns: 1fr;
            padding: 56px 24px 72px;
            gap: 48px;
          }
        }

        /* ── Left ── */
        .hero-left { display: flex; flex-direction: column; align-items: flex-start; gap: 0; }

        .hero-eyebrow {
          display: inline-flex;
          align-items: center;
          gap: 8px;
          font-family: "JetBrains Mono", monospace;
          font-size: 11px;
          letter-spacing: 0.14em;
          text-transform: uppercase;
          color: var(--text-on-dark-soft);
          border: 1px solid var(--line-dark-2);
          border-radius: 2px;
          padding: 4px 10px;
          margin-bottom: 28px;
        }
        .hero-pulse {
          width: 7px; height: 7px;
          background: var(--good);
          border-radius: 50%;
          box-shadow: 0 0 0 0 var(--good);
          animation: pulse-ring 2s ease-out infinite;
        }
        @keyframes pulse-ring {
          0%   { box-shadow: 0 0 0 0 oklch(72% 0.16 150 / 0.7); }
          70%  { box-shadow: 0 0 0 6px oklch(72% 0.16 150 / 0); }
          100% { box-shadow: 0 0 0 0 oklch(72% 0.16 150 / 0); }
        }

        .hero-h1 {
          font-family: "Instrument Serif", "Times New Roman", serif;
          font-weight: 400;
          font-size: clamp(2.6rem, 4.5vw, 3.75rem);
          line-height: 1.06;
          letter-spacing: -0.025em;
          color: var(--text-on-dark);
          margin: 0 0 24px;
        }
        .hero-h1 em {
          font-style: italic;
          color: var(--accent);
        }

        .hero-sub {
          font-family: "Inter Tight", sans-serif;
          font-size: clamp(15px, 1.6vw, 17px);
          color: var(--text-on-dark-soft);
          line-height: 1.7;
          max-width: 46ch;
          margin: 0 0 36px;
        }

        .hero-cta {
          display: flex;
          gap: 12px;
          flex-wrap: wrap;
        }

        /* ── Right: console ── */
        .console-card {
          border: 1px solid var(--line-dark-2);
          border-radius: 8px;
          overflow: hidden;
          background: var(--bg-dark-3);
          box-shadow: 0 0 0 1px var(--line-dark), 0 32px 80px rgba(0,0,0,0.5), 0 0 60px oklch(70% 0.20 295 / 0.06);
          width: 100%;
        }
        .console-bar {
          display: flex;
          align-items: center;
          gap: 7px;
          padding: 11px 16px;
          background: var(--bg-dark-2);
          border-bottom: 1px solid var(--line-dark);
        }
        .cc-dot { width: 11px; height: 11px; border-radius: 50%; flex-shrink: 0; }
        .cc-title {
          font-family: "JetBrains Mono", monospace;
          font-size: 11px;
          color: var(--text-on-dark-soft);
          margin-left: auto;
          letter-spacing: 0.06em;
        }
        .cc-badge {
          display: inline-flex;
          align-items: center;
          gap: 5px;
          font-family: "JetBrains Mono", monospace;
          font-size: 10px;
          letter-spacing: 0.08em;
          color: var(--good);
          border: 1px solid oklch(72% 0.16 150 / 0.3);
          border-radius: 2px;
          padding: 2px 7px;
          margin-left: 10px;
        }
        .cc-badge-dot {
          width: 5px; height: 5px;
          background: var(--good);
          border-radius: 50%;
          animation: pulse-ring 2s ease-out infinite;
        }
        .console-body {
          padding: 20px 22px 24px;
          font-family: "JetBrains Mono", "IBM Plex Mono", monospace;
          font-size: 12.5px;
          line-height: 1.85;
          min-height: 320px;
          overflow: hidden;
        }
        .cc-line { display: block; white-space: pre; }
        .cc-success {
          font-weight: 600;
          position: relative;
        }
        .cc-success::before {
          content: "";
          position: absolute;
          left: -22px; right: -22px;
          top: 0; bottom: 0;
          background: oklch(72% 0.16 150 / 0.05);
        }
        .cc-cursor {
          display: inline-block;
          color: var(--accent);
          font-size: 14px;
          line-height: 1;
        }

        /* ── Stats row ── */
        .hero-stats {
          width: 100%;
          display: grid;
          grid-template-columns: repeat(6, 1fr);
          gap: 1px;
          background: var(--line-dark);
          border-top: 1px solid var(--line-dark);
        }
        @media (max-width: 900px) {
          .hero-stats { grid-template-columns: repeat(3, 1fr); }
        }
        @media (max-width: 600px) {
          .hero-stats { grid-template-columns: repeat(2, 1fr); }
          .hero-wrap { padding: 48px 20px 60px; }
        }
        .hero-stat {
          padding: 28px 24px;
          background: var(--bg-dark);
          display: flex;
          flex-direction: column;
          gap: 6px;
        }
        .stat-n {
          font-family: "Instrument Serif", serif;
          font-size: 2.2rem;
          line-height: 1;
          color: var(--text-on-dark);
          letter-spacing: -0.03em;
        }
        .stat-n sup { color: var(--accent); font-size: 1.1rem; vertical-align: super; }
        .stat-n sub { color: var(--accent); font-size: 1.1rem; }
        .stat-n .green { color: var(--good); }
        .stat-n .dim { color: var(--text-on-dark-soft); font-size: 1.4rem; }
        .stat-l {
          font-family: "JetBrains Mono", monospace;
          font-size: 10px;
          color: var(--text-on-dark-soft);
          letter-spacing: 0.04em;
          line-height: 1.55;
        }
      `}} />

      <div className="hero-wrap">
        {/* ── LEFT ── */}
        <div className="hero-left">
          <div className="hero-eyebrow">
            <span className="hero-pulse" />
            AI DevOps Agent · Open Source
          </div>

          <h1 className="hero-h1">
            The DevOps agent<br />
            that ships the<br />
            <em>whole pipeline.</em>
          </h1>

          <p className="hero-sub">
            Give Pilot a GitHub repo and a cloud provider — it writes the
            Dockerfile, Terraform, Helm chart, and CI/CD pipeline, then
            monitors and scales your service automatically.
          </p>

          <div className="hero-cta">
            <a
              href="https://github.com/yashs33244/devops-agent"
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-primary"
            >
              Deploy free <span className="arrow" />
            </a>
            <a
              href="/docs/quick-start"
              className="btn btn-ghost"
            >
              Read the docs
            </a>
          </div>
        </div>

        {/* ── RIGHT: animated deploy console ── */}
        <div>
          <DeployConsole />
        </div>
      </div>

      {/* ── STATS ── */}
      <div className="hero-stats">
        <div className="hero-stat">
          <div className="stat-n">&lt;&nbsp;6<sub>min</sub></div>
          <div className="stat-l">git push → live service<br/>full pipeline, zero manual steps</div>
        </div>
        <div className="hero-stat">
          <div className="stat-n">~<span className="green">72</span><span className="green dim">%</span></div>
          <div className="stat-l">avg compute cost cut<br/>KEDA scale-to-zero + Karpenter</div>
        </div>
        <div className="hero-stat">
          <div className="stat-n">10</div>
          <div className="stat-l">pipeline steps automated<br/>Dockerfile → deploy → monitor</div>
        </div>
        <div className="hero-stat">
          <div className="stat-n">6</div>
          <div className="stat-l">languages auto-detected<br/>Node · Python · Go · Java · Rust · Ruby</div>
        </div>
        <div className="hero-stat">
          <div className="stat-n">3</div>
          <div className="stat-l">clouds supported<br/>AWS · Azure · GCP</div>
        </div>
        <div className="hero-stat">
          <div className="stat-n"><span className="green">0</span></div>
          <div className="stat-l">secrets in generated output<br/>OIDC + ESO, no static keys</div>
        </div>
      </div>
    </header>
  );
}
