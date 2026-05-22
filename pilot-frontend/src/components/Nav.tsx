export function Nav() {
  return (
    <nav className="nav">
      <div className="nav-logo">
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
      </div>
      <div className="nav-links">
        <a href="#platform">Platform</a>
        <a href="#how">How it works</a>
        <a href="#security">Security</a>
        <a href="#integrations">Integrations</a>
        <a href="#pricing">Pricing</a>
        <a href="#docs">Docs</a>
      </div>
      <div className="nav-cta">
        <a href="#" className="btn btn-ghost">
          Sign in
        </a>
        <a href="#" className="btn btn-primary">
          Deploy free <span className="arrow"></span>
        </a>
      </div>
    </nav>
  );
}
