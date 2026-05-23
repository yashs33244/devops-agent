"use client";

import { useEffect, useState, useRef } from "react";

const COMMANDS = [
  {
    cmd: "/deploy",
    desc: "Full pipeline — Dockerfile → Terraform → Helm → CI/CD",
    output: [
      "  ◆  Running full deployment pipeline...",
      "  ◆  Dockerfile generated               ✓",
      "  ◆  Terraform plan approved             ✓",
      "  ◆  Helm chart deployed                 ✓",
      "  ✓  payment-api live on us-east-1",
    ],
    color: "var(--accent)",
  },
  {
    cmd: "/terraform",
    desc: "Generate Terraform for AWS · Azure · GCP",
    output: [
      "  ◆  Detecting cloud provider...  aws",
      "  ◆  Generating VPC + EKS cluster",
      "  ◆  Writing IAM OIDC trust policy",
      "  ◆  terraform validate            ✓",
      "  ✓  3 files written to ./terraform/",
    ],
    color: "oklch(70% 0.18 220)",
  },
  {
    cmd: "/sre-guard",
    desc: "Start the monitoring daemon — watches all services",
    output: [
      "  ◆  Starting SRE Guard daemon...",
      "  ◆  Connecting to Prometheus :9090",
      "  ◆  Watching: payment-api, auth-svc",
      "  ◆  Alert rules loaded (3 rules)",
      "  ✓  Daemon running  http://localhost:8888",
    ],
    color: "var(--good)",
  },
  {
    cmd: "/optimize-cost",
    desc: "Apply KEDA scale-to-zero + Karpenter to your cluster",
    output: [
      "  ◆  Analysing service traffic patterns",
      "  ◆  Avg CPU utilisation: 12%  → eligible",
      "  ◆  Writing keda.tf + http-scaler.yaml",
      "  ◆  minReplicas: 0  scaledownPeriod: 300",
      "  ✓  Est. saving: 74% compute cost",
    ],
    color: "oklch(76% 0.16 60)",
  },
  {
    cmd: "/audit",
    desc: "Security + drift audit on existing infrastructure",
    output: [
      "  ◆  Running checkov + tflint + trivy...",
      "  ⚠  CRITICAL  hardcoded secret in values.yaml",
      "  ⚠  HIGH      no network policy defined",
      "  ◆  2 critical · 3 high · 1 medium",
      "  ✓  Full report → workspace/audit.md",
    ],
    color: "var(--danger-soft)",
  },
  {
    cmd: "/helm",
    desc: "Generate + lint a production Helm chart",
    output: [
      "  ◆  Generating chart for auth-svc...",
      "  ◆  securityContext.runAsNonRoot: true",
      "  ◆  readOnlyRootFilesystem: true",
      "  ◆  helm lint                       ✓",
      "  ✓  chart/ written · ready to install",
    ],
    color: "oklch(68% 0.18 180)",
  },
];

const TYPEWRITER_SPEED = 38; // ms per char
const HOLD_DURATION   = 2800; // ms to show output before cycling
const OUTPUT_LINE_GAP = 110;  // ms between output lines

