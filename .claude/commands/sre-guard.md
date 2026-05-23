# /sre-guard — Continuous SRE Monitoring Daemon

SRE Guard is a daemon agent that continuously monitors services by watching Prometheus metrics, Kubernetes events, and health endpoints. It can be commanded interactively via REST API or CLI. When an issue is detected it invokes HolmesGPT for AI-powered diagnosis. Lives at `agents/sre-guard/`. REST API: `http://localhost:8888`. Install with `pip install -e agents/sre-guard/` then use `sreguard` CLI.

## Step 1: Ask What to Do

Offer the following options:

1. Start the SRE Guard daemon
2. Stop the daemon
3. Check status of all watched services
4. Add a service to watch
5. Diagnose a specific service (triggers HolmesGPT analysis)
6. Silence alerts for a service
7. View alert history
8. Run a runbook for a service

## Step 2: Collect Operation-Specific Inputs

**For "add service":** ask for:
- Service name
- Prometheus URL (e.g. `http://prometheus:9090`)
- Kubernetes namespace
- Health check URL (e.g. `http://payment-api/health`)

**For "diagnose":** ask for service name and optional time window.

**For "silence":** ask for service name and silence duration (e.g. `30m`, `2h`, `1d`).

**For "run runbook":** ask for service name and which runbook to execute.

## Step 3: Run sreguard Commands

```bash
# Install (once)
pip install -e agents/sre-guard/ --quiet

# Start the daemon (runs on :8888)
sreguard daemon start
# or directly:
python -m sre_guard.daemon --port 8888

# Stop the daemon
sreguard daemon stop

# Check status of all watched services
curl -s http://localhost:8888/status | python3 -m json.tool
# or:
sreguard status

# Add a service to watch
curl -s -X POST http://localhost:8888/watch \
  -H "Content-Type: application/json" \
  -d '{
    "service": "<service_name>",
    "namespace": "<namespace>",
    "prometheusUrl": "<prometheus_url>",
    "healthUrl": "<health_url>"
  }'
# or:
sreguard watch <service_name> \
  --namespace <namespace> \
  --prometheus <prometheus_url> \
  --health-url <health_url>

# Diagnose a service (calls HolmesGPT internally)
curl -s -X POST http://localhost:8888/diagnose/<service_name> | python3 -m json.tool
# or:
sreguard diagnose <service_name>

# Silence alerts
curl -s -X POST http://localhost:8888/silence/<service_name> \
  -H "Content-Type: application/json" \
  -d '{"duration": "<duration>"}'
# or:
sreguard silence <service_name> --duration <duration>

# View alert history
curl -s http://localhost:8888/alerts | python3 -m json.tool
sreguard alerts --service <service_name>

# Run a runbook
sreguard runbook run <runbook_name> --service <service_name>
```

## Step 4: Show Status as a Rich Table

After calling `GET /status`, format the output as:

| Service | Namespace | Health | Active Alerts | Last Checked |
|---------|-----------|--------|---------------|--------------|
| payment-api | production | OK | 0 | 2s ago |
| auth-service | production | DEGRADED | 1 | 5s ago |

Highlight any **DEGRADED** or **DOWN** services and offer to diagnose them immediately.

## Step 5: For "Diagnose" — Show AI Findings

The `/diagnose/<service>` endpoint invokes HolmesGPT. Display the AI findings in structured format:

**Service:** `<service_name>`
**Diagnosis triggered by:** SRE Guard alert / manual request

**Root Cause:**
> (HolmesGPT root cause summary)

**Evidence:**
- (supporting log lines, events, metrics)

**Recommended Actions:**
1. (first action)
2. (second action)

Ask: "Run the recommended fix? (yes / no)"
