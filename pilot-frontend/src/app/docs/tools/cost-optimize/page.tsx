import Link from "next/link";
import type { ReactNode } from "react";

function CodeBlock({ lang, children }: { lang: string; children: string }) {
  return (
    <div
      style={{
        border: "1px solid var(--line-dark)",
        borderRadius: "4px",
        overflow: "hidden",
        margin: "12px 0 20px",
      }}
    >
      <div
        style={{
          padding: "7px 14px",
          background: "var(--bg-dark-3)",
          borderBottom: "1px solid var(--line-dark)",
        }}
      >
        <span
          className="mono"
          style={{
            fontSize: "11px",
            color: "var(--text-on-dark-soft)",
            letterSpacing: "0.1em",
            textTransform: "uppercase",
          }}
        >
          {lang}
        </span>
      </div>
      <pre
        style={{
          margin: 0,
          padding: "16px 18px",
          background: "var(--bg-dark-3)",
          fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          fontSize: "12.5px",
          lineHeight: "1.65",
          color: "var(--text-on-dark)",
          overflowX: "auto",
        }}
      >
        <code>{children}</code>
      </pre>
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div style={{ marginBottom: "36px" }}>
      <div
        className="mono"
        style={{
          fontSize: "11px",
          letterSpacing: "0.1em",
          textTransform: "uppercase",
          color: "var(--text-on-dark-soft)",
          marginBottom: "14px",
          paddingBottom: "8px",
          borderBottom: "1px solid var(--line-dark)",
        }}
      >
        {title}
      </div>
      {children}
    </div>
  );
}

const platforms = [
  { flag: "eks", mechanism: "KEDA + HTTP Add-on", notes: "Install KEDA via Helm chart" },
  { flag: "aks", mechanism: "Built-in KEDA add-on", notes: "Enable in AKS portal / Bicep" },
  { flag: "gke", mechanism: "KEDA or Cloud Run", notes: "Cloud Run is natively serverless — prefer it" },
];

const kedaSettings = [
  { key: "minReplicaCount", value: "0", desc: "Scale to zero when idle" },
  { key: "scaledownPeriod", value: "300", desc: "5 minutes idle before scaling to zero" },
  { key: "targetPendingRequests", value: "100", desc: "Scale up trigger threshold" },
  { key: "maxReplicaCount", value: "10", desc: "Override for prod (default: 10)" },
];

const doNotUse = [
  "Stateful services (databases, message stores)",
  "Message-queue consumers — use KEDA queue scalers instead",
  "Services with less than 60-second cold-start tolerance",
  "Prod services where latency SLOs require pre-warmed replicas",
];

const nativeAlternatives = [
  { platform: "Cloud Run (GCP)", desc: "Native scale-to-zero, no KEDA needed" },
  { platform: "Azure Container Apps", desc: "Native scale-to-zero, no KEDA needed" },
  { platform: "AWS Fargate (ECS)", desc: "Scale to 0 via ECS Service with min=0" },
];

