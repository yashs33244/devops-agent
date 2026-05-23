import type { Metadata } from "next";
import Link from "next/link";
import { Nav } from "@/components/Nav";

export const metadata: Metadata = {
  title: "Pricing",
  description: "Pilot is free. Open source. No catch.",
};

export default function PricingPage() {
  return (
    <>
      <Nav />
      <main
        style={{
          minHeight: "100vh",
          background: "var(--bg-dark)",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          padding: "80px 24px 120px",
          position: "relative",
          overflow: "hidden",
        }}
      >
        <style>{`
          @keyframes drift {
            0%   { transform: translate(0, 0) rotate(0deg); opacity: 0.04; }
            33%  { transform: translate(18px, -12px) rotate(4deg); opacity: 0.07; }
            66%  { transform: translate(-10px, 20px) rotate(-3deg); opacity: 0.05; }
            100% { transform: translate(0, 0) rotate(0deg); opacity: 0.04; }
          }
          .price-bg-mark {
            position: absolute;
            width: 520px;
            height: 520px;
            border: 1px solid var(--line-dark);
            animation: drift 18s ease-in-out infinite;
            pointer-events: none;
          }
          .price-bg-mark-2 {
            position: absolute;
            width: 320px;
            height: 320px;
            border: 1px solid var(--line-dark);
            animation: drift 24s ease-in-out infinite reverse;
            pointer-events: none;
          }
          .price-zero {
            font-family: "Instrument Serif", "Times New Roman", serif;
            font-size: clamp(140px, 28vw, 320px);
            line-height: 0.85;
            letter-spacing: -0.04em;
            color: var(--text-on-dark);
            position: relative;
            user-select: none;
          }
          .price-zero::after {
            content: "$0";
            position: absolute;
            inset: 0;
            color: var(--accent);
            opacity: 0;
            transition: opacity 0.4s ease;
          }
          .price-zero:hover::after {
            opacity: 1;
          }
          .price-tag {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            padding: "4px 12px";
            border: 1px solid var(--line-dark-2);
            border-radius: 2px;
            font-size: 11px;
            letter-spacing: 0.12em;
            text-transform: uppercase;
            color: var(--text-on-dark-soft);
            font-family: "JetBrains Mono", monospace;
            margin-bottom: 40px;
          }
          .price-dot {
            width: 6px;
            height: 6px;
            background: var(--good);
            border-radius: 50%;
            display: inline-block;
          }
          .price-features {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 1px;
            background: var(--line-dark);
            border: 1px solid var(--line-dark);
            border-radius: 4px;
            overflow: hidden;
            max-width: 680px;
            width: 100%;
            margin: 48px auto 0;
          }
          .price-feature {
            background: var(--bg-dark-2);
            padding: 20px 22px;
          }
          .price-feature-label {
            font-size: 11px;
            color: var(--text-on-dark-soft);
            font-family: "JetBrains Mono", monospace;
            letter-spacing: 0.08em;
            text-transform: uppercase;
            margin-bottom: 6px;
          }
          .price-feature-value {
            font-size: 15px;
            color: var(--text-on-dark);
            font-weight: 500;
            font-family: "Inter Tight", sans-serif;
          }
          @media (max-width: 600px) {
            .price-features { grid-template-columns: 1fr 1fr; }
          }
        `}</style>

        {/* Background decorative marks */}
        <div className="price-bg-mark" style={{ top: "10%", right: "5%" }} />
        <div className="price-bg-mark-2" style={{ bottom: "15%", left: "8%" }} />

        {/* Tag */}
        <div className="price-tag">
          <span className="price-dot" />
          Open source · MIT License
        </div>

        {/* The number */}
        <div className="price-zero">free</div>

        {/* Tagline */}
        <p
          style={{
            marginTop: "32px",
            fontSize: "clamp(16px, 2.5vw, 22px)",
            color: "var(--text-on-dark-soft)",
            textAlign: "center",
            maxWidth: "520px",
            lineHeight: 1.5,
            fontFamily: "Inter Tight, sans-serif",
            fontWeight: 400,
          }}
        >
          Free like your free will.{" "}
          <span style={{ color: "var(--text-on-dark)" }}>
            Fork it. Break it. Deploy it. Own it.
          </span>
        </p>

        {/* Manifesto */}
        <p
          style={{
            marginTop: "16px",
            fontSize: "14px",
            color: "var(--text-on-dark-soft)",
            textAlign: "center",
            maxWidth: "440px",
            lineHeight: 1.7,
            opacity: 0.7,
          }}
        >
          No seat licenses. No usage meters. No enterprise tier locked behind a sales call.
          The agent runs on your machine, your cluster, your terms.
        </p>

        {/* Feature grid */}
        <div className="price-features">
          {[
            { label: "Environments", value: "Unlimited" },
            { label: "Deploys", value: "Unlimited" },
            { label: "Clouds", value: "AWS · Azure · GCP" },
            { label: "Agents", value: "All 6 included" },
            { label: "CI minutes", value: "Your own runner" },
            { label: "Support", value: "GitHub Issues" },
          ].map((f) => (
            <div key={f.label} className="price-feature">
              <div className="price-feature-label">{f.label}</div>
              <div className="price-feature-value">{f.value}</div>
            </div>
          ))}
        </div>

        {/* CTAs */}
        <div
          style={{
            display: "flex",
            gap: "12px",
            marginTop: "48px",
            flexWrap: "wrap",
            justifyContent: "center",
          }}
        >
          <a
            href="https://github.com/yashs33244/devops-agent"
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-primary"
          >
            Clone on GitHub <span className="arrow" />
          </a>
          <Link href="/docs/quick-start" className="btn btn-ghost">
            Read the docs
          </Link>
        </div>

        {/* Fine print */}
        <p
          style={{
            marginTop: "32px",
            fontSize: "12px",
            color: "var(--text-on-dark-soft)",
            opacity: 0.45,
            fontFamily: "JetBrains Mono, monospace",
            letterSpacing: "0.04em",
          }}
        >
          * You still pay for your own cloud infrastructure. We don&apos;t touch that.
        </p>
      </main>
    </>
  );
}
