# templates/helm/

Production Helm chart template with hardened security contexts, resource limits, liveness/readiness probes, HPA, and optional KEDA scale-to-zero. Rendered by `tools/helm_gen.py`.

## What's Here

```
chart/
  Chart.yaml           # Chart metadata — name/version substituted from {{SERVICE_NAME}}
  values.yaml          # Default values with {{SERVICE_NAME}}, {{REGISTRY}}, {{PORT}} placeholders
  templates/
    deployment.yaml    # Deployment with full securityContext, probes, resource limits
    service.yaml       # ClusterIP service
    ingress.yaml       # Optional NGINX ingress with TLS
    hpa.yaml           # HorizontalPodAutoscaler (enabled by values.autoscaling.enabled)
    keda.yaml          # HTTPScaledObject for scale-to-zero (enabled by values.keda.enabled)
    networkpolicy.yaml # Default-deny + explicit allow ingress/egress
    serviceaccount.yaml # ServiceAccount with optional IRSA/Workload Identity annotation
    pdb.yaml           # PodDisruptionBudget for prod
    _helpers.tpl       # Chart name/label helpers
  tests/
    test-connection.yaml # Helm test pod
```

## Template Variables (values.yaml)

Key placeholders replaced by `tools/helm_gen.py`:

| Variable | Description |
|----------|-------------|
| `{{SERVICE_NAME}}` | Lowercase slug, e.g. `payment-api` |
| `{{REGISTRY}}` | Container registry URL |
| `{{IMAGE_NAME}}` | Image name without tag |
| `{{PORT}}` | Container listen port |
| `{{NAMESPACE}}` | Kubernetes namespace |

## How to Use / Run

```bash
# Generate (via tool — preferred)
python3 tools/helm_gen.py --service payment-api --cloud aws --port 8080

# Lint the generated chart
helm lint workspace/payment-api/helm

# Dry-run render
helm template payment-api workspace/payment-api/helm \
  --set image.tag=v1.0.0 \
  --namespace payment-api

# Run Helm unit tests (requires helm-unittest plugin)
helm unittest workspace/payment-api/helm

# Install to cluster
helm upgrade --install payment-api workspace/payment-api/helm \
  --namespace payment-api --create-namespace \
  --set image.tag=v1.0.0
```

## Key Details

- Security context defaults (all enforced in the template):
  - `runAsNonRoot: true`
  - `readOnlyRootFilesystem: true`
  - `allowPrivilegeEscalation: false`
  - `capabilities.drop: [ALL]`
- Resource requests/limits are set — override via `values-prod.yaml` for production sizing
- NetworkPolicy is deny-all by default; add explicit allow rules in `values.yaml`
- KEDA and HPA are **mutually exclusive** — enabling `keda.enabled: true` disables HPA
- Override per environment: `helm upgrade ... -f values-dev.yaml -f values-prod.yaml`

## Related

- `templates/keda/` — KEDA HTTPScaledObject used when `keda.enabled: true`
- `tools/helm_gen.py` — renders and validates this chart
- `tools/test_runner.py` — runs `helm lint` + `helm unittest` + `kubectl dry-run` on generated charts
