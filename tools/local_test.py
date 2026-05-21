#!/usr/bin/env python3
"""Validate Terraform against local cloud emulators (LocalStack/Azurite/GCP emulators)."""

import argparse
import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

TOOLS_DIR = Path(__file__).parent
EMULATORS_DIR = TOOLS_DIR / "emulators"

EMULATOR_COMPOSE = {
    "aws": "localstack.yml",
    "azure": "azurite.yml",
    "gcp": "gcp-emulators.yml",
}

EMULATOR_PORTS = {
    "aws": 4566,   # LocalStack
    "azure": 10000, # Azurite blob
    "gcp": 8085,   # Firestore emulator
}

EMULATOR_NAMES = {
    "aws": "LocalStack",
    "azure": "Azurite",
    "gcp": "GCP emulators",
}


def wait_for_port(port: int, timeout: int = 30) -> bool:
    import socket
    start = time.time()
    while time.time() - start < timeout:
        try:
            with socket.create_connection(("localhost", port), timeout=1):
                return True
        except (OSError, ConnectionRefusedError):
            time.sleep(1)
    return False


def start_emulator(cloud: str) -> tuple[bool, str]:
    compose_file = EMULATORS_DIR / EMULATOR_COMPOSE[cloud]
    if not compose_file.exists():
        return False, f"Emulator compose file not found: {compose_file}"

    dc = "docker compose" if shutil.which("docker") else None
    if not dc:
        return False, "Docker not found"

    result = subprocess.run(
        f"{dc} -f {compose_file} up -d",
        shell=True, capture_output=True, text=True
    )
    if result.returncode != 0:
        return False, result.stderr

    port = EMULATOR_PORTS[cloud]
    name = EMULATOR_NAMES[cloud]
    print(f"[local_test] Waiting for {name} on :{port}...")
    if not wait_for_port(port, timeout=45):
        return False, f"{name} did not start on :{port} after 45s"

    print(f"[local_test] {name} ready.")
    return True, ""


def run_terraform_plan(tf_dir: Path, cloud: str, env_vars: dict) -> tuple[bool, str]:
    if not shutil.which("terraform"):
        return False, "terraform not in PATH"

    env = os.environ.copy()
    env.update(env_vars)

    # terraform init
    init = subprocess.run(
        ["terraform", "init", "-input=false", "-reconfigure"],
        cwd=tf_dir, capture_output=True, text=True, env=env, timeout=120
    )
    if init.returncode != 0:
        return False, f"terraform init failed:\n{init.stderr}"

    # terraform plan
    plan = subprocess.run(
        ["terraform", "plan", "-input=false", "-out=tfplan.local"],
        cwd=tf_dir, capture_output=True, text=True, env=env, timeout=120
    )
    output = plan.stdout + plan.stderr
    return plan.returncode == 0, output


def get_emulator_env(cloud: str) -> dict:
    """Return environment variables to point Terraform at local emulators."""
    if cloud == "aws":
        return {
            "AWS_ACCESS_KEY_ID": "test",
            "AWS_SECRET_ACCESS_KEY": "test",
            "AWS_DEFAULT_REGION": "us-east-1",
            "TF_VAR_localstack": "true",
            # LocalStack endpoint overrides
            "AWS_ENDPOINT_URL": "http://localhost:4566",
        }
    elif cloud == "azure":
        return {
            "ARM_USE_AZUREAD": "false",
            "ARM_CLIENT_ID": "test",
            "ARM_CLIENT_SECRET": "test",
            "ARM_SUBSCRIPTION_ID": "00000000-0000-0000-0000-000000000000",
            "ARM_TENANT_ID": "00000000-0000-0000-0000-000000000000",
            "AZURE_STORAGE_CONNECTION_STRING": "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;",
        }
    elif cloud == "gcp":
        return {
            "GOOGLE_CLOUD_PROJECT": "local-test",
            "FIRESTORE_EMULATOR_HOST": "localhost:8080",
            "PUBSUB_EMULATOR_HOST": "localhost:8085",
            "GOOGLE_APPLICATION_CREDENTIALS": "",
            "TF_VAR_project_id": "local-test",
        }
    return {}


def main():
    parser = argparse.ArgumentParser(description="Test Terraform locally with cloud emulators")
    parser.add_argument("--cloud", required=True, choices=["aws", "azure", "gcp"])
    parser.add_argument("--terraform-dir", required=True, help="Path to Terraform files")
    parser.add_argument("--service", required=True, help="Service name")
    parser.add_argument("--skip-emulator", action="store_true", help="Skip starting emulator (assume running)")
    parser.add_argument("--stop-after", action="store_true", help="Stop emulator after test")
    args = parser.parse_args()

    tf_dir = Path(args.terraform_dir).resolve()
    if not tf_dir.exists():
        print(json.dumps({"success": False, "error": f"Terraform dir not found: {tf_dir}"}))
        sys.exit(1)

    emulator_started = False

    if not args.skip_emulator:
        print(f"[local_test] Starting {EMULATOR_NAMES[args.cloud]}...")
        ok, err = start_emulator(args.cloud)
        if not ok:
            print(json.dumps({
                "success": False,
                "error": f"Failed to start emulator: {err}",
                "hint": f"Make sure Docker is running. Compose file: tools/emulators/{EMULATOR_COMPOSE[args.cloud]}",
            }))
            sys.exit(1)
        emulator_started = True

    print(f"[local_test] Running terraform plan against {EMULATOR_NAMES[args.cloud]}...")
    env_vars = get_emulator_env(args.cloud)
    ok, plan_output = run_terraform_plan(tf_dir, args.cloud, env_vars)

    if args.stop_after and emulator_started:
        compose_file = EMULATORS_DIR / EMULATOR_COMPOSE[args.cloud]
        subprocess.run(f"docker compose -f {compose_file} down", shell=True, capture_output=True)
        print(f"[local_test] Stopped {EMULATOR_NAMES[args.cloud]}.")

    status = "passed" if ok else "FAILED"
    print(f"[local_test] Terraform plan {status}")
    if not ok:
        print(plan_output[-3000:])

    print(json.dumps({
        "success": ok,
        "emulator": EMULATOR_NAMES[args.cloud],
        "emulator_url": f"http://localhost:{EMULATOR_PORTS[args.cloud]}",
        "plan_output": plan_output[-2000:],
        "terraform_dir": str(tf_dir),
    }))


if __name__ == "__main__":
    main()
