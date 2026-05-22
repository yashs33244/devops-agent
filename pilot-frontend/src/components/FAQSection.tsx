"use client";

import { useState } from "react";

const faqs = [
  {
    q: "Is Pilot just a wrapper around an LLM?",
    a: "No. Pilot is a long‑running agent with persistent memory of your infra, a typed tool layer over cloud APIs, deterministic planners for IaC diffs, and a model‑agnostic reasoning core. The LLM is one of several substrates — swapped per task. Cost, latency and determinism win.",
  },
  {
    q: "What clouds and stacks does it support?",
    a: "AWS, GCP and Azure on day one. Kubernetes (EKS / GKE / AKS / vanilla), Nomad, Fly.io, bare‑metal via SSH. IaC: Terraform, Pulumi, CDK, Helm. CI: GitHub Actions, GitLab, CircleCI, Buildkite. Observability: Datadog, Grafana, Honeycomb, OpenTelemetry. If it has an API or a shell, Pilot can drive it.",
  },
  {
    q: "Can Pilot really push to production unattended?",
    a: (
      <>
        Yes — within the guardrails you set in policy. By default, anything inside the staging blast
        radius is fully autonomous. Production deploys require either passing canary metrics or a
        human approval, depending on your <span className="mono">policy.rego</span>. Most teams ramp
        Pilot&apos;s autonomy week by week.
      </>
    ),
  },
  {
    q: "What happens when it gets something wrong?",
    a: "Pilot only takes actions it can reverse. Every change is preceded by a dry‑run, the diff is recorded, and the inverse action is queued before execution. When SLOs degrade post‑deploy, Pilot rolls itself back automatically and pages the on‑call with a complete incident packet (root cause, blast radius, mitigation).",
  },
  {
    q: "Do you train on our code or our infra metadata?",
    a: "Never. Your code, your secrets, and your infra topology never leave your boundary in the self‑hosted plan, and are not used to train any shared model in the managed plan. Logs and traces are encrypted at rest with your KMS keys.",
  },
  {
    q: "How is this different from Copilot / Cursor / Devin?",
    a: (
      <>
        Those agents help engineers write code. Pilot replaces the engineer that <em>operates</em>{" "}
        code — pipelines, infra, observability, incident response, cost. It&apos;s not a sidekick in
        your editor; it&apos;s a teammate in your on‑call rotation.
      </>
    ),
  },
  {
    q: "How much does it cost?",
    a: "$0 to start: 1 environment, unlimited deploys. Team plan from $40 / engineer / month. Self‑hosted enterprise pricing is flat by fleet size. Most teams report Pilot pays for itself in cloud savings inside 60 days.",
  },
];

export function FAQSection() {
  const [openIndex, setOpenIndex] = useState<number | null>(null);

  const toggle = (i: number) => {
    setOpenIndex(openIndex === i ? null : i);
  };

  return (
    <section className="faq-section bg-dark">
      <div className="container">
        <div className="faq-grid">
          <div className="faq-head">
            <span className="eyebrow on-dark">// faq</span>
            <h2 className="h-serif">
              The questions <em>everyone</em> asks first.
            </h2>
            <p>Hover any question to peek the answer. Click to keep it pinned open.</p>
          </div>

          <div className="faq-list">
            {faqs.map((item, i) => (
              <div
                key={i}
                className={`faq-item${openIndex === i ? " is-open" : ""}`}
                onClick={() => toggle(i)}
              >
                <div className="q">
                  <h4>{item.q}</h4>
                  <span className="plus" aria-hidden="true"></span>
                </div>
                <div className="a">
                  <div>{item.a}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
