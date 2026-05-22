export function PainSection() {
  return (
    <section className="pain">
      <div className="container">
        <span className="eyebrow">The state of DevOps</span>
        <h2 className="h-serif">
          Your infra is duct‑tape, your<br />
          on‑call rotation is <em>resentment</em>,<br />
          and every deploy is a séance.
        </h2>

        <div className="pain-grid">
          {/* card 1 */}
          <div className="pain-card">
            <span className="num mono">01 — TOIL</span>
            <svg className="ico" viewBox="0 0 96 96" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <polygon points="20,52 40,40 60,52 40,64" fill="#0a0a0b" />
              <polygon points="20,52 20,68 40,80 40,64" fill="#0a0a0b" opacity="0.75" />
              <polygon points="60,52 60,68 40,80 40,64" fill="#0a0a0b" opacity="0.55" />
              <path d="M40,80 L60,72 L72,80 L80,76" stroke="#c43d28" strokeWidth="2.4" fill="none" strokeLinejoin="miter" />
              <rect x="74" y="72" width="6" height="6" fill="#c43d28" />
              <rect x="80" y="78" width="6" height="6" fill="#c43d28" />
              <path d="M55,30 L52,40 L58,38 L54,50" stroke="#c43d28" strokeWidth="2" fill="none" />
            </svg>
            <h3>3 a.m. pages for the same flaky pod</h3>
            <p>
              Your strongest engineer is awake at 3:14, copy‑pasting{" "}
              <span className="mono">kubectl describe</span> into Slack for the fourth night this week.
            </p>
          </div>

          {/* card 2 */}
          <div className="pain-card">
            <span className="num mono">02 — DRIFT</span>
            <svg className="ico" viewBox="0 0 96 96" aria-hidden="true">
              <polygon points="14,42 28,34 42,42 28,50" fill="#0a0a0b" />
              <polygon points="14,42 14,56 28,64 28,50" fill="#0a0a0b" opacity="0.75" />
              <polygon points="42,42 42,56 28,64 28,50" fill="#0a0a0b" opacity="0.55" />
              <polygon points="56,52 70,44 84,52 70,60" fill="#0a0a0b" />
              <polygon points="56,52 56,66 70,74 70,60" fill="#0a0a0b" opacity="0.75" />
              <polygon points="84,52 84,66 70,74 70,60" fill="#0a0a0b" opacity="0.55" />
              <path d="M42,48 q4,-6 8,0 t8,0" stroke="#c43d28" strokeWidth="2" fill="none" />
              <rect x="44" y="44" width="3" height="3" fill="#c43d28" />
              <rect x="52" y="48" width="3" height="3" fill="#c43d28" />
            </svg>
            <h3>&ldquo;It works on my staging&rdquo;</h3>
            <p>
              Six environments. Four Terraform repos. One Notion doc that was last accurate in March.
              The config has drifted, and so has the team.
            </p>
          </div>

          {/* card 3 */}
          <div className="pain-card">
            <span className="num mono">03 — COST</span>
            <svg className="ico" viewBox="0 0 96 96" aria-hidden="true">
              <polygon points="14,68 32,58 50,68 32,78" fill="#0a0a0b" />
              <polygon points="14,68 14,76 32,86 32,78" fill="#0a0a0b" opacity="0.7" />
              <polygon points="50,68 50,76 32,86 32,78" fill="#0a0a0b" opacity="0.5" />
              <polygon points="22,56 38,48 54,56 38,64" fill="#0a0a0b" />
              <path d="M60,72 L72,52 L66,52 L72,40 L78,52 L72,52 Z" fill="#c43d28" />
            </svg>
            <h3>AWS bill nobody can defend</h3>
            <p>
              Three unused RDS instances. A NAT gateway from a 2022 spike. Idle GPUs in{" "}
              <span className="mono">eu‑west‑3</span>. Finance keeps asking. Nobody knows.
            </p>
          </div>

          {/* card 4 */}
          <div className="pain-card">
            <span className="num mono">04 — INCIDENT</span>
            <svg className="ico" viewBox="0 0 96 96" aria-hidden="true">
              <polygon points="48,16 76,28 76,54 48,80 20,54 20,28" fill="#0a0a0b" />
              <polygon points="48,16 76,28 48,40 20,28" fill="#0a0a0b" opacity="0.5" />
              <path d="M48,28 L42,50 L52,48 L46,68" stroke="#c43d28" strokeWidth="2.4" fill="none" />
              <rect x="44" y="44" width="3" height="3" fill="#c43d28" />
              <rect x="50" y="56" width="3" height="3" fill="#c43d28" />
            </svg>
            <h3>Rollbacks are folklore</h3>
            <p>
              You <em>think</em> you can roll back. You&apos;re not sure who has the latest good SHA. The
              runbook lives in a Notion doc that 404s on mobile.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
