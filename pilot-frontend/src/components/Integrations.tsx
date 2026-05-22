export function Integrations() {
  return (
    <section className="integrations" id="integrations">
      <div className="container-wide">
        <div className="bento-header" style={{ marginBottom: "32px" }}>
          <div>
            <span className="eyebrow on-dark">// connections</span>
            <h2 className="h-serif">
              Plays nicely with <em>everything</em> already in your stack.
            </h2>
          </div>
          <p className="lede">
            No rip‑and‑replace. Pilot reads your existing Terraform, your Helm charts, your
            GitHub Actions, and your runbooks — and just gets to work.
          </p>
        </div>

        <div className="int-grid">
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <polygon points="20,4 36,12 36,28 20,36 4,28 4,12" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <polygon points="20,4 36,12 20,20 4,12" fill="#7c3cf0" opacity="0.4" />
            </svg>
            <span className="name">AWS</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <rect x="6" y="6" width="28" height="28" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <rect x="12" y="12" width="16" height="16" fill="#7c3cf0" opacity="0.4" />
            </svg>
            <span className="name">GCP</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <polygon points="20,6 34,20 20,34 6,20" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <polygon points="20,12 28,20 20,28 12,20" fill="#7c3cf0" opacity="0.4" />
            </svg>
            <span className="name">Azure</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <circle cx="20" cy="20" r="14" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <circle cx="20" cy="20" r="6" fill="#7c3cf0" opacity="0.4" />
            </svg>
            <span className="name">k8s</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <rect x="6" y="14" width="28" height="14" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <rect x="6" y="14" width="14" height="14" fill="#7c3cf0" opacity="0.4" />
            </svg>
            <span className="name">Docker</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <polygon points="20,6 32,14 32,26 20,34 8,26 8,14" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <text x="20" y="24" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#ece9e0">tf</text>
            </svg>
            <span className="name">Terraform</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <rect x="6" y="6" width="28" height="20" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <polyline points="10,16 16,22 30,12" stroke="#7c3cf0" strokeWidth="1.6" fill="none" />
            </svg>
            <span className="name">GitHub</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <rect x="8" y="8" width="24" height="24" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <line x1="8" y1="20" x2="32" y2="20" stroke="#7c3cf0" strokeWidth="1.4" />
              <line x1="20" y1="8" x2="20" y2="32" stroke="#7c3cf0" strokeWidth="1.4" />
            </svg>
            <span className="name">GitLab</span>
          </div>

          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <circle cx="20" cy="20" r="13" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <path d="M14,20 a6,6 0 0 0 12,0" fill="none" stroke="#7c3cf0" strokeWidth="1.4" />
            </svg>
            <span className="name">Datadog</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <polygon points="20,6 32,32 8,32" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <line x1="20" y1="14" x2="20" y2="24" stroke="#c43d28" strokeWidth="1.6" />
              <circle cx="20" cy="28" r="1.4" fill="#c43d28" />
            </svg>
            <span className="name">PagerDuty</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <rect x="8" y="8" width="11" height="11" fill="#ece9e0" />
              <rect x="21" y="8" width="11" height="11" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <rect x="8" y="21" width="11" height="11" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <rect x="21" y="21" width="11" height="11" fill="#7c3cf0" />
            </svg>
            <span className="name">Slack</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <circle cx="20" cy="20" r="13" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <circle cx="20" cy="20" r="2" fill="#7c3cf0" />
            </svg>
            <span className="name">Prometheus</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <polygon points="6,20 14,12 22,20 14,28" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <polygon points="18,20 26,12 34,20 26,28" fill="#7c3cf0" opacity="0.4" stroke="#ece9e0" strokeWidth="1.4" />
            </svg>
            <span className="name">Grafana</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <rect x="6" y="10" width="28" height="20" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <text x="20" y="24" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#ece9e0">SQL</text>
            </svg>
            <span className="name">Postgres</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <circle cx="20" cy="20" r="13" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <path d="M14,16 q6,-4 12,0 q-6,4 -12,0 z" fill="#7c3cf0" opacity="0.5" />
            </svg>
            <span className="name">Redis</span>
          </div>
          <div className="int-tile">
            <svg viewBox="0 0 40 40">
              <polygon points="20,4 34,28 6,28" fill="none" stroke="#ece9e0" strokeWidth="1.4" />
              <polygon points="20,12 28,26 12,26" fill="#7c3cf0" opacity="0.4" />
            </svg>
            <span className="name">Vercel</span>
          </div>
        </div>

        <p
          className="mono"
          style={{ marginTop: "28px", fontSize: "12px", color: "var(--text-on-dark-soft)", letterSpacing: "0.06em" }}
        >
          + 80 more · open MCP &amp; REST APIs · BYO tools via shell wrapper
        </p>
      </div>
    </section>
  );
}
