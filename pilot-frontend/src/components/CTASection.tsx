export function CTASection() {
  return (
    <section className="cta-section bg-dark">
      <div className="container">
        <div className="cta-inner">
          <span className="eyebrow on-dark">// ready when you are</span>
          <h2 className="h-serif">
            Give your DevOps to a machine.<br />
            Get your <em>weekends</em> back.
          </h2>
          <p>
            Eleven minutes from <span className="mono">pilot init</span> to your first autonomous
            deploy. No card. No sales call. No yak shaving.
          </p>
          <div className="cta-buttons">
            <a href="#" className="btn btn-primary">
              Deploy your first service <span className="arrow"></span>
            </a>
            <a href="#" className="btn btn-ghost">
              Talk to a human first
            </a>
          </div>
        </div>

        {/* big iso footer composition */}
        <div style={{ marginTop: "60px", position: "relative" }}>
          <svg
            viewBox="0 0 1280 360"
            style={{ width: "100%", display: "block" }}
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <defs>
              <pattern id="pixGradFooter" x="0" y="0" width="6" height="6" patternUnits="userSpaceOnUse">
                <rect width="6" height="6" fill="#7c3cf0" />
                <rect width="3" height="3" fill="#9b5cff" />
                <rect x="3" y="3" width="3" height="3" fill="#5b1bd6" />
              </pattern>
            </defs>

            {/* back row of cubes */}
            <g className="lift" opacity="0.7">
              <g transform="translate(160 60)">
                <polygon points="0,40 60,8 120,40 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="0,40 0,100 60,132 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="120,40 120,100 60,132 60,72" fill="#141418" stroke="#2a2a30" />
              </g>
              <g transform="translate(320 100)">
                <polygon points="0,40 60,8 120,40 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="0,40 0,100 60,132 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="120,40 120,100 60,132 60,72" fill="#141418" stroke="#2a2a30" />
              </g>
              <g transform="translate(480 60)">
                <polygon points="0,40 60,8 120,40 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="0,40 0,100 60,132 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="120,40 120,100 60,132 60,72" fill="#141418" stroke="#2a2a30" />
              </g>
              <g transform="translate(640 100)">
                <polygon points="0,40 60,8 120,40 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="0,40 0,100 60,132 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="120,40 120,100 60,132 60,72" fill="#141418" stroke="#2a2a30" />
              </g>
              <g transform="translate(800 60)">
                <polygon points="0,40 60,8 120,40 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="0,40 0,100 60,132 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="120,40 120,100 60,132 60,72" fill="#141418" stroke="#2a2a30" />
              </g>
              <g transform="translate(960 100)">
                <polygon points="0,40 60,8 120,40 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="0,40 0,100 60,132 60,72" fill="#0a0a0b" stroke="#2a2a30" />
                <polygon points="120,40 120,100 60,132 60,72" fill="#141418" stroke="#2a2a30" />
              </g>
            </g>

            {/* one violet accent cube center-stage */}
            <g className="lift" transform="translate(560 170)">
              <polygon points="0,40 60,8 120,40 60,72" fill="url(#pixGradFooter)" stroke="#0a0a0b" />
              <polygon points="0,40 0,100 60,132 60,72" fill="#ece9e0" stroke="#0a0a0b" />
              <polygon points="120,40 120,100 60,132 60,72" fill="#0a0a0b" stroke="#0a0a0b" />
            </g>
          </svg>
        </div>
      </div>
    </section>
  );
}
