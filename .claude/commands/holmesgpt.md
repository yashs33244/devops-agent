# /holmesgpt — AI-Powered Kubernetes Incident Investigation

HolmesGPT is an AI SRE tool that investigates Kubernetes incidents using natural language. It queries live cluster state, logs, and events to surface root causes. Lives at `agents/holmesgpt/`. Install once with `pip install -e agents/holmesgpt/`.

## Step 1: Gather Inputs

Ask the user what they want to investigate. Offer these preset options or accept a free-form description:

1. Pod crash / CrashLoopBackOff
2. High latency / slow responses
3. OOMKilled container
4. Network error / connection refused
5. Deployment rollout failing
6. PVC full / storage issues
7. Custom query (user describes it)

Then ask for context (all optional but recommended):

- **Namespace** — e.g. `default`, `production`
- **Pod or deployment name** — e.g. `payment-api-7d9f8b`
- **Service name** — e.g. `payment-api`
- **Time window** — e.g. `last 30 minutes`, `last 2 hours`

## Step 2: Install HolmesGPT (if not already)

```bash
pip install -e agents/holmesgpt/ --quiet
holmes --version
```

## Step 3: Run the Investigation

Use the appropriate command based on the incident type:

```bash
# General natural language query
holmes ask "why is pod <pod_name> crashing in namespace <namespace>"

# Investigate a specific pod
holmes ask "what caused OOMKilled on <pod_name> in <namespace> in the last <time_window>"

# Find pods matching a selector
holmes find --selector app=<service_name> --namespace <namespace>

# Investigate active Alertmanager alerts
holmes investigate alert --alertmanager-url http://localhost:9093

# High latency investigation
holmes ask "why is <service_name> experiencing high latency in <namespace>"

# Deployment failing
holmes ask "why is deployment <name> failing to roll out in <namespace>"
```

## Step 4: Show Structured Findings

Present the holmes output in this format:

**Root Cause:**
> (one-sentence summary of what holmes identified)

**Evidence:**
- (log lines, events, or metric anomalies holmes found)
- (list each piece of supporting evidence)

**Recommended Actions:**
1. (first action to take)
2. (second action, if any)

## Step 5: Offer to Execute Fix

If holmes suggests a fix command (e.g., restart a pod, scale a deployment, adjust resource limits), ask the user:

> Holmes suggests: `kubectl rollout restart deployment/<name> -n <namespace>`
> Shall I run this? (yes / no)

Only run the fix command after explicit confirmation.
