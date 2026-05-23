# devops-agent

AI-powered DevOps automation platform — monorepo.

Powered by Claude Code. Covers the full lifecycle: Dockerize → Terraform → Helm → CI/CD → Secrets → Local testing → Cost optimization → SRE monitoring.

---

## Monorepo Layout

```
devops-agent/
├── tools/           ← Orchestration CLI tools (Python) — the agent brain
├── templates/       ← Reusable Terraform, Dockerfile, Helm, CI/CD templates
├── services/        ← First-party sample services (python-api, go-api, nextjs-demo)
├── agents/          ← Integrated sub-agents (5 specialist agents + sre-guard)
│   ├── holmesgpt/   AI incident investigation
│   ├── kagent/      Kubernetes AI fleet agent
│   ├── nightshift/  Cost optimization scheduler
│   ├── opensre/     SRE runbook automation
│   ├── plural/      GitOps multi-cloud deployment
│   └── sre-guard/   Persistent monitoring daemon
└── workspace/       ← Gitignored — cloned repos land here
```

---

## Quick Start

```bash
pip install -r requirements.txt

# Full pipeline
python3 tools/workflow.py \
  --repo https://github.com/your-org/your-app \
  --service my-service --cloud aws --env dev --with-keda

# Or just open this folder in Claude Code and describe what you want:
# "Deploy my Node.js API from github.com/acme/api to AWS with a Postgres DB"
```

---

## Claude Code Slash Commands

Open this folder in Claude Code and type `/` to access all commands:

### Infrastructure Commands

| Command | What it does |
|---------|-------------|
| `/deploy` | **Full pipeline** — asks for service, repo, cloud, region, env, use case, backing services, scale-to-zero preference → runs the entire 10-step workflow |
| `/dockerize` | Detect language → generate multi-stage distroless Dockerfile → hadolint lint → Trivy scan. Guards against overwriting existing Dockerfiles. |
| `/terraform` | Generate Terraform modules (EKS/AKS/GKE + registry + DB) → `terraform fmt` → `terraform validate`. Asks for cloud, region, env, use case. |
| `/helm` | Generate production Helm chart with security contexts + resource limits + health probes → `helm lint` → dry-run template. |
| `/secrets` | Scan repo for secrets/env vars → walk through IRSA vs ESO vs Sealed Secrets decision tree → scaffold the right manifests. |
| `/test` | Run the full TDD suite: Dockerfile (hadolint + Trivy) → Terraform (tflint + checkov + validate) → Helm (lint + unittest) → GitHub Actions (act) → integration (kind). Auto-fixes failures. |
| `/local-test` | Start matching cloud emulator (LocalStack/Azurite/GCP) → run `local_test.py` → Terraform plan against emulator. No real cloud credentials needed. |
| `/optimize-cost` | Check eligibility → apply car-painter KEDA scale-to-zero → explain expected 60–90% cost savings. |
| `/audit` | Audit existing Dockerfile/Terraform/Helm against the security checklist → findings as CRITICAL / HIGH / MEDIUM / LOW. |
| `/status` | Monorepo snapshot: all agents, services, kind cluster health, Prometheus/Grafana, running emulators, recent git log. |

### Agent Commands

| Command | Agent | What it does |
|---------|-------|-------------|
| `/holmesgpt` | HolmesGPT | AI-powered Kubernetes incident investigation. Ask about pod crashes, high latency, OOMKilled, failing deployments. Gets root cause + evidence + recommended actions. |
| `/kagent` | kagent | Manage Kubernetes AI agents in your cluster. Create, list, run tasks, view logs. Generates CRD YAML specs. |
| `/nightshift` | Nightshift | Add/list/remove cost-saving schedules (scale to 0 at night, back up in morning). Estimates monthly savings. |
| `/opensre` | OpenSRE | Run, list, create, and test SRE runbooks. Automated response to alerts with approval gates. |
| `/plural` | Plural | GitOps deployments across multiple clusters. Deploy, promote (dev→staging→prod), rollback, view fleet status. |
| `/sre-guard` | SRE Guard | Control the persistent monitoring daemon. Start/stop, add services to watch, diagnose incidents, silence alerts, view alert history. |

---

## Agents

### HolmesGPT (`agents/holmesgpt/`)

**Language:** Python | **Entrypoint:** `holmes` CLI

AI-powered SRE investigation tool. Connects to your Kubernetes cluster and Alertmanager, then uses Claude/GPT to investigate incidents in natural language.

```bash
pip install -e agents/holmesgpt/
holmes ask "why is the payment-api pod crashing in namespace prod?"
holmes investigate alert --alertmanager-url http://localhost:9093
holmes find --selector app=payment-api --namespace prod
```

**Capabilities:**
- Correlates pod logs, events, resource metrics, and Alertmanager alerts
- Gives structured root cause analysis with evidence
- Supports AWS/GCP/Azure managed K8s + self-hosted
- Integrates with PagerDuty, OpsGenie, Prometheus

**Use via Claude:** `/holmesgpt` → describe the incident → get AI findings

