#!/usr/bin/env python3
"""
helm_gen.py — Helm chart generator for the DevOps agent.

Usage:
    python helm_gen.py --service myapp --cloud aws --port 8080 --namespace production

Outputs a JSON result:
    {"success": true, "helm_dir": "/path/to/helm", "lint_passed": true, "test_passed": true}
"""
from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

TEMPLATE_DIR = Path(__file__).resolve().parent.parent / "templates" / "helm" / "chart"

CLOUD_SA_ANNOTATIONS: dict[str, dict[str, str]] = {
    "aws": {
        "eks.amazonaws.com/role-arn": "SET_BY_TERRAFORM",
    },
    "azure": {
        "azure.workload.identity/client-id": "SET_BY_TERRAFORM",
    },
    "gcp": {
        "iam.gke.io/gcp-service-account": "SET_BY_TERRAFORM",
    },
}

# Files where template variable substitution is performed
TEMPLATE_EXTENSIONS = {".yaml", ".yml", ".tpl", ".json", ".md", ".txt"}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _replace_in_file(path: Path, replacements: dict[str, str]) -> None:
    """Perform all {{KEY}} replacements in a single file."""
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return  # skip binary files

    original = text
    for key, value in replacements.items():
        text = text.replace(f"{{{{{key}}}}}", value)

    if text != original:
        path.write_text(text, encoding="utf-8")


def _replace_in_tree(root: Path, replacements: dict[str, str]) -> None:
    """Recursively replace template variables in all text files under root."""
    for file_path in root.rglob("*"):
        if file_path.is_file() and file_path.suffix in TEMPLATE_EXTENSIONS:
            _replace_in_file(file_path, replacements)


def _inject_sa_annotations(helm_dir: Path, cloud: str) -> None:
    """
    Patch values.yaml to add the cloud-specific ServiceAccount annotation
    so Workload Identity / IRSA works out of the box.
    """
    annotations = CLOUD_SA_ANNOTATIONS.get(cloud)
    if not annotations:
        return

    values_path = helm_dir / "values.yaml"
    if not values_path.exists():
        return

    text = values_path.read_text(encoding="utf-8")

    # Build annotation block (indented 2 spaces, under serviceAccount.annotations)
    annotation_lines = "\n".join(
        f"  {k}: \"{v}\"" for k, v in annotations.items()
    )

    # Replace the empty annotations block under serviceAccount
    old_block = "serviceAccount:\n  create: true\n  # If empty, a name is generated using the fullname template\n  name: \"\"\n  annotations: {}"
    new_block = (
        f"serviceAccount:\n"
        f"  create: true\n"
        f"  # If empty, a name is generated using the fullname template\n"
        f"  name: \"\"\n"
        f"  annotations:\n"
        f"{annotation_lines}"
    )

    if old_block in text:
        text = text.replace(old_block, new_block)
        values_path.write_text(text, encoding="utf-8")
    else:
        # Fallback: append a comment if the exact block is not found (template was customised)
        print(
            f"[helm_gen] WARNING: Could not locate serviceAccount.annotations block to patch. "
            f"Add the following manually to values.yaml:\n{annotation_lines}",
            file=sys.stderr,
        )


def _set_external_secrets_backend(helm_dir: Path, cloud: str) -> None:
    """Set the externalSecrets.cloudBackend value to match the chosen cloud."""
    values_path = helm_dir / "values.yaml"
    if not values_path.exists():
        return

    text = values_path.read_text(encoding="utf-8")
    text = text.replace(
        '  secretPathPrefix: ""',
        f'  cloudBackend: "{cloud}"\n  secretPathPrefix: ""',
    )
    values_path.write_text(text, encoding="utf-8")


def _run(cmd: list[str], cwd: Path | None = None) -> tuple[int, str, str]:
    """Run a subprocess and return (returncode, stdout, stderr)."""
    result = subprocess.run(
        cmd,
        cwd=cwd,
        capture_output=True,
        text=True,
    )
    return result.returncode, result.stdout, result.stderr


# ---------------------------------------------------------------------------
# Core generator
# ---------------------------------------------------------------------------

