import Link from "next/link";

function CodeBlock({ lang, children }: { lang: string; children: string }) {
  return (
    <div
      style={{
        border: "1px solid var(--line-dark)",
        borderRadius: "4px",
        overflow: "hidden",
        margin: "12px 0 20px",
      }}
    >
      <div
        style={{
          padding: "7px 14px",
          background: "var(--bg-dark-3)",
          borderBottom: "1px solid var(--line-dark)",
        }}
      >
        <span
          className="mono"
          style={{
            fontSize: "11px",
            color: "var(--text-on-dark-soft)",
            letterSpacing: "0.1em",
            textTransform: "uppercase",
          }}
        >
          {lang}
        </span>
      </div>
      <pre
        style={{
          margin: 0,
          padding: "16px 18px",
          background: "var(--bg-dark-3)",
          fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          fontSize: "12.5px",
          lineHeight: "1.65",
          color: "var(--text-on-dark)",
          overflowX: "auto",
        }}
      >
        <code>{children}</code>
      </pre>
    </div>
  );
}

function LangBadge({ children }: { children: string }) {
  return (
    <span
      className="mono"
      style={{
        fontSize: "10px",
        padding: "2px 8px",
        border: "1px solid var(--line-dark-2)",
        color: "var(--text-on-dark-soft)",
        background: "var(--bg-dark-3)",
        borderRadius: "2px",
        letterSpacing: "0.04em",
      }}
    >
      {children}
    </span>
  );
}