---

### kagent (`agents/kagent/`)

**Language:** Go | **Entrypoint:** kagent CLI + Kubernetes controller

Kubernetes-native framework for building, deploying, and running AI agents as K8s resources. Deploy agents (Claude, GPT, Gemini) with access to tools (kubectl, Helm, Terraform, custom APIs) via Kubernetes CRDs.

```bash
# Install controller into cluster
kubectl apply -f agents/kagent/deploy/

# Create an agent
kubectl apply -f - <<EOF
apiVersion: kagent.dev/v1alpha1
kind: Agent
metadata:
  name: infra-agent
spec:
  model: claude-sonnet-4-6
  tools: [kubectl, helm, prometheus]
  systemPrompt: "You are an infra engineer..."
EOF

# Run a task
kagent run infra-agent "scale the payment-api to 3 replicas"
```

**Capabilities:**
- Declarative agent definitions as Kubernetes CRDs
- Built-in tools: kubectl, Helm, Prometheus, custom HTTP
- Agent-to-Agent (A2A) protocol for multi-agent workflows
- Exposes agents as MCP servers for IDE integration
- UI dashboard for observing agent runs

**Use via Claude:** `/kagent` → choose operation → get YAML spec or CLI commands

---

### Nightshift (`agents/nightshift/`)

**Language:** Go | **Entrypoint:** `nightshift-api` + `nightshift-worker`

Cost optimization scheduler that scales Kubernetes workloads to 0 replicas during off-hours (nights, weekends) and back up before business hours. The `nightshift-worker-claude` variant uses Claude AI to make intelligent scaling decisions based on traffic patterns.

```bash
cd agents/nightshift
go run cmd/nightshift-api/main.go   # REST API on :8080
go run cmd/nightshift-worker/main.go # executes schedules

# Add a schedule via API
curl -X POST http://localhost:8080/schedules \
  -d '{"namespace":"default","name":"payment-api","scaleDown":"0 22 * * 1-5","scaleUp":"0 7 * * 1-5","timezone":"Asia/Kolkata"}'
```

**Capabilities:**
- Cron-based scale-down/scale-up for Deployments and StatefulSets
- Timezone-aware scheduling (global teams)
- AI-driven mode: analyzes traffic patterns and auto-suggests optimal schedules
- Savings dashboard: estimated $/month saved per workload
- Integrates with KEDA for complementary scale-to-zero

**Use via Claude:** `/nightshift` → add schedule or check savings estimate

---

### OpenSRE (`agents/opensre/`)

**Language:** Python + Node.js | **Entrypoint:** `opensre` CLI

SRE runbook automation platform. Define runbooks as YAML (steps: restart pod, scale up, notify Slack, check logs, rollback deploy) and trigger them manually or in response to Alertmanager alerts. Supports approval gates for destructive actions.

```bash
pip install -e agents/opensre/
opensre list                                    # list available runbooks
opensre run high-memory --service payment-api   # run a runbook
opensre create                                  # interactive runbook builder
opensre history --service payment-api           # view past executions
```

**Capabilities:**
- YAML-defined runbooks with conditional steps
- Alertmanager webhook integration (auto-trigger on alert)
- Human approval gates (Slack approval flow)
- Step types: kubectl exec, HTTP call, Slack notify, PagerDuty, script, wait
- Full audit log of all runbook executions

**Use via Claude:** `/opensre` → run runbook or create new one

---

### Plural (`agents/plural/`)

**Language:** Elixir/Phoenix + Python utilities | **Entrypoint:** `plural` CLI + web UI

GitOps multi-cloud deployment platform. Manages applications across multiple Kubernetes clusters using a unified control plane. Supports promotion pipelines (dev → staging → prod) with automated gates.

```bash
# Deploy an application
plural deploy --app payment-api --cluster prod-eks --values values.prod.yaml

# Promote through environments
plural promote --app payment-api --from staging --to prod

# View fleet status
plural status --all-clusters

# Audit S3 buckets (utility)
python3 agents/plural/bin/s3_audit.py --region us-east-1
```

**Capabilities:**
- Multi-cluster GitOps with automatic sync
- Promotion pipelines with automated testing gates
- Rollback to any previous revision across all clusters
- Application marketplace with pre-built configs
- Cost breakdown per application per cluster

**Use via Claude:** `/plural` → deploy, promote, or view fleet

---

### SRE Guard (`agents/sre-guard/`)

**Language:** Python | **Entrypoint:** `sreguard` CLI + REST daemon on `:8888`

A persistent monitoring daemon that sits as a guard for your deployed services. Runs continuously, polls Prometheus metrics and health endpoints every 30 seconds, watches Kubernetes events, and fires alerts when thresholds are crossed. When an incident is detected, it can auto-invoke HolmesGPT for AI diagnosis.

