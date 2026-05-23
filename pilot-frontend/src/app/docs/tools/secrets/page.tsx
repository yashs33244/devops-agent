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

const outputFiles = [
  { file: "external-secret.yaml", desc: "ESO ExternalSecret manifest — pulls from cloud secrets manager" },
  { file: "secret-template.yaml", desc: "Native K8s Secret template for dev-only use" },
  { file: "github-secrets.md", desc: "Checklist of GitHub Actions secrets/variables to configure" },
  { file: "secrets-checklist.md", desc: "Human-readable audit checklist with management strategy per secret" },
];

const patternTypes = [
  "Database connection URLs (postgres://, mysql://, mongodb://)",
  "Cache URLs (redis://, rediss://)",
  "AWS cloud credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)",
  "Azure credentials (AZURE_CLIENT_SECRET, AZURE_CLIENT_ID)",
  "GCP credentials (GOOGLE_APPLICATION_CREDENTIALS, GCP_SA_KEY)",
  "Generic API keys (API_KEY, APIKEY, _TOKEN patterns)",
  "Application secrets (SECRET_KEY, APP_SECRET, JWT_SECRET)",
  "Third-party OAuth tokens (GITHUB_TOKEN, SLACK_TOKEN, STRIPE_SECRET)",
  "Database passwords (DB_PASSWORD, DATABASE_PASSWORD)",
  "Private keys (BEGIN RSA PRIVATE KEY, BEGIN PRIVATE KEY)",
  "Connection strings (.env: CONN_STRING, CONNECTION_STRING)",
  "Webhook secrets (WEBHOOK_SECRET, SIGNING_SECRET)",
];

const decisionTree = [
  {
    condition: "Cloud credential (AWS_*, AZURE_*, GCP_*)",
    strategy: "IRSA / Workload Identity",
    note: "Never use static access keys",
    color: "var(--danger)",
  },
  {
    condition: "Dynamic / rotatable secret",
    strategy: "External Secrets Operator + cloud secrets manager",
    note: "AWS Secrets Manager / Azure Key Vault / GCP Secret Manager",
    color: "var(--accent)",
  },
  {
    condition: "Static, changes rarely",
    strategy: "Sealed Secrets",
    note: "Encrypted in Git — safe to commit",
    color: "var(--good)",
  },
  {
    condition: "Dev only",
    strategy: "K8s native Secret",
    note: "Acceptable in dev — do NOT use in prod",
    color: "var(--text-on-dark-soft)",
  },
];

export default function SecretsPage() {
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
        <span style={{ color: "var(--text-on-dark)" }}>Secrets</span>
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
        Security
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        secrets_manager.py
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
        Scans your repository for 12 categories of secret pattern, interactively confirms each
        finding, routes them to the correct management strategy, and generates ESO manifests and
        audit checklists ready to commit.
      </p>

      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Walks{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>.env.example</code>,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>docker-compose.yml</code>,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>k8s/**/*.yaml</code>, and{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>helm/**/values.yaml</code>{" "}
          matching 12 regex patterns. For each match it confirms interactively and then applies
          the appropriate management strategy from the decision tree below.
        </p>
      </Section>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/secrets_manager.py \\
  --repo-path ./my-app \\
  --service payment-api \\
  --cloud aws \\
  --output-dir ./my-app/secrets \\
  --helm-dir ./my-app/helm

# Non-interactive (CI):
python3 tools/secrets_manager.py ... --non-interactive`}</CodeBlock>
      </Section>

      <Section title="Decision Tree">
        <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
          {decisionTree.map((item) => (
            <div
              key={item.condition}
              style={{
                padding: "14px 18px",
                background: "var(--bg-dark-2)",
                border: "1px solid var(--line-dark)",
                borderLeft: `3px solid ${item.color}`,
                borderRadius: "4px",
              }}
            >
              <div
                className="mono"
                style={{
                  fontSize: "12px",
                  color: item.color,
                  marginBottom: "6px",
                  fontWeight: 600,
                }}
              >
                {item.condition}
              </div>
              <div style={{ fontSize: "13px", color: "var(--text-on-dark)", marginBottom: "4px", fontWeight: 500 }}>
                {item.strategy}
              </div>
              <div style={{ fontSize: "12px", color: "var(--text-on-dark-soft)" }}>
                {item.note}
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Output Files">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {outputFiles.map((row, i) => (
            <div
              key={row.file}
              style={{
                display: "grid",
                gridTemplateColumns: "220px 1fr",
                padding: "10px 16px",
                borderBottom: i < outputFiles.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.file}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{row.desc}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Detected Pattern Types">
        <ul
          style={{
            margin: 0,
            padding: "0 0 0 18px",
            fontSize: "13px",
            color: "var(--text-on-dark-soft)",
            lineHeight: 2,
            columns: 2,
            columnGap: "32px",
          }}
        >
          {patternTypes.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      </Section>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools/local-test"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          Next step: Local Test →
        </Link>
      </div>
    </div>
  );
}