def generate(
    service: str,
    namespace: str,
    cloud: str,
    port: int,
    output_dir: Path,
    app_version: str = "0.1.0",
) -> dict:
    """
    Copy the template chart to output_dir, substitute variables,
    optionally run helm lint + helm unittest.

    Returns a result dict.
    """
    result: dict = {
        "success": False,
        "helm_dir": str(output_dir),
        "lint_passed": None,
        "test_passed": None,
        "errors": [],
    }

    # ── 1. Validate inputs ─────────────────────────────────────────────────
    if not TEMPLATE_DIR.exists():
        result["errors"].append(f"Template directory not found: {TEMPLATE_DIR}")
        return result

    if cloud not in ("aws", "azure", "gcp", ""):
        result["errors"].append(f"Unknown cloud '{cloud}'. Choose aws, azure, or gcp.")
        return result

    # ── 2. Copy template tree ──────────────────────────────────────────────
    if output_dir.exists():
        print(f"[helm_gen] Removing existing output dir: {output_dir}")
        shutil.rmtree(output_dir)

    print(f"[helm_gen] Copying template chart to: {output_dir}")
    shutil.copytree(TEMPLATE_DIR, output_dir)

    # ── 3. Variable substitution ───────────────────────────────────────────
    replacements = {
        "SERVICE_NAME": service,
        "APP_VERSION": app_version,
        "PORT": str(port),
        "NAMESPACE": namespace,
        # IMAGE_NAME and REGISTRY are intentionally left as placeholders
        # so callers can fill them in via --set at helm upgrade time.
        "IMAGE_NAME": service,
        "REGISTRY": "REGISTRY_PLACEHOLDER",
    }
    print(f"[helm_gen] Substituting template variables: {list(replacements.keys())}")
    _replace_in_tree(output_dir, replacements)

    # ── 4. Cloud-specific patches ──────────────────────────────────────────
    if cloud:
        print(f"[helm_gen] Applying cloud={cloud} ServiceAccount annotations")
        _inject_sa_annotations(output_dir, cloud)
        _set_external_secrets_backend(output_dir, cloud)

    # ── 5. helm lint ───────────────────────────────────────────────────────
    helm_bin = shutil.which("helm")
    if helm_bin:
        print(f"[helm_gen] Running: helm lint {output_dir}")
        rc, stdout, stderr = _run([helm_bin, "lint", str(output_dir), "--strict"])
        lint_passed = rc == 0
        result["lint_passed"] = lint_passed
        if stdout:
            print(stdout)
        if stderr:
            print(stderr, file=sys.stderr)
        if not lint_passed:
            result["errors"].append(f"helm lint failed (rc={rc})")
    else:
        print("[helm_gen] WARNING: 'helm' not found in PATH — skipping lint")
        result["lint_passed"] = None

    # ── 6. helm unittest ───────────────────────────────────────────────────
    unittest_available = False
    if helm_bin:
        rc_check, stdout_check, _ = _run(
            [helm_bin, "plugin", "list"],
        )
        unittest_available = "unittest" in stdout_check

    if unittest_available:
        print(f"[helm_gen] Running: helm unittest {output_dir}")
        rc, stdout, stderr = _run(
            [helm_bin, "unittest", str(output_dir), "--color"],
        )
        test_passed = rc == 0
        result["test_passed"] = test_passed
        if stdout:
            print(stdout)
        if stderr:
            print(stderr, file=sys.stderr)
        if not test_passed:
            result["errors"].append(f"helm unittest failed (rc={rc})")
    else:
        print("[helm_gen] INFO: helm-unittest plugin not installed — skipping unit tests")
        result["test_passed"] = None

    # ── 7. Finish ──────────────────────────────────────────────────────────
    lint_ok = result["lint_passed"] is not False   # None (skipped) counts as ok
    test_ok = result["test_passed"] is not False
    result["success"] = lint_ok and test_ok and not result["errors"]

    return result


# ---------------------------------------------------------------------------
# CLI entry-point
# ---------------------------------------------------------------------------

def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate a production-ready Helm chart for a service.",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "--service",
        required=True,
        help="Service name (used as chart name, deployment name, label selector, etc.)",
    )
    parser.add_argument(
        "--namespace",
        default="default",
        help="Kubernetes namespace for the generated values.yaml",
    )
    parser.add_argument(
        "--cloud",
        choices=["aws", "azure", "gcp", ""],
        default="",
        help="Cloud provider — injects Workload Identity / IRSA annotations into ServiceAccount",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8080,
        help="Container port the service listens on",
    )
    parser.add_argument(
        "--output-dir",
        default=None,
        help="Output directory for the generated chart (default: workspace/<service>/helm)",
    )
    parser.add_argument(
        "--app-version",
        default="0.1.0",
        help="appVersion written into Chart.yaml",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Print result as JSON (useful for programmatic callers)",
    )
    return parser.parse_args()


def main() -> None:
    args = _parse_args()

    if args.output_dir:
        output_dir = Path(args.output_dir).resolve()
    else:
        # Default: workspace/<service>/helm relative to repo root
        repo_root = Path(__file__).resolve().parent.parent
        output_dir = repo_root / "workspace" / args.service / "helm"

    result = generate(
        service=args.service,
        namespace=args.namespace,
        cloud=args.cloud,
        port=args.port,
        output_dir=output_dir,
        app_version=args.app_version,
    )

    if args.json_output:
        print(json.dumps(result, indent=2))
    else:
        status = "SUCCESS" if result["success"] else "FAILED"
        print(f"\n[helm_gen] {status}")
        print(f"  helm_dir    : {result['helm_dir']}")
        print(f"  lint_passed : {result['lint_passed']}")
        print(f"  test_passed : {result['test_passed']}")
        if result["errors"]:
            print("  errors:")
            for err in result["errors"]:
                print(f"    - {err}")

    sys.exit(0 if result["success"] else 1)


if __name__ == "__main__":
    main()
