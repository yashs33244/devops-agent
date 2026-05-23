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

const tools = [
  {
    slug: "dockerize",
    name: "dockerize.py",
    desc: "Language detection, multi-stage Dockerfile generation, hadolint + Trivy validation.",
  },
  {
    slug: "terraform",
    name: "terraform_gen.py",
    desc: "Cloud-specific Terraform scaffolding for EKS, AKS, and GKE with auto fmt + validate.",
  },
  {
    slug: "helm",
    name: "helm_gen.py",
    desc: "Production Helm chart with security contexts, KEDA, ESO, and Prometheus integration.",
  },
  {
    slug: "cicd",
    name: "cicd_setup.py",
    desc: "OIDC-native GitHub Actions CI + CD pipelines. SHA-pinned, no static credentials.",
  },
  {
    slug: "secrets",
    name: "secrets_manager.py",
    desc: "Repo secret scanner with ESO manifest generation and IRSA/Workload Identity routing.",
  },
  {
    slug: "local-test",
    name: "local_test.py",
    desc: "LocalStack / Azurite / GCP emulator harness for offline Terraform validation.",
  },
  {
    slug: "cost-optimize",
    name: "cost_optimize.py",
    desc: "Car-painter KEDA scale-to-zero applier. Typical saving: 60–90% compute cost.",
  },
  {
    slug: "test-runner",
    name: "test_runner.py",
    desc: "Five-stage test suite: Dockerfile, Terraform, Helm, GitHub Actions, Integration.",
  },
];

export default function ToolsIndexPage() {
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

      {/* Eyebrow */}
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
        CLI Tools
      </div>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(36px, 4vw, 54px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        Tools
      </h1>

      <p
        style={{
          fontSize: "16px",
          color: "var(--text-on-dark-soft)",
          margin: "0 0 12px",
          lineHeight: 1.6,
          maxWidth: "58ch",
        }}
      >
        Orchestration CLIs for every step of the DevOps pipeline.
      </p>

      <p
        style={{
          fontSize: "13px",
          color: "var(--text-on-dark-soft)",
          margin: "0 0 40px",
          lineHeight: 1.6,
          maxWidth: "58ch",
          borderBottom: "1px solid var(--line-dark)",
          paddingBottom: "32px",
          fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
        }}
      >
        Tools are designed to run in sequence via{" "}
        <code
          style={{
            color: "var(--accent)",
            background: "var(--bg-dark-3)",
            padding: "1px 6px",
            borderRadius: "2px",
            fontSize: "12px",
          }}
        >
          workflow.py
        </code>
        , or individually.
      </p>

      {/* 2×4 grid */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(2, 1fr)",
          gap: "1px",
          background: "var(--line-dark)",
          border: "1px solid var(--line-dark)",
          borderRadius: "4px",
          overflow: "hidden",
        }}
      >
        {tools.map((tool) => (
          <Link
            key={tool.slug}
            href={`/docs/tools/${tool.slug}`}
            className="tool-card"
            style={{
              display: "block",
              padding: "22px 24px",
              background: "var(--bg-dark-2)",
              borderLeft: "2px solid transparent",
              transition: "background .15s, border-color .15s",
            }}
          >
            <div
              style={{
                fontWeight: 600,
                fontSize: "15px",
                color: "var(--text-on-dark)",
                marginBottom: "6px",
                fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
                letterSpacing: "-0.01em",
              }}
            >
              {tool.name}
            </div>
            <p
              style={{
                margin: "0 0 14px",
                fontSize: "13px",
                color: "var(--text-on-dark-soft)",
                lineHeight: 1.5,
              }}
            >
              {tool.desc}
            </p>
            <span
              style={{
                fontSize: "12px",
                color: "var(--accent)",
                fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
              }}
            >
              View docs →
            </span>
          </Link>
        ))}
      </div>

      <style>{`
        .tool-card:hover {
          background: var(--bg-dark-3) !important;
          border-left-color: var(--accent) !important;
        }
      `}</style>

      {/* Execution order note */}
      <div
        style={{
          marginTop: "40px",
          padding: "16px 20px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderLeft: "3px solid var(--accent)",
          borderRadius: "4px",
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
          Execution Order
        </div>
        <CodeBlock lang="bash">{`python3 tools/workflow.py \\
  --service payment-api \\
  --repo ./my-app \\
  --cloud aws \\
  --region us-east-1 \\
  --env dev`}</CodeBlock>
        <p
          style={{
            margin: 0,
            fontSize: "13px",
            color: "var(--text-on-dark-soft)",
            lineHeight: 1.6,
          }}
        >
          <code
            className="mono"
            style={{ color: "var(--accent)", fontSize: "12px" }}
          >
            workflow.py
          </code>{" "}
          orchestrates all 8 tools in sequence: Dockerize → Secrets → Terraform → Helm → CI/CD → Test → Local Test → Cost Optimize.
          Each step must pass before the next begins.
        </p>
      </div>
    </div>
  );
}
