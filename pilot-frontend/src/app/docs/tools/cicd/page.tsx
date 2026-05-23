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

const oidcSetup = [
  {
    cloud: "aws",
    action: "aws-actions/configure-aws-credentials",
    key: "role-to-assume",
    value: "arn:aws:iam::ACCOUNT:role/github-oidc-role",
  },
  {
    cloud: "azure",
    action: "azure/login",
    key: "client-id, tenant-id, subscription-id",
    value: "GitHub environment variables (not secrets)",
  },
  {
    cloud: "gcp",
    action: "google-github-actions/auth",
    key: "workload_identity_provider",
    value: "projects/PROJECT/locations/global/workloadIdentityPools/...",
  },
];

const ciJobs = [
  { name: "lint", desc: "ESLint / pylint / golangci-lint depending on language detected" },
  { name: "test", desc: "Unit tests with coverage upload to Codecov" },
  { name: "docker-build", desc: "Multi-stage build, Docker layer caching via GitHub cache" },
  { name: "trivy-scan", desc: "HIGH/CRITICAL CVE scan — fails the workflow if any found" },
  { name: "push", desc: "Push to ECR / ACR / Artifact Registry (only on main or tag)" },
];

const cdJobs = [
  { name: "terraform-plan", desc: "Runs on every PR — posts plan summary as PR comment" },
  { name: "terraform-apply", desc: "Runs on merge to main — applies the plan" },
  { name: "helm-upgrade", desc: "helm upgrade --install --atomic --wait --timeout 5m" },
];

export default function CicdPage() {
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
        <span style={{ color: "var(--text-on-dark)" }}>CI/CD</span>
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
        Automation
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        cicd_setup.py
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
        Generates <code className="mono" style={{ fontSize: "13px" }}>.github/workflows/ci.yml</code> and{" "}
        <code className="mono" style={{ fontSize: "13px" }}>cd.yml</code> with OIDC federation, SHA-pinned
        actions, per-job minimum permissions, and concurrency cancel-in-progress. No static cloud
        credentials in GitHub Secrets — ever.
      </p>

      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Selects the correct OIDC authentication action for the target cloud, generates two workflow files,
          and writes a <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>SECRETS.md</code> listing
          every GitHub secret and repository variable that needs to be configured.
        </p>
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: 0, maxWidth: "66ch" }}>
          Every action step is pinned to a full commit SHA, not a floating tag. Permissions follow
          least-privilege: each job only gets the permissions it needs.
        </p>
      </Section>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/cicd_setup.py \\
  --repo-path ./my-app \\
  --cloud aws \\
  --service payment-api`}</CodeBlock>
      </Section>

      <Section title="OIDC Setup (No Static Credentials)">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
            marginBottom: "14px",
          }}
        >
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "80px 1fr 1fr",
              padding: "8px 16px",
              background: "var(--bg-dark-3)",
              borderBottom: "1px solid var(--line-dark)",
              gap: "16px",
            }}
          >
            {["Cloud", "Action Used", "Key Config"].map((h) => (
              <span key={h} className="mono" style={{ fontSize: "10px", letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--text-on-dark-soft)" }}>
                {h}
              </span>
            ))}
          </div>
          {oidcSetup.map((row, i) => (
            <div
              key={row.cloud}
              style={{
                display: "grid",
                gridTemplateColumns: "80px 1fr 1fr",
                padding: "12px 16px",
                borderBottom: i < oidcSetup.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "start",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.cloud}</code>
              <code className="mono" style={{ fontSize: "11px", color: "var(--text-on-dark)" }}>{row.action}</code>
              <span style={{ fontSize: "12px", color: "var(--text-on-dark-soft)" }}>{row.key}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="CI Pipeline Jobs">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {ciJobs.map((job, i) => (
            <div
              key={job.name}
              style={{
                display: "grid",
                gridTemplateColumns: "140px 1fr",
                padding: "10px 16px",
                borderBottom: i < ciJobs.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{job.name}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{job.desc}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="CD Pipeline Jobs">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {cdJobs.map((job, i) => (
            <div
              key={job.name}
              style={{
                display: "grid",
                gridTemplateColumns: "180px 1fr",
                padding: "10px 16px",
                borderBottom: i < cdJobs.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{job.name}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{job.desc}</span>
            </div>
          ))}
        </div>
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
          Also generated
        </div>
        <p style={{ margin: 0, fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6 }}>
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>SECRETS.md</code> — a
          checklist of every GitHub Actions secret and repository variable to configure, including the OIDC
          role ARN / client ID and the container registry URL. Nothing is left for you to guess.
        </p>
      </div>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools/secrets"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          Next step: Secrets Manager →
        </Link>
      </div>
    </div>
  );
}
