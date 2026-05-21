# GitHub Actions Secrets — go-api

Add these secrets to your GitHub repository at:

  Settings → Secrets and variables → Actions → New repository secret


## Required Secrets

| Secret Name | Description | Where to get it |
|-------------|-------------|-----------------|
| `GCP_PROJECT_ID` | GCP project ID | GCP Console → Project Info |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | WIF provider resource name | GCP IAM → Workload Identity |
| `GCP_SERVICE_ACCOUNT` | SA email for OIDC | GCP IAM → Service Accounts |
| `GKE_CLUSTER_NAME` | GKE cluster name | GCP Console → Kubernetes Engine |
| `GKE_CLUSTER_ZONE` | Cluster zone/region | GCP Console → Kubernetes Engine |
| `ARTIFACT_REGISTRY_REPO` | Artifact Registry repo | GCP Console → Artifact Registry |

## Variables (non-sensitive)

| Variable Name | Example Value | Description |
|---------------|---------------|-------------|
| `SERVICE_NAME` | `go-api` | Service slug |
| `NAMESPACE` | `go-api` | K8s namespace |

## Notes
- Use OIDC federation where possible — no long-lived static credentials
- Rotate secrets regularly; ESO will auto-sync from cloud secrets manager
- Never put secret values in GitHub Actions `env:` block as plain text