export function SlashCommands() {
  const [activeIdx, setActiveIdx] = useState(0);
  const [typed,     setTyped]     = useState("");
  const [outputLines, setOutputLines] = useState<string[]>([]);
  const [phase, setPhase] = useState<"typing" | "output" | "holding" | "erasing">("typing");
  const timeouts = useRef<ReturnType<typeof setTimeout>[]>([]);

  const clearAll = () => {
    timeouts.current.forEach(clearTimeout);
    timeouts.current = [];
  };

  useEffect(() => {
    clearAll();
    const cmd = COMMANDS[activeIdx].cmd;

    if (phase === "typing") {
      // Type the command character by character
      for (let i = 0; i <= cmd.length; i++) {
        const t = setTimeout(() => {
          setTyped(cmd.slice(0, i));
          if (i === cmd.length) {
            const next = setTimeout(() => setPhase("output"), 300);
            timeouts.current.push(next);
          }
        }, i * TYPEWRITER_SPEED);
        timeouts.current.push(t);
      }
    }

    if (phase === "output") {
      setOutputLines([]);
      const lines = COMMANDS[activeIdx].output;
      lines.forEach((line, i) => {
        const t = setTimeout(() => {
          setOutputLines(prev => [...prev, line]);
          if (i === lines.length - 1) {
            const next = setTimeout(() => setPhase("holding"), 200);
            timeouts.current.push(next);
          }
        }, i * OUTPUT_LINE_GAP);
        timeouts.current.push(t);
      });
    }

    if (phase === "holding") {
      const t = setTimeout(() => setPhase("erasing"), HOLD_DURATION);
      timeouts.current.push(t);
    }

    if (phase === "erasing") {
      const cmd2 = COMMANDS[activeIdx].cmd;
      for (let i = cmd2.length; i >= 0; i--) {
        const t = setTimeout(() => {
          setTyped(cmd2.slice(0, i));
          setOutputLines([]);
          if (i === 0) {
            const next = setTimeout(() => {
              setActiveIdx(prev => (prev + 1) % COMMANDS.length);
              setPhase("typing");
            }, 200);
            timeouts.current.push(next);
          }
        }, (cmd2.length - i) * (TYPEWRITER_SPEED * 0.6));
        timeouts.current.push(t);
      }
    }

    return clearAll;
  }, [activeIdx, phase]);

  const active = COMMANDS[activeIdx];

  return (
    <section
      id="slash-commands"
      style={{
        background: "var(--bg-dark)",
        padding: "120px 0",
        borderTop: "1px solid var(--line-dark)",
      }}
    >
      <style>{`
        .sc-wrap {
          max-width: 1100px;
          margin: 0 auto;
          padding: 0 24px;
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 72px;
          align-items: center;
        }
        @media (max-width: 860px) {
          .sc-wrap { grid-template-columns: 1fr; gap: 48px; }
        }

        /* ── Command list ── */
        .sc-list {
          display: flex;
          flex-direction: column;
          gap: 2px;
        }
        .sc-item {
          display: flex;
          flex-direction: column;
          gap: 2px;
          padding: 12px 14px;
          border-radius: 4px;
          cursor: default;
          border: 1px solid transparent;
          transition: background 0.15s ease, border-color 0.15s ease;
        }
        .sc-item.active {
          background: var(--bg-dark-3);
          border-color: var(--line-dark-2);
        }
        .sc-cmd {
          font-family: "JetBrains Mono", monospace;
          font-size: 14px;
          font-weight: 500;
          letter-spacing: 0.02em;
          transition: color 0.15s ease;
        }
        .sc-desc {
          font-family: "Inter Tight", sans-serif;
          font-size: 12px;
          color: var(--text-on-dark-soft);
          opacity: 0;
          max-height: 0;
          overflow: hidden;
          transition: opacity 0.2s ease, max-height 0.2s ease;
        }
        .sc-item.active .sc-desc {
          opacity: 1;
          max-height: 40px;
        }

        /* ── Claude Code mock ── */
        .cc-mock {
          border: 1px solid var(--line-dark-2);
          border-radius: 8px;
          overflow: hidden;
          background: var(--bg-dark-3);
          box-shadow: 0 24px 64px rgba(0,0,0,0.45), 0 0 0 1px var(--line-dark);
        }
        .cc-mock-bar {
          background: var(--bg-dark-2);
          border-bottom: 1px solid var(--line-dark);
          padding: 10px 14px;
          display: flex;
          align-items: center;
          gap: 7px;
        }
        .cc-m-dot { width: 11px; height: 11px; border-radius: 50%; }
        .cc-m-title {
          font-family: "JetBrains Mono", monospace;
          font-size: 11px;
          color: var(--text-on-dark-soft);
          margin-left: auto;
          letter-spacing: 0.06em;
        }
        .cc-mock-body {
          padding: 20px 20px 24px;
          min-height: 240px;
          font-family: "JetBrains Mono", monospace;
          font-size: 12.5px;
          line-height: 1.8;
        }
        .cc-prompt-row {
          display: flex;
          align-items: center;
          gap: 8px;
          margin-bottom: 4px;
        }
        .cc-prompt-label {
          font-size: 11px;
          color: var(--text-on-dark-soft);
          opacity: 0.5;
          flex-shrink: 0;
        }
        .cc-typed {
          font-size: 14px;
          font-weight: 500;
          transition: color 0.15s ease;
        }
        .cc-blink {
          display: inline-block;
          width: 2px;
          height: 16px;
          background: var(--accent);
          margin-left: 1px;
          vertical-align: middle;
          animation: blink-bar 1s step-end infinite;
        }
        @keyframes blink-bar {
          0%, 100% { opacity: 1; }
          50%       { opacity: 0; }
        }
        .cc-output-line {
          display: block;
          color: var(--text-on-dark-soft);
          font-size: 12px;
          line-height: 1.85;
          animation: fade-in-line 0.18s ease;
        }
        .cc-output-line.success {
          color: var(--good);
          font-weight: 600;
        }
        .cc-output-line.warn {
          color: var(--danger-soft);
        }
        @keyframes fade-in-line {
          from { opacity: 0; transform: translateY(3px); }
          to   { opacity: 1; transform: translateY(0); }
        }
      `}</style>

      <div className="sc-wrap">
        {/* ── Left: eyebrow + headline + command list ── */}
        <div>
          <span
            className="eyebrow on-dark"
            style={{ marginBottom: "20px", display: "inline-flex" }}
          >
            // slash commands
          </span>
          <h2
            className="h-serif"
            style={{
              fontSize: "clamp(36px, 4vw, 56px)",
              margin: "0 0 16px",
              color: "var(--text-on-dark)",
            }}
          >
            Every tool, one<br />
            <em style={{ color: "var(--accent)", fontStyle: "italic" }}>
              slash away.
            </em>
          </h2>
          <p
            style={{
              fontSize: "15px",
              color: "var(--text-on-dark-soft)",
              lineHeight: 1.65,
              maxWidth: "44ch",
              margin: "0 0 36px",
              fontFamily: "Inter Tight, sans-serif",
            }}
          >
            16 Claude Code commands that run directly in your editor.
            Infra ops without leaving your terminal.
          </p>

          <div className="sc-list">
            {COMMANDS.map((c, i) => (
              <div
                key={c.cmd}
                className={`sc-item${i === activeIdx ? " active" : ""}`}
                onMouseEnter={() => {
                  clearAll();
                  setActiveIdx(i);
                  setTyped("");
                  setOutputLines([]);
                  setPhase("typing");
                }}
              >
                <span
                  className="sc-cmd"
                  style={{ color: i === activeIdx ? c.color : "var(--text-on-dark-soft)" }}
                >
                  {c.cmd}
                </span>
                <span className="sc-desc">{c.desc}</span>
              </div>
            ))}
          </div>
        </div>

        {/* ── Right: Claude Code mock terminal ── */}
        <div className="cc-mock">
          <div className="cc-mock-bar">
            <span className="cc-m-dot" style={{ background: "#ff5f56" }} />
            <span className="cc-m-dot" style={{ background: "#ffbd2e" }} />
            <span className="cc-m-dot" style={{ background: "#27c93f" }} />
            <span className="cc-m-title">claude code</span>
          </div>

          <div className="cc-mock-body">
            {/* Previous context hint */}
            <div style={{ color: "rgba(156,156,163,0.35)", fontSize: "11px", marginBottom: "14px", fontFamily: "JetBrains Mono, monospace" }}>
              ✓ &nbsp;payment-api repo loaded · aws · us-east-1
            </div>

            {/* Prompt row */}
            <div className="cc-prompt-row">
              <span className="cc-prompt-label">›</span>
              <span
                className="cc-typed"
                style={{ color: active.color }}
              >
                {typed}
              </span>
              {phase !== "holding" && <span className="cc-blink" />}
            </div>

            {/* Output lines */}
            <div style={{ marginTop: "8px" }}>
              {outputLines.map((line, i) => (
                <span
                  key={i}
                  className={`cc-output-line${
                    line.includes("✓") && i === outputLines.length - 1
                      ? " success"
                      : line.includes("⚠")
                      ? " warn"
                      : ""
                  }`}
                >
                  {line}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
