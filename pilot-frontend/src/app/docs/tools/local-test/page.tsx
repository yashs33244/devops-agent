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

const emulators = [
  {
    cloud: "aws",
    tool: "LocalStack",
    port: "4566",
    compose: "tools/emulators/localstack.yml",
  },
  {
    cloud: "azure",
    tool: "Azurite",
    port: "10000",
    compose: "tools/emulators/azurite.yml",
  },
  {
    cloud: "gcp",
    tool: "Firestore (8080) + Pub/Sub (8085)",
    port: "8080 / 8085",
    compose: "tools/emulators/gcp-emulators.yml",
  },
];

const awsEnvVars = [
  { key: "AWS_ENDPOINT_URL", value: "http://localhost:4566" },
  { key: "AWS_ACCESS_KEY_ID", value: "test" },
  { key: "AWS_SECRET_ACCESS_KEY", value: "test" },
  { key: "AWS_DEFAULT_REGION", value: "us-east-1" },
];

export default function LocalTestPage() {
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
        <span style={{ color: "var(--text-on-dark)" }}>Local Test</span>
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
        Emulator
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        local_test.py
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
        Starts the matching cloud emulator via Docker Compose, sets the correct environment variables,
        and runs <code className="mono" style={{ fontSize: "13px" }}>terraform plan</code> against
        the emulator to validate your infrastructure code without real cloud credentials or costs.
      </p>

      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Checks if the emulator container is already running and starts it if not. Then sets the
          appropriate environment variables and runs{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>terraform init</code> +{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>terraform plan</code>{" "}
          pointed at the emulator endpoint.
        </p>
      </Section>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/local_test.py \\
  --cloud aws \\
  --terraform-dir ./my-app/terraform \\
  --service payment-api`}</CodeBlock>
      </Section>

      <Section title="Emulators">
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
            {["Cloud", "Emulator", "Docker Compose file"].map((h) => (
              <span key={h} className="mono" style={{ fontSize: "10px", letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--text-on-dark-soft)" }}>
                {h}
              </span>
            ))}
          </div>
          {emulators.map((row, i) => (
            <div
              key={row.cloud}
              style={{
                display: "grid",
                gridTemplateColumns: "80px 1fr 1fr",
                padding: "12px 16px",
                borderBottom: i < emulators.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.cloud}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark)" }}>
                {row.tool}{" "}
                <span style={{ fontSize: "11px", color: "var(--text-on-dark-soft)" }}>:{row.port}</span>
              </span>
              <code className="mono" style={{ fontSize: "11px", color: "var(--text-on-dark-soft)" }}>{row.compose}</code>
            </div>
          ))}
        </div>

        <p style={{ fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6, margin: "0 0 12px", maxWidth: "66ch" }}>
          Start an emulator manually before running the tool:
        </p>
        <CodeBlock lang="bash">{`# AWS
docker compose -f tools/emulators/localstack.yml up -d

# Azure
docker compose -f tools/emulators/azurite.yml up -d

# GCP
docker compose -f tools/emulators/gcp-emulators.yml up -d`}</CodeBlock>
      </Section>

      <Section title="AWS Environment Variables (Set Automatically)">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {awsEnvVars.map((row, i) => (
            <div
              key={row.key}
              style={{
                display: "grid",
                gridTemplateColumns: "260px 1fr",
                padding: "10px 16px",
                borderBottom: i < awsEnvVars.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.key}</code>
              <code className="mono" style={{ fontSize: "12px", color: "var(--text-on-dark)" }}>{row.value}</code>
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
          Pro tip
        </div>
        <p style={{ margin: 0, fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6 }}>
          Use{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>tflocal</code>{" "}
          (LocalStack&apos;s Terraform wrapper) instead of plain{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>terraform</code>{" "}
          for more accurate AWS emulation. Install with{" "}
          <code className="mono" style={{ fontSize: "12px" }}>pip install terraform-local</code>.
        </p>
      </div>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools/cost-optimize"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          Next step: Cost Optimize →
        </Link>
      </div>
    </div>
  );
}
