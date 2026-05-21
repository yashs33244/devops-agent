#!/usr/bin/env python3
"""Generate GitHub Actions CI/CD workflows for a service."""

import argparse
import json
import os
import shutil
import sys
from pathlib import Path

TEMPLATES_DIR = Path(__file__).parent.parent / "templates" / "github-actions"

REGISTRY_URLS = {
    "aws": "{AWS_ACCOUNT_ID}.dkr.ecr.{REGION}.amazonaws.com",
    "azure": "{SERVICE_NAME}acr.azurecr.io",
    "gcp": "{REGION}-docker.pkg.dev/{PROJECT_ID}/{SERVICE_NAME}",
}

CLOUD_AUTH_STEPS = {
    "aws": """
      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: ${{ vars.AWS_REGION }}

      - name: Login to Amazon ECR
        id: login-ecr
        uses: aws-actions/amazon-ecr-login@v2
""",
    "azure": """
      - name: Azure login
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Login to ACR
        run: az acr login --name ${{ vars.ACR_NAME }}
""",
    "gcp": """
      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.GCP_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ secrets.GCP_SERVICE_ACCOUNT }}

      - name: Configure Docker for Artifact Registry
        run: gcloud auth configure-docker ${{ vars.GCP_REGION }}-docker.pkg.dev --quiet
""",
}


def render(content: str, vars: dict) -> str:
    for k, v in vars.items():
        content = content.replace(f"{{{{{k}}}}}", str(v))
    return content


def main():
    parser = argparse.ArgumentParser(description="Generate GitHub Actions CI/CD workflows")
    parser.add_argument("--repo-path", required=True, help="Path to the cloned repository")
    parser.add_argument("--cloud", required=True, choices=["aws", "azure", "gcp"])
    parser.add_argument("--service", required=True, help="Service name (slug)")
    parser.add_argument("--registry-url", help="Container registry URL (auto-detected if not given)")
    parser.add_argument("--branch", default="main", help="Production branch name")
    parser.add_argument("--env", default="dev", choices=["dev", "staging", "prod"])
    args = parser.parse_args()

    repo_path = Path(args.repo_path).resolve()
    if not repo_path.exists():
        print(json.dumps({"success": False, "error": f"Repo path not found: {repo_path}"}))
        sys.exit(1)

    workflows_dir = repo_path / ".github" / "workflows"
    workflows_dir.mkdir(parents=True, exist_ok=True)

    registry_url = args.registry_url or REGISTRY_URLS[args.cloud]
    auth_steps = CLOUD_AUTH_STEPS[args.cloud]

    template_vars = {
        "SERVICE_NAME": args.service,
        "CLOUD": args.cloud.upper(),
        "REGISTRY_URL": registry_url,
        "BRANCH": args.branch,
        "ENVIRONMENT": args.env,
        "CLOUD_AUTH_STEPS": auth_steps,
    }

    written = []
    for template_file in TEMPLATES_DIR.glob("*.yml"):
        dst = workflows_dir / template_file.name
        content = render(template_file.read_text(), template_vars)
        dst.write_text(content)
        written.append(str(dst))
        print(f"[cicd_setup] Wrote {dst}")

    # Write a secrets reference doc
    secrets_doc = f"""# GitHub Secrets & Variables Required

## Repository Secrets (Settings → Secrets and variables → Actions)
"""
    if args.cloud == "aws":
        secrets_doc += """
- `AWS_ROLE_ARN` — IAM role ARN for OIDC auth (e.g., arn:aws:iam::123456789:role/github-actions)
"""
    elif args.cloud == "azure":
        secrets_doc += """
- `AZURE_CLIENT_ID` — Service principal client ID (federated credential)
- `AZURE_TENANT_ID` — Azure tenant ID
- `AZURE_SUBSCRIPTION_ID` — Azure subscription ID
"""
    elif args.cloud == "gcp":
        secrets_doc += """
- `GCP_WORKLOAD_IDENTITY_PROVIDER` — Workload Identity Provider resource name
- `GCP_SERVICE_ACCOUNT` — Service account email
"""

    secrets_doc += f"""
## Repository Variables (Settings → Secrets and variables → Actions → Variables)
- `{"AWS_REGION" if args.cloud == "aws" else "GCP_REGION" if args.cloud == "gcp" else "ACR_NAME"}` — Your cloud region / registry name

## Enable OIDC (Recommended — no long-lived secrets)
See: https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect
"""
    (repo_path / ".github" / "SECRETS.md").write_text(secrets_doc)

    print(json.dumps({
        "success": True,
        "ci_path": str(workflows_dir / "ci.yml"),
        "cd_path": str(workflows_dir / "cd.yml"),
        "secrets_doc": str(repo_path / ".github" / "SECRETS.md"),
        "cloud": args.cloud,
        "files_written": written,
    }))


if __name__ == "__main__":
    main()
