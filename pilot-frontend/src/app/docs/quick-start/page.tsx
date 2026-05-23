import Link from "next/link";

function CodeBlock({
  lang,
  children,
}: {
  lang: string;
  children: string;
}) {
  return (
    <div
      style={{
        border: "1px solid var(--line-dark)",
        borderRadius: "4px",
        overflow: "hidden",
        margin: "16px 0",
      }}
    >
      <div
        style={{
          padding: "8px 14px",
          background: "var(--bg-dark-3)",
          borderBottom: "1px solid var(--line-dark)",
          display: "flex",
          alignItems: "center",
          gap: "8px",
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
          fontSize: "13px",
          lineHeight: "1.6",
          color: "var(--text-on-dark)",
          overflowX: "auto",
          whiteSpace: "pre",
        }}
      >
        <code>{children}</code>
      </pre>
    </div>
  );
}

function SectionHeading({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <h2
      id={id}
      style={{
        fontFamily: "var(--font-instrument-serif, 'Instrument Serif', serif)",
        fontWeight: 400,
        fontSize: "28px",
        letterSpacing: "-0.02em",
        margin: "48px 0 16px",
        color: "var(--text-on-dark)",
        scrollMarginTop: "80px",
      }}
    >
      {children}
    </h2>
  );
}

const generatedFiles = [
  { file: "Dockerfile", desc: "Multi-stage, nonroot, distroless base" },
  { file: "terraform/main.tf", desc: "VPC, cluster, registry (AWS / GCP / Azure)" },
  { file: "terraform/variables.tf", desc: "Parameterized for dev / staging / prod" },
  { file: "helm/Chart.yaml", desc: "Production Helm chart with security contexts" },
  { file: "helm/values.yaml", desc: "Resource limits, probes, autoscaling" },
  { file: ".github/workflows/ci.yml", desc: "Lint + test + build on every push" },
  { file: ".github/workflows/cd.yml", desc: "Push to registry + helm upgrade on merge" },
  { file: "secrets/eso-manifests/", desc: "External Secrets Operator manifests" },
  { file: "helm/templates/keda-scaler.yaml", desc: "KEDA HTTPScaledObject (if --with-keda)" },
];

