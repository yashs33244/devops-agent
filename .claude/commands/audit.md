# /audit — Audit an Existing Service for Improvements

Audit an existing service's Dockerfile, Terraform, and Helm chart against the security checklist and best practices, then report findings ordered by severity.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Path to the existing service** — must contain at least one of: `Dockerfile`, `terraform/`, `helm/`
2. **Service name** — lowercase, hyphens only
3. **Cloud provider** — `aws`, `azure`, or `gcp` (needed for cloud-specific checks)

## Step 2: Establish Baseline with Test Runner

```bash
python3 tools/test_runner.py \
  --service <service_name> \
  --repo-path <service_path> \
  --terraform-dir <service_path>/terraform \
  --helm-dir <service_path>/helm \
  --cloud <cloud>
```

Note which tests pass and which fail — this is the baseline before any changes.

## Step 3: Run Security Checks

```bash
# Check for hardcoded secrets
grep -rn "password\s*=" <service_path> --include="*.yaml" --include="*.tf" --include="*.env"
grep -rn "secret\s*=" <service_path> --include="*.yaml" --include="*.tf"
grep -rn "AWS_ACCESS_KEY_ID\|AWS_SECRET_ACCESS_KEY" <service_path>

# Scan image for CVEs (if Dockerfile present)
trivy image --severity HIGH,CRITICAL <service_name>:latest 2>/dev/null || echo "Build image first to scan"

# Terraform security scan
checkov -d <service_path>/terraform --quiet 2>/dev/null || echo "checkov not installed"

# Helm security context check
grep -r "runAsNonRoot\|readOnlyRootFilesystem\|allowPrivilegeEscalation" <service_path>/helm/ || echo "Security contexts missing"
```

## Step 4: Check for Outdated Versions

- Docker base image tags — are they pinned or using `latest`?
- Terraform provider versions — any without version constraints?
- Helm chart `appVersion` — does it match the image being deployed?
- GitHub Actions steps — are they pinned to full SHAs?

## Step 5: Check for Scale-to-Zero Opportunity

Look for signs the service is bursty or low-traffic:
- Stateless HTTP deployment?
- No `HorizontalPodAutoscaler` with `minReplicas > 0`?

If applicable, suggest running `/optimize-cost`.

## Step 6: Report Findings

Present findings ordered by priority. Use this exact format:

**CRITICAL** (fix before any deployment):
- Hardcoded secrets found
- HIGH/CRITICAL CVEs in base image
- Public storage buckets / unauthenticated endpoints

**HIGH** (fix before prod):
- Missing OIDC / using static cloud credentials
- No network policies defined
- Missing resource requests/limits
- Containers running as root

**MEDIUM** (fix in next sprint):
- Outdated provider/base image versions
- Missing or incomplete health probes
- No PodDisruptionBudget for prod workloads
- Missing resource tags/labels

**LOW** (nice to have):
- Style improvements
- Optional scale-to-zero savings
- Non-optimal instance sizes for cost

End with: total issue count by severity and a recommended fix order.
