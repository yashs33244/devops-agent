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

const chartManifests = [
  { name: "Deployment", desc: "Rolling update strategy, pod anti-affinity, resource limits" },
  { name: "Service", desc: "ClusterIP (default) or LoadBalancer via values.yaml" },
  { name: "Ingress", desc: "nginx / ALB annotations, TLS via cert-manager" },
  { name: "HPA", desc: "CPU + memory autoscaler, configurable min/max replicas" },
  { name: "KEDA HTTPScaledObject", desc: "Scale-to-zero (opt-in: keda.enabled: true)" },
  { name: "ExternalSecret", desc: "ESO manifest pulling from the cloud secrets manager" },
  { name: "ServiceMonitor", desc: "Prometheus scrape config for /metrics endpoint" },
  { name: "PrometheusRule", desc: "4 alert rules: error rate, latency p99, restart rate, CPU" },
  { name: "Grafana Dashboard ConfigMap", desc: "Pre-built dashboard JSON mounted via sidecar" },
  { name: "PodDisruptionBudget", desc: "Minimum available: 1 for prod workloads" },
];

const securityDefaults = [
  { key: "runAsNonRoot", value: "true" },
  { key: "readOnlyRootFilesystem", value: "true" },
  { key: "capabilities.drop", value: "[ALL]" },
  { key: "allowPrivilegeEscalation", value: "false" },
];

export default function HelmPage() {
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
        <span style={{ color: "var(--text-on-dark)" }}>Helm</span>
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
        Kubernetes
      </span>

      <h1
        className="h-serif"
        style={{
          fontSize: "clamp(32px, 4vw, 48px)",
          margin: "0 0 16px",
          color: "var(--text-on-dark)",
        }}
      >
        helm_gen.py
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
        Generates a production-grade Helm chart with security contexts, resource limits,
        liveness/readiness probes, KEDA scale-to-zero, ESO secrets, and Prometheus alerting
        built in from day one.
      </p>

      <Section title="What it does">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Copies <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>templates/helm/chart/</code> to{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>&lt;service&gt;/helm/</code>,
          substitutes <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{SERVICE_NAME}}"}</code>,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{PORT}}"}</code>, and{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{"{{APP_VERSION}}"}</code>,
          then injects cloud-appropriate ServiceAccount annotations.
        </p>
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: 0, maxWidth: "66ch" }}>
          After generation it runs{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>helm lint --strict</code> and,
          if the helm-unittest plugin is installed,{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>helm unittest</code>.
        </p>
      </Section>

      <Section title="CLI Usage">
        <CodeBlock lang="bash">{`python3 tools/helm_gen.py \\
  --service payment-api \\
  --cloud aws \\
  --port 8000`}</CodeBlock>
      </Section>

      <Section title="Generated Chart Contents">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {chartManifests.map((item, i) => (
            <div
              key={item.name}
              style={{
                display: "grid",
                gridTemplateColumns: "220px 1fr",
                padding: "10px 16px",
                borderBottom: i < chartManifests.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: i % 2 === 0 ? "var(--bg-dark-2)" : "var(--bg-dark-3)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <span style={{ fontSize: "13px", fontWeight: 500, color: "var(--text-on-dark)" }}>{item.name}</span>
              <span style={{ fontSize: "13px", color: "var(--text-on-dark-soft)" }}>{item.desc}</span>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Security Defaults (values.yaml)">
        <div
          style={{
            border: "1px solid var(--line-dark)",
            borderRadius: "4px",
            overflow: "hidden",
          }}
        >
          {securityDefaults.map((item, i) => (
            <div
              key={item.key}
              style={{
                display: "grid",
                gridTemplateColumns: "260px 1fr",
                padding: "10px 16px",
                borderBottom: i < securityDefaults.length - 1 ? "1px solid var(--line-dark)" : "none",
                background: "var(--bg-dark-2)",
                gap: "16px",
                alignItems: "center",
              }}
            >
              <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{item.key}</code>
              <code className="mono" style={{ fontSize: "12px", color: "var(--text-on-dark)" }}>{item.value}</code>
            </div>
          ))}
        </div>
      </Section>

      <Section title="KEDA Scale-to-Zero Integration">
        <p style={{ fontSize: "14px", color: "var(--text-on-dark-soft)", lineHeight: 1.65, margin: "0 0 14px", maxWidth: "66ch" }}>
          Set <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>keda.enabled: true</code> in{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>values.yaml</code> to activate
          the HTTPScaledObject for the car-painter scale-to-zero pattern.
        </p>
        <CodeBlock lang="yaml">{`# values.yaml
keda:
  enabled: true
  minReplicaCount: 0
  maxReplicaCount: 10
  scaledownPeriod: 300          # 5 min idle → scale to zero
  targetPendingRequests: 100`}</CodeBlock>
      </Section>

      <div
        style={{
          padding: "14px 18px",
          background: "var(--bg-dark-2)",
          border: "1px solid var(--line-dark)",
          borderLeft: "3px solid var(--accent)",
          borderRadius: "4px",
          marginBottom: "48px",
        }}
      >
        <div
          className="mono"
          style={{
            fontSize: "11px",
            letterSpacing: "0.08em",
            textTransform: "uppercase",
            color: "var(--text-on-dark-soft)",
            marginBottom: "8px",
          }}
        >
          ServiceAccount Annotations
        </div>
        <p style={{ margin: 0, fontSize: "13px", color: "var(--text-on-dark-soft)", lineHeight: 1.6 }}>
          For AWS, the ServiceAccount is annotated with{" "}
          <code className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>eks.amazonaws.com/role-arn</code>.
          For Azure and GCP, Workload Identity annotations are injected instead. No static cloud credentials
          are ever written into the chart.
        </p>
      </div>

      <div style={{ borderTop: "1px solid var(--line-dark)", paddingTop: "24px" }}>
        <Link
          href="/docs/tools/cicd"
          style={{
            fontSize: "14px",
            color: "var(--accent)",
            fontFamily: "var(--font-jetbrains-mono, 'JetBrains Mono', monospace)",
          }}
        >
          Next step: CI/CD →
        </Link>
      </div>
    </div>
  );
}
