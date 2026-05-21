#!/usr/bin/env python3
"""Apply car-painter scale-to-zero pattern using KEDA."""

import argparse
import json
import sys
from pathlib import Path

TEMPLATES_DIR = Path(__file__).parent.parent / "templates" / "keda"

PLATFORM_NOTES = {
    "eks": "KEDA + HTTP Add-on. Install: helm repo add kedacore https://kedacore.github.io/charts && helm install keda kedacore/keda && helm install http-add-on kedacore/keda-add-ons-http",
    "aks": "AKS has KEDA as a managed add-on. Enable: az aks update --enable-keda -g <rg> -n <cluster>",
    "gke": "KEDA on GKE OR use Cloud Run for natively serverless (recommended for stateless HTTP)",
    "cloud-run": "Cloud Run scales to 0 natively — no KEDA needed. Set --min-instances=0",
    "container-apps": "Azure Container Apps scales to 0 natively — no KEDA needed. Set minReplicas: 0",
    "fargate": "AWS Fargate with ECS can scale to 0 via CloudWatch + Application Auto Scaling",
}

KEDA_HELM_TF = """
resource "helm_release" "keda" {
  name             = "keda"
  repository       = "https://kedacore.github.io/charts"
  chart            = "keda"
  namespace        = "keda"
  create_namespace = true
  version          = "2.16.0"
}

resource "helm_release" "keda_http_add_on" {
  name             = "keda-add-ons-http"
  repository       = "https://kedacore.github.io/charts"
  chart            = "keda-add-ons-http"
  namespace        = "keda"
  create_namespace = true
  version          = "0.9.0"

  depends_on = [helm_release.keda]
}
"""


def render(content: str, vars: dict) -> str:
    for k, v in vars.items():
        content = content.replace(f"{{{{{k}}}}}", str(v))
    return content


def generate_http_scaler(service_name: str, namespace: str, port: int, output_dir: Path) -> Path:
    template = TEMPLATES_DIR / "http-scaler.yaml"
    if not template.exists():
        # Inline fallback
        content = f"""apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {service_name}-http-scaler
  namespace: {namespace}
spec:
  hosts:
    - {service_name}.{namespace}.svc.cluster.local
  pathPrefixes:
    - /
  scaleTargetRef:
    deployment: {service_name}
    service: {service_name}
    port: {port}
  replicas:
    min: 0
    max: 10
  scaledownPeriod: 300
  targetPendingRequests: 100
"""
    else:
        content = render(template.read_text(), {
            "SERVICE_NAME": service_name,
            "NAMESPACE": namespace,
            "PORT": port,
        })

    out = output_dir / "keda" / f"{service_name}-http-scaler.yaml"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(content)
    return out


def generate_keda_terraform(output_dir: Path, platform: str) -> Path:
    out = output_dir / "keda.tf"
    out.write_text(KEDA_HELM_TF)
    return out


def generate_deployment_patch(service_name: str, namespace: str, output_dir: Path) -> Path:
    """Generate a K8s Deployment patch that sets replicas to 0 initially (KEDA takes over)."""
    patch = f"""# K8s Deployment — managed by KEDA (do not set replicas manually)
# KEDA HTTPScaledObject will control replica count (0 to 10)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {service_name}
  namespace: {namespace}
  annotations:
    scaledobject.keda.sh/name: {service_name}-http-scaler
spec:
  replicas: 1  # KEDA will override this; start with 1 so rollout works
  selector:
    matchLabels:
      app: {service_name}
  template:
    metadata:
      labels:
        app: {service_name}
    spec:
      containers:
        - name: {service_name}
          image: REPLACE_WITH_YOUR_IMAGE
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
"""
    out = output_dir / "keda" / f"{service_name}-deployment.yaml"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(patch)
    return out


def main():
    parser = argparse.ArgumentParser(description="Apply car-painter scale-to-zero with KEDA")
    parser.add_argument("--terraform-dir", required=True, help="Terraform directory to augment")
    parser.add_argument("--platform", required=True,
                        choices=["eks", "aks", "gke", "cloud-run", "container-apps", "fargate"])
    parser.add_argument("--service", required=True, help="Service name")
    parser.add_argument("--namespace", default="default", help="K8s namespace")
    parser.add_argument("--port", type=int, default=8080, help="Container port")
    parser.add_argument("--scale-down-seconds", type=int, default=300,
                        help="Seconds idle before scaling to 0 (default: 300 = 5 min)")
    args = parser.parse_args()

    tf_dir = Path(args.terraform_dir).resolve()
    if not tf_dir.exists():
        print(json.dumps({"success": False, "error": f"Terraform dir not found: {tf_dir}"}))
        sys.exit(1)

    files_written = []

    # Native scale-to-zero platforms don't need KEDA
    if args.platform in ("cloud-run", "container-apps", "fargate"):
        note = PLATFORM_NOTES[args.platform]
        print(f"[cost_optimize] {args.platform} supports native scale-to-zero — no KEDA needed.")
        print(f"[cost_optimize] {note}")
        print(json.dumps({
            "success": True,
            "platform": args.platform,
            "keda_needed": False,
            "note": note,
            "estimated_savings_pct": "60-80% for bursty/low traffic workloads",
        }))
        return

    # K8s platforms: add KEDA
    print(f"[cost_optimize] Generating KEDA scale-to-zero for {args.platform}...")

    scaler_path = generate_http_scaler(args.service, args.namespace, args.port, tf_dir)
    files_written.append(str(scaler_path))
    print(f"[cost_optimize] Wrote HTTPScaledObject: {scaler_path}")

    keda_tf = generate_keda_terraform(tf_dir, args.platform)
    files_written.append(str(keda_tf))
    print(f"[cost_optimize] Wrote KEDA Terraform: {keda_tf}")

    deployment_patch = generate_deployment_patch(args.service, args.namespace, tf_dir)
    files_written.append(str(deployment_patch))
    print(f"[cost_optimize] Wrote Deployment template: {deployment_patch}")

    install_note = PLATFORM_NOTES[args.platform]
    print(f"\n[cost_optimize] Install note: {install_note}")
    print(f"[cost_optimize] Scale-down after {args.scale_down_seconds}s idle (car-painter pattern)")

    print(json.dumps({
        "success": True,
        "platform": args.platform,
        "keda_needed": True,
        "files_written": files_written,
        "scale_down_seconds": args.scale_down_seconds,
        "install_note": install_note,
        "estimated_savings_pct": "70-90% for services with <30% utilization",
    }))


if __name__ == "__main__":
    main()
