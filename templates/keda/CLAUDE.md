# templates/keda/

KEDA HTTPScaledObject template implementing the car-painter scale-to-zero pattern — scales pods to 0 on 5 minutes of idle traffic and back up within 60 seconds on first request.

## What's Here

```
http-scaler.yaml    # HTTPScaledObject — the car-painter pattern manifest
```

## Template Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `{{SERVICE_NAME}}` | Lowercase service slug | required |
| `{{NAMESPACE}}` | Kubernetes namespace | required |
| `{{PORT}}` | Container target port | required |

Rendered by `tools/cost_optimize.py` or when `--scale-to-zero yes` is passed to `tools/workflow.py`.

## How to Use / Run

```bash
# Apply via tool (preferred — also validates KEDA is installed)
python3 tools/cost_optimize.py \
  --terraform-dir workspace/payment-api/terraform \
  --platform eks

# Or apply directly after substituting variables
kubectl apply -f workspace/payment-api/helm/templates/keda.yaml

# Install KEDA + HTTP add-on first (EKS/AKS/GKE)
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda -n keda --create-namespace
helm install keda-add-ons-http kedacore/keda-add-ons-http -n keda

# Verify scale-to-zero is working
kubectl get httpscaledobject -n <namespace>
```

## Key Details

- **`scaledownPeriod: 300`** — pods scale to 0 after 5 minutes of no traffic
- **`targetPendingRequests: 100`** — scale up triggers when queue > 100 in-flight requests
- **`minReplicaCount: 0`** — enables true scale-to-zero (no idle pods = no idle cost)
- **`maxReplicaCount: 10`** — override to higher values for prod
- Typical cost saving: **60–90%** for bursty or low-traffic services
- **Do NOT use** for: stateful services, databases, message-queue consumers (use KEDA queue scalers instead), services with < 60s cold-start tolerance

## Platform Notes

| Platform | Use this template? | Notes |
|----------|--------------------|-------|
| EKS | Yes | Install KEDA + HTTP add-on via Helm |
| AKS | Yes | KEDA add-on is built-in (enable in portal/bicep) |
| GKE | Yes, or use Cloud Run | Cloud Run has native scale-to-zero — prefer it for stateless HTTP |
| Cloud Run / Container Apps / Fargate | No | Native scale-to-zero — no KEDA needed |

## Related

- `templates/helm/` — Helm chart that embeds this template when `keda.enabled: true`
- `tools/cost_optimize.py` — renders and applies this template
- `agents/nightshift/` — scheduled/policy-driven scale-down complement
- Root `CLAUDE.md` → Car-Painter Scale-to-Zero Pattern — full decision tree
