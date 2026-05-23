import Link from "next/link";

export function Nav() {
  return (
    <nav className="nav">
      <Link href="/" className="nav-logo" style={{ textDecoration: "none", color: "inherit" }}>
        <span className="nav-logo-mark" aria-hidden="true"></span>
        <span>Pilot</span>
        <span
          className="mono"
          style={{
            fontSize: "11px",
            color: "var(--text-on-dark-soft)",
            letterSpacing: ".14em",
            marginLeft: "6px",
          }}
        >
          / DEVOPS&nbsp;AGENT
        </span>
      </Link>
      <div className="nav-links">
        <a href="#platform">Platform</a>
        <a href="#how">How it works</a>
        <a href="#security">Security</a>
        <a href="#integrations">Integrations</a>
        <a href="/pricing">Pricing</a>
        <a href="/docs">Docs</a>
      </div>
      <div className="nav-cta">
<a href="https://github.com/yashs33244/devops-agent" target="_blank" rel="noopener noreferrer" className="btn btn-primary">
          Deploy free <span className="arrow"></span>
        </a>
      </div>
    </nav>
  );
}
