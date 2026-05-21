# templates/github-actions/

GitHub Actions CI/CD workflow templates. Rendered by `tools/cicd_setup.py` into `workspace/<service>/.github/workflows/`.

## What's Here

```
ci.yml                    # CI pipeline: lint → test → Docker build → trivy scan
cd.yml                    # CD pipeline: push to registry → helm upgrade (OIDC auth)
security-scan.yml         # Standalone Trivy + checkov security scan workflow
_reusable-docker-build.yml   # Reusable workflow: build + push multi-arch image
_reusable-terraform.yml      # Reusable workflow: terraform fmt/validate/plan/apply
```

## Key Details

- **No static cloud credentials** — all cloud auth uses OIDC federation (AWS: `aws-actions/configure-aws-credentials`, Azure: `azure/login`, GCP: `google-github-actions/auth`)
- All `uses:` references are pinned to full SHA (not `@v3` tags) — required by security checklist
- CD workflow calls `helm upgrade --install` — idempotent, safe to re-run
- `_reusable-*` workflows use `workflow_call` trigger — compose them in ci.yml/cd.yml
- OIDC setup requires adding a GitHub identity provider to your cloud account; `tools/cicd_setup.py` outputs the exact IAM/role config needed

## OIDC Setup per Cloud

| Cloud | Action | Required GitHub Secret |
|-------|--------|----------------------|
| AWS | `aws-actions/configure-aws-credentials` | `AWS_ROLE_ARN` (no key needed) |
| Azure | `azure/login` | `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID` |
| GCP | `google-github-actions/auth` | `GCP_WORKLOAD_IDENTITY_PROVIDER`, `GCP_SERVICE_ACCOUNT` |

## How to Use / Run

```bash
# Generate workflows (via tool — preferred)
python3 tools/cicd_setup.py \
  --repo-path workspace/payment-api \
  --cloud aws \
  --service payment-api

# Test locally with act
act push --dry-run
act push -j build

# After generating, check for hardcoded secrets
grep -rn "password\s*=" workspace/payment-api/.github/ --include="*.yml"
```

## Related

- `tools/cicd_setup.py` — renders and writes these templates
- Root `CLAUDE.md` → Security Checklist — GitHub Actions SHA pinning and OIDC requirements
- `workspace/<service>/secrets/github-secrets.md` — exact secrets to add to GitHub repository settings