```bash
pip install -e agents/sre-guard/

# Start the daemon (background)
sreguard daemon start

# Add a service to monitor
sreguard watch payment-api \
  --prometheus http://prometheus:9090 \
  --namespace prod \
  --health http://payment-api/health

# Check what's being monitored
sreguard status

# AI-diagnose a service
sreguard diagnose payment-api

# Silence noisy alerts
sreguard silence payment-api --minutes 30

# View alert history
sreguard logs payment-api --tail 50

# Stop the daemon
sreguard daemon stop
```

**REST API (port 8888):**

| Endpoint | Method | Action |
|----------|--------|--------|
| `/status` | GET | All watched services + current alert state |
| `/watch` | POST | Add service to watch list |
| `/watch/{service}` | DELETE | Stop watching |
| `/diagnose/{service}` | POST | Trigger holmesgpt AI investigation |
| `/silence/{service}` | POST | Mute alerts for N minutes |
| `/runbook/{service}` | POST | Execute a predefined runbook |

**Alert rules (configured in `agents/sre-guard/config/sre-guard.yaml`):**
- HighErrorRate: >5% 5xx responses over 5 min → critical
- HighLatency: p95 latency >1s over 5 min → warning
- PodDown: `up` metric drops below 1 → critical

**Use via Claude:** `/sre-guard` → start daemon, add services, diagnose incidents

---

## CI/CD Workflows

All 10 workflows have `workflow_dispatch` — trigger any from the GitHub Actions tab.

| Workflow | Triggers | Jobs |
|----------|----------|------|
| `ci.yml` | `tools/**`, `templates/**` | ruff lint, pytest, terraform validate (3 clouds), helm lint, hadolint |
| `ci-services.yml` | `services/**` | python test + Docker build, Go test + Docker build, Next.js build + Docker build, Helm lint |
| `ci-holmesgpt.yml` | `agents/holmesgpt/**` | Ruff lint, pytest (non-LLM), Docker build |
| `ci-kagent.yml` | `agents/kagent/**` | Go build + test, Helm lint |
| `ci-nightshift.yml` | `agents/nightshift/**` | Go build + test + vet |
| `ci-opensre.yml` | `agents/opensre/**` | Python lint + test, Node lint + build |
| `ci-plural.yml` | `agents/plural/**` | Python test, Docker build |
| `security.yml` | All pushes + weekly Monday | Trivy repo scan → SARIF, Gitleaks secret scan, CodeQL Python |
| `release.yml` | `v*.*.*` tags | Auto changelog + GitHub Release |
| `all-green.yml` | All pushes | Single required check for branch protection |

All action steps are SHA-pinned (no floating `@v3` tags).

---

## Templates

| Path | What's inside |
|------|--------------|
| `templates/terraform/aws/` | EKS 1.31 + ECR + RDS + VPC (7 split files) |
| `templates/terraform/azure/` | AKS + ACR + PostgreSQL Flexible Server + Key Vault |
| `templates/terraform/gcp/` | GKE Autopilot + Artifact Registry + Cloud SQL + Secret Manager |
| `templates/dockerfiles/` | Multi-stage distroless Dockerfiles for Node/Python/Go/Java |
| `templates/github-actions/` | CI + CD templates with OIDC (no static credentials) |
| `templates/helm/` | Production chart: security contexts, KEDA, ESO, ServiceMonitor, PrometheusRule, Grafana dashboard |
| `templates/keda/` | HTTPScaledObject — car-painter scale-to-zero (min=0, scaledown=5min) |
| `templates/monitoring/` | Prometheus v3 + Grafana 11 + Alertmanager Docker Compose |

---

## Security Defaults

Every generated artefact enforces:
- OIDC cloud auth — no static credentials in CI
- IRSA / Workload Identity — no IAM keys mounted in pods
- External Secrets Operator — secrets never in Git
- `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`
- Distroless final images — no shell, no package manager in prod
- Trivy scan gate — no HIGH/CRITICAL CVEs to pass

---

## Car-Painter Scale-to-Zero

Scales pods to **0 replicas** after 5 min idle, back to 1 in <60s on first request. 60–90% compute savings for bursty/low-traffic services.

| Platform | Mechanism |
|----------|-----------|
| EKS | KEDA + HTTP Add-on |
| AKS | Built-in KEDA add-on |
| GKE | KEDA or Cloud Run |
| Cloud Run / Container Apps / Fargate | Native — no KEDA needed |

Apply with: `/optimize-cost` or `python3 tools/cost_optimize.py --terraform-dir <dir> --platform eks`

---

## Local Cloud Emulators

```bash
docker compose -f tools/emulators/localstack.yml up -d     # AWS (port 4566)
docker compose -f tools/emulators/azurite.yml up -d        # Azure (port 10000)
docker compose -f tools/emulators/gcp-emulators.yml up -d  # GCP (ports 8080, 8085)
```

---

## Adding a New Service

1. Put source code in `services/<name>/`
2. Run `/deploy` in Claude Code and provide the details, **or**
3. Run `python3 tools/workflow.py --repo services/<name> --service <name> --cloud <cloud> --env dev`

The agent auto-detects language, generates Dockerfile + Terraform + Helm + CI/CD, discovers secrets, tests everything against a local emulator, and summarises what to push and what to configure in GitHub.
