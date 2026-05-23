export function Hero() {
  return (
    <header className="hero bg-dark">
      {/* Responsive CSS specifically injected for mobile-friendliness and animations */}
      <style dangerouslySetInnerHTML={{
        __html: `
          .hero-container {
            padding: 4rem 2rem 5rem;
            max-width: 1200px;
            margin: 0 auto;
            overflow: hidden;
          }

          .hero-inner {
            display: flex;
            flex-direction: column;
            gap: 3rem;
            align-items: center;
          }

          .hero-left {
            flex: 1;
            width: 100%;
            display: flex;
            flex-direction: column;
            align-items: center;
            text-align: center;
            gap: 0.25rem;
          }
          
          .hero-right {
            min-width: 50%;
            margin-top: -2rem;
            align-self: center;
          }
          
          .hero-cta {
            display: flex;
            flex-direction: column;
            gap: 1rem;
            width: 100%;
            max-width: 320px;
            margin-top: 0.5rem;
          }
          
          .btn {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 0.875rem 1.5rem;
            border-radius: 4px;
            font-weight: 600;
            text-decoration: none;
            transition: all 0.2s ease;
          }
          
          .btn-primary {
            background: #7c3cf0;
            color: #fff;
          }
          
          .btn-ghost {
            background: transparent;
            color: #fff;
            border: 1px solid #3a3a40;
          }

          .hero-stats {
            display: grid;
            grid-template-columns: 1fr;
            gap: 2rem;
            margin-top: 3rem;
            border-top: 1px solid #2a2a30;
            padding-top: 3rem;
          }
          
          .hero-stat {
            display: flex;
            flex-direction: column;
            align-items: center;
            text-align: center;
          }
          
          .logos-row {
            display: flex;
            flex-wrap: wrap;
            justify-content: center;
            gap: 1.5rem;
            align-items: center;
            color: #fff;
            opacity: 0.6;
          }

          /* Desktop Overrides */
          @media (min-width: 768px) {
            .hero-stats {
              grid-template-columns: repeat(2, 1fr);
            }
          }

          @media (min-width: 1024px) {
            .hero-container {
              padding: 0rem 0rem 4rem;
            }
            .hero-inner {
              flex-direction: row;
              gap: 5rem;
              align-items: center;
            }
            .hero-left {
              align-items: flex-start;
              text-align: left;
            }
            .hero-cta {
              flex-direction: row;
              max-width: none;
              justify-content: flex-start;
            }
            .hero-right {
              min-width: 52%;
              margin-top: 0;
            }
            .hero-stats {
              grid-template-columns: repeat(4, 1fr);
              margin-top: 5rem;
            }
            .hero-stat {
              align-items: flex-start;
              text-align: left;
            }
            .logos-row {
              gap: 3rem;
            }
          }

          /* Simple Glow Animation on Hover (No Transform/Bounce) */
          .hover-lift {
            transition: filter 0.3s ease;
            cursor: crosshair;
          }
          .hover-lift:hover {
            filter: drop-shadow(0px 0px 20px rgba(155, 92, 255, 0.8)) brightness(1.3);
          }
          
          /* Smooth, Unidirectional Flow Animations */
          .pipeline-flow {
            stroke-dasharray: 12 12;
            animation: flowAnim 3s linear infinite;
          }
          .pipeline-flow-reverse {
            stroke-dasharray: 12 12;
            animation: flowAnimRev 3s linear infinite;
          }
          
          @keyframes flowAnim { 
            0% { stroke-dashoffset: 24; } 
            100% { stroke-dashoffset: 0; } 
          }
          @keyframes flowAnimRev { 
            0% { stroke-dashoffset: -24; } 
            100% { stroke-dashoffset: 0; } 
          }
        `
      }} />

      <div className="hero-container">
        <div className="hero-inner">
          {/* LEFT SIDE: Content */}
          <div className="hero-left">
            <span style={{ display: "inline-flex", alignItems: "center", marginBottom: "2rem", padding: "0.5rem 1.25rem", background: "rgba(155, 92, 255, 0.1)", color: "#9b5cff", borderRadius: "100px", fontSize: "0.875rem", fontFamily: "monospace" }}>
              <span style={{ width: "8px", height: "8px", background: "#22c55e", borderRadius: "50%", marginRight: "8px", boxShadow: "0 0 8px #22c55e" }}></span>
              AGENT v2.0 LIVE IN PROD
            </span>
            <h1 className="h-serif" style={{ fontSize: "clamp(2.8rem, 5.5vw, 4.25rem)", lineHeight: 1.15, marginBottom: "2rem", color: "#fff", fontWeight: 800, letterSpacing: "-0.02em" }}>
              The AI Site Reliability<br />
              Engineer that <em style={{ color: "#9b5cff", fontStyle: "italic" }}>never</em> sleeps.
            </h1>
            <p className="hero-sub" style={{ fontSize: "1.2rem", color: "#9c9ca3", lineHeight: 1.75, marginBottom: "2.5rem", maxWidth: "500px" }}>
              Provisions infra, runs deployments, and remediates drift — automatically. You push code. Pilot handles the pager.
            </p>
            <div className="hero-cta">
              <a href="https://github.com/yashs33244/devops-agent" target="_blank" rel="noopener noreferrer" className="btn btn-primary">
                Deploy Agent
              </a>
              <a href="https://github.com/yashs33244/devops-agent" target="_blank" rel="noopener noreferrer" className="btn btn-ghost">
                View Audit Logs
              </a>
            </div>
          </div>

          {/* RIGHT SIDE: Tightly cropped SVG viewBox to remove excess top/bottom space */}
          <div className="hero-right" style={{ marginTop: "-2rem" }}>
            <svg
              className="hero-iso"
              viewBox="0 100 800 600"
              xmlns="http://www.w3.org/2000/svg"
              aria-hidden="true"
              style={{ width: "100%", height: "auto", display: "block" }}
            >
              <defs>
                <pattern id="pixGrad" x="0" y="0" width="12" height="12" patternUnits="userSpaceOnUse" patternTransform="rotate(30)">
                  <rect width="12" height="12" fill="#5b1bd6" />
                  <rect width="6" height="6" fill="#9b5cff" />
                  <rect x="6" y="6" width="6" height="6" fill="#7c3cf0" />
                </pattern>

                <filter id="neonGlow" x="-50%" y="-50%" width="200%" height="200%">
                  <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur" />
                  <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
                </filter>

                {/* Cube Prototypes */}
                <g id="gray-cube">
                  <polygon points="0,0 60,-30 120,0 60,30" fill="#2a2a30" stroke="#3a3a40" strokeWidth="1.5" strokeLinejoin="round" />
                  <polygon points="0,0 0,60 60,90 60,30" fill="#0a0a0b" stroke="#1d1d22" strokeWidth="1.5" strokeLinejoin="round" />
                  <polygon points="120,0 120,60 60,90 60,30" fill="#141418" stroke="#1d1d22" strokeWidth="1.5" strokeLinejoin="round" />
                </g>

                <g id="purple-cube">
                  <polygon points="0,0 60,-30 120,0 60,30" fill="#7c3cf0" stroke="#9b5cff" strokeWidth="1.5" strokeLinejoin="round" />
                  <polygon points="0,0 0,60 60,90 60,30" fill="#3d1a7a" stroke="#5b1bd6" strokeWidth="1.5" strokeLinejoin="round" />
                  <polygon points="120,0 120,60 60,90 60,30" fill="#220d47" stroke="#5b1bd6" strokeWidth="1.5" strokeLinejoin="round" />
                </g>

                <g id="pattern-cube">
                  <polygon points="0,0 60,-30 120,0 60,30" fill="url(#pixGrad)" stroke="#9b5cff" strokeWidth="1.5" strokeLinejoin="round" />
                  <polygon points="0,0 0,60 60,90 60,30" fill="#3d1a7a" stroke="#0a0a0b" strokeWidth="1.5" strokeLinejoin="round" />
                  <polygon points="120,0 120,60 60,90 60,30" fill="#141418" stroke="#0a0a0b" strokeWidth="1.5" strokeLinejoin="round" />
                </g>

                <g id="agent-cube">
                  <use href="#gray-cube" />
                  <g stroke="#06b6d4" strokeWidth="1.5" strokeLinecap="round">
                    <line x1="20" y1="35" x2="45" y2="47.5" />
                    <line x1="20" y1="45" x2="35" y2="52.5" />
                    <line x1="20" y1="55" x2="40" y2="65" />
                  </g>
                  <circle cx="45" cy="30" r="3" fill="#06b6d4" filter="url(#neonGlow)" />
                </g>
              </defs>

              {/* ================= BACKGROUND GRID ================= */}
              <g opacity="0.1" stroke="#7c3cf0" strokeWidth="1">
                <line x1="-200" y1="0" x2="1000" y2="600" />
                <line x1="-200" y1="100" x2="1000" y2="700" />
                <line x1="-200" y1="200" x2="1000" y2="800" />
                <line x1="-200" y1="300" x2="1000" y2="900" />
                <line x1="-200" y1="400" x2="1000" y2="1000" />
                <line x1="-200" y1="-100" x2="1000" y2="500" />
                <line x1="-200" y1="600" x2="1000" y2="0" />
                <line x1="-200" y1="700" x2="1000" y2="100" />
                <line x1="-200" y1="800" x2="1000" y2="200" />
                <line x1="-200" y1="900" x2="1000" y2="300" />
                <line x1="-200" y1="500" x2="1000" y2="-100" />
                <line x1="-200" y1="400" x2="1000" y2="-200" />
              </g>

              {/* ================= THIN PIPELINE PATHS ================= */}
              <g fill="none" strokeLinejoin="round" opacity="0.8">
                {/* Structural Lines */}
                <polyline points="200,500 400,400 600,500" stroke="#2a2a30" strokeWidth="2" />
                <polyline points="300,350 400,300 500,350" stroke="#2a2a30" strokeWidth="2" />
                <polyline points="400,400 400,300" stroke="#2a2a30" strokeWidth="2" />
                
                {/* Glowing Flow Lines */}
                <polyline points="200,500 400,400 600,500" stroke="#7c3cf0" strokeWidth="1" />
                <polyline points="200,500 400,400 600,500" stroke="#9b5cff" strokeWidth="2" className="pipeline-flow" />
                
                <polyline points="300,350 400,300 500,350" stroke="#06b6d4" strokeWidth="1" />
                <polyline points="300,350 400,300 500,350" stroke="#06b6d4" strokeWidth="2" className="pipeline-flow-reverse" />
                
                <polyline points="400,400 400,300" stroke="#22c55e" strokeWidth="1" />
                <polyline points="400,400 400,300" stroke="#22c55e" strokeWidth="2" className="pipeline-flow" />

                {/* Packet indicators */}
                <circle cx="280" cy="460" r="3" fill="#9b5cff" filter="url(#neonGlow)" />
                <circle cx="520" cy="460" r="3" fill="#9b5cff" filter="url(#neonGlow)" />
              </g>

              {/* ================= CONNECTED CUBE NODES ================= */}
              {/* Center 1: Bottom Left Node */}
              <g transform="translate(80, 400) scale(1.9)" className="hover-lift">
                <use href="#purple-cube" x="0" y="0" />
              </g>

              {/* Center 2: Bottom Middle Node */}
              <g transform="translate(310, 320) scale(1.35)" className="hover-lift">
                <use href="#pattern-cube" x="0" y="0" />
              </g>

              {/* Center 3: Bottom Right Node */}
              <g transform="translate(510, 415) scale(1.55)" className="hover-lift">
                <use href="#agent-cube" x="0" y="0" />
              </g>

              {/* Center 4: Top Left Node */}
              <g transform="translate(240, 290) scale(0.85)" className="hover-lift">
                <use href="#gray-cube" x="0" y="0" />
              </g>

              {/* Center 5: Top Middle Node */}
              <g transform="translate(330, 220) scale(1.1)" className="hover-lift">
                <use href="#agent-cube" x="0" y="0" />
              </g>

              {/* Center 6: Top Right Node */}
              <g transform="translate(450, 290) scale(0.85)" className="hover-lift">
                <use href="#purple-cube" x="0" y="0" />
              </g>

              {/* ================= SCATTERED / AMBIENT CUBES ================= */}
              <g transform="translate(60, 180) scale(0.55)" className="hover-lift">
                <use href="#purple-cube" x="0" y="0" />
              </g>
              <g transform="translate(670, 230) scale(0.65)" className="hover-lift">
                <use href="#pattern-cube" x="0" y="0" />
              </g>
              <g transform="translate(120, 630) scale(0.9)" className="hover-lift">
                <use href="#gray-cube" x="0" y="0" />
              </g>
              <g transform="translate(610, 600) scale(0.8)" className="hover-lift">
                <use href="#purple-cube" x="0" y="0" />
              </g>
              <g transform="translate(410, 130) scale(0.45)" className="hover-lift">
                <use href="#gray-cube" x="0" y="0" />
              </g>
              <g transform="translate(510, 195) scale(0.55)" className="hover-lift">
                <use href="#gray-cube" x="0" y="0" />
              </g>
            </svg>
          </div>
        </div>

        {/* Stats bar */}
        <div className="hero-stats">
          <div className="hero-stat">
            <span className="n h-serif" style={{ display: "block", fontSize: "2.5rem", color: "#fff", marginBottom: "0.25rem" }}>
              &lt; 2<span style={{ color: "#9b5cff", fontSize: "1.5rem" }}>min</span>
            </span>
            <span className="l mono" style={{ fontSize: "0.875rem", color: "#9c9ca3", fontFamily: "monospace" }}>mean time to recovery (MTTR)</span>
          </div>
          <div className="hero-stat">
            <span className="n h-serif" style={{ display: "block", fontSize: "2.5rem", color: "#fff", marginBottom: "0.25rem" }}>
              100<span style={{ color: "#22c55e" }}>%</span>
            </span>
            <span className="l mono" style={{ fontSize: "0.875rem", color: "#9c9ca3", fontFamily: "monospace" }}>infrastructure drift remediated</span>
          </div>
          <div className="hero-stat">
            <span className="n h-serif" style={{ display: "block", fontSize: "2.5rem", color: "#fff", marginBottom: "0.25rem" }}>$0.02</span>
            <span className="l mono" style={{ fontSize: "0.875rem", color: "#9c9ca3", fontFamily: "monospace" }}>compute cost per pipeline run</span>
          </div>
          <div className="hero-stat">
            <span className="n h-serif" style={{ display: "block", fontSize: "2.5rem", color: "#fff", marginBottom: "0.25rem" }}>
              ∞
            </span>
            <span className="l mono" style={{ fontSize: "0.875rem", color: "#9c9ca3", fontFamily: "monospace" }}>midnight alerts handled solo</span>
          </div>
        </div>

        {/* Logo strip */}
        <div style={{ marginTop: "4rem", textAlign: "center" }}>
          <span style={{ display: "block", fontSize: "0.875rem", color: "#9c9ca3", marginBottom: "1.5rem", textTransform: "uppercase", letterSpacing: "1px" }}>
            Automating infrastructure for engineering teams at
          </span>
          <div className="logos-row">
            <span style={{ fontSize: "16px", fontWeight: "bold", fontFamily: "monospace" }}>&lt;Kubernetics/&gt;</span>
            <span style={{ fontSize: "14px", fontFamily: "monospace" }}>[ DATA_LAKE_IO ]</span>
            <span style={{ fontSize: "20px", fontWeight: 800 }}>
              NEXUS<span style={{ color: "#06b6d4" }}>·</span>
            </span>
            <span style={{ fontSize: "22px", fontStyle: "italic", fontFamily: "serif" }}>VoidSystems</span>
            <span style={{ fontSize: "14px", fontFamily: "monospace", border: "1px solid currentColor", padding: "4px 8px" }}>SYS_OPS</span>
          </div>
        </div>
      </div>
    </header>
  );
}