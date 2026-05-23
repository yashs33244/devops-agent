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

const cloudOptions = [
  { flag: "aws", resources: "EKS + ECR + RDS + VPC" },
  { flag: "azure", resources: "AKS + ACR + PostgreSQL Flexible Server" },
  { flag: "gcp", resources: "GKE Autopilot + Artifact Registry + Cloud SQL" },
];

const useCases = ["web_app", "microservice", "batch_job", "data_pipeline", "scheduled_task"];

const generatedFiles = [
  { file: "versions.tf", desc: "Required provider version constraints" },
  { file: "providers.tf", desc: "Provider configuration blocks" },
  { file: "variables.tf", desc: "Input variable declarations" },
  { file: "locals.tf", desc: "Computed local values and name prefixes" },
  { file: "backend.tf", desc: "Remote state backend (commented — see bootstrap note)" },
  { file: "main.tf", desc: "Core resource definitions" },
  { file: "outputs.tf", desc: "Output values exposed after apply" },
];

const stateBackends = [
  { cloud: "aws", backend: "S3 + DynamoDB lock table" },
  { cloud: "azure", backend: "Azure Storage Account + container" },
  { cloud: "gcp", backend: "GCS bucket" },
];

export default function TerraformPage() {
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
        <span style={{ color: "var(--text-on-dark)" }}>Terraform</span>
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
        IaC
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        terraform_gen.py
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
        Copies the correct cloud template to the output directory, substitutes variables, generates
        a <code className="mono" style={{ fontSize: "13px" }}>terraform.tfvars</code> file, then runs{" "}
        <code className="mono" style={{ fontSize: "13px" }}>terraform fmt</code> and{" "}
        <code className="mono" style={{ fontSize: "13px" }}>terraform validate</code> automatically.
      </p>

      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Selects the right template directory from{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>templates/terraform/&lt;cloud&gt;/</code>,
          substitutes placeholder tokens throughout all{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>.tf</code> files,
          and writes 7 split files to the output directory.
        </p>
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: 0, maxWidth: "66ch" }}>
          Template variables substituted: <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{SERVICE_NAME}}"}</code>,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{REGION}}"}</code>,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{ENVIRONMENT}}"}</code>,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{CLUSTER_NAME}}"}</code>.
        </p>
      </Section>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/terraform_gen.py \\
  --cloud aws \\
  --service payment-api \\
  --use-case web_app \\
  --region us-east-1 \\
  --env dev`}</CodeBlock>
      </Section>

      <Section title="Cloud Options">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "120px 1fr",
              padding: "8px 16px",
              background: "var(--bg-dark-3)",
              borderBottom: "1px solid var(--line-dark)",
              gap: "16px",
            }}
          >
            {["--cloud", "Resources provisioned"].map((h) => (
              <span key={h} className="mono" style={{ fontSize: "10px", letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--text-on-dark-soft)" }}>
                {h}
              </span>
            ))}
          </div>
          {cloudOptions.map((row, i) => (
            <div
              key={row.flag}
              style={{
                display: "grid",
                gridTemplateColumns: "120px 1fr",
                padding: "10px 16px",
                borderBottom: i < cloudOptions.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.flag}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{row.resources}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Use Case Options">
        <div style={{ display: "flex", flexWrap: "wrap", gap: "8px" }}>
          {useCases.map((uc) => (
            <code
              key={uc}
              className="mono"
              style={{
                fontSize: "12px",
                padding: "4px 10px",
                border: "1px solid var(--line-dark-2)",
                background: "var(--bg-dark-3)",
                borderRadius: "2px",
                color: "var(--text-on-dark-soft)",
              }}
            >
              {uc}
            </code>
          ))}
        </div>
      </Section>

      <Section title="Generated Files">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {generatedFiles.map((row, i) => (
            <div
              key={row.file}
              style={{
                display: "grid",
                gridTemplateColumns: "180px 1fr",
                padding: "10px 16px",
                borderBottom: i < generatedFiles.length - 1 ? "1px solid var(--line-dark)" : "none",
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

      <Section title="Remote State Backends">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
            marginBottom: "14px",
          }}
        >
          {stateBackends.map((row, i) => (
            <div
              key={row.cloud}
              style={{
                display: "grid",
                gridTemplateColumns: "100px 1fr",
                padding: "10px 16px",
                borderBottom: i < stateBackends.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.cloud}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{row.backend}</span>
            </div>
          ))}
        </div>
        <p style={{ fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6, margin: 0, maxWidth: "66ch" }}>
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>backend.tf</code> is
          generated with the remote state block commented out. Bootstrap the state bucket/container first,
          then uncomment and run <code className="mono" style={{ fontSize: "12px" }}>terraform init</code>.
        </p>
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
          Post-generation (automatic)
        </div>
        <p style={{ margin: 0, fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6 }}>
          After writing all <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>.tf</code> files,
          the tool runs <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>terraform fmt</code> to
          normalise formatting, then{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>terraform validate -backend=false</code> to
          catch syntax and type errors without requiring real cloud credentials.
        </p>
      </div>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools/helm"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          Next step: Helm →
        </Link>
      </div>
    </div>
  );
}
