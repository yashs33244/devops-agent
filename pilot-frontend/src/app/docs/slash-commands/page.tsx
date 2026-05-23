import Link from "next/link";

interface CommandDef {
  cmd: string;
  desc: string;
  inputs: string;
  outputs: string;
}

const infraCommands: CommandDef[] = [
  {
    cmd: "/deploy",
    desc: "Full pipeline orchestrator — runs every step from clone to CI/CD setup",
    inputs: "Service name, repo URL, cloud provider, region, environment, KEDA preference",
    outputs: "Dockerfile, Terraform, Helm chart, CI/CD workflows, ESO secrets, cost estimate",
  },
  {
    cmd: "/dockerize",
    desc: "Generate or validate a Dockerfile for the detected language/framework",
    inputs: "Repo path or URL, service name",
    outputs: "Dockerfile at repo root, hadolint + trivy scan results",
  },
  {
    cmd: "/terraform",
    desc: "Generate cloud-specific Terraform for the service and environment",
    inputs: "Cloud provider, service name, use case, region, environment",
    outputs: "terraform/ directory with main.tf, variables.tf, outputs.tf; runs fmt + validate",
  },
  {
    cmd: "/helm",
    desc: "Generate a production-ready Helm chart with security contexts and probes",
    inputs: "Service name, cloud provider, container port",
    outputs: "helm/ directory; runs helm lint and helm unittest",
  },
  {
    cmd: "/secrets",
    desc: "Scan repo for secrets and generate ESO manifests or sealed secrets",
    inputs: "Repo path, service name, cloud provider, output directory",
    outputs: "ESO ClusterSecretStore + ExternalSecret manifests, github-secrets.md checklist",
  },
  {
    cmd: "/test",
    desc: "Run all validation layers: Dockerfile, Terraform, Helm, GitHub Actions, integration",
    inputs: "Service name, repo path, terraform dir, helm dir, cloud provider",
    outputs: "Pass/fail report for each layer; exits non-zero if any test fails",
  },
  {
    cmd: "/local-test",
    desc: "Validate Terraform against a local cloud emulator (LocalStack / Azurite / GCP emulators)",
    inputs: "Cloud provider, terraform directory, service name",
    outputs: "Emulator apply results, resource inventory, teardown confirmation",
  },
  {
    cmd: "/optimize-cost",
    desc: "Apply KEDA scale-to-zero configuration to the Helm chart",
    inputs: "Terraform directory, platform (eks / aks / gke)",
    outputs: "KEDA HTTPScaledObject manifest, updated Helm values, estimated monthly savings",
  },
  {
    cmd: "/audit",
    desc: "Audit existing Terraform, Helm, and Dockerfiles against the security checklist",
    inputs: "Repo path (must contain existing infrastructure files)",
    outputs: "Prioritized findings (CRITICAL / HIGH / MEDIUM / LOW), diff suggestions",
  },
  {
    cmd: "/status",
    desc: "Show the current state of all generated artifacts for a service",
    inputs: "Service name, workspace path",
    outputs: "Summary table of generated files, last-run test results, cost estimate",
  },
];

const agentCommands: CommandDef[] = [
  {
    cmd: "/holmesgpt",
    desc: "Query HolmesGPT with a plain-English incident question",
    inputs: 'Incident question in quotes, e.g. "Why are pods crash-looping in namespace X?"',
    outputs: "Root cause analysis, ranked hypotheses, relevant log excerpts",
  },
  {
    cmd: "/kagent",
    desc: "Execute a fleet operation via kagent's intent API",
    inputs: "Intent description, target namespace or cluster, dry-run flag",
    outputs: "KAgentTask YAML applied to cluster, execution plan, confirmation prompt before mutation",
  },
  {
    cmd: "/nightshift",
    desc: "Configure scale-down / scale-up schedules for non-production namespaces",
    inputs: "Namespace or label selector, cron expressions for down/up, timezone",
    outputs: "Nightshift annotation applied to namespace, projected monthly savings",
  },
  {
    cmd: "/opensre",
    desc: "Run or list available OpenSRE runbooks",
    inputs: "Runbook name or alert label selector, variable overrides",
    outputs: "Step-by-step runbook execution with live output, audit log entry",
  },
  {
    cmd: "/plural",
    desc: "Manage Plural deployments, promotions, and release status",
    inputs: "Action (build / deploy / promote), target context, commit message",
    outputs: "Plural CLI output, deployment status, drift report",
  },
  {
    cmd: "/sre-guard",
    desc: "Run a one-shot SRE Guard health check or configure the monitoring daemon",
    inputs: "Service name, namespace, action (check / configure / status)",
    outputs: "Health check report, SLO compliance status, runbook trigger log",
  },
];

