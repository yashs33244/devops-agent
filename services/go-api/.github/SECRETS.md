# GitHub Secrets & Variables Required

## Repository Secrets (Settings → Secrets and variables → Actions)

- `GCP_WORKLOAD_IDENTITY_PROVIDER` — Workload Identity Provider resource name
- `GCP_SERVICE_ACCOUNT` — Service account email

## Repository Variables (Settings → Secrets and variables → Actions → Variables)
- `GCP_REGION` — Your cloud region / registry name

## Enable OIDC (Recommended — no long-lived secrets)
See: https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect
