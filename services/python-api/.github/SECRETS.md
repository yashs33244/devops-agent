# GitHub Secrets & Variables Required

## Repository Secrets (Settings → Secrets and variables → Actions)

- `AWS_ROLE_ARN` — IAM role ARN for OIDC auth (e.g., arn:aws:iam::123456789:role/github-actions)

## Repository Variables (Settings → Secrets and variables → Actions → Variables)
- `AWS_REGION` — Your cloud region / registry name

## Enable OIDC (Recommended — no long-lived secrets)
See: https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect
