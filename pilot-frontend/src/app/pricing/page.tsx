"use client";

import type { Metadata } from "next";
import Link from "next/link";
import { Nav } from "@/components/Nav";
import { useEffect, useRef, useState } from "react";

const TICKER_ITEMS = [
  "$0 / month",
  "$0 / year",
  "$0 forever",
  "no seat licenses",
  "no usage meters",
  "no enterprise tier",
  "no sales call required",
  "no freemium trap",
  "no credit card",
  "just free",
  "$0 / month",
];

const MANIFESTO_LINES = [
  "> initializing pricing engine...",
  "> calculating your bill...",
  "> applying enterprise discount...",
  "> checking seat licenses...",
  "> ERROR: no billing module found",
  "> ERROR: no payment processor",
  "> ERROR: no pricing tiers",
  "> ...",
  "> oh.",
  "> it's just free.",
];

const FAKE_TIERS = [
  {
    name: "Starter",
    price: "$49/mo",
    limits: ["5 deploys/day", "1 cloud only", "No Terraform", "Email support (72h)"],
  },
  {
    name: "Growth",
    price: "$199/mo",
    limits: ["25 deploys/day", "2 clouds", "Helm included", "Slack support"],
    popular: true,
  },
  {
    name: "Enterprise",
    price: "Contact Sales",
    limits: ["Unlimited deploys", "All clouds", "Everything", "Dedicated CSM"],
  },
];

function GlitchText({ text }: { text: string }) {
  return (
    <div className="glitch-wrap" aria-label={text}>
      <span className="glitch-main">{text}</span>
      <span className="glitch-r" aria-hidden>{text}</span>
      <span className="glitch-b" aria-hidden>{text}</span>
    </div>
  );
}

function Ticker() {
  return (
    <div className="ticker-wrap">
      <div className="ticker-track">
        {[...TICKER_ITEMS, ...TICKER_ITEMS].map((item, i) => (
          <span key={i} className="ticker-item">
            <span className="ticker-dot" />
            {item}
          </span>
        ))}
      </div>
    </div>
  );
}

function Terminal() {
  const [lines, setLines] = useState<string[]>([]);
  const [cursor, setCursor] = useState(true);

  useEffect(() => {
    let i = 0;
    const interval = setInterval(() => {
      if (i < MANIFESTO_LINES.length) {
        const idx = i++;
        setLines((prev) => [...prev, MANIFESTO_LINES[idx]]);
      } else {
        clearInterval(interval);
      }
    }, 420);
    const blink = setInterval(() => setCursor((c) => !c), 530);
    return () => { clearInterval(interval); clearInterval(blink); };
  }, []);

  return (
    <div className="terminal-box">
      <div className="terminal-bar">
        <span className="t-dot" style={{ background: "#ff5f56" }} />
        <span className="t-dot" style={{ background: "#ffbd2e" }} />
        <span className="t-dot" style={{ background: "#27c93f" }} />
        <span className="t-title">pilot — pricing</span>
      </div>
      <div className="terminal-body">
        {lines.map((line, i) => (
          <div
            key={i}
            className="t-line"
            style={{
              color: line.startsWith("> ERROR")
                ? "var(--danger-soft)"
                : line.startsWith("> oh") || line === "> it's just free."
                ? "var(--good)"
                : "var(--text-on-dark-soft)",
              fontWeight: line === "> it's just free." ? 600 : 400,
            }}
          >
            {line}
          </div>
        ))}
        {lines.length < MANIFESTO_LINES.length && (
          <span className="t-cursor" style={{ opacity: cursor ? 1 : 0 }}>█</span>
        )}
      </div>
    </div>
  );
}

function FakeTierCard({ tier, index }: { tier: typeof FAKE_TIERS[0]; index: number }) {
  const [destroyed, setDestroyed] = useState(false);

  return (
    <div
      className={`fake-tier ${destroyed ? "tier-destroyed" : ""}`}
      style={{ animationDelay: `${index * 0.15}s` }}
    >
      {tier.popular && <div className="tier-badge">Most Popular</div>}
      <div className="tier-name">{tier.name}</div>
      <div className="tier-price">{tier.price}</div>
      <ul className="tier-limits">
        {tier.limits.map((l) => (
          <li key={l}>{l}</li>
        ))}
      </ul>
      <button
        className="tier-btn"
        onClick={() => setDestroyed(true)}
        onMouseEnter={() => setDestroyed(true)}
      >
        Get started
      </button>
      <div className="tier-x">✕</div>
    </div>
  );
}

