# /optimize-cost — Apply Car-Painter Scale-to-Zero

Apply the car-painter scale-to-zero pattern to a service to reduce compute costs by 60–90% for bursty or low-traffic workloads.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Terraform directory** — path to the service's Terraform files (e.g. `workspace/payment-api/terraform`)
2. **Platform** — `eks`, `aks`, or `gke`
3. **Service name** — lowercase, hyphens only

Before proceeding, confirm the service is eligible:
- Stateless HTTP service? (scale-to-zero is NOT appropriate for stateful services, databases, or queue consumers)
- Can tolerate a ~60-second cold start on first request after idle?

If the answer to either is no, explain why scale-to-zero is not recommended and stop.

## Step 2: Run Cost Optimizer

```bash
python3 tools/cost_optimize.py \
  --terraform-dir <terraform_dir> \
  --platform <platform>
```

## Step 3: Explain What Was Applied

After the tool runs, explain to the user exactly what was configured:

- **KEDA HTTPScaledObject** added with:
  - `minReplicaCount: 0` — scales to zero after 5 minutes of idle traffic
  - `scaledownPeriod: 300` — 5-minute idle window before scale-down
  - `targetPendingRequests: 100` — scale up when queue exceeds 100 in-flight requests
  - `maxReplicaCount: 10` — cap (override for prod as needed)
- KEDA installation command if not already present:
  ```bash
  helm repo add kedacore https://kedacore.github.io/charts
  helm install keda kedacore/keda --namespace keda --create-namespace
  ```

## Step 4: Report Back

Tell the user:

- Files modified or created
- Estimated cost savings: **60–90% reduction** for bursty/low-traffic services
- Cold-start latency to set expectations (~60 seconds from zero to first response)
- When NOT to use this (stateful services, queue consumers, < 60s cold-start tolerance)
- For prod: recommend increasing `maxReplicaCount` and setting `minReplicaCount: 1` to avoid cold starts on SLA-sensitive endpoints
