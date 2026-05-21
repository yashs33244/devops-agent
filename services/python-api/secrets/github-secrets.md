# GitHub Actions Secrets — python-api

Add these secrets to your GitHub repository at:

  Settings → Secrets and variables → Actions → New repository secret


## Required Secrets

| Secret Name | Description | Where to get it |
|-------------|-------------|-----------------|
| `AWS_ACCOUNT_ID` | Your AWS account ID | AWS Console → top-right menu |
| `AWS_REGION` | Target AWS region (e.g. us-east-1) | Your infra config |
| `ECR_REPOSITORY` | ECR repo name | AWS ECR Console |
| `EKS_CLUSTER_NAME` | EKS cluster name | AWS EKS Console |
| `SECRETS_MANAGER_ARN_PREFIX` | ARN prefix for secrets | AWS Secrets Manager |

## Variables (non-sensitive)

| Variable Name | Example Value | Description |
|---------------|---------------|-------------|
| `SERVICE_NAME` | `python-api` | Service slug |
| `NAMESPACE` | `python-api` | K8s namespace |

## Notes
- Use OIDC federation where possible — no long-lived static credentials
- Rotate secrets regularly; ESO will auto-sync from cloud secrets manager
- Never put secret values in GitHub Actions `env:` block as plain text