const agents = [
  {
    id: "holmesgpt",
    name: "HolmesGPT",
    lang: "Python",
    tagline: "AI-powered incident investigation",
    description: [
      "HolmesGPT correlates logs, metrics, and distributed traces to surface root causes automatically during an incident. Instead of manually pivoting between Grafana, Loki, and your alerting tool, you ask HolmesGPT a question in plain English and it returns a structured diagnosis.",
      "It integrates natively with PagerDuty, OpsGenie, Prometheus, and Loki. When an alert fires, HolmesGPT can be invoked automatically via the SRE Guard daemon, running its investigation before a human is even paged.",
      "All findings are written to a structured runbook-compatible output format so OpenSRE can pick up remediation automatically.",
    ],
    capabilities: [
      "Natural-language incident queries against live observability data",
      "Automatic root-cause hypothesis ranking with confidence scores",
      "PagerDuty / OpsGenie alert enrichment",
      "Prometheus + Loki + Jaeger integration",
      "Structured JSON output compatible with OpenSRE runbooks",
    ],
    install: `cd holmesgpt
pip install -r requirements.txt
export HOLMES_API_KEY=<your-key>`,
    run: `python3 holmes.py ask \\
  "Why are pods in the payment-api namespace crash-looping?"`,
    slashCmd: "/holmesgpt",
    slashDesc: 'Invoke from Claude Code: /holmesgpt "your incident question"',
  },
  {
    id: "kagent",
    name: "kagent",
    lang: "Go",
    tagline: "Kubernetes-native AI agent for fleet operations",
    description: [
      "kagent is a Kubernetes operator that embeds an AI planner directly into your cluster. It watches deployments, monitors for anomalies, and can execute corrective actions — rolling restarts, HPA adjustments, node cordon/drain — without leaving the cluster.",
      "Unlike kubectl-based scripts, kagent understands intent. You describe the desired outcome (e.g. 'drain node ip-10-0-1-5 with zero downtime') and it generates the safe action sequence, confirming before executing in production.",
      "kagent exposes a CRD-based API so all operations are GitOps-friendly and auditable.",
    ],
    capabilities: [
      "CRD-based intent API (KAgentTask, KAgentPolicy)",
      "Safe drain/cordon with PDB awareness",
      "Automated rollout progression and canary promotion",
      "Multi-cluster fleet operations via kubeconfig federation",
      "Dry-run mode with diff output before any mutation",
    ],
    install: `helm repo add kagent https://kagent.dev/charts
helm install kagent kagent/kagent \\
  -n kagent-system --create-namespace`,
    run: `kubectl apply -f - <<EOF
apiVersion: kagent.dev/v1
kind: KAgentTask
metadata:
  name: drain-node
spec:
  intent: "Drain ip-10-0-1-5 with zero disruption to the payment-api deployment"
EOF`,
    slashCmd: "/kagent",
    slashDesc: "Invoke from Claude Code: /kagent — runs fleet operation tasks interactively",
  },
  {
    id: "nightshift",
    name: "Nightshift",
    lang: "Go",
    tagline: "Cost optimization scheduler",
    description: [
      "Nightshift scales down non-production Kubernetes workloads on a cron schedule and restores them before business hours. A typical dev/staging cluster running 8×5 instead of 24×7 saves 60–70% of compute cost with zero manual effort.",
      "It integrates with Pilot's KEDA scale-to-zero pattern for maximum savings: Nightshift handles overnight shutdown while KEDA handles intra-day idle scale-to-zero. The two are additive.",
      "Nightshift stores intended replica counts as annotations before scaling down, so it always restores to the exact pre-shutdown state — including HPA min/max values.",
    ],
    capabilities: [
      "Cron-based scale-down / scale-up schedules per namespace or label selector",
      "Stores intended replica counts as annotations before shutdown",
      "Respects PodDisruptionBudgets during shutdown sequencing",
      "Slack / PagerDuty notification on each schedule event",
      "Cost report: estimated monthly savings per workload",
    ],
    install: `helm repo add nightshift https://nightshift.dev/charts
helm install nightshift nightshift/nightshift \\
  -n nightshift-system --create-namespace \\
  --set schedule.timezone=Asia/Kolkata`,
    run: `kubectl annotate namespace staging \\
  nightshift.io/schedule="0 20 * * 1-5|0 8 * * 1-5"`,
    slashCmd: "/nightshift",
    slashDesc: "Invoke from Claude Code: /nightshift — configures schedules interactively",
  },
  {
    id: "opensre",
    name: "OpenSRE",
    lang: "Python + Node",
    tagline: "Runbook automation engine",
    description: [
      "OpenSRE converts Markdown runbooks into executable incident workflows. Each runbook is a YAML-annotated Markdown file: human-readable for engineers, machine-executable for the automation layer.",
      "When an alert fires (via HolmesGPT, PagerDuty webhook, or manual trigger), OpenSRE selects the matching runbook, executes each step, and reports status back to your incident channel. Steps can include kubectl commands, API calls, database queries, and escalation logic.",
      "All runbooks live in Git. Changes go through normal PR review. No proprietary runbook DSL to learn.",
    ],
    capabilities: [
      "Markdown-native runbook format with YAML frontmatter for metadata",
      "Step execution: kubectl, bash, HTTP, SQL, PagerDuty escalate",
      "Alert-to-runbook routing via label matchers",
      "Dry-run mode: simulate runbook execution without side effects",
      "Audit log: every step, its output, and the triggering alert are recorded",
    ],
    install: `cd opensre
pip install -r requirements.txt
npm install   # for the Node.js webhook listener
cp config.example.yaml config.yaml`,
    run: `# Start the webhook listener
python3 opensre/server.py --config config.yaml

# Manually trigger a runbook
python3 opensre/cli.py run runbooks/pod-crashloop.md \\
  --vars service=payment-api,namespace=production`,
    slashCmd: "/opensre",
    slashDesc: "Invoke from Claude Code: /opensre — runs a runbook or lists available ones",
  },
  {
    id: "plural",
    name: "Plural",
    lang: "Elixir",
    tagline: "GitOps multi-cloud deployment platform",
    description: [
      "Plural is a GitOps-native platform for managing Helm releases across multiple clusters and cloud providers from a single control plane. It abstracts the differences between EKS, AKS, and GKE so you can promote a release from dev to staging to prod with a single Git merge.",
      "Pilot uses Plural as the deployment layer when you need multi-cloud or multi-region consistency. The /plural slash command scaffolds your Plural configuration from Pilot's generated Helm charts.",
      "Plural's marketplace also provides pre-configured stacks (PostgreSQL, Redis, monitoring) that slot into Pilot's Terraform output.",
    ],
    capabilities: [
      "Single control plane for EKS, AKS, GKE, and bare-metal clusters",
      "Promotion pipelines: dev → staging → prod with automated gate checks",
      "Drift detection: alerts when cluster state diverges from Git",
      "Built-in marketplace of pre-configured infrastructure stacks",
      "OIDC-native: all cloud credentials via Workload Identity",
    ],
    install: `# Install Plural CLI
brew install pluralsh/plural/plural

# Bootstrap your workspace
plural init
plural build --only <your-app>
plural deploy --commit "initial deploy"`,
    run: `# Promote from staging to prod
plural deploy --context prod --commit "promote v1.4.2"`,
    slashCmd: "/plural",
    slashDesc: "Invoke from Claude Code: /plural — manages Plural deployments and promotions",
  },
  {
    id: "sre-guard",
    name: "SRE Guard",
    lang: "Python",
    tagline: "Monitoring daemon with auto-remediation",
    description: [
      "SRE Guard is a lightweight Python daemon that watches your deployments for SLO breaches and triggers the OpenSRE runbook engine automatically. It bridges the gap between alerting (Prometheus/Alertmanager) and remediation (OpenSRE) without requiring a human to be the relay.",
      "Configuration is a single YAML file listing which alerts map to which runbooks, with optional approval gates for destructive actions. In production, destructive steps always require human confirmation via Slack. In dev/staging, SRE Guard can run fully autonomously.",
      "SRE Guard ships as a GitHub Actions workflow (ci-sre-guard.yml) that runs on every deployment, and as a standalone Kubernetes deployment for continuous monitoring.",
    ],
    capabilities: [
      "Prometheus Alertmanager webhook receiver",
      "Alert-to-runbook routing with label-based matching",
      "Approval gates for destructive runbook steps (Slack DM confirmation)",
      "GitHub Actions integration: runs post-deploy health checks automatically",
      "Structured incident report written to GitHub Step Summary",
    ],
    install: `pip install -r requirements.txt

# Copy and edit the config
cp sre-guard/config.example.yaml sre-guard/config.yaml`,
    run: `# Run as a daemon
python3 sre-guard/daemon.py --config sre-guard/config.yaml

# Or run a one-shot health check (used in CI)
python3 sre-guard/check.py \\
  --service payment-api \\
  --namespace production`,
    slashCmd: "/sre-guard",
    slashDesc: "Invoke from Claude Code: /sre-guard — runs a health check or configures the daemon",
  },
];