export default function QuickStartPage() {
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
        Getting Started
      </div>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(36px, 4vw, 54px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        Quick Start
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
        Get your app deployed to any cloud in under 10 minutes. No YAML wrangling, no
        cloud console clicking — just one command or slash command in Claude Code.
      </p>

      {/* Prerequisites */}
      <SectionHeading id="prerequisites">Prerequisites</SectionHeading>
      <ul
        style={{
          margin: "0 0 8px",
          padding: "0 0 0 20px",
          color: "var(--text-on-dark-soft)",
          fontSize: "14px",
          lineHeight: 2,
        }}
      >
        <li>Node.js 22+ (for the frontend)</li>
        <li>Python 3.12+ (for the tools)</li>
        <li>Docker (for building and testing images)</li>
        <li>Git</li>
        <li>
          <a
            href="https://docs.anthropic.com/en/docs/claude-code"
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: "var(--accent)" }}
          >
            Claude Code CLI
          </a>{" "}
          (for slash commands)
        </li>
      </ul>

      {/* Installation */}
      <SectionHeading id="installation">Installation</SectionHeading>
      <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", margin: "0 0 8px", lineHeight: 1.6 }}>
        Clone the repo and install Python dependencies:
      </p>
      <CodeBlock lang="bash">{`git clone https://github.com/yashs33244/devops-agent
cd devops-agent
pip install -r requirements.txt`}</CodeBlock>

      {/* Your first deploy */}
      <SectionHeading id="first-deploy">Your first deploy</SectionHeading>
      <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", margin: "0 0 24px", lineHeight: 1.6 }}>
        The fastest path is through Claude Code slash commands. Open the project in Claude
        Code and follow these steps:
      </p>

      {[
        {
          n: "01",
          title: "Open in Claude Code",
          body: "Navigate to your devops-agent directory and open Claude Code. Pilot's CLAUDE.md is loaded automatically.",
        },
        {
          n: "02",
          title: 'Type "/deploy"',
          body: (
            <>
              Run the{" "}
              <code
                className="mono"
                style={{
                  fontSize: "12px",
                  background: "var(--bg-dark-3)",
                  padding: "2px 6px",
                  border: "1px solid var(--line-dark)",
                  borderRadius: "2px",
                }}
              >
                /deploy
              </code>{" "}
              slash command. Pilot will start gathering requirements.
            </>
          ),
        },
        {
          n: "03",
          title: "Answer the prompts",
          body: "Pilot asks for: service name, GitHub repo URL, cloud provider (aws / gcp / azure), region, environment (dev / staging / prod), and optional KEDA scale-to-zero.",
        },
        {
          n: "04",
          title: "Pilot generates everything",
          body: "Dockerfile + Terraform + Helm chart + CI/CD workflows + ESO secret manifests are written to your workspace. Every file is linted and validated before being declared ready.",
        },
        {
          n: "05",
          title: "Push to GitHub — CI runs automatically",
          body: "The generated CI workflow triggers on push. On merge to main, CD deploys to your cluster via helm upgrade. No manual steps.",
        },
      ].map((step) => (
        <div
          key={step.n}
          style={{
            display: "flex",
            gap: "20px",
            marginBottom: "4px",
            padding: "20px 22px",
            background: "var(--bg-dark-2)",
            border: "1px solid var(--line-dark)",
            borderBottom: "none",
          }}
        >
          <span
            className="mono"
            style={{
              fontSize: "11px",
              color: "var(--text-on-dark-soft)",
              opacity: 0.5,
              letterSpacing: "0.1em",
              flexShrink: 0,
              paddingTop: "2px",
            }}
          >
            {step.n}
          </span>
          <div>
            <div
              style={{
                fontWeight: 500,
                fontSize: "14px",
                marginBottom: "6px",
                color: "var(--text-on-dark)",
                letterSpacing: "-0.01em",
              }}
            >
              {step.title}
            </div>
            <div
              style={{
                fontSize: "13px",
                color: "var(--text-on-dark-soft)",
                lineHeight: 1.55,
              }}
            >
              {step.body}
            </div>
          </div>
        </div>
      ))}
      <div
        style={{
          height: "1px",
          background: "var(--line-dark)",
          marginBottom: "40px",
        }}
      />

      {/* Run directly */}
      <SectionHeading id="configuration">Or run directly</SectionHeading>
      <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", margin: "0 0 8px", lineHeight: 1.6 }}>
        Bypass Claude Code and call the workflow orchestrator directly:
      </p>
      <CodeBlock lang="bash">{`python3 tools/workflow.py \\
  --repo https://github.com/your-org/your-app \\
  --service my-service \\
  --cloud aws \\
  --env dev \\
  --with-keda`}</CodeBlock>

      <p
        style={{
          fontSize: "14px",
          color: "var(--text-on-dark-soft)",
          lineHeight: 1.6,
          margin: "0 0 8px",
        }}
      >
        For individual steps, use the specific tool directly. Example — generate only Terraform:
      </p>
      <CodeBlock lang="bash">{`python3 tools/terraform_gen.py \\
  --cloud aws \\
  --service my-service \\
  --use-case web_app \\
  --region us-east-1 \\
  --env dev`}</CodeBlock>

      {/* What gets generated */}
      <SectionHeading id="generated-files">What gets generated</SectionHeading>
      <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", margin: "0 0 16px", lineHeight: 1.6 }}>
        Every run of{" "}
        <code
          className="mono"
          style={{
            fontSize: "12px",
            background: "var(--bg-dark-3)",
            padding: "2px 6px",
            border: "1px solid var(--line-dark)",
            borderRadius: "2px",
          }}
        >
          workflow.py
        </code>{" "}
        or{" "}
        <code
          className="mono"
          style={{
            fontSize: "12px",
            background: "var(--bg-dark-3)",
            padding: "2px 6px",
            border: "1px solid var(--line-dark)",
            borderRadius: "2px",
          }}
        >
          /deploy
        </code>{" "}
        produces:
      </p>
      <table
        style={{
          width: "100%",
          borderCollapse: "collapse",
          fontSize: "13px",
          border: "1px solid var(--line-dark)",
        }}
      >
        <thead>
          <tr style={{ background: "var(--bg-dark-3)" }}>
            <th
              style={{
                padding: "10px 16px",
                textAlign: "left",
                fontWeight: 500,
                color: "var(--text-on-dark-soft)",
                borderBottom: "1px solid var(--line-dark)",
                fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
                fontSize: "11px",
                letterSpacing: "0.08em",
              }}
            >
              FILE / PATH
            </th>
            <th
              style={{
                padding: "10px 16px",
                textAlign: "left",
                fontWeight: 500,
                color: "var(--text-on-dark-soft)",
                borderBottom: "1px solid var(--line-dark)",
                fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
                fontSize: "11px",
                letterSpacing: "0.08em",
              }}
            >
              DESCRIPTION
            </th>
          </tr>
        </thead>
        <tbody>
          {generatedFiles.map((row, i) => (
            <tr
              key={row.file}
              style={{ background: i % 2 === 0 ? "var(--bg-dark-2)" : "transparent" }}
            >
              <td
                style={{
                  padding: "10px 16px",
                  borderBottom: "1px solid var(--line-dark)",
                  fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
                  fontSize: "12px",
                  color: "var(--accent)",
                  whiteSpace: "nowrap",
                }}
              >
                {row.file}
              </td>
              <td
                style={{
                  padding: "10px 16px",
                  borderBottom: "1px solid var(--line-dark)",
                  color: "var(--text-on-dark-soft)",
                }}
              >
                {row.desc}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* Next steps */}
      <div
        style={{
          marginTop: "48px",
          padding: "22px 24px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderLeft: "3px solid var(--accent)",
          borderRadius: "4px",
        }}
      >
        <div
          style={{
            fontSize: "13px",
            fontWeight: 500,
            marginBottom: "10px",
            color: "var(--text-on-dark)",
          }}
        >
          Next steps
        </div>
        <div style={{ display: "flex", gap: "20px", flexWrap: "wrap" }}>
          <Link href="/docs/agents" style={{ fontSize: "13px", color: "var(--accent)" }}>
            Meet the agents →
          </Link>
          <Link href="/docs/slash-commands" style={{ fontSize: "13px", color: "var(--accent)" }}>
            All slash commands →
          </Link>
          <Link href="/docs/tools/secrets" style={{ fontSize: "13px", color: "var(--accent)" }}>
            Secrets management →
          </Link>
        </div>
      </div>
    </div>
  );
}
