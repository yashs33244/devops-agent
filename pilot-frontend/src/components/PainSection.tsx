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
              {/* Clock face */}
              <circle cx="44" cy="44" r="26" fill="#0a0a0b" stroke="#3a3a40" strokeWidth="1.5"/>
              <circle cx="44" cy="44" r="2.5" fill="#5a5a62"/>
              {/* Hour ticks at 12/3/6/9 */}
              <rect x="42.5" y="19" width="3" height="5" rx="1" fill="#3a3a40"/>
              <rect x="69" y="42.5" width="5" height="3" rx="1" fill="#3a3a40"/>
              <rect x="42.5" y="70" width="3" height="5" rx="1" fill="#3a3a40"/>
              <rect x="18" y="42.5" width="5" height="3" rx="1" fill="#3a3a40"/>
              {/* Hour hand → 3 o'clock */}
              <line x1="44" y1="44" x2="62" y2="44" stroke="#ece9e0" strokeWidth="2.2" strokeLinecap="round"/>
              {/* Minute hand → just past 12 (~3:02) */}
              <line x1="44" y1="44" x2="46" y2="22" stroke="#ece9e0" strokeWidth="1.5" strokeLinecap="round"/>
              {/* Red alert badge */}
              <circle cx="68" cy="24" r="12" fill="#c43d28"/>
              <rect x="66" y="17" width="4" height="9" rx="1.5" fill="white"/>
              <rect x="66" y="28" width="4" height="4" rx="1.5" fill="white"/>
              {/* Three repeat dots — "again, again, again" */}
              <circle cx="34" cy="84" r="3.5" fill="#c43d28"/>
              <circle cx="44" cy="84" r="3.5" fill="#c43d28"/>
              <circle cx="54" cy="84" r="3.5" fill="#c43d28"/>
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
              {/* Staging box */}
              <rect x="6" y="28" width="36" height="26" rx="2" fill="#0a0a0b" stroke="#3a3a40" strokeWidth="1.5"/>
              <text x="24" y="38" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="7" fill="#5a5a62">STAGING</text>
              {/* Green dot — works */}
              <circle cx="24" cy="47" r="5" fill="#1a3a1a" stroke="#27c93f" strokeWidth="1.5"/>
              <circle cx="24" cy="47" r="2" fill="#27c93f"/>
              <text x="24" y="66" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="7" fill="#5a5a62">✓ works</text>
              {/* Prod box */}
              <rect x="54" y="28" width="36" height="26" rx="2" fill="#0a0a0b" stroke="#c43d28" strokeWidth="1.5"/>
              <text x="72" y="38" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="7" fill="#5a5a62">PROD</text>
              {/* Red dot — broken */}
              <circle cx="72" cy="47" r="5" fill="#3a0a0a" stroke="#c43d28" strokeWidth="1.5"/>
              <text x="72" y="51" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#c43d28">✗</text>
              <text x="72" y="66" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="7" fill="#c43d28">✗ broke</text>
              {/* Shared origin line at top — diverging */}
              <line x1="24" y1="20" x2="24" y2="28" stroke="#3a3a40" strokeWidth="1.5"/>
              <line x1="24" y1="20" x2="72" y2="20" stroke="#3a3a40" strokeWidth="1.5" strokeDasharray="3 2"/>
              <line x1="72" y1="20" x2="72" y2="28" stroke="#c43d28" strokeWidth="1.5"/>
              {/* ≠ center */}
              <text x="48" y="48" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="14" fill="#c43d28">≠</text>
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
              {/* Y axis */}
              <line x1="12" y1="78" x2="12" y2="8" stroke="#3a3a40" strokeWidth="1.5"/>
              {/* X axis */}
              <line x1="12" y1="78" x2="88" y2="78" stroke="#3a3a40" strokeWidth="1.5"/>
              {/* Bar 1 — small */}
              <rect x="18" y="65" width="13" height="13" fill="#2a2a30" rx="1"/>
              {/* Bar 2 — medium */}
              <rect x="35" y="54" width="13" height="24" fill="#2a2a30" rx="1"/>
              {/* Bar 3 — larger */}
              <rect x="52" y="44" width="13" height="34" fill="#2a2a30" rx="1"/>
              {/* Bar 4 — HUGE, clipped off the top, red */}
              <rect x="69" y="8" width="13" height="70" fill="#c43d28" rx="1"/>
              {/* Arrow going up off chart */}
              <line x1="75" y1="8" x2="75" y2="3" stroke="#c43d28" strokeWidth="2" strokeLinecap="round"/>
              <polygon points="75,1 71,7 79,7" fill="#c43d28"/>
              {/* $ on the red bar */}
              <text x="75" y="22" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="11" fontWeight="bold" fill="white">$</text>
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
              {/* Commit timeline */}
              <line x1="10" y1="56" x2="86" y2="56" stroke="#3a3a40" strokeWidth="1.5"/>
              {/* Good commits */}
              <circle cx="22" cy="56" r="7" fill="#1d1d22" stroke="#3a3a40" strokeWidth="1.5"/>
              <circle cx="44" cy="56" r="7" fill="#1d1d22" stroke="#3a3a40" strokeWidth="1.5"/>
              <circle cx="66" cy="56" r="7" fill="#1d1d22" stroke="#3a3a40" strokeWidth="1.5"/>
              {/* Bad commit — red */}
              <circle cx="86" cy="56" r="7" fill="#3a0a0a" stroke="#c43d28" strokeWidth="1.5"/>
              <text x="86" y="60" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="9" fill="#c43d28">✗</text>
              {/* Dashed revert arc going back (broken / folklore) */}
              <path d="M86,49 Q66,20 44,49" stroke="#c43d28" strokeWidth="1.5" fill="none" strokeDasharray="4 3"/>
              {/* Arrowhead at end of revert arc — pointing left-down */}
              <polygon points="44,49 52,42 52,52" fill="#c43d28"/>
              {/* Big "?" — nobody knows the good SHA */}
              <text x="44" y="34" textAnchor="middle" fontFamily="JetBrains Mono" fontSize="20" fontWeight="bold" fill="#c43d28">?</text>
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
