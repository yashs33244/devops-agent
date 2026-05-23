import Link from "next/link";
import type { ReactNode } from "react";

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

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div style={{ marginBottom: "36px" }}>
      <div
        className="mono"
        style={{
          fontSize: "11px",
          letterSpacing: "0.1em",
          textTransform: "uppercase",
          color: "var(--text-on-dark-soft)",
          marginBottom: "14px",
          paddingBottom: "8px",
          borderBottom: "1px solid var(--line-dark)",
        }}
      >
        {title}
      </div>
      {children}
    </div>
  );
}

const stages = [
  {
    num: 1,
    name: "Dockerfile",
    tools: ["hadolint", "docker build", "container-structure-test", "trivy"],
    desc: "Lint, build, and scan the container image for HIGH/CRITICAL CVEs",
  },
  {
    num: 2,
    name: "Terraform",
    tools: ["terraform fmt", "tflint", "checkov", "terraform validate", "Terratest"],
    desc: "Format check, lint, security policy scan, syntax validation, and Go-based integration tests",
  },
  {
    num: 3,
    name: "Helm",
    tools: ["helm lint --strict", "kubectl dry-run", "helm-unittest"],
    desc: "Strict lint, server-side dry-run manifest validation, and unit tests",
  },
  {
    num: 4,
    name: "GitHub Actions",
    tools: ["act push --dry-run"],
    desc: "Local workflow runner dry-run to catch syntax and step errors before pushing",
  },
  {
    num: 5,
    name: "Integration",
    tools: ["kind", "helm install", "curl /health"],
    desc: "Spin up a local kind cluster, install the chart, and verify the /health endpoint responds",
  },
];

const optionalTools = [
  { tool: "hadolint", install: "brew install hadolint" },
  { tool: "trivy", install: "brew install trivy" },
  { tool: "tflint", install: "brew install tflint" },
  { tool: "checkov", install: "pip install checkov" },
  { tool: "act", install: "brew install act" },
  { tool: "helm-unittest", install: "helm plugin install https://github.com/helm-unittest/helm-unittest" },
];

