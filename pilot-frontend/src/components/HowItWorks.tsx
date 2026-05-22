export function HowItWorks() {
  return (
    <section className="flow" id="how">
      <div className="container">
        <span className="eyebrow">// how it works</span>
        <h2
          className="h-serif"
          style={{ fontSize: "clamp(40px,5vw,72px)", margin: "20px 0 0", maxWidth: "14ch" }}
        >
          Three boxes. Connect them. <em>Walk away.</em>
        </h2>

        <div className="flow-grid">
          {/* step 1 */}
          <div className="flow-step">
            <span className="step-n">STEP · 01</span>
            <h3>Point Pilot at your repo.</h3>
            <p>
              One <span className="mono">pilot init</span>. It reads your code, your existing IaC,
              your env. No magic — it tells you exactly what it plans to take over.
            </p>
            <div className="scene">
              <svg viewBox="0 0 320 220" xmlns="http://www.w3.org/2000/svg" style={{ width: "100%", height: "100%" }}>
                <g>
                  <polygon points="60,80 150,40 240,80 150,120" fill="#0a0a0b" stroke="#0a0a0b" />
                  <polygon points="60,80 60,160 150,200 150,120" fill="#0a0a0b" stroke="#0a0a0b" />
                  <polygon points="240,80 240,160 150,200 150,120" fill="#1d1d22" stroke="#0a0a0b" />
                  <line x1="160" y1="135" x2="225" y2="100" stroke="#3a3a40" />
                  <line x1="160" y1="150" x2="225" y2="115" stroke="#3a3a40" />
                  <line x1="160" y1="165" x2="225" y2="130" stroke="#3a3a40" />
                  <line x1="160" y1="180" x2="225" y2="145" stroke="#3a3a40" />
                  <text x="150" y="86" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="11" fill="#fff" transform="skewX(-30) translate(78 -10)">repo</text>
                  <rect x="240" y="40" width="74" height="24" fill="#7c3cf0" />
                  <text x="277" y="56" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#fff">→ pilot</text>
                </g>
              </svg>
            </div>
          </div>

          {/* step 2 */}
          <div className="flow-step">
            <span className="step-n">STEP · 02</span>
            <h3>Approve the plan.</h3>
            <p>
              Pilot drafts the IaC, the pipeline, the dashboards, the alerts. You diff it, comment,
              merge. Then it ships, end to end.
            </p>
            <div className="scene">
              <svg viewBox="0 0 320 220" xmlns="http://www.w3.org/2000/svg" style={{ width: "100%", height: "100%" }}>
                <g>
                  <rect x="50" y="40" width="150" height="160" fill="#fff" stroke="#0a0a0b" strokeWidth="1.4" />
                  <line x1="60" y1="60" x2="180" y2="60" stroke="#0a0a0b" strokeWidth="2" />
                  <line x1="60" y1="74" x2="170" y2="74" stroke="#888" />
                  <line x1="60" y1="86" x2="160" y2="86" stroke="#888" />
                  <rect x="60" y="98" width="120" height="10" fill="#c43d28" opacity="0.25" />
                  <line x1="60" y1="120" x2="170" y2="120" stroke="#888" />
                  <rect x="60" y="132" width="100" height="10" fill="#22c55e" opacity="0.4" />
                  <line x1="60" y1="154" x2="160" y2="154" stroke="#888" />
                  <line x1="60" y1="166" x2="170" y2="166" stroke="#888" />
                  <rect x="130" y="80" width="150" height="120" fill="#7c3cf0" />
                  <text x="150" y="108" fontFamily="JetBrains Mono" fontSize="11" fill="#fff">PLAN</text>
                  <text x="150" y="128" fontFamily="JetBrains Mono" fontSize="9" fill="#fff" opacity="0.8">+ 12 resources</text>
                  <text x="150" y="142" fontFamily="JetBrains Mono" fontSize="9" fill="#fff" opacity="0.8">~ 3 modified</text>
                  <text x="150" y="156" fontFamily="JetBrains Mono" fontSize="9" fill="#fff" opacity="0.8">- 0 destroyed</text>
                  <rect x="150" y="170" width="80" height="20" fill="#0a0a0b" />
                  <text x="190" y="184" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#fff">approve →</text>
                </g>
              </svg>
            </div>
          </div>

          {/* step 3 */}
          <div className="flow-step">
            <span className="step-n">STEP · 03</span>
            <h3>It runs the night shift.</h3>
            <p>
              Pilot deploys, watches, scales, patches, rotates secrets, and writes the postmortem.
              You sleep through the on‑call rotation you used to dread.
            </p>
            <div className="scene">
              <svg viewBox="0 0 320 220" xmlns="http://www.w3.org/2000/svg" style={{ width: "100%", height: "100%" }}>
                <circle cx="60" cy="50" r="22" fill="#0a0a0b" />
                <circle cx="68" cy="46" r="18" fill="#f3efe7" />
                <rect x="120" y="34" width="3" height="3" fill="#0a0a0b" />
                <rect x="160" y="20" width="3" height="3" fill="#0a0a0b" />
                <rect x="200" y="40" width="3" height="3" fill="#0a0a0b" />
                <rect x="240" y="28" width="3" height="3" fill="#0a0a0b" />
                <g>
                  <polygon points="100,140 150,114 200,140 150,166" fill="#0a0a0b" />
                  <polygon points="100,140 100,178 150,204 150,166" fill="#0a0a0b" opacity="0.75" />
                  <polygon points="200,140 200,178 150,204 150,166" fill="#1d1d22" opacity="0.95" />
                  <circle cx="150" cy="138" r="3" fill="#22c55e" className="pulse" />
                  <circle cx="160" cy="132" r="3" fill="#22c55e" className="pulse" />
                  <circle cx="140" cy="132" r="3" fill="#22c55e" className="pulse" />
                </g>
                <g fontFamily="JetBrains Mono" fontSize="11" fill="#0a0a0b">
                  <text x="220" y="100">z</text>
                  <text x="232" y="92" fontSize="14">z</text>
                  <text x="248" y="80" fontSize="18">Z</text>
                </g>
              </svg>
            </div>
          </div>
        </div>

        <div style={{ marginTop: "28px", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <p className="mono" style={{ fontSize: "12px", color: "var(--ink-quiet)" }}>
            // ALL THREE STEPS, FIRST DEPLOY, UNDER 11 MINUTES MEDIAN
          </p>
          <a href="#" className="btn btn-dark">
            Read the technical brief <span className="arrow"></span>
          </a>
        </div>
      </div>
    </section>
  );
}
