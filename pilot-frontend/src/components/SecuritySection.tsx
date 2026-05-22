export function SecuritySection() {
  return (
    <section className="bento-section bg-dark" id="security">
      <div className="container-wide">
        <div className="bento-header">
          <div>
            <span className="eyebrow on-dark">// security &amp; trust</span>
            <h2 className="h-serif">
              A robot in production needs <em>guardrails</em>, not just guts.
            </h2>
          </div>
          <p className="lede">
            Pilot operates with least‑privilege roles, dry‑runs every change, and asks for a human
            signature on anything that touches money, identity, or data.
          </p>
        </div>

        <div className="bento">
          {/* guardrails */}
          <div className="bento-card b-c1">
            <span className="label">/ guardrails</span>
            <div className="stage">
              <svg viewBox="0 0 520 280" style={{ width: "100%", height: "100%" }} aria-hidden="true">
                <defs>
                  <pattern id="pixGradSec" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g opacity="0.55" fill="none" stroke="#7c3cf0">
                  <ellipse cx="340" cy="165" rx="170" ry="96" strokeWidth="1" strokeDasharray="2 4" />
                  <ellipse cx="340" cy="165" rx="130" ry="74" strokeWidth="1" strokeDasharray="2 6" opacity="0.7" />
                  <ellipse cx="340" cy="165" rx="95" ry="54" strokeWidth="1" strokeDasharray="2 6" opacity="0.45" />
                </g>
                <g className="lift" transform="translate(220 40)">
                  <polygon points="0,60 120,0 240,60 120,120" fill="url(#pixGradSec)" stroke="#0a0a0b" strokeWidth="1.5" />
                  <polygon points="0,60 0,170 120,230 120,120" fill="#ece9e0" stroke="#0a0a0b" strokeWidth="1.5" />
                  <g transform="matrix(1 0.5 0 1 0 0) translate(15 50)">
                    <rect x="0" y="0" width="90" height="6" fill="#0a0a0b" />
                    <rect x="0" y="86" width="90" height="6" fill="#0a0a0b" />
                    <circle cx="8" cy="3" r="2" fill="#ece9e0" />
                    <circle cx="45" cy="3" r="2" fill="#ece9e0" />
                    <circle cx="82" cy="3" r="2" fill="#ece9e0" />
                    <circle cx="8" cy="89" r="2" fill="#ece9e0" />
                    <circle cx="45" cy="89" r="2" fill="#ece9e0" />
                    <circle cx="82" cy="89" r="2" fill="#ece9e0" />
                  </g>
                  <polygon points="240,60 240,170 120,230 120,120" fill="#0a0a0b" stroke="#0a0a0b" strokeWidth="1.5" />
                  <g transform="matrix(1 -0.5 0 1 180 165)">
                    <circle cx="0" cy="0" r="42" fill="none" stroke="#3a3a40" strokeWidth="1.4" />
                    <circle cx="0" cy="0" r="34" fill="none" stroke="#3a3a40" strokeWidth="1" />
                    <circle cx="0" cy="0" r="26" fill="none" stroke="#3a3a40" strokeWidth="1" />
                    <line x1="-42" y1="0" x2="-22" y2="0" stroke="#9b5cff" strokeWidth="2.5" />
                    <line x1="42" y1="0" x2="22" y2="0" stroke="#9b5cff" strokeWidth="2.5" />
                    <line x1="0" y1="-42" x2="0" y2="-22" stroke="#9b5cff" strokeWidth="2.5" />
                    <line x1="0" y1="42" x2="0" y2="22" stroke="#9b5cff" strokeWidth="2.5" />
                    <circle cx="-42" cy="0" r="5" fill="#9b5cff" />
                    <circle cx="42" cy="0" r="5" fill="#9b5cff" />
                    <circle cx="0" cy="-42" r="5" fill="#9b5cff" />
                    <circle cx="0" cy="42" r="5" fill="#9b5cff" />
                    <circle cx="0" cy="0" r="10" fill="#9b5cff" />
                    <circle cx="0" cy="0" r="4" fill="#0a0a0b" />
                  </g>
                  <circle cx="180" cy="105" r="3" fill="#22c55e" className="pulse" />
                  <text x="190" y="108" fontFamily="JetBrains Mono" fontSize="8" fill="#ece9e0">SEALED</text>
                </g>
                <g fontFamily="JetBrains Mono" fontSize="10" fill="#9c9ca3">
                  <rect x="20" y="22" width="3" height="3" fill="#9b5cff" />
                  <text x="30" y="30">SOC&nbsp;2 · Type&nbsp;II</text>
                  <rect x="20" y="42" width="3" height="3" fill="#9b5cff" />
                  <text x="30" y="50">ISO&nbsp;27001</text>
                  <rect x="20" y="62" width="3" height="3" fill="#9b5cff" />
                  <text x="30" y="70">HIPAA‑ready</text>
                  <rect x="20" y="82" width="3" height="3" fill="#9b5cff" />
                  <text x="30" y="90">GDPR + EU residency</text>
                  <rect x="20" y="102" width="3" height="3" fill="#9b5cff" />
                  <text x="30" y="110">PCI · in&nbsp;progress</text>
                </g>
                <g opacity="0.5" stroke="#1c1c22" strokeWidth="0.6">
                  <path d="M40,250 L520,200" />
                  <path d="M40,270 L520,220" />
                </g>
              </svg>
            </div>
            <div className="body">
              <h3>Production‑grade by default</h3>
              <p>Least privilege, audit log on every action, dry‑runs on every change, human approval on every blast radius.</p>
            </div>
          </div>

          {/* approvals */}
          <div className="bento-card b-c2">
            <span className="label">/ approvals</span>
            <div className="stage">
              <svg viewBox="0 0 520 280" style={{ width: "100%", height: "100%" }} aria-hidden="true">
                <defs>
                  <pattern id="pixGradAppr" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g opacity="0.5" stroke="#1c1c22" strokeWidth="0.6">
                  <path d="M0,220 L520,140" />
                  <path d="M0,240 L520,160" />
                  <path d="M0,260 L520,180" />
                </g>
                <g className="lift">
                  <g transform="translate(20 80)">
                    <polygon points="0,30 50,5 100,30 50,55" fill="#ece9e0" stroke="#0a0a0b" strokeWidth="1.4" />
                    <g transform="matrix(1 0.5 -1 0.5 50 30)">
                      <rect x="-30" y="-10" width="60" height="20" fill="none" stroke="#7c3cf0" strokeWidth="1.6" />
                      <text x="0" y="4" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#7c3cf0">DRAFTED</text>
                    </g>
                    <polygon points="0,30 0,80 50,105 50,55" fill="#0a0a0b" stroke="#0a0a0b" strokeWidth="1.4" />
                    <polygon points="100,30 100,80 50,105 50,55" fill="#1d1d22" stroke="#0a0a0b" strokeWidth="1.4" />
                    <text x="50" y="130" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#ece9e0">PILOT · agent</text>
                  </g>
                  <g transform="translate(140 60)">
                    <polygon points="0,30 50,5 100,30 50,55" fill="#ece9e0" stroke="#0a0a0b" strokeWidth="1.4" />
                    <g transform="matrix(1 0.5 -1 0.5 50 30)">
                      <circle cx="0" cy="0" r="14" fill="none" stroke="#22c55e" strokeWidth="1.6" />
                      <polyline points="-6,0 -1,5 7,-4" fill="none" stroke="#22c55e" strokeWidth="2" />
                    </g>
                    <polygon points="0,30 0,80 50,105 50,55" fill="#0a0a0b" stroke="#0a0a0b" strokeWidth="1.4" />
                    <polygon points="100,30 100,80 50,105 50,55" fill="#1d1d22" stroke="#0a0a0b" strokeWidth="1.4" />
                    <text x="50" y="130" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#ece9e0">LEAD · signed</text>
                  </g>
                  <g transform="translate(260 80)">
                    <polygon points="0,30 50,5 100,30 50,55" fill="url(#pixGradAppr)" stroke="#0a0a0b" strokeWidth="1.4" />
                    <g transform="matrix(1 0.5 -1 0.5 50 30)" className="pulse">
                      <circle cx="0" cy="0" r="13" fill="none" stroke="#fff" strokeWidth="1.6" />
                      <text x="0" y="3" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="11" fill="#fff">?</text>
                    </g>
                    <polygon points="0,30 0,80 50,105 50,55" fill="#ece9e0" stroke="#0a0a0b" strokeWidth="1.4" />
                    <polygon points="100,30 100,80 50,105 50,55" fill="#0a0a0b" stroke="#0a0a0b" strokeWidth="1.4" />
                    <text x="50" y="130" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#9b5cff">SECURITY · pending</text>
                  </g>
                  <g transform="translate(380 100)">
                    <polygon points="0,30 50,5 100,30 50,55" fill="none" stroke="#3a3a40" strokeWidth="1.4" strokeDasharray="3 3" />
                    <polygon points="0,30 0,80 50,105 50,55" fill="none" stroke="#3a3a40" strokeWidth="1.4" strokeDasharray="3 3" />
                    <polygon points="100,30 100,80 50,105 50,55" fill="none" stroke="#3a3a40" strokeWidth="1.4" strokeDasharray="3 3" />
                    <text x="50" y="130" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">PROD · queued</text>
                  </g>
                  <g fill="none" stroke="#7c3cf0" strokeWidth="1.5" opacity="0.85">
                    <path d="M120,140 L160,130" className="pipe-flow" />
                    <path d="M240,120 L280,140" className="pipe-flow" />
                    <path d="M360,140 L400,160" className="pipe-flow" />
                  </g>
                </g>
                <g>
                  <rect x="20" y="234" width="480" height="34" fill="#0e0e10" stroke="#2a2a30" />
                  <text x="32" y="253" fontFamily="JetBrains Mono" fontSize="10" fill="#ece9e0">
                    ⏵ <tspan fill="#9c9ca3">action:</tspan> rotate kms‑key · pay‑2
                  </text>
                  <text x="32" y="265" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">blast radius: 1 svc · 3 pods · SLA window T+15min</text>
                  <circle cx="482" cy="251" r="3" fill="#22c55e" className="pulse" />
                </g>
              </svg>
            </div>
            <div className="body">
              <h3>Human in the loop, when it matters</h3>
              <p>Approvals routed by policy. Pilot waits patiently. Everything else, it just ships.</p>
            </div>
          </div>

          {/* audit log */}
          <div className="bento-card b-tall">
            <span className="label">/ audit&nbsp;log</span>
            <svg viewBox="0 0 280 90" style={{ width: "100%", height: "80px", margin: "6px 0 6px" }} aria-hidden="true">
              <defs>
                <pattern id="pixGradAudit" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                  <rect width="6" height="6" fill="#7c3cf0" />
                  <rect width="3" height="3" fill="#9b5cff" />
                  <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                </pattern>
              </defs>
              <g className="lift" transform="translate(70 4)">
                <g transform="translate(0 0)">
                  <polygon points="0,22 70,0 140,22 70,44" fill="#0a0a0b" stroke="#3a3a40" />
                </g>
                <g transform="translate(0 14)">
                  <polygon points="0,22 70,0 140,22 70,44" fill="#ece9e0" stroke="#0a0a0b" />
                  <line x1="18" y1="22" x2="58" y2="6" stroke="#0a0a0b" strokeWidth="1" />
                  <line x1="30" y1="28" x2="82" y2="10" stroke="#0a0a0b" strokeWidth="1" />
                  <line x1="48" y1="32" x2="104" y2="14" stroke="#0a0a0b" strokeWidth="1" />
                </g>
                <g transform="translate(0 28)">
                  <polygon points="0,22 70,0 140,22 70,44" fill="url(#pixGradAudit)" stroke="#0a0a0b" />
                </g>
              </g>
            </svg>
            <div className="stage" style={{ margin: "0 0 12px" }}>
              <div className="term" style={{ height: "100%", border: "1px solid var(--line-dark-2)" }}>
                <div className="row"><span className="muted">14:02:11</span> <span className="ok">✓</span> deploy <span className="prompt">api</span> v126→v127</div>
                <div className="row"><span className="muted">14:04:48</span> <span className="ok">✓</span> scaled <span className="prompt">worker</span> 4→6</div>
                <div className="row"><span className="muted">14:09:02</span> <span className="warn">!</span> rotated kms-key <span className="prompt">pay-2</span></div>
                <div className="row"><span className="muted">14:11:33</span> <span className="ok">✓</span> closed alert <span className="prompt">cpu-eu1</span></div>
                <div className="row"><span className="muted">14:18:09</span> <span className="ok">✓</span> applied tf plan <span className="prompt">#3a91</span></div>
                <div className="row"><span className="muted">14:24:17</span> <span className="warn">!</span> blocked: prod ddl needs sign-off</div>
                <div className="row"><span className="muted">14:31:50</span> <span className="ok">✓</span> patched cve-2026-19284</div>
                <div className="row"><span className="muted">14:42:09</span> <span className="ok">✓</span> nightly snapshot · 12 dbs</div>
                <div className="row"><span className="muted">15:01:00</span> <span className="ok">✓</span> rebalanced traffic eu→us 8%</div>
              </div>
            </div>
            <div className="body">
              <h3>Every action, signed and queryable</h3>
              <p>Append‑only audit log, exportable to your SIEM. Ask Pilot anything about its own behaviour.</p>
            </div>
          </div>

          {/* secrets */}
          <div className="bento-card b-c3">
            <span className="label">/ secrets</span>
            <div className="stage">
              <svg viewBox="0 0 280 200" style={{ width: "100%", height: "100%" }} aria-hidden="true">
                <defs>
                  <pattern id="pixGradSecrets" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g opacity="0.55">
                  <ellipse cx="140" cy="118" rx="110" ry="38" fill="none" stroke="#7c3cf0" strokeWidth="1" strokeDasharray="3 4" />
                </g>
                <g className="lift" transform="translate(80 30)">
                  <polygon points="0,40 60,10 120,40 60,70" fill="url(#pixGradSecrets)" stroke="#0a0a0b" strokeWidth="1.4" />
                  <polygon points="0,40 0,108 60,138 60,70" fill="#ece9e0" stroke="#0a0a0b" strokeWidth="1.4" />
                  <g transform="matrix(1 0.5 0 1 18 60)">
                    <rect x="0" y="0" width="4" height="22" fill="#0a0a0b" />
                    <rect x="-3" y="0" width="10" height="6" fill="#0a0a0b" />
                  </g>
                  <polygon points="120,40 120,108 60,138 60,70" fill="#0a0a0b" stroke="#0a0a0b" strokeWidth="1.4" />
                  <g transform="matrix(1 -0.5 0 1 90 92)">
                    <circle cx="0" cy="0" r="18" fill="none" stroke="#3a3a40" strokeWidth="1" />
                    <circle cx="0" cy="0" r="12" fill="none" stroke="#9b5cff" strokeWidth="1.4" />
                    <line x1="-18" y1="0" x2="-12" y2="0" stroke="#9b5cff" strokeWidth="2" />
                    <line x1="18" y1="0" x2="12" y2="0" stroke="#9b5cff" strokeWidth="2" />
                    <line x1="0" y1="-18" x2="0" y2="-12" stroke="#9b5cff" strokeWidth="2" />
                    <line x1="0" y1="18" x2="0" y2="12" stroke="#9b5cff" strokeWidth="2" />
                    <circle cx="0" cy="0" r="3" fill="#9b5cff" />
                  </g>
                  <circle cx="96" cy="55" r="2.4" fill="#22c55e" className="pulse" />
                </g>
                <g>
                  <g transform="translate(20 100) rotate(-18)" opacity="0.55">
                    <circle cx="6" cy="0" r="5" fill="none" stroke="#7c3cf0" strokeWidth="1.4" />
                    <rect x="10" y="-1.4" width="22" height="2.8" fill="#7c3cf0" />
                    <rect x="26" y="-1.4" width="2.6" height="5" fill="#7c3cf0" />
                    <rect x="30" y="-1.4" width="2.6" height="5" fill="#7c3cf0" />
                  </g>
                  <g transform="translate(130 14) rotate(28)">
                    <circle cx="6" cy="0" r="6" fill="#9b5cff" />
                    <circle cx="6" cy="0" r="2.2" fill="#0a0a0b" />
                    <rect x="12" y="-1.6" width="28" height="3.2" fill="#9b5cff" />
                    <rect x="32" y="-1.6" width="2.8" height="6" fill="#9b5cff" />
                    <rect x="36" y="-1.6" width="2.8" height="6" fill="#9b5cff" />
                  </g>
                  <g transform="translate(238 88) rotate(-160)" opacity="0.35">
                    <circle cx="6" cy="0" r="5" fill="none" stroke="#ece9e0" strokeWidth="1.2" />
                    <rect x="10" y="-1.4" width="22" height="2.8" fill="#ece9e0" />
                    <rect x="26" y="-1.4" width="2.6" height="5" fill="#ece9e0" />
                    <line x1="0" y1="-7" x2="34" y2="7" stroke="#c43d28" strokeWidth="1.4" />
                  </g>
                </g>
                <text x="14" y="190" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">rotated · 14:09 · t‑0:21:38</text>
              </svg>
            </div>
            <div className="body">
              <h3>Secret rotation, hands‑off</h3>
              <p>Pilot rotates, distributes and revokes — auto, on schedule, on compromise.</p>
            </div>
          </div>

          {/* self-hosted */}
          <div className="bento-card b-c4">
            <span className="label">/ self‑hosted</span>
            <div className="stage">
              <svg viewBox="0 0 280 200" style={{ width: "100%", height: "100%" }} aria-hidden="true">
                <defs>
                  <pattern id="pixGradSelf" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g className="lift" transform="translate(20 14)">
                  <polygon points="0,60 120,0 240,60 120,120" fill="none" stroke="#3a3a40" strokeWidth="1.2" strokeDasharray="4 4" />
                  <polygon points="0,60 0,142 120,202 120,120" fill="none" stroke="#3a3a40" strokeWidth="1.2" strokeDasharray="4 4" />
                  <polygon points="240,60 240,142 120,202 120,120" fill="none" stroke="#3a3a40" strokeWidth="1.2" strokeDasharray="4 4" />
                  <g fill="#7c3cf0">
                    <rect x="-3" y="57" width="6" height="6" />
                    <rect x="237" y="57" width="6" height="6" />
                    <rect x="117" y="-3" width="6" height="6" />
                    <rect x="117" y="199" width="6" height="6" />
                  </g>
                  <g transform="translate(46 56)">
                    <polygon points="0,16 24,4 48,16 24,28" fill="url(#pixGradSelf)" stroke="#0a0a0b" />
                    <polygon points="0,16 0,42 24,54 24,28" fill="#ece9e0" stroke="#0a0a0b" />
                    <polygon points="48,16 48,42 24,54 24,28" fill="#0a0a0b" stroke="#0a0a0b" />
                    <text x="24" y="68" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="8" fill="#9c9ca3">pilot</text>
                  </g>
                  <g transform="translate(96 92)">
                    <polygon points="0,16 24,4 48,16 24,28" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="0,16 0,42 24,54 24,28" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="48,16 48,42 24,54 24,28" fill="#141418" stroke="#3a3a40" />
                    <text x="24" y="68" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="8" fill="#9c9ca3">k8s</text>
                  </g>
                  <g transform="translate(146 60)">
                    <polygon points="0,16 24,4 48,16 24,28" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="0,16 0,42 24,54 24,28" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="48,16 48,42 24,54 24,28" fill="#141418" stroke="#3a3a40" />
                    <line x1="4" y1="30" x2="22" y2="40" stroke="#3a3a40" />
                    <line x1="4" y1="36" x2="22" y2="46" stroke="#3a3a40" />
                    <text x="24" y="68" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="8" fill="#9c9ca3">db</text>
                  </g>
                </g>
                <g>
                  <rect x="6" y="4" width="3" height="3" fill="#7c3cf0" />
                  <text x="14" y="12" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">YOUR&nbsp;BOUNDARY</text>
                </g>
                <text x="6" y="194" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">your‑vpc · your‑kms · your‑data</text>
              </svg>
            </div>
            <div className="body">
              <h3>Runs in your VPC</h3>
              <p>Your cloud, your account, your data. Pilot&apos;s control plane never touches it.</p>
            </div>
          </div>

          {/* policy */}
          <div className="bento-card b-c5">
            <span className="label">/ policy</span>
            <div className="stage" style={{ margin: "6px 0 12px" }}>
              <div className="term" style={{ height: "100%", fontSize: "11px", border: "1px solid var(--line-dark-2)" }}>
                <div className="row"><span className="muted"># policy.rego — pilot allow/deny</span></div>
                <div className="row"><span className="warn">deny</span>&nbsp;{"{"}</div>
                <div className="row">&nbsp;&nbsp;input.action == <span className="prompt">&quot;drop_table&quot;</span></div>
                <div className="row">&nbsp;&nbsp;input.env&nbsp;&nbsp;&nbsp; == <span className="prompt">&quot;prod&quot;</span></div>
                <div className="row">{"}"}</div>
                <div className="row"><span className="ok">allow</span> {"{"}</div>
                <div className="row">&nbsp;&nbsp;input.action == <span className="prompt">&quot;scale&quot;</span></div>
                <div className="row">&nbsp;&nbsp;input.replicas &lt;= <span className="prompt">32</span></div>
                <div className="row">{"}"}</div>
                <div className="row"><span className="muted">↪ enforced on 142 actions today · 0 violations</span></div>
              </div>
            </div>
            <div className="body">
              <h3>Policy as code</h3>
              <p>OPA‑native. Encode &ldquo;Pilot may never…&rdquo; once, enforce it everywhere.</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
