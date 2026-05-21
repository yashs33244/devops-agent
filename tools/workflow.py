#!/usr/bin/env python3
"""Full DevOps pipeline orchestrator — runs all steps in sequence."""

import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

TOOLS_DIR = Path(__file__).parent
ROOT_DIR = TOOLS_DIR.parent
WORKSPACE_DIR = ROOT_DIR / "workspace"


def run_tool(script: str, args: list[str]) -> dict:
    result = subprocess.run(
        [sys.executable, str(TOOLS_DIR / script)] + args,
        capture_output=False,  # let output stream to terminal
        text=True,
    )
    return {"exit_code": result.returncode, "success": result.returncode == 0}


def clone_repo(url: str, service_name: str) -> Path:
    dest = WORKSPACE_DIR / service_name
    if dest.exists():
        print(f"[workflow] {dest} already exists, skipping clone.")
        return dest
    WORKSPACE_DIR.mkdir(parents=True, exist_ok=True)
    result = subprocess.run(["git", "clone", url, str(dest)], capture_output=True, text=True)
    if result.returncode != 0:
        print(f"[workflow] git clone failed: {result.stderr}")
        sys.exit(1)
    print(f"[workflow] Cloned {url} → {dest}")
    return dest


def print_separator(title: str):
    width = 60
    print(f"\n{'─' * width}")
    print(f"  {title}")
    print(f"{'─' * width}")


def print_summary(service: str, cloud: str, repo_path: Path, results: dict):
    tf_dir = repo_path / "terraform"
    print(f"""
╔══════════════════════════════════════════════════════════╗
║              opscore DevOps Agent — Summary              ║
╚══════════════════════════════════════════════════════════╝

Service:    {service}
Cloud:      {cloud.upper()}
Repo:       {repo_path}

Files created:
  Dockerfile:   {repo_path}/Dockerfile
  Terraform:    {tf_dir}/
  CI workflow:  {repo_path}/.github/workflows/ci.yml
  CD workflow:  {repo_path}/.github/workflows/cd.yml
  KEDA scaler:  {tf_dir}/keda/{service}-http-scaler.yaml  (if requested)

Next steps:
  1. Review generated files above
  2. Push to GitHub: git push origin main
  3. Add secrets listed in {repo_path}/.github/SECRETS.md
  4. For prod deploy: cd {tf_dir} && terraform init && terraform apply
  5. Apply KEDA: kubectl apply -f {tf_dir}/keda/
""")


def main():
    parser = argparse.ArgumentParser(
        description="Run the full DevOps pipeline: clone → dockerize → terraform → ci/cd → local test"
    )
    parser.add_argument("--repo", required=True, help="GitHub repo URL or local path")
    parser.add_argument("--service", required=True, help="Service name (slug, e.g. payment-api)")
    parser.add_argument("--use-case", default="web_app",
                        choices=["web_app", "microservice", "batch_job", "scheduled_task", "data_pipeline"])
    parser.add_argument("--cloud", required=True, choices=["aws", "azure", "gcp"])
    parser.add_argument("--region", help="Cloud region")
    parser.add_argument("--env", default="dev", choices=["dev", "staging", "prod"])
    parser.add_argument("--with-keda", action="store_true", help="Apply car-painter scale-to-zero")
    parser.add_argument("--skip-local-test", action="store_true", help="Skip local emulator test")
    parser.add_argument("--skip-build", action="store_true", help="Skip docker build")
    parser.add_argument("--skip-monitoring", action="store_true", help="Skip Prometheus + Grafana stack")
    args = parser.parse_args()

    results = {}

    # ── Step 1: Clone ─────────────────────────────────────────────────────
    print_separator("Step 1: Clone Repository")
    is_url = args.repo.startswith("http://") or args.repo.startswith("git@")
    if is_url:
        repo_path = clone_repo(args.repo, args.service)
    else:
        repo_path = Path(args.repo).resolve()
        if not repo_path.exists():
            print(f"[workflow] Local path not found: {repo_path}")
            sys.exit(1)
        print(f"[workflow] Using local path: {repo_path}")

    # ── Step 2: Dockerize ─────────────────────────────────────────────────
    print_separator("Step 2: Dockerize")
    dockerize_args = ["--path", str(repo_path), "--service", args.service]
    if args.skip_build:
        dockerize_args.append("--no-build")
    r = run_tool("dockerize.py", dockerize_args)
    results["dockerize"] = r
    if not r["success"]:
        print("[workflow] Dockerize failed. Check output above.")
        sys.exit(1)

    # ── Step 3: Terraform ─────────────────────────────────────────────────
    print_separator("Step 3: Generate Terraform")
    tf_args = [
        "--cloud", args.cloud,
        "--service", args.service,
        "--use-case", args.use_case,
        "--env", args.env,
        "--output-dir", str(repo_path / "terraform"),
    ]
    if args.region:
        tf_args += ["--region", args.region]
    r = run_tool("terraform_gen.py", tf_args)
    results["terraform"] = r

    # ── Step 4: CI/CD ─────────────────────────────────────────────────────
    print_separator("Step 4: Setup CI/CD")
    cicd_args = [
        "--repo-path", str(repo_path),
        "--cloud", args.cloud,
        "--service", args.service,
        "--env", args.env,
    ]
    r = run_tool("cicd_setup.py", cicd_args)
    results["cicd"] = r

    # ── Step 5: Local Test ────────────────────────────────────────────────
    if not args.skip_local_test:
        print_separator("Step 5: Local Emulator Test")
        local_test_args = [
            "--cloud", args.cloud,
            "--terraform-dir", str(repo_path / "terraform"),
            "--service", args.service,
        ]
        r = run_tool("local_test.py", local_test_args)
        results["local_test"] = r

    # ── Step 6: Cost Optimize (KEDA) ──────────────────────────────────────
    if args.with_keda:
        print_separator("Step 6: Car-Painter Scale-to-Zero (KEDA)")
        platform_map = {"aws": "eks", "azure": "aks", "gcp": "gke"}
        keda_args = [
            "--terraform-dir", str(repo_path / "terraform"),
            "--platform", platform_map[args.cloud],
            "--service", args.service,
        ]
        r = run_tool("cost_optimize.py", keda_args)
        results["keda"] = r

    # ── Step 7: Monitoring Setup ──────────────────────────────────────────
    if not args.skip_monitoring:
        print_separator("Step 7: Monitoring (Prometheus + Grafana)")
        print(f"[workflow] Starting local Prometheus + Grafana stack...")
        monitoring_compose = ROOT_DIR / "templates" / "monitoring" / "docker-compose.monitoring.yml"
        result = subprocess.run(
            f"docker compose -f {monitoring_compose} up -d",
            shell=True, capture_output=True, text=True
        )
        if result.returncode == 0:
            print("[workflow] Prometheus: http://localhost:9090")
            print("[workflow] Grafana:    http://localhost:3001 (admin/devops-agent)")
        else:
            print(f"[workflow] Monitoring stack failed: {result.stderr[:500]}")

    # ── Summary ───────────────────────────────────────────────────────────
    print_summary(args.service, args.cloud, repo_path, results)

    all_ok = all(v.get("success", True) for v in results.values())
    sys.exit(0 if all_ok else 1)


if __name__ == "__main__":
    main()