function CommandTable({ commands }: { commands: CommandDef[] }) {
  return (
    <div style={{ border: "1px solid var(--line-dark)", borderRadius: "4px", overflow: "hidden" }}>
      {commands.map((cmd, i) => (
        <div
          key={cmd.cmd}
          id={cmd.cmd.replace("/", "")}
          style={{
            borderBottom: i < commands.length - 1 ? "1px solid var(--line-dark)" : "none",
            padding: "20px 22px",
            background: i % 2 === 0 ? "var(--bg-dark-2)" : "transparent",
            scrollMarginTop: "80px",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "flex-start",
              justifyContent: "space-between",
              gap: "16px",
              marginBottom: "10px",
            }}
          >
            <code
              className="mono"
              style={{
                fontSize: "14px",
                color: "var(--accent)",
                fontWeight: 500,
                flexShrink: 0,
              }}
            >
              {cmd.cmd}
            </code>
          </div>
          <p
            style={{
              margin: "0 0 14px",
              fontSize: "13px",
              color: "var(--text-on-dark)",
              lineHeight: 1.5,
            }}
          >
            {cmd.desc}
          </p>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "12px",
            }}
          >
            <div>
              <div
                className="mono"
                style={{
                  fontSize: "10px",
                  letterSpacing: "0.1em",
                  textTransform: "uppercase",
                  color: "var(--text-on-dark-soft)",
                  opacity: 0.6,
                  marginBottom: "4px",
                }}
              >
                Inputs
              </div>
              <div style={{ fontSize: "12px", color: "var(--text-on-dark-soft)", lineHeight: 1.5 }}>
                {cmd.inputs}
              </div>
            </div>
            <div>
              <div
                className="mono"
                style={{
                  fontSize: "10px",
                  letterSpacing: "0.1em",
                  textTransform: "uppercase",
                  color: "var(--text-on-dark-soft)",
                  opacity: 0.6,
                  marginBottom: "4px",
                }}
              >
                Outputs
              </div>
              <div style={{ fontSize: "12px", color: "var(--text-on-dark-soft)", lineHeight: 1.5 }}>
                {cmd.outputs}
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export default function SlashCommandsPage() {
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
        Reference
      </div>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(36px, 4vw, 54px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        Slash Commands
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
        16 commands built directly into Claude Code. Type any of these inside a Claude Code
        session opened on the devops-agent directory.
      </p>

      {/* Quick jump strip */}
      <div
        style={{
          display: "flex",
          flexWrap: "wrap",
          gap: "6px",
          marginBottom: "48px",
          padding: "16px 18px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderRadius: "4px",
        }}
      >
        {[...infraCommands, ...agentCommands].map((c) => (
          <a
            key={c.cmd}
            href={`#${c.cmd.replace("/", "")}`}
            className="mono"
            style={{
              fontSize: "12px",
              color: "var(--accent)",
              padding: "3px 8px",
              border: "1px solid var(--line-dark)",
              borderRadius: "2px",
              background: "var(--bg-dark-3)",
              transition: "border-color .15s, color .15s",
            }}
          >
            {c.cmd}
          </a>
        ))}
      </div>

      {/* Infrastructure commands */}
      <section style={{ marginBottom: "56px" }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "12px",
            marginBottom: "20px",
          }}
        >
          <h2
            className="h-sans"
            style={{
              fontSize: "16px",
              margin: 0,
              color: "var(--text-on-dark)",
              letterSpacing: "-0.02em",
            }}
          >
            Infrastructure
          </h2>
          <span
            className="mono"
            style={{
              fontSize: "11px",
              padding: "2px 8px",
              border: "1px solid var(--line-dark-2)",
              color: "var(--text-on-dark-soft)",
              background: "var(--bg-dark-3)",
              borderRadius: "2px",
            }}
          >
            10 commands
          </span>
        </div>
        <CommandTable commands={infraCommands} />
      </section>

      {/* Agent commands */}
      <section style={{ marginBottom: "56px" }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "12px",
            marginBottom: "20px",
          }}
        >
          <h2
            className="h-sans"
            style={{
              fontSize: "16px",
              margin: 0,
              color: "var(--text-on-dark)",
              letterSpacing: "-0.02em",
            }}
          >
            Agents
          </h2>
          <span
            className="mono"
            style={{
              fontSize: "11px",
              padding: "2px 8px",
              border: "1px solid var(--line-dark-2)",
              color: "var(--text-on-dark-soft)",
              background: "var(--bg-dark-3)",
              borderRadius: "2px",
            }}
          >
            6 commands
          </span>
        </div>
        <CommandTable commands={agentCommands} />
      </section>

      {/* Usage note */}
      <div
        style={{
          padding: "20px 22px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderLeft: "3px solid var(--accent)",
          borderRadius: "4px",
        }}
      >
        <div
          style={{
            fontWeight: 500,
            fontSize: "13px",
            color: "var(--text-on-dark)",
            marginBottom: "8px",
          }}
        >
          How slash commands work
        </div>
        <p
          style={{
            margin: 0,
            fontSize: "13px",
            color: "var(--text-on-dark-soft)",
            lineHeight: 1.6,
            maxWidth: "60ch",
          }}
        >
          Slash commands are defined in{" "}
          <code
            className="mono"
            style={{
              fontSize: "12px",
              background: "var(--bg-dark-3)",
              padding: "1px 5px",
              border: "1px solid var(--line-dark)",
              borderRadius: "2px",
            }}
          >
            .claude/commands/
          </code>
          . Each file is a Markdown prompt template that Claude Code loads automatically.
          They call the Python tools in{" "}
          <code
            className="mono"
            style={{
              fontSize: "12px",
              background: "var(--bg-dark-3)",
              padding: "1px 5px",
              border: "1px solid var(--line-dark)",
              borderRadius: "2px",
            }}
          >
            tools/
          </code>{" "}
          under the hood — so everything can also be run directly without Claude Code.
        </p>
      </div>
    </div>
  );
}
