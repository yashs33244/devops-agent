import Link from "next/link";

const quickNavCards = [
  {
    href: "/docs/quick-start",
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <path d="M10 2L12.5 7.5H18L13.5 11L15.5 17L10 13.5L4.5 17L6.5 11L2 7.5H7.5L10 2Z" fill="var(--accent)" />
      </svg>
    ),
    title: "Quick Start",
    desc: "From repo to deployed in 10 minutes",
  },
  {
    href: "/docs/slash-commands",
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <rect x="2" y="3" width="16" height="14" rx="0" stroke="var(--accent)" strokeWidth="1.4" />
        <path d="M6 10l2.5 2.5L6 15M11 15h3" stroke="var(--accent)" strokeWidth="1.4" strokeLinecap="square" />
      </svg>
    ),
    title: "Slash Commands",
    desc: "16 commands built into Claude Code",
  },
  {
    href: "/docs/agents",
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <circle cx="10" cy="8" r="4" stroke="var(--accent)" strokeWidth="1.4" />
        <path d="M3 18c0-3.866 3.134-7 7-7s7 3.134 7 7" stroke="var(--accent)" strokeWidth="1.4" strokeLinecap="square" />
      </svg>
    ),
    title: "Agents",
    desc: "6 specialist agents for every layer of the stack",
  },
  {
    href: "/docs/templates",
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <rect x="3" y="3" width="6" height="6" stroke="var(--accent)" strokeWidth="1.4" />
        <rect x="11" y="3" width="6" height="6" stroke="var(--accent)" strokeWidth="1.4" />
        <rect x="3" y="11" width="6" height="6" stroke="var(--accent)" strokeWidth="1.4" />
        <rect x="11" y="11" width="6" height="6" stroke="var(--accent)" strokeWidth="1.4" />
      </svg>
    ),
    title: "Templates",
    desc: "Battle-tested Terraform, Helm, and Dockerfile templates",
  },
  {
    href: "/docs/tools/cicd",
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <circle cx="4" cy="10" r="2" stroke="var(--accent)" strokeWidth="1.4" />
        <circle cx="10" cy="10" r="2" stroke="var(--accent)" strokeWidth="1.4" />
        <circle cx="16" cy="10" r="2" stroke="var(--accent)" strokeWidth="1.4" />
        <path d="M6 10h2M12 10h2" stroke="var(--accent)" strokeWidth="1.4" />
      </svg>
    ),
    title: "CI/CD",
    desc: "OIDC-native GitHub Actions, SHA-pinned",
  },
  {
    href: "/docs/tools/secrets",
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <rect x="4" y="9" width="12" height="9" rx="0" stroke="var(--accent)" strokeWidth="1.4" />
        <path d="M7 9V6a3 3 0 016 0v3" stroke="var(--accent)" strokeWidth="1.4" strokeLinecap="square" />
        <circle cx="10" cy="14" r="1.5" fill="var(--accent)" />
      </svg>
    ),
    title: "Security",
    desc: "Zero static credentials, distroless images, ESO",
  },
];

const agents = [
  {
    id: "holmesgpt",
    name: "HolmesGPT",
    lang: "Python",
    desc: "AI-powered incident investigation. Correlates logs, metrics, and traces to surface root causes automatically. Integrates with PagerDuty, OpsGenie, and Prometheus.",
  },
  {
    id: "kagent",
    name: "kagent",
    lang: "Go",
    desc: "Kubernetes-native AI agent for fleet operations. Manages rollouts, scales workloads, and diagnoses pod failures across multi-cluster environments.",
  },
  {
    id: "nightshift",
    name: "Nightshift",
    lang: "Go",
    desc: "Cost optimization scheduler. Spins down non-production workloads on a cron, restores them before business hours. Pairs with KEDA scale-to-zero for maximum savings.",
  },
  {
    id: "opensre",
    name: "OpenSRE",
    lang: "Python + Node",
    desc: "Runbook automation engine. Converts Markdown runbooks into executable incident workflows triggered by alerts.",
  },
  {
    id: "plural",
    name: "Plural",
    lang: "Elixir",
    desc: "GitOps-native multi-cloud deployment platform. Manages Helm releases across AWS, GCP, and Azure from a single control plane.",
  },
  {
    id: "sre-guard",
    name: "SRE Guard",
    lang: "Python",
    desc: "Lightweight monitoring daemon. Watches deployments and fires the /opensre runbook automatically when an SLO breach is detected.",
  },
];