export default function TestRunnerPage() {
  return (
    <div>
      {/* Breadcrumb */}
      <div
        style={{
          marginBottom: "32px",
          display: "flex",
          alignItems: "center",
          gap: "8px",
          fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          fontSize: "13px",
          color: "var(--text-on-dark-soft)",
        }}
      >
        <Link href="/docs" style={{ color: "var(--text-on-dark-soft)" }}>Docs</Link>
        <span>/</span>
        <Link href="/docs/tools" style={{ color: "var(--text-on-dark-soft)" }}>Tools</Link>
        <span>/</span>
        <span style={{ color: "var(--text-on-dark)" }}>Test Runner</span>
      </div>

      <span
        className="mono"
        style={{
          fontSize: "10px",
          padding: "3px 9px",
          border: "1px solid var(--line-dark-2)",
          color: "var(--text-on-dark-soft)",
          background: "var(--bg-dark-3)",
          borderRadius: "2px",
          letterSpacing: "0.08em",
          textTransform: "uppercase",
          display: "inline-block",
          marginBottom: "16px",
        }}
      >
        Testing
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        test_runner.py
      </h1>

      <p
        style={{
          fontSize: "15px",
          color: "var(--text-on-dark-soft)",
          margin: "0 0 40px",
          lineHeight: 1.65,
          maxWidth: "62ch",
          borderBottom: "1px solid var(--line-dark)",
          paddingBottom: "32px",
        }}
      >
        Runs 5 test stages in sequence — Dockerfile, Terraform, Helm, GitHub Actions, Integration.
        Each stage must pass before the next begins. Use <code className="mono" style={{ fontSize: "13px" }}>--only</code>{" "}
        to run a subset, <code className="mono" style={{ fontSize: "13px" }}>--fail-fast</code> to
        stop on first failure. Missing tools show as SKIPPED with install hints, not failures.
      </p>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/test_runner.py \\
  --service payment-api \\
  --repo-path ./my-app \\
  --terraform-dir ./my-app/terraform \\
  --helm-dir ./my-app/helm \\
  --cloud aws

# Run a subset of stages:
python3 tools/test_runner.py ... --only dockerfile,terraform

# Stop on first failure:
python3 tools/test_runner.py ... --fail-fast`}</CodeBlock>
      </Section>

      <Section title="5 Test Stages">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {stages.map((stage, i) => (
            <div
              key={stage.name}
              style={{
                padding: "16px 20px",
                borderBottom: i < stages.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "12px",
                  marginBottom: "8px",
                }}
              >
                <span
                  className="mono"
                  style={{
                    fontSize: "11px",
                    color: "var(--text-on-dark-soft)",
                    background: "var(--bg-dark-3)",
                    border: "1px solid var(--line-dark-2)",
                    borderRadius: "2px",
                    padding: "2px 7px",
                    flexShrink: 0,
                  }}
                >
                  {stage.num}
                </span>
                <span
                  style={{
                    fontWeight: 600,
                    fontSize: "14px",
                    color: "var(--text-on-dark)",
                  }}
                >
                  {stage.name}
                </span>
              </div>
              <p
                style={{
                  margin: "0 0 10px",
                  fontSize: "13px",
                  color: "var(--text-on-dark-soft)",
                  lineHeight: 1.5,
                  maxWidth: "60ch",
                }}
              >
                {stage.desc}
              </p>
              <div style={{ display: "flex", flexWrap: "wrap", gap: "6px" }}>
                {stage.tools.map((t) => (
                  <code
                    key={t}
                    className="mono"
                    style={{
                      fontSize: "11px",
                      padding: "2px 8px",
                      border: "1px solid var(--line-dark-2)",
                      background: "var(--bg-dark-3)",
                      borderRadius: "2px",
                      color: "var(--accent)",
                    }}
                  >
                    {t}
                  </code>
                ))}
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="SKIPPED vs FAILED">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          If an optional tool is not installed on the host, the corresponding check is marked{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--text-on-dark-soft)" }}>SKIPPED</code>{" "}
          with an installation hint printed to stdout. The stage does not fail because of a missing
          tool — only because of a failing test.
        </p>
      </Section>

      <Section title="Install Optional Tools">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
            marginBottom: "14px",
          }}
        >
          {optionalTools.map((row, i) => (
            <div
              key={row.tool}
              style={{
                display: "grid",
                gridTemplateColumns: "140px 1fr",
                padding: "9px 16px",
                borderBottom: i < optionalTools.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.tool}</code>
              <code className="mono" style={{ fontSize: "11px", color: "var(--text-on-dark-soft)" }}>{row.install}</code>
            </div>
          ))}
        </div>
        <CodeBlock lang="bash">{`# Quick install (macOS):
brew install hadolint trivy tflint act
pip install checkov
helm plugin install https://github.com/helm-unittest/helm-unittest`}</CodeBlock>
      </Section>

      <div
        style={{
          padding: "14px 18px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderLeft: "3px solid var(--accent)",
          borderRadius: "4px",
          marginBottom: "48px",
        }}
      >
        <div
          className="mono"
          style={{
            fontSize: "11px",
            letterSpacing: "0.08em",
            textTransform: "uppercase",
            color: "var(--text-on-dark-soft)",
            marginBottom: "8px",
          }}
        >
          Gate rule
        </div>
        <p style={{ margin: 0, fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6 }}>
          No step in the pipeline is declared &quot;done&quot; while{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>test_runner.py</code>{" "}
          reports failures. Fix failures before moving on — &quot;it should work&quot; is not a passing test.
        </p>
      </div>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          ← Back to Tools index
        </Link>
      </div>
    </div>
  );
}