export default function CostOptimizePage() {
  return (
    <div>
      {/* Breadcrumb */}
      <div
        style={{
          marginBottom: "32px",
          display: "flex",
          alignItems: "center",
          gap: "8px",
          fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          fontSize: "13px",
          color: "var(--text-on-dark-soft)",
        }}
      >
        <Link href="/docs" style={{ color: "var(--text-on-dark-soft)" }}>Docs</Link>
        <span>/</span>
        <Link href="/docs/tools" style={{ color: "var(--text-on-dark-soft)" }}>Tools</Link>
        <span>/</span>
        <span style={{ color: "var(--text-on-dark)" }}>Cost Optimize</span>
      </div>

      <span
        className="mono"
        style={{
          fontSize: "10px",
          padding: "3px 9px",
          border: "1px solid var(--line-dark-2)",
          color: "var(--text-on-dark-soft)",
          background: "var(--bg-dark-3)",
          borderRadius: "2px",
          letterSpacing: "0.08em",
          textTransform: "uppercase",
          display: "inline-block",
          marginBottom: "16px",
        }}
      >
        Cost
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        cost_optimize.py
      </h1>

      <p
        style={{
          fontSize: "15px",
          color: "var(--text-on-dark-soft)",
          margin: "0 0 40px",
          lineHeight: 1.65,
          maxWidth: "62ch",
          borderBottom: "1px solid var(--line-dark)",
          paddingBottom: "32px",
        }}
      >
        Applies the car-painter KEDA scale-to-zero pattern to your Kubernetes service.
        Generates{" "}
        <code className="mono" style={{ fontSize: "13px" }}>keda.tf</code>,{" "}
        <code className="mono" style={{ fontSize: "13px" }}>http-scaler.yaml</code>, and patches
        the deployment to set{" "}
        <code className="mono" style={{ fontSize: "13px" }}>minReplicas: 0</code>.
        Typical saving: <strong style={{ color: "var(--good)" }}>60–90% compute cost</strong> for
        bursty or low-traffic services.
      </p>

      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Generates three artefacts that wire KEDA&apos;s HTTP add-on into your service.
          Pods scale to zero on 5 minutes of idle traffic and spin back up within 60 seconds
          on the first incoming request.
        </p>
      </Section>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/cost_optimize.py \\
  --terraform-dir ./my-app/terraform \\
  --platform eks \\
  --service payment-api`}</CodeBlock>
      </Section>

      <Section title="Platform Options">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "100px 1fr 1fr",
              padding: "8px 16px",
              background: "var(--bg-dark-3)",
              borderBottom: "1px solid var(--line-dark)",
              gap: "16px",
            }}
          >
            {["--platform", "Mechanism", "Notes"].map((h) => (
              <span key={h} className="mono" style={{ fontSize: "10px", letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--text-on-dark-soft)" }}>
                {h}
              </span>
            ))}
          </div>
          {platforms.map((row, i) => (
            <div
              key={row.flag}
              style={{
                display: "grid",
                gridTemplateColumns: "100px 1fr 1fr",
                padding: "10px 16px",
                borderBottom: i < platforms.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.flag}</code>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark)" }}>{row.mechanism}</span>
              <span style={{ fontSize: "12px", color: "var(--text-on-dark-soft)" }}>{row.notes}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Key KEDA Settings">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {kedaSettings.map((row, i) => (
            <div
              key={row.key}
              style={{
                display: "grid",
                gridTemplateColumns: "220px 80px 1fr",
                padding: "10px 16px",
                borderBottom: i < kedaSettings.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{row.key}</code>
              <code className="mono" style={{ fontSize: "12px", color: "var(--text-on-dark)", fontWeight: 600 }}>{row.value}</code>
              <span style={{ fontSize: "12px", color: "var(--text-on-dark-soft)" }}>{row.desc}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="When NOT to Use">
        <ul
          style={{
            margin: 0,
            padding: "0 0 0 18px",
            fontSize: "13px",
            color: "var(--text-on-dark-soft)",
            lineHeight: 2,
          }}
        >
          {doNotUse.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </Section>

      <Section title="Node-Level Savings — Karpenter">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 16px", maxWidth: "66ch" }}>
          KEDA removes idle <em>pods</em>. Karpenter removes idle <em>nodes</em>. Use both together for
          maximum savings: KEDA scales your pods to zero, then Karpenter consolidates and terminates
          the now-empty nodes automatically.
        </p>
        <div style={{ border: "1px solid var(--line-dark)", borderRadius: "4px", overflow: "hidden", marginBottom: "20px" }}>
          {[
            { label: "What it does", value: "Right-sizes and consolidates EC2 nodes in real time. Replaces Cluster Autoscaler on EKS." },
            { label: "Spot support", value: "Automatically provisions Spot instances and handles interruptions — 60–90% node cost reduction." },
            { label: "Consolidation", value: "NodePool consolidateAfter: 30s — evicts pods and terminates underutilised nodes within 30 seconds." },
            { label: "Works with KEDA", value: "When KEDA scales pods to 0, Karpenter sees empty nodes and terminates them. Full cost elimination when idle." },
          ].map((row, i) => (
            <div key={row.label} style={{ display: "grid", gridTemplateColumns: "180px 1fr", padding: "10px 16px", borderBottom: i < 3 ? "1px solid var(--line-dark)" : "none", background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)", gap: "16px", alignItems: "start" }}>
              <span className="mono" style={{ fontSize: "11px", color: "var(--text-on-dark-soft)", textTransform: "uppercase" as const, letterSpacing: "0.06em", paddingTop: "2px" }}>{row.label}</span>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark)", lineHeight: 1.55 }}>{row.value}</span>
            </div>
          ))}
        </div>
        <p style={{ fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6, margin: "0 0 12px", maxWidth: "66ch" }}>
          Minimal NodePool that enables Spot + consolidation:
        </p>
        <div style={{ border: "1px solid var(--line-dark)", borderRadius: "4px", overflow: "hidden", margin: "0 0 8px" }}>
          <div style={{ padding: "7px 14px", background: "var(--bg-dark-3)", borderBottom: "1px solid var(--line-dark)" }}>
            <span className="mono" style={{ fontSize: "11px", color: "var(--text-on-dark-soft)", letterSpacing: "0.1em", textTransform: "uppercase" as const }}>yaml — karpenter NodePool</span>
          </div>
          <pre style={{ margin: 0, padding: "16px 18px", background: "var(--bg-dark-3)", fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)", fontSize: "12.5px", lineHeight: "1.65", color: "var(--text-on-dark)", overflowX: "auto" as const }}>
            <code>{`apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["spot", "on-demand"]
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: default
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
    consolidateAfter: 30s
  limits:
    cpu: "100"
    memory: 400Gi`}</code>
          </pre>
        </div>
        <p style={{ fontSize: "12px", color: "var(--text-on-dark-soft)", opacity: 0.6, fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)", margin: "8px 0 0" }}>
          Install: <code>helm install karpenter oci://public.ecr.aws/karpenter/karpenter --version 1.0.0 -n karpenter</code>
        </p>
      </Section>

      <Section title="Native Serverless Alternatives">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Always prefer managed serverless over K8s + KEDA when the workload is stateless HTTP —
          lower operational overhead and lower cost.
        </p>
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {nativeAlternatives.map((row, i) => (
            <div
              key={row.platform}
              style={{
                display: "grid",
                gridTemplateColumns: "220px 1fr",
                padding: "10px 16px",
                borderBottom: i < nativeAlternatives.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <span style={{ fontSize: "13px", fontWeight: 500, color: "var(--text-on-dark)" }}>{row.platform}</span>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{row.desc}</span>
            </div>
          ))}
        </div>
      </Section>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools/test-runner"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          Next step: Test Runner →
        </Link>
      </div>
    </div>
  );
}
