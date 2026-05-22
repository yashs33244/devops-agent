#!/usr/bin/env python3
"""
secrets_manager.py — DevOps Agent Secrets Discovery & Manifest Generator

Scans a repository for secret requirements, walks the user through confirming
each one, and generates secrets manifests (ESO ExternalSecret or K8s native Secret).

Usage:
  python3 tools/secrets_manager.py --repo-path ./workspace/my-service \
      --service my-service --cloud aws [--output-dir ./output] [--non-interactive]
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from textwrap import dedent
from typing import Optional

# ──────────────────────────────────────────────────────────────────────────────
# Data model
# ──────────────────────────────────────────────────────────────────────────────

class SecretCategory(str, Enum):
    DATABASE    = "database"
    CACHE       = "cache"
    CLOUD_CRED  = "cloud-cred"
    API_KEY     = "api-key"
    APP_SECRET  = "app-secret"
    THIRD_PARTY = "third-party"
    UNKNOWN     = "unknown"


CLOUD_CRED_WARNING = (
    "WARNING: Use IRSA (AWS) / Workload Identity (Azure/GCP) instead of static keys."
)

# Ordered list of (pattern, category, note)
DETECTION_RULES: list[tuple[re.Pattern[str], SecretCategory, str]] = [
    # Database
    (re.compile(r'\b(DATABASE_URL|DB_PASSWORD|DB_HOST|DB_PORT|DB_NAME|DB_USER)\b'),
     SecretCategory.DATABASE, ""),
    (re.compile(r'\bPOSTGRES_[A-Z_]+\b'),
     SecretCategory.DATABASE, ""),
    (re.compile(r'\b(MYSQL_[A-Z_]+|MONGO_URI|MONGODB_[A-Z_]+)\b'),
     SecretCategory.DATABASE, ""),
    # Cache
    (re.compile(r'\b(REDIS_URL|REDIS_PASSWORD|REDIS_HOST)\b'),
     SecretCategory.CACHE, ""),
    # Cloud credentials — always warn
    (re.compile(r'\b(AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|AWS_[A-Z_]+)\b'),
     SecretCategory.CLOUD_CRED, CLOUD_CRED_WARNING),
    (re.compile(r'\b(AZURE_CLIENT_ID|AZURE_CLIENT_SECRET|AZURE_TENANT_ID|AZURE_[A-Z_]+)\b'),
     SecretCategory.CLOUD_CRED, CLOUD_CRED_WARNING),
    (re.compile(r'\b(GCP_[A-Z_]+|GOOGLE_APPLICATION_CREDENTIALS|GOOGLE_CLOUD_[A-Z_]+|GOOGLE_[A-Z_]+)\b'),
     SecretCategory.CLOUD_CRED, CLOUD_CRED_WARNING),
    # Third-party services
    (re.compile(r'\b(STRIPE_[A-Z_]+|SENDGRID_[A-Z_]+|TWILIO_[A-Z_]+)\b'),
     SecretCategory.THIRD_PARTY, ""),
    # App secrets
    (re.compile(r'\b(JWT_SECRET|SESSION_SECRET|COOKIE_SECRET|ENCRYPTION_KEY)\b'),
     SecretCategory.APP_SECRET, ""),
    # Generic API keys / tokens / secrets
    (re.compile(r'\b[A-Z][A-Z0-9_]*_API_KEY\b'),
     SecretCategory.API_KEY, ""),
    (re.compile(r'\bAPI_KEY\b'),
     SecretCategory.API_KEY, ""),
    (re.compile(r'\b[A-Z][A-Z0-9_]*_SECRET\b'),
     SecretCategory.APP_SECRET, ""),
    (re.compile(r'\b[A-Z][A-Z0-9_]*_TOKEN\b'),
     SecretCategory.API_KEY, ""),
]

# Files / globs to scan
SCAN_TARGETS = [
    ".env.example",
    ".env.sample",
    ".env.template",
    "docker-compose.yml",
    "docker-compose.yaml",
]
SCAN_GLOBS = [
    "k8s/**/*.yaml",
    "k8s/**/*.yml",
    "helm/**/values.yaml",
    "helm/**/values.yml",
    "**/.env.example",
    "**/.env.sample",
]

# Characters we consider "safe" env-var name chars
_ENV_VAR_RE = re.compile(r'\b[A-Z][A-Z0-9_]{2,}\b')


@dataclass
class DetectedSecret:
    name: str
    category: SecretCategory
    note: str = ""
    source_file: str = ""
    include: bool = True          # user decision
    use_irsa: bool = False        # skip static key, configure IRSA instead


# ──────────────────────────────────────────────────────────────────────────────
# Detection
# ──────────────────────────────────────────────────────────────────────────────

def _collect_candidate_files(repo_path: Path) -> list[Path]:
    """Return every file we should scan for secret names."""
    found: list[Path] = []

    # Explicit filenames at repo root
    for name in SCAN_TARGETS:
        p = repo_path / name
        if p.is_file():
            found.append(p)

    # Glob patterns
    for pattern in SCAN_GLOBS:
        found.extend(repo_path.glob(pattern))

    # De-duplicate while preserving order
    seen: set[Path] = set()
    result: list[Path] = []
    for p in found:
        rp = p.resolve()
        if rp not in seen:
            seen.add(rp)
            result.append(p)
    return result


def _scan_file(path: Path) -> list[tuple[str, SecretCategory, str]]:
    """Return list of (var_name, category, note) tuples found in a single file."""
    try:
        text = path.read_text(errors="replace")
    except OSError:
        return []

    hits: list[tuple[str, SecretCategory, str]] = []
    for var_name in _ENV_VAR_RE.findall(text):
        for pattern, category, note in DETECTION_RULES:
            if pattern.fullmatch(var_name):
                hits.append((var_name, category, note))
                break
    return hits


def detect_secrets(repo_path: Path) -> list[DetectedSecret]:
    """Scan the repo and return deduplicated DetectedSecret list."""
    candidates = _collect_candidate_files(repo_path)
    seen: dict[str, DetectedSecret] = {}

    for file_path in candidates:
        rel = str(file_path.relative_to(repo_path))
        for var_name, category, note in _scan_file(file_path):
            if var_name not in seen:
                seen[var_name] = DetectedSecret(
                    name=var_name,
                    category=category,
                    note=note,
                    source_file=rel,
                )

    # Sort: non-cloud-creds first, then cloud creds
    return sorted(
        seen.values(),
        key=lambda s: (s.category == SecretCategory.CLOUD_CRED, s.name),
    )


# ──────────────────────────────────────────────────────────────────────────────
# Interactive flow
# ──────────────────────────────────────────────────────────────────────────────

def _yn(prompt: str, default_yes: bool = True) -> bool:
    suffix = "[Y/n]" if default_yes else "[y/N]"
    while True:
        raw = input(f"{prompt} {suffix}: ").strip().lower()
        if raw in ("", "y", "yes"):
            return True if raw != "" or default_yes else False
        if raw in ("n", "no"):
            return False
        print("  Please enter y or n.")


def _choose(prompt: str, choices: list[str], default: str) -> str:
    options = "/".join(choices)
    while True:
        raw = input(f"{prompt} [{options}] (default: {default}): ").strip().lower()
        if raw == "":
            return default
        if raw in choices:
            return raw
        print(f"  Please choose one of: {options}")


def run_interactive(
    secrets: list[DetectedSecret],
    service: str,
    cloud: str,
) -> tuple[list[DetectedSecret], str]:
    """
    Walk the user through confirming secrets and choosing a store.
    Returns (confirmed_secrets, store_type) where store_type is 'eso' or 'native'.
    """
    RESET  = "\033[0m"
    BOLD   = "\033[1m"
    YELLOW = "\033[33m"
    CYAN   = "\033[36m"

    print(f"\n{BOLD}Detected secrets for service: {service}{RESET}")
    print("─" * 50)

    for idx, secret in enumerate(secrets, start=1):
        cat_label = f"({secret.category.value:<12})"
        print(f"\n{CYAN}[{idx}] {secret.name:<30}{RESET} {cat_label}")
        if secret.source_file:
            print(f"     Found in: {secret.source_file}")

        if secret.category == SecretCategory.CLOUD_CRED:
            print(f"     {YELLOW}{BOLD}{secret.note}{RESET}")
            use_irsa = _yn(
                "     Skip static key and configure IRSA/Workload Identity instead?",
                default_yes=True,
            )
            secret.use_irsa = use_irsa
            secret.include = not use_irsa  # if IRSA, exclude from K8s Secret
        else:
            secret.include = _yn("     Add to K8s Secret?", default_yes=True)

    included = [s for s in secrets if s.include]
    irsa_only = [s for s in secrets if s.use_irsa]

    if irsa_only:
        print(f"\n{YELLOW}IRSA/Workload Identity will be configured for:{RESET}")
        for s in irsa_only:
            print(f"  • {s.name}")

    print()
    store_type = _choose(
        "Secrets store: ESO (External Secrets Operator) or K8s Native?",
        choices=["eso", "native"],
        default="eso",
    )

    if store_type == "eso":
        backend_map = {"aws": "AWS Secrets Manager", "azure": "Azure Key Vault", "gcp": "GCP Secret Manager"}
        detected_backend = backend_map.get(cloud, "Unknown")
        print(f"  Auto-detected cloud backend: {BOLD}{detected_backend}{RESET} (from --cloud {cloud})")

    return included, store_type


# ──────────────────────────────────────────────────────────────────────────────
# Output generators
# ──────────────────────────────────────────────────────────────────────────────

def _secret_k8s_name(service: str) -> str:
    return f"{service}-secrets"


def generate_checklist(
    secrets: list[DetectedSecret],
    service: str,
    cloud: str,
    store_type: str,
    output_dir: Path,
) -> Path:
    lines: list[str] = [
        f"# Secrets Checklist — {service}\n",
        "Generated by secrets_manager.py\n",
        f"Cloud: {cloud.upper()} | Store: {'External Secrets Operator (ESO)' if store_type == 'eso' else 'K8s Native Secret'}\n",
        "\n## Secrets Summary\n",
        "| # | Variable | Category | Storage | IRSA/WI | Notes |",
        "|---|----------|----------|---------|---------|-------|",
    ]

    all_secrets = [s for s in secrets]  # includes both included and irsa

    for idx, s in enumerate(all_secrets, start=1):
        storage = "IRSA/Workload Identity" if s.use_irsa else (
            "ESO → Cloud Secrets Manager" if store_type == "eso" else "K8s Secret"
        )
        irsa_flag = "YES" if s.use_irsa else "—"
        note = s.note if s.note else "—"
        lines.append(f"| {idx} | `{s.name}` | {s.category.value} | {storage} | {irsa_flag} | {note} |")

    lines += [
        "\n## Action Items\n",
        "- [ ] Create secrets in cloud secrets manager before deploying",
        "- [ ] Apply ExternalSecret manifests (or native Secret manifests)",
        "- [ ] Configure GitHub Actions secrets (see github-secrets.md)",
        "- [ ] Verify IRSA/Workload Identity annotations on service account",
        "",
    ]

    if cloud == "aws":
        lines += [
            "## AWS IRSA Setup\n",
            "1. Create IAM role with trust policy for the service account",
            "2. Annotate K8s ServiceAccount:",
            "   ```yaml",
            "   annotations:",
            f"     eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT_ID:role/{service}-role",
            "   ```",
            "3. Remove AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY from all configs",
            "",
        ]
    elif cloud == "azure":
        lines += [
            "## Azure Workload Identity Setup\n",
            "1. Create managed identity and federate with AKS OIDC issuer",
            "2. Annotate K8s ServiceAccount:",
            "   ```yaml",
            "   annotations:",
            "     azure.workload.identity/client-id: <CLIENT_ID>",
            "   ```",
            "3. Remove AZURE_CLIENT_SECRET from all configs",
            "",
        ]
    elif cloud == "gcp":
        lines += [
            "## GCP Workload Identity Setup\n",
            "1. Create GCP Service Account and bind to K8s SA",
            "2. Annotate K8s ServiceAccount:",
            "   ```yaml",
            "   annotations:",
            f"     iam.gke.io/gcp-service-account: {service}@PROJECT_ID.iam.gserviceaccount.com",
            "   ```",
            "3. Remove GCP_* / GOOGLE_APPLICATION_CREDENTIALS from all configs",
            "",
        ]

    out = output_dir / "secrets-checklist.md"
    out.write_text("\n".join(lines))
    return out


def generate_external_secret(
    secrets: list[DetectedSecret],
    service: str,
    cloud: str,
    namespace: str,
    output_dir: Path,
) -> Path:
    """Generate an ESO ExternalSecret manifest."""

    if cloud == "aws":
        store_ref = "aws-secrets-manager"
        remote_ref_template = dedent("""\
            - secretKey: {name}
              remoteRef:
                key: {service}/secrets
                property: {name}""")
        store_manifest = dedent("""\
            ---
            # ClusterSecretStore for AWS Secrets Manager
            apiVersion: external-secrets.io/v1beta1
            kind: ClusterSecretStore
            metadata:
              name: aws-secrets-manager
            spec:
              provider:
                aws:
                  service: SecretsManager
                  region: us-east-1   # REPLACE with your region
                  auth:
                    jwt:
                      serviceAccountRef:
                        name: external-secrets-sa
                        namespace: external-secrets
            """)
    elif cloud == "azure":
        store_ref = "azure-keyvault"
        remote_ref_template = dedent("""\
            - secretKey: {name}
              remoteRef:
                key: {name_kebab}""")
        store_manifest = dedent("""\
            ---
            # ClusterSecretStore for Azure Key Vault
            apiVersion: external-secrets.io/v1beta1
            kind: ClusterSecretStore
            metadata:
              name: azure-keyvault
            spec:
              provider:
                azurekv:
                  vaultUrl: https://REPLACE-WITH-VAULT-NAME.vault.azure.net
                  authType: WorkloadIdentity
                  serviceAccountRef:
                    name: external-secrets-sa
                    namespace: external-secrets
            """)
    else:  # gcp
        store_ref = "gcp-secret-manager"
        remote_ref_template = dedent("""\
            - secretKey: {name}
              remoteRef:
                key: {service}-{name_lower}""")
        store_manifest = dedent("""\
            ---
            # ClusterSecretStore for GCP Secret Manager
            apiVersion: external-secrets.io/v1beta1
            kind: ClusterSecretStore
            metadata:
              name: gcp-secret-manager
            spec:
              provider:
                gcpsm:
                  projectID: REPLACE-WITH-GCP-PROJECT-ID
                  auth:
                    workloadIdentity:
                      clusterLocation: us-central1   # REPLACE
                      clusterName: REPLACE-WITH-CLUSTER-NAME
                      serviceAccountRef:
                        name: external-secrets-sa
                        namespace: external-secrets
            """)

    data_entries: list[str] = []
    for s in secrets:
        entry = remote_ref_template.format(
            name=s.name,
            name_kebab=s.name.replace("_", "-").lower(),
            name_lower=s.name.lower(),
            service=service,
        )
        data_entries.append(entry)

    data_block = "\n  ".join(data_entries) if data_entries else "  # no secrets selected"

    manifest = dedent(f"""\
        {store_manifest}
        ---
        # ExternalSecret for {service}
        # Generated by secrets_manager.py
        apiVersion: external-secrets.io/v1beta1
        kind: ExternalSecret
        metadata:
          name: {service}-external-secret
          namespace: {namespace}
        spec:
          refreshInterval: 1h
          secretStoreRef:
            name: {store_ref}
            kind: ClusterSecretStore
          target:
            name: {_secret_k8s_name(service)}
            creationPolicy: Owner
            template:
              type: Opaque
          data:
          {data_block}
        """)

    out = output_dir / "external-secret.yaml"
    out.write_text(manifest)
    return out


def generate_native_secret(
    secrets: list[DetectedSecret],
    service: str,
    namespace: str,
    output_dir: Path,
) -> Path:
    """Generate a K8s native Secret template with placeholder values."""
    data_lines = "\n".join(
        f"  {s.name}: REPLACE_WITH_REAL_VALUE  # {s.category.value}"
        for s in secrets
    )
    manifest = dedent(f"""\
        # K8s Native Secret template for {service}
        # IMPORTANT: Replace all REPLACE_WITH_REAL_VALUE placeholders before applying.
        # Do NOT commit real values to Git — use Sealed Secrets or a secrets manager.
        # Generated by secrets_manager.py
        apiVersion: v1
        kind: Secret
        metadata:
          name: {_secret_k8s_name(service)}
          namespace: {namespace}
          labels:
            app: {service}
