# templates/terraform/

Terraform templates for AWS, Azure, and GCP. Rendered by `tools/terraform_gen.py` into `workspace/<service>/terraform/`.

## What's Here

```
aws/
  main.tf        # EKS cluster, ECR repo, RDS (optional), VPC
  variables.tf   # All input variables with descriptions and defaults
  outputs.tf     # Cluster endpoint, ECR URL, DB endpoint
  providers.tf   # AWS provider with version constraint
  versions.tf    # Terraform + provider version pins
  backend.tf     # S3 + DynamoDB state backend (templated)
  locals.tf      # Common tags, name prefixes

azure/           # AKS + ACR + PostgreSQL Flexible Server (same file structure)
gcp/             # GKE Autopilot + Artifact Registry + Cloud SQL (same file structure)
tests/           # Terratest integration tests
```

## Variable Substitution

`tools/terraform_gen.py` replaces these placeholders before writing output:

| Placeholder | Source flag | Example |
|-------------|-------------|---------|
| `{{SERVICE_NAME}}` | `--service` | `payment-api` |
| `{{REGION}}` | `--region` | `us-east-1` |
| `{{ENVIRONMENT}}` | `--env` | `dev` |
| `{{CLOUD}}` | `--cloud` | `aws` |
| `{{USE_CASE}}` | `--use-case` | `web_app` |

## How to Use / Run

```bash
# Generate (via tool — preferred)
python3 tools/terraform_gen.py \
  --cloud aws --service payment-api \
  --use-case web_app --region us-east-1 --env dev

# After generation, always validate
cd workspace/payment-api/terraform
terraform fmt
terraform validate -backend=false

# Plan (requires real cloud credentials)
terraform plan

# Emulator run (AWS only)
tflocal plan   # requires LocalStack running
```

## Key Details

- All three clouds follow identical file structure — same variable names where possible to ease multi-cloud work
- `backend.tf` uses S3 (AWS) / Azure Blob (Azure) / GCS (GCP) with encryption at rest — never use local state in prod
- Dev defaults: smallest viable node size (e.g., `t3.small` on AWS). Prod sizing comments are inline in `variables.tf`
- RDS / PostgreSQL / Cloud SQL are optional — only rendered when `--backing-services postgres` is passed to `terraform_gen.py`
- Run `tests/` with Terratest: `cd tests && go test -timeout 30m ./...`

## Related

- `tools/terraform_gen.py` — renders these templates
- `tools/local_test.py` — validates with LocalStack / Azurite / GCP emulators
- Root `CLAUDE.md` → Security Checklist — Terraform state encryption requirements
