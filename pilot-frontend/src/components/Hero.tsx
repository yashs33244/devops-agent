export function Hero() {
  return (
    <header className="hero bg-dark">
      <div className="container-wide">
        <div className="hero-inner">
          <div className="hero-left">
            <span className="hero-tag">
              <span className="dot"></span> Now in public beta · v1.4 shipped today
            </span>
            <h1 className="h-serif">
              The DevOps engineer<br />
              that <em>never</em> sleeps,<br />
              context‑switches,<br />
              or pages on call.
            </h1>
            <p className="hero-sub">
              Pilot is an autonomous DevOps agent. It ships your app to AWS, GCP or bare‑metal,
              writes the Terraform, owns the CI/CD, watches the logs, and rolls itself back
              when something smells off. You bring code. Pilot brings the infrastructure.
            </p>
            <div className="hero-cta">
              <a href="#" className="btn btn-primary">
                Start deploying <span className="arrow"></span>
              </a>
              <a href="#" className="btn btn-ghost">
                Watch 90‑sec demo
              </a>
            </div>
          </div>

          {/* HERO ISOMETRIC ILLUSTRATION */}
          <div className="hero-right">
            <svg
              className="hero-iso"
              viewBox="0 0 720 720"
              xmlns="http://www.w3.org/2000/svg"
              aria-hidden="true"
            >
              <defs>
                <pattern id="pixGrad" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                  <rect width="6" height="6" fill="#7c3cf0" />
                  <rect width="3" height="3" fill="#9b5cff" />
                  <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
                </pattern>
                <pattern id="pixDot" x="0" y="0" width="4" height="4" patternUnits="userSpaceOnUse">
                  <rect width="4" height="4" fill="#0a0a0b" />
                  <rect x="0" y="0" width="1" height="1" fill="#2a2a30" />
                </pattern>
                <filter id="boxShadow" x="-50%" y="-50%" width="200%" height="200%">
                  <feGaussianBlur in="SourceAlpha" stdDeviation="6" />
                  <feOffset dx="0" dy="8" result="off" />
                  <feComponentTransfer><feFuncA type="linear" slope="0.4" /></feComponentTransfer>
                  <feMerge><feMergeNode /><feMergeNode in="SourceGraphic" /></feMerge>
                </filter>
              </defs>

              {/* Faint pixel grid ground */}
              <g opacity="0.55">
                <g stroke="#1c1c22" strokeWidth="0.6">
                  <g transform="translate(0 360)">
                    <line x1="-100" y1="0" x2="820" y2="-530" />
                    <line x1="-100" y1="50" x2="820" y2="-480" />
                    <line x1="-100" y1="100" x2="820" y2="-430" />
                    <line x1="-100" y1="150" x2="820" y2="-380" />
                    <line x1="-100" y1="200" x2="820" y2="-330" />
                    <line x1="-100" y1="250" x2="820" y2="-280" />
                    <line x1="-100" y1="300" x2="820" y2="-230" />
                    <line x1="-100" y1="350" x2="820" y2="-180" />
                  </g>
                  <g transform="translate(0 360)">
                    <line x1="-100" y1="-530" x2="820" y2="0" />
                    <line x1="-100" y1="-480" x2="820" y2="50" />
                    <line x1="-100" y1="-430" x2="820" y2="100" />
                    <line x1="-100" y1="-380" x2="820" y2="150" />
                    <line x1="-100" y1="-330" x2="820" y2="200" />
                    <line x1="-100" y1="-280" x2="820" y2="250" />
                    <line x1="-100" y1="-230" x2="820" y2="300" />
                    <line x1="-100" y1="-180" x2="820" y2="350" />
                  </g>
                </g>
              </g>

              {/* BACK BOX: server rack */}
              <g className="iso-server lift">
                <polygon points="80,300 220,220 360,300 220,380" fill="#1d1d22" stroke="#3a3a40" strokeWidth="1.2" />
                <polygon points="80,300 80,460 220,540 220,380" fill="#0a0a0b" stroke="#2a2a30" strokeWidth="1.2" />
                <polygon points="360,300 360,460 220,540 220,380" fill="#141418" stroke="#2a2a30" strokeWidth="1.2" />
                <g>
                  <line x1="240" y1="395" x2="345" y2="335" stroke="#3a3a40" strokeWidth="1" />
                  <line x1="240" y1="420" x2="345" y2="360" stroke="#3a3a40" strokeWidth="1" />
                  <line x1="240" y1="445" x2="345" y2="385" stroke="#3a3a40" strokeWidth="1" />
                  <line x1="240" y1="470" x2="345" y2="410" stroke="#3a3a40" strokeWidth="1" />
                  <line x1="240" y1="495" x2="345" y2="435" stroke="#3a3a40" strokeWidth="1" />
                  <circle cx="335" cy="345" r="2.5" fill="#22c55e" className="pulse" />
                  <circle cx="335" cy="370" r="2.5" fill="#22c55e" />
                  <circle cx="335" cy="395" r="2.5" fill="#f59e0b" className="pulse" />
                  <circle cx="335" cy="420" r="2.5" fill="#22c55e" />
                  <circle cx="335" cy="445" r="2.5" fill="#22c55e" />
                </g>
                <g transform="translate(180 290)">
                  <polygon points="0,0 80,-45 95,-37 15,8" fill="#7c3cf0" />
                  <text x="40" y="-15" fontFamily="JetBrains Mono" fontSize="9" fill="#fff" transform="skewX(-30) rotate(-22)">PROD-01</text>
                </g>
              </g>

              {/* MIDDLE BOX: deployment unit */}
              <g className="iso-deploy" transform="translate(80 -30)">
                <polygon points="280,330 440,250 600,330 440,410" fill="url(#pixGrad)" stroke="#0a0a0b" strokeWidth="1.4" />
                <polygon points="280,330 280,470 440,550 440,410" fill="#ece9e0" stroke="#0a0a0b" strokeWidth="1.4" />
                <polygon points="600,330 600,470 440,550 440,410" fill="#0a0a0b" stroke="#0a0a0b" strokeWidth="1.4" />
                <g>
                  <rect x="305" y="350" width="60" height="6" fill="#0a0a0b" transform="skewY(30)" />
                  <rect x="305" y="370" width="90" height="6" fill="#0a0a0b" opacity="0.6" transform="skewY(30)" />
                  <rect x="305" y="390" width="40" height="6" fill="#0a0a0b" opacity="0.8" transform="skewY(30)" />
                  <rect x="305" y="410" width="110" height="6" fill="#0a0a0b" opacity="0.5" transform="skewY(30)" />
                  <rect x="305" y="430" width="70" height="6" fill="#0a0a0b" transform="skewY(30)" />
                </g>
                <g>
                  <polygon points="465,360 580,360 580,490 465,540" fill="#101014" stroke="#2a2a30" strokeWidth="0.8" />
                  <g fontFamily="JetBrains Mono" fontSize="8" fill="#22c55e">
                    <text x="475" y="380" transform="skewY(30) translate(0 -120)">$ pilot deploy</text>
                    <text x="475" y="395" transform="skewY(30) translate(0 -120)" fill="#9b5cff">→ building image</text>
                    <text x="475" y="410" transform="skewY(30) translate(0 -120)" fill="#9c9ca3">→ pushing 3.2MB</text>
                    <text x="475" y="425" transform="skewY(30) translate(0 -120)" fill="#22c55e">✓ healthy</text>
                  </g>
                </g>
              </g>

              {/* FRONT BOX: small accent cube */}
              <g className="iso-cube" transform="translate(-20 80)">
                <polygon points="160,460 240,420 320,460 240,500" fill="#7c3cf0" stroke="#0a0a0b" strokeWidth="1.2" />
                <polygon points="160,460 160,520 240,560 240,500" fill="#3d1a7a" stroke="#0a0a0b" strokeWidth="1.2" />
                <polygon points="320,460 320,520 240,560 240,500" fill="#5b1bd6" stroke="#0a0a0b" strokeWidth="1.2" />
                <path d="M225,445 L245,425 L240,440 L255,438 L235,460 L240,448 Z" fill="#fff" opacity="0.9" />
              </g>

              {/* Connection lines */}
              <g fill="none" stroke="#7c3cf0" strokeWidth="1.5" opacity="0.75">
                <path d="M220,540 L240,560 L240,580 L440,470" className="pipe-flow" />
                <path d="M520,330 L600,290 L640,290" className="pipe-flow" />
              </g>

              {/* Floating data tags */}
              <g className="iso-tag" filter="url(#boxShadow)">
                <rect x="530" y="170" width="170" height="48" fill="#111114" stroke="#2a2a30" />
                <circle cx="545" cy="194" r="4" fill="#22c55e" />
                <text x="558" y="190" fontFamily="JetBrains Mono" fontSize="9" fill="#ece9e0">DEPLOY · us-east-1</text>
                <text x="558" y="206" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">latency 42ms · p99 ok</text>
              </g>
              <g className="iso-tag" filter="url(#boxShadow)">
                <rect x="40" y="540" width="180" height="48" fill="#111114" stroke="#2a2a30" />
                <text x="54" y="560" fontFamily="JetBrains Mono" fontSize="9" fill="#9b5cff">PILOT · agent.thread</text>
                <text x="54" y="576" fontFamily="JetBrains Mono" fontSize="9" fill="#9c9ca3">applied terraform plan</text>
              </g>

              {/* Tiny pixel cloud */}
              <g opacity="0.9">
                <rect x="500" y="80" width="8" height="8" fill="#7c3cf0" />
                <rect x="508" y="72" width="8" height="8" fill="#9b5cff" />
                <rect x="516" y="80" width="8" height="8" fill="#7c3cf0" />
                <rect x="492" y="88" width="8" height="8" fill="#5b1bd6" />
                <rect x="524" y="88" width="8" height="8" fill="#5b1bd6" />
              </g>
            </svg>
          </div>
        </div>

        {/* Stats bar */}
        <div className="hero-stats">
          <div className="hero-stat">
            <span className="n h-serif">
              14<span style={{ color: "var(--accent-2)" }}>×</span>
            </span>
            <span className="l mono">faster than a human SRE</span>
          </div>
          <div className="hero-stat">
            <span className="n h-serif">99.98%</span>
            <span className="l mono">uptime across managed fleets</span>
          </div>
          <div className="hero-stat">
            <span className="n h-serif">$0.40</span>
            <span className="l mono">avg cost per deploy</span>
          </div>
          <div className="hero-stat">
            <span className="n h-serif">∞</span>
            <span className="l mono">midnight pages handled solo</span>
          </div>
        </div>

        {/* Logo strip */}
        <div className="logos">
          <span className="logos-label">Shipping prod for teams at</span>
          <div className="logos-row">
            <span className="h-sans" style={{ fontSize: "18px", letterSpacing: "-0.04em" }}>◆ northwave</span>
            <span className="mono" style={{ fontSize: "13px" }}>[ LATCH/io ]</span>
            <span className="h-sans" style={{ fontSize: "20px", letterSpacing: "-0.04em" }}>
              parable<span style={{ color: "var(--accent-2)" }}>·</span>
            </span>
            <span className="h-serif" style={{ fontSize: "22px", fontStyle: "italic" }}>Vellum</span>
            <span className="mono" style={{ fontSize: "13px" }}>▣ HARBOUR</span>
            <span className="h-sans" style={{ fontSize: "18px", letterSpacing: "-0.04em" }}>⌬ ortus labs</span>
            <span className="h-serif" style={{ fontSize: "22px" }}>Kiln &amp; Co.</span>
          </div>
        </div>
      </div>
    </header>
  );
}
