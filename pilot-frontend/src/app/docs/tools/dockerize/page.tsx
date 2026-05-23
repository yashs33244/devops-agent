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

const langDetection = [
  { file: "package.json", lang: "Node.js", template: "templates/dockerfiles/node/Dockerfile" },
  { file: "requirements.txt / pyproject.toml", lang: "Python", template: "templates/dockerfiles/python/Dockerfile" },
  { file: "go.mod", lang: "Go", template: "templates/dockerfiles/go/Dockerfile" },
  { file: "pom.xml / build.gradle", lang: "Java", template: "templates/dockerfiles/java/Dockerfile" },
  { file: "Cargo.toml", lang: "Rust", template: "Inline multi-stage (generated)" },
];

export default function DockerizePage() {
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
        <span style={{ color: "var(--text-on-dark)" }}>Dockerize</span>
      </div>

      {/* Tag badge */}
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
        Container
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        dockerize.py
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
        Detects the application language from project files, selects the correct Dockerfile template,
        substitutes template variables, then builds and validates the image automatically.
        Produces a multi-stage, distroless final image optimised for size and security.
      </p>

      {/* What it does */}
      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          The tool walks the target directory looking for well-known language indicator files.
          Once a match is found it copies the corresponding Dockerfile template, substitutes the
          template variables for the actual service name and entry point, and writes the result to
          the project root.
        </p>
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: 0, maxWidth: "66ch" }}>
          If Docker is available on the host, it immediately runs a build to verify the Dockerfile is valid.
          Hadolint and Trivy scans follow if those tools are installed.
        </p>
      </Section>

      {/* Language detection */}
      <Section title="Language Detection">
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
              gridTemplateColumns: "1fr 1fr 1fr",
              padding: "8px 16px",
              background: "var(--bg-dark-3)",
              borderBottom: "1px solid var(--line-dark)",
            }}
          >
            {["File Found", "Language", "Template"].map((h) => (
              <span
                key={h}
                className="mono"
                style={{ fontSize: "10px", letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--text-on-dark-soft)" }}
              >
                {h}
              </span>
            ))}
          </div>
          {langDetection.map((row, i) => (
            <div
              key={row.lang}
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr 1fr",
                padding: "10px 16px",
                borderBottom: i < langDetection.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "8px",
                alignItems: "center",
              }}
            >
              <code
                className="mono"
                style={{ fontSize: "12px", color: "var(--accent)" }}
              >
                {row.file}
              </code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark)" }}>{row.lang}</span>
              <code
                className="mono"
                style={{ fontSize: "11px", color: "var(--text-on-dark-soft)" }}
              >
                {row.template}
              </code>
            </div>
          ))}
        </div>
      </Section>

      {/* CLI usage */}
      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/dockerize.py --path ./my-app --service my-service

# With force overwrite of existing Dockerfile:
python3 tools/dockerize.py --path ./my-app --service my-service --force`}</CodeBlock>
      </Section>

      {/* Template variables */}
      <Section title="Template Variables">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.6, margin: "0 0 14px", maxWidth: "66ch" }}>
          The following placeholders are substituted in the selected template before writing the final Dockerfile:
        </p>
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {[
            { variable: "{{PORT}}", desc: "Exposed container port (default: 8000)" },
            { variable: "{{MAIN_FILE}}", desc: "Python entry-point module (Python only)" },
            { variable: "{{BINARY_NAME}}", desc: "Compiled binary name written by go build (Go only)" },
          ].map((row, i, arr) => (
            <div
              key={row.variable}
              style={{
                display: "grid",
                gridTemplateColumns: "200px 1fr",
                padding: "10px 16px",
                borderBottom: i < arr.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.variable}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{row.desc}</span>
            </div>
          ))}
        </div>
      </Section>

      {/* Output */}
      <Section title="Output">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 10px", maxWidth: "66ch" }}>
          Writes a single file: <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>&lt;path&gt;/Dockerfile</code>
        </p>
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: 0, maxWidth: "66ch" }}>
          All templates use multi-stage builds. The final stage is based on a Google Distroless image —
          no shell, no package manager, minimal attack surface.
        </p>
      </Section>

      {/* Validation */}
      <Section title="Validation Steps (Automatic)">
        <ul
          style={{
            margin: 0,
            padding: "0 0 0 18px",
            fontSize: "13px",
            color: "var(--text-on-dark-soft)",
            lineHeight: 2,
          }}
        >
          <li><strong style={{ color: "var(--text-on-dark)" }}>hadolint</strong> — lint check for Dockerfile best practices</li>
          <li><strong style={{ color: "var(--text-on-dark)" }}>docker build</strong> — full build test (skipped if Docker not available)</li>
          <li><strong style={{ color: "var(--text-on-dark)" }}>Trivy</strong> — HIGH/CRITICAL CVE scan on the built image (skipped if Trivy not installed)</li>
        </ul>
      </Section>

      {/* Important note */}
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
          Important
        </div>
        <p style={{ margin: 0, fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6 }}>
          <code className="mono" style={{ color: "var(--accent)", fontSize: "12px" }}>dockerize.py</code> will
          never overwrite an existing Dockerfile without the{" "}
          <code className="mono" style={{ color: "var(--accent)", fontSize: "12px" }}>--force</code> flag.
          When <code className="mono" style={{ color: "var(--accent)", fontSize: "12px" }}>--force</code> is
          passed, a unified diff is printed and confirmation is required before writing.
        </p>
      </div>

      {/* Next step */}
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
