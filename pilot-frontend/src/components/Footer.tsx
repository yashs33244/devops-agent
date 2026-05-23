export function Footer() {
  return (
    <footer className="bg-dark">
      <div className="container">
        <div className="footer-row">
          <div className="left">
            <span
              className="nav-logo-mark"
              aria-hidden="true"
              style={{ width: "18px", height: "18px" }}
            ></span>
            <span className="mono">PILOT / DEVOPS AGENT · v1.4.0</span>
            <span style={{ color: "var(--ink-quiet)" }}>© 2026</span>
          </div>
          <div className="links">
            <a href="/docs">Docs</a>
            <a href="#">Changelog</a>
            <a href="#">Status</a>
            <a href="#">Security</a>
            <a href="#">Careers</a>
            <a href="#">Contact</a>
          </div>
        </div>
      </div>
    </footer>
  );
}
