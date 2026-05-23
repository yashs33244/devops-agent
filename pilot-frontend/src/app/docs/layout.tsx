import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Docs — Pilot DevOps Agent",
  description: "Documentation for Pilot — the autonomous DevOps agent.",
};

const sidebarSections = [
  {
    title: "Getting Started",
    items: [
      { label: "Quick Start", href: "/docs/quick-start" },
      { label: "Installation", href: "/docs/quick-start#installation" },
      { label: "Configuration", href: "/docs/quick-start#configuration" },
    ],
  },
  {
    title: "Tools",
    items: [
      { label: "Dockerize", href: "/docs/tools/dockerize" },
      { label: "Terraform", href: "/docs/tools/terraform" },
      { label: "Helm", href: "/docs/tools/helm" },
      { label: "CI/CD", href: "/docs/tools/cicd" },
      { label: "Secrets", href: "/docs/tools/secrets" },
      { label: "Local Test", href: "/docs/tools/local-test" },
      { label: "Cost Optimize", href: "/docs/tools/cost-optimize" },
      { label: "Test Runner", href: "/docs/tools/test-runner" },
    ],
  },
  {
    title: "Agents",
    items: [
      { label: "HolmesGPT", href: "/docs/agents#holmesgpt" },
      { label: "kagent", href: "/docs/agents#kagent" },
      { label: "Nightshift", href: "/docs/agents#nightshift" },
      { label: "OpenSRE", href: "/docs/agents#opensre" },
      { label: "Plural", href: "/docs/agents#plural" },
      { label: "SRE Guard", href: "/docs/agents#sre-guard" },
    ],
  },
  {
    title: "Templates",
    items: [
      { label: "Dockerfiles", href: "/docs/templates#dockerfiles" },
      { label: "Terraform", href: "/docs/templates#terraform" },
      { label: "GitHub Actions", href: "/docs/templates#github-actions" },
      { label: "Helm", href: "/docs/templates#helm" },
      { label: "KEDA", href: "/docs/templates#keda" },
      { label: "Monitoring", href: "/docs/templates#monitoring" },
    ],
  },
  {
    title: "Slash Commands",
    items: [
      { label: "/deploy", href: "/docs/slash-commands#deploy" },
      { label: "/dockerize", href: "/docs/slash-commands#dockerize" },
      { label: "/terraform", href: "/docs/slash-commands#terraform" },
      { label: "/helm", href: "/docs/slash-commands#helm" },
      { label: "/secrets", href: "/docs/slash-commands#secrets" },
      { label: "/test", href: "/docs/slash-commands#test" },
      { label: "/local-test", href: "/docs/slash-commands#local-test" },
      { label: "/optimize-cost", href: "/docs/slash-commands#optimize-cost" },
      { label: "/audit", href: "/docs/slash-commands#audit" },
      { label: "/status", href: "/docs/slash-commands#status" },
      { label: "/holmesgpt", href: "/docs/slash-commands#holmesgpt" },
      { label: "/kagent", href: "/docs/slash-commands#kagent" },
      { label: "/nightshift", href: "/docs/slash-commands#nightshift" },
      { label: "/opensre", href: "/docs/slash-commands#opensre" },
      { label: "/plural", href: "/docs/slash-commands#plural" },
      { label: "/sre-guard", href: "/docs/slash-commands#sre-guard" },
    ],
  },
];

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div style={{ display: "flex", minHeight: "100vh", paddingTop: "61px" }}>
      {/* Sidebar */}
      <aside
        style={{
          width: "240px",
          flexShrink: 0,
          borderRight: "1px solid var(--line-dark)",
          background: "var(--bg-dark)",
          position: "sticky",
          top: "61px",
          height: "calc(100vh - 61px)",
          overflowY: "auto",
          padding: "28px 0 40px",
        }}
      >
        <div style={{ padding: "0 20px 16px" }}>
          <Link
            href="/docs"
            style={{
              fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
              fontSize: "11px",
              letterSpacing: "0.14em",
              textTransform: "uppercase",
              color: "var(--text-on-dark-soft)",
              display: "flex",
              alignItems: "center",
              gap: "8px",
            }}
          >
            <span
              style={{
                width: "16px",
                height: "16px",
                background: "var(--text-on-dark)",
                display: "inline-block",
                position: "relative",
                overflow: "hidden",
                flexShrink: 0,
              }}
            />
            Pilot Docs
          </Link>
        </div>

        <nav>
          {sidebarSections.map((section) => (
            <div key={section.title} style={{ marginBottom: "8px" }}>
              <div
                style={{
                  padding: "10px 20px 6px",
                  fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
                  fontSize: "10px",
                  letterSpacing: "0.14em",
                  textTransform: "uppercase",
                  color: "var(--text-on-dark-soft)",
                  opacity: 0.6,
                }}
              >
                {section.title}
              </div>
              {section.items.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  style={{
                    display: "block",
                    padding: "6px 20px 6px 18px",
                    fontSize: "13px",
                    color: "var(--text-on-dark-soft)",
                    borderLeft: "2px solid transparent",
                    transition: "color .15s, border-color .15s, background .15s",
                    lineHeight: "1.4",
                  }}
                  className="docs-sidebar-link"
                >
                  {item.label}
                </Link>
              ))}
            </div>
          ))}
        </nav>

        <style>{`
          .docs-sidebar-link:hover {
            color: var(--text-on-dark) !important;
            border-left-color: var(--accent) !important;
            background: rgba(255,255,255,0.03);
          }
        `}</style>
      </aside>

      {/* Main content */}
      <main
        style={{
          flex: 1,
          minWidth: 0,
          padding: "48px 64px 96px",
          maxWidth: "860px",
        }}
      >
        {children}
      </main>
    </div>
  );
}
