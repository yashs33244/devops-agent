# DevOps Agent

AI-powered DevOps automation agent. Powered by Claude Code.

## What It Does

1. **Dockerizes** your app (detects language, generates production Dockerfile, builds + tests)
2. **Provisions infrastructure** (Terraform for AWS/Azure/GCP — EKS/AKS/GKE + registry + DB)
3. **Sets up CI/CD** (GitHub Actions — CI pipeline + CD with Terraform + K8s deploy)
4. **Tests locally** (LocalStack/Azurite/GCP emulators before touching real cloud)
5. **Optimizes cost** (car-painter scale-to-zero via KEDA — pods → 0 when idle, back in <60s)

## Quick Start

```bash
# Full pipeline: clone → dockerize → terraform → ci/cd → local test
python3 tools/workflow.py \
  --repo https://github.com/your-org/your-app \
  --service my-service \
  --cloud aws \
  --env dev \
  --with-keda

# Individual tools
python3 tools/dockerize.py --path ./my-app --service my-service
python3 tools/terraform_gen.py --cloud aws --service my-service --env dev
python3 tools/cicd_setup.py --repo-path ./my-app --cloud aws --service my-service
python3 tools/local_test.py --cloud aws --terraform-dir ./my-app/terraform --service my-service
python3 tools/cost_optimize.py --terraform-dir ./my-app/terraform --platform eks --service my-service
```

## Or just ask Claude

Open this folder in Claude Code and describe what you want to deploy. Claude will follow the workflow in CLAUDE.md automatically.

```
"Deploy my Node.js payment API from github.com/acme/payment-api to AWS with a Postgres DB"
```

## Car-Painter Scale-to-Zero

The car-painter pattern scales pods to **0 replicas** when idle for 5 min, then back to 1 within 60 seconds on first request. 70-90% cost savings for bursty/low-traffic services.

Uses:
- **EKS**: KEDA + HTTP Add-on
- **AKS**: Built-in KEDA add-on (enabled by default in this Terraform)
- **GKE**: KEDA or Cloud Run (natively serverless)

## Cloud Emulators (Local Testing)

| Cloud | Tool | Port |
|-------|------|------|
| AWS | LocalStack | 4566 |
| Azure | Azurite | 10000 |
| GCP | Firestore+Pub/Sub emulators | 8080, 8085 |

```bash
# Start emulator manually
docker compose -f tools/emulators/localstack.yml up -d    # AWS
docker compose -f tools/emulators/azurite.yml up -d       # Azure
docker compose -f tools/emulators/gcp-emulators.yml up -d # GCP
```

## Structure

```
devops-agent/
├── CLAUDE.md                  ← Agent personality & workflow
├── tools/
│   ├── workflow.py            ← Full pipeline orchestrator
│   ├── dockerize.py           ← Dockerfile generation
│   ├── terraform_gen.py       ← Terraform generation
│   ├── cicd_setup.py          ← GitHub Actions setup
│   ├── local_test.py          ← Local emulator testing
│   ├── cost_optimize.py       ← KEDA scale-to-zero
│   └── emulators/             ← Docker Compose for local clouds
├── templates/
│   ├── terraform/aws/         ← EKS + ECR + RDS
│   ├── terraform/azure/       ← AKS + ACR + PostgreSQL
│   ├── terraform/gcp/         ← GKE Autopilot + Artifact Registry
│   ├── dockerfiles/           ← Node/Python/Go/Java
│   ├── github-actions/        ← CI + CD workflows
│   └── keda/                  ← HTTPScaledObject (car-painter)
├── workspace/                 ← Cloned repos land here
├── holmesgpt/                 ← SRE incident investigation
├── kagent/                    ← K8s AI agent
├── nightshift/                ← Cost optimization scheduler
├── opensre/                   ← SRE runbook automation
└── plural/                    ← GitOps multi-cloud deployment
```

## v2 Roadmap

- SRE agent integration (holmesgpt + opensre)
- Multi-cluster fleet management (kagent)
- GitOps deployment via plural
- Automated incident runbooks
- Cost anomaly detection + auto-remediation
