# devops-agent

AI-powered DevOps automation platform — monorepo.

Powered by Claude Code. Orchestrates Dockerization, Terraform provisioning, Helm charts, CI/CD, secrets management, local testing, and cost optimization across AWS / Azure / GCP.

## Monorepo Layout

```
devops-agent/
├── tools/                  ← Orchestration CLI tools (Python)
│   ├── workflow.py          full pipeline runner
│   ├── dockerize.py         Dockerfile generation + build test
│   ├── terraform_gen.py     Terraform generation from templates
│   ├── helm_gen.py          Helm chart generation + lint
│   ├── cicd_setup.py        GitHub Actions CI + CD (OIDC)
│   ├── secrets_manager.py   Secret discovery + ESO manifests
│   ├── local_test.py        LocalStack / Azurite / GCP emulator tests
│   ├── cost_optimize.py     KEDA scale-to-zero applier
│   ├── test_runner.py       All-layers TDD runner
│   └── emulators/           Docker Compose for local clouds
│
├── templates/              ← Reusable infrastructure templates
│   ├── terraform/aws/       EKS + ECR + RDS + VPC
│   ├── terraform/azure/     AKS + ACR + PostgreSQL Flexible Server
│   ├── terraform/gcp/       GKE Autopilot + Artifact Registry + Cloud SQL
│   ├── dockerfiles/         Multi-stage distroless Dockerfiles (Node/Python/Go/Java)
│   ├── github-actions/      CI + CD workflow templates (OIDC, SHA-pinned)
│   ├── helm/                Production Helm chart template (security contexts, monitoring)
│   ├── keda/                HTTPScaledObject (car-painter scale-to-zero)
│   └── monitoring/          Prometheus + Grafana Docker Compose stack
│
├── services/               ← First-party sample services (deployed to kind for testing)
│   ├── python-api/          FastAPI + Prometheus metrics + Helm chart
│   ├── go-api/              Go + Chi + Prometheus metrics + Helm chart
│   └── nextjs-demo/         Next.js App Router + /api/health + /api/metrics
│
├── agents/                 ← Integrated sub-agents (reference implementations)
│   ├── holmesgpt/           AI SRE incident investigation (Python)
│   ├── kagent/              Kubernetes AI fleet agent (Go)
│   ├── nightshift/          Cost optimization scheduler (Go)
│   ├── opensre/             SRE runbook automation (Python + Node)
│   └── plural/              GitOps multi-cloud deployment
│
└── workspace/              ← Gitignored — cloned repos land here
```

## Quick Start

```bash
# Install dependencies
pip install -r requirements.txt

# Full pipeline: repo → Docker → Terraform → Helm → CI/CD → tests
python3 tools/workflow.py \
  --repo https://github.com/your-org/your-app \
  --service my-service \
  --cloud aws \
  --env dev \
  --with-keda

# Individual tools
python3 tools/dockerize.py     --path ./my-app --service my-service
python3 tools/terraform_gen.py --cloud aws --service my-service --env dev --region us-east-1
python3 tools/helm_gen.py      --service my-service --cloud aws --port 8000
python3 tools/cicd_setup.py    --repo-path ./my-app --cloud aws --service my-service
python3 tools/secrets_manager.py --repo-path ./my-app --service my-service --cloud aws
python3 tools/local_test.py    --cloud aws --terraform-dir ./my-app/terraform --service my-service
python3 tools/cost_optimize.py --terraform-dir ./my-app/terraform --platform eks --service my-service
python3 tools/test_runner.py   --service my-service --repo-path ./my-app --cloud aws
```

## Or just ask Claude

Open this folder in Claude Code and describe what you want to deploy. Claude follows the full workflow in `CLAUDE.md` automatically.

```
"Deploy my Node.js payment API from github.com/acme/payment-api to AWS with a Postgres DB"
```

## CI/CD Workflows

| Workflow | Triggers on |
|----------|-------------|
| `ci.yml` | `tools/**`, `templates/**` — lint + validate orchestration layer |
| `ci-services.yml` | `services/**` — test + Docker build all 3 sample services |
| `ci-holmesgpt.yml` | `agents/holmesgpt/**` — Python lint + pytest |
| `ci-kagent.yml` | `agents/kagent/**` — Go build + test + Helm lint |
| `ci-nightshift.yml` | `agents/nightshift/**` — Go build + test |
| `ci-opensre.yml` | `agents/opensre/**` — Python + Node lint + test |
| `ci-plural.yml` | `agents/plural/**` — Python test + Docker build |
| `security.yml` | All pushes + weekly — Trivy + Gitleaks + CodeQL |
| `release.yml` | `v*.*.*` tags — auto changelog + GitHub Release |

All action steps are pinned to full commit SHAs (no floating `@v3` tags).

## Car-Painter Scale-to-Zero

Scales pods to **0 replicas** after 5 min idle, back to 1 within 60 s on first request. 70–90% compute savings for bursty or low-traffic services.

| Platform | Mechanism |
|----------|-----------|
| EKS | KEDA + HTTP Add-on |
| AKS | Built-in KEDA add-on |
| GKE | KEDA or Cloud Run (natively serverless) |
| Cloud Run / Container Apps / Fargate | Native — no KEDA needed |

## Local Cloud Emulators

| Cloud | Tool | Start |
|-------|------|-------|
| AWS | LocalStack | `docker compose -f tools/emulators/localstack.yml up -d` |
| Azure | Azurite | `docker compose -f tools/emulators/azurite.yml up -d` |
| GCP | Firestore + Pub/Sub | `docker compose -f tools/emulators/gcp-emulators.yml up -d` |

## Security Defaults

Every generated artefact enforces:
- OIDC cloud auth (no static credentials in CI)
- IRSA / Workload Identity (no IAM keys mounted in pods)
- External Secrets Operator (secrets never in Git)
- `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `capabilities.drop: [ALL]`
- Distroless final images (no shell, no package manager in prod)
- Trivy scan — no HIGH/CRITICAL CVEs required to pass

## Adding a New Service

1. Put source code in `services/<name>/`
2. Run `python3 tools/workflow.py --repo services/<name> --service <name> --cloud <cloud> --env dev`
3. The agent Dockerizes, generates Terraform + Helm, sets up CI/CD, and runs all tests