type: Opaque
        stringData:
        {data_lines if data_lines else "  # no secrets selected"}
        """)

    out = output_dir / "secret-template.yaml"
    out.write_text(manifest)
    return out


def generate_github_secrets(
    secrets: list[DetectedSecret],
    service: str,
    cloud: str,
    store_type: str,
    output_dir: Path,
) -> Path:
    lines: list[str] = [
        f"# GitHub Actions Secrets — {service}\n",
        "Add these secrets to your GitHub repository at:\n",
        "  Settings → Secrets and variables → Actions → New repository secret\n",
        "\n## Required Secrets\n",
        "| Secret Name | Description | Where to get it |",
        "|-------------|-------------|-----------------|",
    ]

    # Cloud-specific OIDC secrets
    if cloud == "aws":
        lines += [
            "| `AWS_ACCOUNT_ID` | Your AWS account ID | AWS Console → top-right menu |",
            "| `AWS_REGION` | Target AWS region (e.g. us-east-1) | Your infra config |",
            "| `ECR_REPOSITORY` | ECR repo name | AWS ECR Console |",
            "| `EKS_CLUSTER_NAME` | EKS cluster name | AWS EKS Console |",
        ]
        if store_type == "eso":
            lines.append("| `SECRETS_MANAGER_ARN_PREFIX` | ARN prefix for secrets | AWS Secrets Manager |")
    elif cloud == "azure":
        lines += [
            "| `AZURE_CLIENT_ID` | App registration client ID (OIDC) | Azure AD → App registrations |",
            "| `AZURE_TENANT_ID` | Azure tenant ID | Azure AD → Overview |",
            "| `AZURE_SUBSCRIPTION_ID` | Subscription ID | Azure Portal → Subscriptions |",
            "| `ACR_REGISTRY` | Azure Container Registry URL | ACR → Overview |",
            "| `AKS_CLUSTER_NAME` | AKS cluster name | Azure Portal |",
            "| `AKS_RESOURCE_GROUP` | Resource group of AKS | Azure Portal |",
        ]
    elif cloud == "gcp":
        lines += [
            "| `GCP_PROJECT_ID` | GCP project ID | GCP Console → Project Info |",
            "| `GCP_WORKLOAD_IDENTITY_PROVIDER` | WIF provider resource name | GCP IAM → Workload Identity |",
            "| `GCP_SERVICE_ACCOUNT` | SA email for OIDC | GCP IAM → Service Accounts |",
            "| `GKE_CLUSTER_NAME` | GKE cluster name | GCP Console → Kubernetes Engine |",
            "| `GKE_CLUSTER_ZONE` | Cluster zone/region | GCP Console → Kubernetes Engine |",
            "| `ARTIFACT_REGISTRY_REPO` | Artifact Registry repo | GCP Console → Artifact Registry |",
        ]

    # App-specific secrets that must be injected at deploy time
    non_cloud = [s for s in secrets if s.category != SecretCategory.CLOUD_CRED]
    if non_cloud and store_type == "native":
        lines.append("\n## Application Secrets (only needed for K8s native mode)\n")
        lines += [
            "| Secret Name | Description | Category |",
            "|-------------|-------------|----------|",
        ]
        for s in non_cloud:
            lines.append(f"| `{s.name}` | {s.name.replace('_', ' ').title()} | {s.category.value} |")

    lines += [
        "\n## Variables (non-sensitive)\n",
        "| Variable Name | Example Value | Description |",
        "|---------------|---------------|-------------|",
        f"| `SERVICE_NAME` | `{service}` | Service slug |",
        f"| `NAMESPACE` | `{service}` | K8s namespace |",
        "",
        "## Notes",
        "- Use OIDC federation where possible — no long-lived static credentials",
        "- Rotate secrets regularly; ESO will auto-sync from cloud secrets manager",
        "- Never put secret values in GitHub Actions `env:` block as plain text",
        "",
    ]

    out = output_dir / "github-secrets.md"
    out.write_text("\n".join(lines))
    return out


def update_helm_values(
    secrets: list[DetectedSecret],
    service: str,
    store_type: str,
    helm_dir: Optional[Path],
) -> Optional[Path]:
    """Append a secrets section to helm/values.yaml if it exists."""
    if not helm_dir or not helm_dir.is_dir():
        return None

    values_path = helm_dir / "values.yaml"
    if not values_path.exists():
        return None

    existing = values_path.read_text()
    if "# secrets_manager_generated" in existing:
        return values_path  # already patched

    secret_names = "\n".join(f"  - {s.name}" for s in secrets)
    addon = dedent(f"""

        # secrets_manager_generated — do not edit this block manually
        secrets:
          enabled: true
          store: {store_type}
          secretName: {_secret_k8s_name(service)}
          keys:
        {secret_names if secret_names else "  []"}
        """)

    values_path.write_text(existing + addon)
    return values_path


# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Scan a repository for secrets and generate management manifests.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("--repo-path", required=True, help="Path to the repository root")
    parser.add_argument("--service", required=True, help="Service name slug (e.g. payment-api)")
    parser.add_argument("--cloud", required=True, choices=["aws", "azure", "gcp"],
                        help="Cloud provider")
    parser.add_argument("--output-dir", default=None,
                        help="Directory to write manifests (default: <repo-path>/secrets)")
    parser.add_argument("--namespace", default=None,
                        help="K8s namespace (default: <service>)")
    parser.add_argument("--helm-dir", default=None,
                        help="Helm chart directory to update values.yaml")
    parser.add_argument("--non-interactive", action="store_true",
                        help="Skip all prompts; auto-include all non-cloud-cred secrets, use ESO")
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    repo_path = Path(args.repo_path).resolve()
    if not repo_path.is_dir():
        print(f"ERROR: --repo-path '{repo_path}' does not exist or is not a directory.", file=sys.stderr)
        return 1

    namespace = args.namespace or args.service
    output_dir = Path(args.output_dir).resolve() if args.output_dir else repo_path / "secrets"
    output_dir.mkdir(parents=True, exist_ok=True)

    helm_dir = Path(args.helm_dir).resolve() if args.helm_dir else None

    # ── Detect ────────────────────────────────────────────────────────────────
    print(f"\nScanning {repo_path} for secrets...")
    secrets = detect_secrets(repo_path)

    if not secrets:
        print("No secrets detected. Exiting.")
        return 0

    print(f"Found {len(secrets)} unique secret variable(s) across scanned files.")

    # ── Confirm ───────────────────────────────────────────────────────────────
    if args.non_interactive:
        store_type = "eso"
        for s in secrets:
            if s.category == SecretCategory.CLOUD_CRED:
                s.use_irsa = True
                s.include = False
            else:
                s.include = True
        included = [s for s in secrets if s.include]
        print(f"Non-interactive mode: including {len(included)} secrets, using ESO.")
    else:
        included, store_type = run_interactive(secrets, args.service, args.cloud)

    # ── Generate outputs ──────────────────────────────────────────────────────
    generated: list[str] = []

    checklist_path = generate_checklist(secrets, args.service, args.cloud, store_type, output_dir)
    generated.append(str(checklist_path))

    if included:
        if store_type == "eso":
            eso_path = generate_external_secret(included, args.service, args.cloud, namespace, output_dir)
            generated.append(str(eso_path))
        else:
            native_path = generate_native_secret(included, args.service, namespace, output_dir)
            generated.append(str(native_path))

    gh_path = generate_github_secrets(secrets, args.service, args.cloud, store_type, output_dir)
    generated.append(str(gh_path))

    helm_path = update_helm_values(included, args.service, store_type, helm_dir)
    if helm_path:
        generated.append(str(helm_path))

    # ── Summary ───────────────────────────────────────────────────────────────
    BOLD  = "\033[1m"
    GREEN = "\033[32m"
    RESET = "\033[0m"

    print(f"\n{BOLD}{'━' * 52}{RESET}")
    print(f"{BOLD}  Secrets Manager — Output{RESET}")
    print(f"{BOLD}{'━' * 52}{RESET}")
    print(f"  Service  : {args.service}")
    print(f"  Cloud    : {args.cloud.upper()}")
    print(f"  Store    : {'External Secrets Operator (ESO)' if store_type == 'eso' else 'K8s Native Secret'}")
    print(f"  Secrets  : {len(included)} included, {len(secrets) - len(included)} skipped/IRSA")
    print(f"\n{GREEN}Generated files:{RESET}")
    for p in generated:
        print(f"  • {p}")
    print(f"\n{BOLD}Next steps:{RESET}")
    print("  1. Review secrets-checklist.md and action each item")
    print("  2. Populate secrets in your cloud secrets manager")
    if store_type == "eso":
        print("  3. Install ESO: helm repo add external-secrets https://charts.external-secrets.io")
        print("     helm install external-secrets external-secrets/external-secrets -n external-secrets --create-namespace")
        print("  4. Apply external-secret.yaml: kubectl apply -f secrets/external-secret.yaml")
    else:
        print("  3. Fill in real values in secret-template.yaml (never commit real values!)")
        print("  4. kubectl apply -f secrets/secret-template.yaml")
    print("  5. Add GitHub Actions secrets listed in github-secrets.md")
    print(f"{BOLD}{'━' * 52}{RESET}\n")

    return 0


if __name__ == "__main__":
    sys.exit(main())