export default function DocsPage() {
  return (
    <div>
      {/* Hero */}
      <div
        style={{
          borderBottom: "1px solid var(--line-dark)",
          paddingBottom: "40px",
          marginBottom: "48px",
        }}
      >
        <div
          style={{
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
            fontSize: "11px",
            letterSpacing: "0.16em",
            textTransform: "uppercase",
            color: "var(--text-on-dark-soft)",
            marginBottom: "16px",
            display: "flex",
            alignItems: "center",
            gap: "8px",
          }}
        >
          <span
            style={{
              width: "8px",
              height: "8px",
              background: "var(--accent)",
              display: "inline-block",
            }}
          />
          Reference
        </div>
        <h1
          className="h-serif"
          style={{
            fontSize: "clamp(42px, 5vw, 68px)",
            margin: "0 0 16px",
            color: "var(--text-on-dark)",
          }}
        >
          Documentation
        </h1>
        <p
          style={{
            fontSize: "17px",
            color: "var(--text-on-dark-soft)",
            margin: "0 0 28px",
            lineHeight: 1.5,
            maxWidth: "52ch",
          }}
        >
          Everything you need to deploy, monitor, and scale with Pilot.
        </p>
        <div style={{ display: "flex", gap: "12px", alignItems: "center" }}>
          <Link href="/docs/quick-start" className="btn btn-primary">
            Quick Start
            <span className="arrow" />
          </Link>
          <a
            href="https://github.com/yashs33244/devops-agent"
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-ghost"
          >
            View on GitHub
          </a>
        </div>
      </div>

      {/* Quick nav cards — 2x3 grid */}
      <section style={{ marginBottom: "64px" }}>
        <h2
          className="h-sans"
          style={{
            fontSize: "13px",
            letterSpacing: "0.1em",
            textTransform: "uppercase",
            color: "var(--text-on-dark-soft)",
            marginBottom: "20px",
            fontWeight: 500,
          }}
        >
          Explore
        </h2>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            gap: "1px",
            background: "var(--line-dark)",
            border: "1px solid var(--line-dark)",
          }}
        >
          {quickNavCards.map((card) => (
            <Link
              key={card.href}
              href={card.href}
              className="docs-nav-card"
              style={{
                display: "flex",
                alignItems: "flex-start",
                gap: "14px",
                padding: "20px 22px",
                background: "var(--bg-dark-2)",
                transition: "background .15s, border-color .15s",
                borderLeft: "2px solid transparent",
              }}
            >
              <span style={{ marginTop: "2px", flexShrink: 0 }}>{card.icon}</span>
              <span>
                <span
                  style={{
                    display: "block",
                    fontSize: "14px",
                    fontWeight: 500,
                    color: "var(--text-on-dark)",
                    marginBottom: "4px",
                    letterSpacing: "-0.01em",
                  }}
                >
                  {card.title}
                </span>
                <span
                  style={{
                    fontSize: "13px",
                    color: "var(--text-on-dark-soft)",
                    lineHeight: 1.4,
                  }}
                >
                  {card.desc}
                </span>
              </span>
            </Link>
          ))}
        </div>
        <style>{`
          .docs-nav-card:hover {
            background: var(--bg-dark-3) !important;
            border-left-color: var(--accent) !important;
          }
        `}</style>
      </section>

      {/* Agents section */}
      <section>
        <div
          style={{
            display: "flex",
            alignItems: "baseline",
            justifyContent: "space-between",
            marginBottom: "20px",
          }}
        >
          <h2
            className="h-sans"
            style={{
              fontSize: "13px",
              letterSpacing: "0.1em",
              textTransform: "uppercase",
              color: "var(--text-on-dark-soft)",
              margin: 0,
              fontWeight: 500,
            }}
          >
            Sub-Agents
          </h2>
          <Link
            href="/docs/agents"
            style={{
              fontSize: "12px",
              color: "var(--accent)",
              fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
            }}
          >
            View all →
          </Link>
        </div>
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {agents.map((agent, i) => (
            <div
              key={agent.id}
              style={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                padding: "18px 22px",
                borderBottom: i < agents.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "24px",
              }}
            >
              <div style={{ flex: 1, minWidth: 0 }}>
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "10px",
                    marginBottom: "6px",
                  }}
                >
                  <span
                    style={{
                      fontWeight: 600,
                      fontSize: "14px",
                      color: "var(--text-on-dark)",
                    }}
                  >
                    {agent.name}
                  </span>
                  <span
                    className="mono"
                    style={{
                      fontSize: "10px",
                      padding: "2px 7px",
                      border: "1px solid var(--line-dark-2)",
                      color: "var(--text-on-dark-soft)",
                      background: "var(--bg-dark-3)",
                      borderRadius: "2px",
                      letterSpacing: "0.04em",
                    }}
                  >
                    {agent.lang}
                  </span>
                </div>
                <p
                  style={{
                    margin: 0,
                    fontSize: "13px",
                    color: "var(--text-on-dark-soft)",
                    lineHeight: 1.5,
                    maxWidth: "56ch",
                  }}
                >
                  {agent.desc}
                </p>
              </div>
              <Link
                href={`/docs/agents#${agent.id}`}
                style={{
                  fontSize: "12px",
                  color: "var(--accent)",
                  whiteSpace: "nowrap",
                  fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
                  flexShrink: 0,
                  paddingTop: "2px",
                }}
              >
                View docs →
              </Link>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