export default function AgentsPage() {
  return (
    <div>
      {/* Breadcrumb */}
      <div style={{ marginBottom: "32px" }}>
        <Link
          href="/docs"
          style={{
            fontSize: "13px",
            color: "var(--text-on-dark-soft)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          ← Docs
        </Link>
      </div>

      <div
        style={{
          fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          fontSize: "11px",
          letterSpacing: "0.16em",
          textTransform: "uppercase",
          color: "var(--text-on-dark-soft)",
          marginBottom: "12px",
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
        Sub-Agents
      </div>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(36px, 4vw, 54px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        Agents
      </h1>
      <p
        style={{
          fontSize: "16px",
          color: "var(--text-on-dark-soft)",
          margin: "0 0 40px",
          lineHeight: 1.6,
          maxWidth: "58ch",
          borderBottom: "1px solid var(--line-dark)",
          paddingBottom: "32px",
        }}
      >
        Six specialist sub-agents, each handling a distinct layer of the stack. Pilot
        orchestrates them so you don&apos;t have to know which one to call.
      </p>

      {/* TOC */}
      <div
        style={{
          padding: "16px 20px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderRadius: "4px",
          marginBottom: "48px",
        }}
      >
        <div
          className="mono"
          style={{
            fontSize: "11px",
            letterSpacing: "0.1em",
            textTransform: "uppercase",
            color: "var(--text-on-dark-soft)",
            marginBottom: "10px",
          }}
        >
          On this page
        </div>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "6px 20px" }}>
          {agents.map((a) => (
            <a
              key={a.id}
              href={`#${a.id}`}
              style={{
                fontSize: "13px",
                color: "var(--accent)",
                fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
              }}
            >
              {a.name}
            </a>
          ))}
        </div>
      </div>

      {/* Agent entries */}
      {agents.map((agent, i) => (
        <section
          key={agent.id}
          id={agent.id}
          style={{
            marginBottom: "64px",
            scrollMarginTop: "80px",
          }}
        >
          <div
            style={{
              borderTop: i === 0 ? "none" : "1px solid var(--line-dark)",
              paddingTop: i === 0 ? 0 : "48px",
            }}
          >
            {/* Header */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "12px",
                marginBottom: "12px",
              }}
            >
              <h2
                className="h-sans"
                style={{
                  fontSize: "22px",
                  margin: 0,
                  color: "var(--text-on-dark)",
                  letterSpacing: "-0.025em",
                }}
              >
                {agent.name}
              </h2>
              <LangBadge>{agent.lang}</LangBadge>
            </div>
            <p
              style={{
                fontSize: "14px",
                color: "var(--text-on-dark-soft)",
                margin: "0 0 4px",
                fontStyle: "italic",
              }}
            >
              {agent.tagline}
            </p>

            {/* Description */}
            <div style={{ marginTop: "20px" }}>
              {agent.description.map((para, pi) => (
                <p
                  key={pi}
                  style={{
                    fontSize: "14px",
                    color: "var(--text-on-dark-soft)",
                    lineHeight: 1.65,
                    margin: "0 0 14px",
                    maxWidth: "66ch",
                  }}
                >
                  {para}
                </p>
              ))}
            </div>

            {/* Capabilities */}
            <div style={{ marginTop: "24px", marginBottom: "24px" }}>
              <div
                className="mono"
                style={{
                  fontSize: "11px",
                  letterSpacing: "0.1em",
                  textTransform: "uppercase",
                  color: "var(--text-on-dark-soft)",
                  marginBottom: "10px",
                }}
              >
                Key Capabilities
              </div>
              <ul
                style={{
                  margin: 0,
                  padding: "0 0 0 18px",
                  fontSize: "13px",
                  color: "var(--text-on-dark-soft)",
                  lineHeight: 1.9,
                }}
              >
                {agent.capabilities.map((cap, ci) => (
                  <li key={ci}>{cap}</li>
                ))}
              </ul>
            </div>

            {/* Install */}
            <div
              className="mono"
              style={{
                fontSize: "11px",
                letterSpacing: "0.1em",
                textTransform: "uppercase",
                color: "var(--text-on-dark-soft)",
                marginBottom: "4px",
              }}
            >
              Installation
            </div>
            <CodeBlock lang="bash">{agent.install}</CodeBlock>

            {/* Run */}
            <div
              className="mono"
              style={{
                fontSize: "11px",
                letterSpacing: "0.1em",
                textTransform: "uppercase",
                color: "var(--text-on-dark-soft)",
                marginBottom: "4px",
              }}
            >
              Usage
            </div>
            <CodeBlock lang="bash">{agent.run}</CodeBlock>

            {/* Slash command */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "14px",
                padding: "14px 18px",
                background: "var(--bg-dark-2)",
                border: "1px solid var(--line-dark)",
                borderLeft: "3px solid var(--accent)",
                borderRadius: "4px",
              }}
            >
              <code
                className="mono"
                style={{
                  fontSize: "13px",
                  color: "var(--accent)",
                  flexShrink: 0,
                }}
              >
                {agent.slashCmd}
              </code>
              <span
                style={{
                  fontSize: "13px",
                  color: "var(--text-on-dark-soft)",
                }}
              >
                {agent.slashDesc}
              </span>
            </div>
          </div>
        </section>
      ))}
    </div>
  );
}
