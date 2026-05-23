export function BentoSection() {
  return (
    <section className="bento-section bg-dark" id="platform">
      <div className="container-wide">
        <div className="bento-header">
          <div>
            <span className="eyebrow on-dark">// the platform</span>
            <h2 className="h-serif">
              One agent. The entire path from <em>git push</em> to &ldquo;still healthy at 4 a.m.&rdquo;
            </h2>
          </div>
          <p className="lede">
            Pilot owns infrastructure as a first‑class actor. It writes the IaC, picks the
            cloud, runs the deploy, watches the metrics, fixes what it can, and pages a human
            only when it really, genuinely, should.
          </p>
        </div>

        <div className="bento">
          {/* BIG — service graph */}
          <div className="bento-card b-c1">
            <span className="label">/ service&nbsp;graph</span>
            <div className="stage">
              <svg viewBox="0 0 520 280" xmlns="http://www.w3.org/2000/svg" style={{ width: "100%", height: "100%" }}>
                <defs>
                  <pattern id="pixGradBento" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g opacity="0.5">
                  <g stroke="#1c1c22" strokeWidth="0.5">
                    <path d="M0,140 L520,-90 M0,180 L520,-50 M0,220 L520,-10 M0,260 L520,30 M0,300 L520,70" />
                    <path d="M0,-90 L520,140 M0,-50 L520,180 M0,-10 L520,220 M0,30 L520,260 M0,70 L520,300" />
                  </g>
                </g>
                <g className="lift">
                  <g transform="translate(60 110)">
                    <polygon points="0,20 30,4 60,20 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="0,20 0,52 30,68 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="60,20 60,52 30,68 30,36" fill="#141418" stroke="#3a3a40" />
                    <text x="6" y="80" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">api · ok</text>
                  </g>
                  <g transform="translate(200 60)">
                    <polygon points="0,20 30,4 60,20 30,36" fill="url(#pixGradBento)" stroke="#0a0a0b" />
                    <polygon points="0,20 0,52 30,68 30,36" fill="#ece9e0" stroke="#0a0a0b" />
                    <polygon points="60,20 60,52 30,68 30,36" fill="#0a0a0b" stroke="#0a0a0b" />
                    <text x="-4" y="80" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">worker · scaling</text>
                  </g>
                  <g transform="translate(340 130)">
                    <polygon points="0,20 30,4 60,20 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="0,20 0,52 30,68 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="60,20 60,52 30,68 30,36" fill="#141418" stroke="#3a3a40" />
                    <line x1="6" y1="42" x2="26" y2="54" stroke="#3a3a40" />
                    <line x1="6" y1="50" x2="26" y2="62" stroke="#3a3a40" />
                    <text x="2" y="92" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">postgres · primary</text>
                  </g>
                  <g transform="translate(420 50)">
                    <polygon points="0,20 30,4 60,20 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="0,20 0,52 30,68 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="60,20 60,52 30,68 30,36" fill="#141418" stroke="#3a3a40" />
                    <text x="2" y="80" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">cdn · 99.99</text>
                  </g>
                  <g transform="translate(120 200)">
                    <polygon points="0,20 30,4 60,20 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="0,20 0,52 30,68 30,36" fill="#0a0a0b" stroke="#3a3a40" />
                    <polygon points="60,20 60,52 30,68 30,36" fill="#141418" stroke="#3a3a40" />
                    <text x="2" y="92" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">queue</text>
                  </g>
                </g>
                <g fill="none" stroke="#7c3cf0" strokeWidth="1.5" opacity="0.8">
                  <path d="M120,130 L200,90" className="pipe-flow" />
                  <path d="M260,90 L340,150" className="pipe-flow" />
                  <path d="M260,80 L420,70" className="pipe-flow" />
                  <path d="M120,160 L150,210" className="pipe-flow" />
                  <path d="M180,222 L340,170" className="pipe-flow" />
                </g>
              </svg>
            </div>
            <div className="body">
              <h3>Live service graph, owned</h3>
              <p>Pilot maps every service, queue, db, and edge node it ships — and keeps the map alive as the topology changes.</p>
            </div>
          </div>

          {/* BIG — pipeline */}
          <div className="bento-card b-c2">
            <span className="label">/ pipelines</span>
            <div className="stage">
              <svg viewBox="0 0 520 280" xmlns="http://www.w3.org/2000/svg" style={{ width: "100%", height: "100%" }}>
                <defs>
                  <pattern id="pixGradPipe" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g className="lift">
                  <g transform="translate(30 100)">
                    <rect x="0" y="0" width="80" height="80" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="40" y="44" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#ece9e0">git</text>
                    <text x="40" y="58" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">push</text>
                  </g>
                  <g transform="translate(140 100)">
                    <rect x="0" y="0" width="80" height="80" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="40" y="44" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#ece9e0">build</text>
                    <text x="40" y="58" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#22c55e">✓ 1m12s</text>
                  </g>
                  <g transform="translate(250 100)">
                    <rect x="0" y="0" width="80" height="80" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="40" y="44" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#ece9e0">test</text>
                    <text x="40" y="58" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#22c55e">✓ 412 ok</text>
                  </g>
                  <g transform="translate(360 100)">
                    <rect x="0" y="0" width="80" height="80" fill="url(#pixGradPipe)" stroke="#0a0a0b" />
                    <text x="40" y="44" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#fff">canary</text>
                    <text x="40" y="58" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#fff">3% live</text>
                  </g>
                  <g transform="translate(450 100)">
                    <rect x="0" y="0" width="60" height="80" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="30" y="44" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#ece9e0">prod</text>
                    <circle cx="30" cy="62" r="3" fill="#22c55e" className="pulse" />
                  </g>
                </g>
                <g stroke="#7c3cf0" strokeWidth="1.5" fill="none" opacity="0.9">
                  <line x1="110" y1="140" x2="140" y2="140" className="pipe-flow" />
                  <line x1="220" y1="140" x2="250" y2="140" className="pipe-flow" />
                  <line x1="330" y1="140" x2="360" y2="140" className="pipe-flow" />
                  <line x1="440" y1="140" x2="450" y2="140" className="pipe-flow" />
                </g>
                <g fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">
                  <line x1="30" y1="210" x2="510" y2="210" stroke="#3a3a40" strokeWidth="0.6" />
                  <text x="30" y="226">14:02</text>
                  <text x="140" y="226">14:03</text>
                  <text x="250" y="226">14:04</text>
                  <text x="360" y="226">14:05</text>
                  <text x="450" y="226">14:06</text>
                </g>
              </svg>
            </div>
            <div className="body">
              <h3>CI/CD that reasons</h3>
              <p>Canary, blue/green, or progressive rollout — Pilot picks the strategy and bails out the moment the SLO budget twitches.</p>
            </div>
          </div>

          {/* terminal */}
          <div className="bento-card b-c3">
            <span className="label">/ chat&nbsp;ops</span>
            <div className="stage" style={{ margin: "6px 0 12px" }}>
              <div className="term" style={{ height: "100%", border: "1px solid var(--line-dark-2)" }}>
                <div className="row"><span className="prompt">$</span><span>pilot, why is checkout slow in eu-west?</span></div>
                <div className="row muted">↪ scanning traces · 14s window</div>
                <div className="row"><span className="muted">↪</span> 73% of latency from <span className="warn">payments-svc</span> cold starts</div>
                <div className="row"><span className="muted">↪</span> bumped min-replicas 2→4 in eu-west-1</div>
                <div className="row"><span className="ok">✓</span> p99 back under 220ms</div>
              </div>
            </div>
            <div className="body">
              <h3>Talk to your infra</h3>
              <p>Plain English in. Diffs, deploys, and incident commentary out.</p>
            </div>
          </div>

          {/* cost */}
          <div className="bento-card b-c4">
            <span className="label">/ cost</span>
            <div className="stage">
              <svg viewBox="0 0 280 200" style={{ width: "100%", height: "100%" }} aria-hidden="true">
                <defs>
                  <pattern id="pixGradCost" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g className="lift">
                  <rect x="20" y="30" width="22" height="140" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="46" y="50" width="22" height="120" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="72" y="70" width="22" height="100" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="98" y="86" width="22" height="84" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="124" y="100" width="22" height="70" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="150" y="118" width="22" height="52" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="176" y="128" width="22" height="42" fill="#0a0a0b" stroke="#3a3a40" />
                  <rect x="202" y="138" width="22" height="32" fill="url(#pixGradCost)" />
                  <rect x="228" y="142" width="22" height="28" fill="url(#pixGradCost)" />
                  <polyline points="32,30 58,50 84,70 110,86 136,100 162,118 188,128 214,138 240,142"
                    fill="none" stroke="#9b5cff" strokeWidth="2" />
                  <text x="180" y="22" fontFamily="JetBrains Mono" fontSize="10" fill="#9b5cff">−38% MoM</text>
                </g>
              </svg>
            </div>
            <div className="body">
              <h3>Cost as a first‑class metric</h3>
              <p>Pilot right‑sizes, schedules, and kills the orphaned NAT gateway you forgot.</p>
            </div>
          </div>

          {/* rollback */}
          <div className="bento-card b-c5">
            <span className="label">/ rollback</span>
            <div className="stage">
              <svg viewBox="0 0 280 200" style={{ width: "100%", height: "100%" }} aria-hidden="true">
                <defs>
                  <pattern id="pixGradRollback" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill="#7c3cf0" />
                    <rect width="3" height="3" fill="#9b5cff" />
                    <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                  </pattern>
                </defs>
                <g className="lift">
                  <g>
                    <rect x="14" y="60" width="40" height="30" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="34" y="78" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">v23</text>
                  </g>
                  <g>
                    <rect x="62" y="60" width="40" height="30" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="82" y="78" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">v24</text>
                  </g>
                  <g>
                    <rect x="110" y="60" width="40" height="30" fill="#0a0a0b" stroke="#3a3a40" />
                    <text x="130" y="78" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">v25</text>
                  </g>
                  <g>
                    <rect x="158" y="60" width="40" height="30" fill="url(#pixGradRollback)" />
                    <text x="178" y="78" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#fff">v26</text>
                  </g>
                  <g>
                    <rect x="206" y="60" width="40" height="30" fill="#0a0a0b" stroke="#c43d28" />
                    <text x="226" y="78" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#c43d28">v27 ✗</text>
                  </g>
                  {/* Revert path: drop below v27 → left to v26 → rise into v26 */}
                  <path d="M226,90 L226,118 L178,118 L178,100" stroke="#9b5cff" strokeWidth="1.8" fill="none" strokeLinejoin="round" strokeLinecap="round"/>
                  {/* Upward arrowhead — tip lands at bottom edge of v26 */}
                  <polygon points="178,90 172,102 184,102" fill="#9b5cff"/>
                  <text x="202" y="142" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="10" fill="#9b5cff">↺ reverted in 11s</text>
                </g>
              </svg>
            </div>
            <div className="body">
              <h3>Self‑healing rollouts</h3>
              <p>Detect bad release, halt rollout, revert. No paging, no Slack thread.</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