function NoiseOverlay() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    let frame = 0;
    let raf: number;
    const draw = () => {
      frame++;
      if (frame % 3 !== 0) { raf = requestAnimationFrame(draw); return; }
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
      const img = ctx.createImageData(canvas.width, canvas.height);
      for (let i = 0; i < img.data.length; i += 4) {
        const v = Math.random() > 0.985 ? Math.floor(Math.random() * 180) : 0;
        img.data[i] = v;
        img.data[i + 1] = v;
        img.data[i + 2] = v;
        img.data[i + 3] = v ? 60 : 0;
      }
      ctx.putImageData(img, 0, 0);
      raf = requestAnimationFrame(draw);
    };
    draw();
    return () => cancelAnimationFrame(raf);
  }, []);
  return (
    <canvas
      ref={canvasRef}
      style={{
        position: "fixed",
        inset: 0,
        pointerEvents: "none",
        zIndex: 0,
        mixBlendMode: "screen",
      }}
    />
  );
}

export default function PricingPage() {
  const [tiersVisible, setTiersVisible] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setTiersVisible(true), 600);
    return () => clearTimeout(t);
  }, []);

  return (
    <>
      <style>{`
        /* ── Glitch ── */
        .glitch-wrap {
          position: relative;
          display: inline-block;
          font-family: "Instrument Serif", "Times New Roman", serif;
          font-size: clamp(120px, 24vw, 300px);
          line-height: 0.85;
          letter-spacing: -0.04em;
          color: var(--text-on-dark);
          user-select: none;
        }
        .glitch-main, .glitch-r, .glitch-b {
          display: block;
        }
        .glitch-r, .glitch-b {
          position: absolute;
          inset: 0;
        }
        .glitch-r {
          color: oklch(65% 0.25 15);
          animation: glitch-r 3.5s infinite;
          clip-path: polygon(0 20%, 100% 20%, 100% 40%, 0 40%);
          mix-blend-mode: screen;
        }
        .glitch-b {
          color: oklch(65% 0.25 260);
          animation: glitch-b 3.5s infinite;
          clip-path: polygon(0 55%, 100% 55%, 100% 75%, 0 75%);
          mix-blend-mode: screen;
        }
        @keyframes glitch-r {
          0%, 92%, 100% { transform: translate(0, 0); opacity: 0; }
          93%  { transform: translate(-6px, 2px); opacity: 1; }
          94%  { transform: translate(4px, -1px); opacity: 1; }
          95%  { transform: translate(-3px, 3px); opacity: 1; }
          96%  { transform: translate(0, 0); opacity: 0; }
          97%  { transform: translate(8px, -4px); opacity: 1; }
          98%  { transform: translate(-2px, 1px); opacity: 0; }
        }
        @keyframes glitch-b {
          0%, 92%, 100% { transform: translate(0, 0); opacity: 0; }
          93%  { transform: translate(6px, -2px); opacity: 1; }
          94%  { transform: translate(-4px, 1px); opacity: 1; }
          95%  { transform: translate(3px, -3px); opacity: 1; }
          96%  { transform: translate(0, 0); opacity: 0; }
          97%  { transform: translate(-8px, 4px); opacity: 1; }
          98%  { transform: translate(2px, -1px); opacity: 0; }
        }

        /* ── Ticker ── */
        .ticker-wrap {
          width: 100%;
          overflow: hidden;
          border-top: 1px solid var(--line-dark);
          border-bottom: 1px solid var(--line-dark);
          padding: 12px 0;
          margin: 48px 0;
          background: var(--bg-dark-2);
        }
        .ticker-track {
          display: flex;
          gap: 0;
          white-space: nowrap;
          animation: ticker 22s linear infinite;
        }
        .ticker-item {
          display: inline-flex;
          align-items: center;
          gap: 12px;
          padding: 0 32px;
          font-family: "JetBrains Mono", monospace;
          font-size: 13px;
          letter-spacing: 0.06em;
          color: var(--text-on-dark-soft);
          text-transform: uppercase;
        }
        .ticker-dot {
          width: 5px; height: 5px;
          background: var(--accent);
          border-radius: 50%;
          flex-shrink: 0;
        }
        @keyframes ticker {
          0% { transform: translateX(0); }
          100% { transform: translateX(-50%); }
        }

        /* ── Terminal ── */
        .terminal-box {
          border: 1px solid var(--line-dark-2);
          border-radius: 6px;
          overflow: hidden;
          width: 100%;
          max-width: 520px;
          background: var(--bg-dark-3);
        }
        .terminal-bar {
          background: var(--bg-dark-2);
          padding: 9px 14px;
          display: flex;
          align-items: center;
          gap: 7px;
          border-bottom: 1px solid var(--line-dark);
        }
        .t-dot { width: 11px; height: 11px; border-radius: 50%; }
        .t-title {
          font-family: "JetBrains Mono", monospace;
          font-size: 11px;
          color: var(--text-on-dark-soft);
          margin-left: auto;
          letter-spacing: 0.06em;
        }
        .terminal-body {
          padding: 18px 20px;
          min-height: 220px;
          font-family: "JetBrains Mono", monospace;
          font-size: 13px;
          line-height: 2;
        }
        .t-line { display: block; }
        .t-cursor { color: var(--accent); }

        /* ── Fake tiers ── */
        .tiers-row {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 16px;
          max-width: 820px;
          width: 100%;
          margin: 0 auto;
          position: relative;
        }
        @media (max-width: 700px) { .tiers-row { grid-template-columns: 1fr; } }
        .fake-tier {
          position: relative;
          border: 1px solid var(--line-dark-2);
          border-radius: 6px;
          padding: 24px 20px 20px;
          background: var(--bg-dark-2);
          transition: opacity 0.25s ease, filter 0.25s ease, transform 0.25s ease;
          cursor: default;
          overflow: hidden;
        }
        .fake-tier:hover, .tier-destroyed {
          opacity: 0.18;
          filter: blur(2px) saturate(0);
          transform: scale(0.97) rotate(-1deg);
        }
        .tier-badge {
          position: absolute;
          top: -1px; right: 16px;
          background: var(--accent);
          color: #fff;
          font-family: "JetBrains Mono", monospace;
          font-size: 9px;
          letter-spacing: 0.12em;
          text-transform: uppercase;
          padding: 3px 10px;
          border-radius: 0 0 4px 4px;
        }
        .tier-name {
          font-family: "JetBrains Mono", monospace;
          font-size: 10px;
          letter-spacing: 0.14em;
          text-transform: uppercase;
          color: var(--text-on-dark-soft);
          margin-bottom: 8px;
        }
        .tier-price {
          font-family: "Inter Tight", sans-serif;
          font-size: 28px;
          font-weight: 600;
          color: var(--text-on-dark);
          margin-bottom: 16px;
          letter-spacing: -0.03em;
        }
        .tier-limits {
          list-style: none;
          margin: 0 0 20px;
          padding: 0;
          font-size: 12px;
          color: var(--text-on-dark-soft);
          line-height: 2.2;
          font-family: "Inter Tight", sans-serif;
        }
        .tier-btn {
          width: 100%;
          padding: 9px;
          border: 1px solid var(--line-dark-2);
          border-radius: 4px;
          font-family: "JetBrains Mono", monospace;
          font-size: 12px;
          color: var(--text-on-dark-soft);
          background: var(--bg-dark-3);
          cursor: pointer;
          letter-spacing: 0.06em;
        }
        .tier-x {
          position: absolute;
          inset: 0;
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 72px;
          color: var(--danger);
          opacity: 0;
          transition: opacity 0.2s ease;
          pointer-events: none;
          font-family: "Inter Tight", sans-serif;
          font-weight: 100;
        }
        .fake-tier:hover .tier-x, .tier-destroyed .tier-x { opacity: 0.6; }

        /* ── Tiers label ── */
        .tiers-label {
          font-family: "JetBrains Mono", monospace;
          font-size: 11px;
          letter-spacing: 0.1em;
          text-transform: uppercase;
          color: var(--text-on-dark-soft);
          text-align: center;
          margin-bottom: 20px;
          opacity: 0.5;
        }

        /* ── Scanlines ── */
        .scanlines {
          position: fixed;
          inset: 0;
          pointer-events: none;
          z-index: 1;
          background: repeating-linear-gradient(
            to bottom,
            transparent,
            transparent 2px,
            rgba(0,0,0,0.04) 2px,
            rgba(0,0,0,0.04) 4px
          );
        }

        /* ── Sub-copy ── */
        .anti-copy {
          font-family: "Inter Tight", sans-serif;
          font-size: clamp(16px, 2.2vw, 21px);
          color: var(--text-on-dark-soft);
          text-align: center;
          line-height: 1.55;
          max-width: 500px;
          margin: 28px auto 0;
        }
        .anti-copy strong { color: var(--text-on-dark); font-weight: 500; }

        /* ── Infra note ── */
        .infra-note {
          font-family: "JetBrains Mono", monospace;
          font-size: 11px;
          color: var(--text-on-dark-soft);
          opacity: 0.4;
          letter-spacing: 0.04em;
          margin-top: 20px;
        }

        /* ── Drift bg ── */
        @keyframes drift {
          0%, 100% { transform: translate(0,0) rotate(0deg); opacity: 0.04; }
          33% { transform: translate(18px,-12px) rotate(4deg); opacity: 0.07; }
          66% { transform: translate(-10px,20px) rotate(-3deg); opacity: 0.05; }
        }
        .bg-mark {
          position: absolute;
          border: 1px solid var(--line-dark);
          animation: drift 18s ease-in-out infinite;
          pointer-events: none;
        }

        /* ── Slash tag ── */
        .slash-tag {
          display: inline-flex;
          align-items: center;
          gap: 8px;
          border: 1px solid var(--line-dark-2);
          border-radius: 2px;
          font-size: 11px;
          letter-spacing: 0.12em;
          text-transform: uppercase;
          color: var(--text-on-dark-soft);
          font-family: "JetBrains Mono", monospace;
          padding: 4px 12px;
          margin-bottom: 36px;
        }
        .slash-dot { width: 6px; height: 6px; background: var(--good); border-radius: 50%; }

        /* ── CTAs ── */
        .cta-row {
          display: flex;
          gap: 12px;
          justify-content: center;
          flex-wrap: wrap;
          margin-top: 44px;
        }
      `}</style>

      <div className="scanlines" />
      <NoiseOverlay />
      <Nav />

      <main
        style={{
          minHeight: "100vh",
          background: "var(--bg-dark)",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          padding: "100px 24px 140px",
          position: "relative",
          overflow: "hidden",
        }}
      >
        {/* bg marks */}
        <div className="bg-mark" style={{ width: 600, height: 600, top: "5%", right: "-8%", animationDuration: "20s" }} />
        <div className="bg-mark" style={{ width: 300, height: 300, bottom: "10%", left: "-4%", animationDuration: "26s", animationDirection: "reverse" }} />

        {/* tag */}
        <div className="slash-tag">
          <span className="slash-dot" />
          Open source · MIT License · No catch
        </div>

        {/* THE HERO */}
        <GlitchText text="free" />

        <p className="anti-copy">
          Free like your free will.{" "}
          <strong>Fork it. Break it. Deploy it. Own it.</strong>
        </p>
        <p className="anti-copy" style={{ fontSize: "14px", opacity: 0.6, marginTop: "12px" }}>
          No seat licenses. No usage meters. No enterprise tier locked behind a sales call.
          The agent runs on your machine, your cluster, your terms.
        </p>

        {/* Scrolling ticker */}
        <div style={{ width: "100vw", position: "relative", left: "50%", transform: "translateX(-50%)" }}>
          <Ticker />
        </div>

        {/* Two-col: terminal + feature grid */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: "32px",
            maxWidth: "900px",
            width: "100%",
            alignItems: "start",
          }}
        >
          <Terminal />

          {/* Feature grid */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "1px",
              background: "var(--line-dark)",
              border: "1px solid var(--line-dark)",
              borderRadius: "6px",
              overflow: "hidden",
            }}
          >
            {[
              { label: "Environments", value: "Unlimited" },
              { label: "Deploys", value: "Unlimited" },
              { label: "Clouds", value: "AWS · Azure · GCP" },
              { label: "Agents", value: "All 6 included" },
              { label: "CI minutes", value: "Your runner" },
              { label: "License", value: "MIT" },
              { label: "Support", value: "GitHub Issues" },
              { label: "Monthly cost", value: "$0.00" },
            ].map((f) => (
              <div
                key={f.label}
                style={{
                  background: "var(--bg-dark-2)",
                  padding: "18px 18px",
                }}
              >
                <div
                  style={{
                    fontSize: "10px",
                    color: "var(--text-on-dark-soft)",
                    fontFamily: "JetBrains Mono, monospace",
                    letterSpacing: "0.1em",
                    textTransform: "uppercase",
                    marginBottom: "6px",
                  }}
                >
                  {f.label}
                </div>
                <div
                  style={{
                    fontSize: "14px",
                    color: f.value === "$0.00" ? "var(--good)" : "var(--text-on-dark)",
                    fontWeight: 500,
                    fontFamily: "Inter Tight, sans-serif",
                  }}
                >
                  {f.value}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Fake SaaS tiers — just to destroy them */}
        <div style={{ width: "100%", maxWidth: "900px", marginTop: "72px" }}>
          <p className="tiers-label">
            — what we could have charged you —
          </p>
          {tiersVisible && (
            <div className="tiers-row">
              {FAKE_TIERS.map((tier, i) => (
                <FakeTierCard key={tier.name} tier={tier} index={i} />
              ))}
            </div>
          )}
          <p
            style={{
              textAlign: "center",
              marginTop: "20px",
              fontFamily: "JetBrains Mono, monospace",
              fontSize: "11px",
              color: "var(--danger-soft)",
              letterSpacing: "0.08em",
              opacity: 0.6,
            }}
          >
            ↑ hover to delete
          </p>
        </div>

        {/* CTAs */}
        <div className="cta-row">
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
        <p className="infra-note">
          * You still pay for your own cloud infrastructure. We don&apos;t touch that.
        </p>
      </main>
    </>
  );
}
