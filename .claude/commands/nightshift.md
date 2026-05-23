# /nightshift — Kubernetes Cost Optimization Scheduler

Nightshift schedules scale-down and scale-up of Kubernetes workloads based on time-of-day or day-of-week rules (e.g., scale to 0 at night, restore in the morning). Typical savings: 60–80% on dev/staging clusters. Lives at `agents/nightshift/`. Runs as `nightshift-api` (REST) + `nightshift-worker` (executor). The `nightshift-worker-claude` variant uses Claude AI for intelligent scaling decisions.

## Step 1: Ask What to Do

Offer the following options:

1. Add a schedule for a workload
2. List existing schedules
3. Remove a schedule
4. Get savings estimate
5. Start the nightshift API
6. Check nightshift status

## Step 2: Collect Operation-Specific Inputs

**For "add schedule":** ask for:
- Namespace
- Workload name (e.g. `payment-api`)
- Workload type: `Deployment` or `StatefulSet`
- Scale-down time (cron expression, e.g. `0 20 * * 1-5` for 8 PM weekdays)
- Scale-up time (cron expression, e.g. `0 8 * * 1-5` for 8 AM weekdays)
- Minimum replicas when active (e.g. `2`)
- Timezone (e.g. `Asia/Kolkata`, `UTC`, `America/New_York`)

**For "savings estimate":** ask for:
- Current replica count
- Node cost per hour (USD)
- Hours per day the service can be scaled down

**For "remove schedule":** ask for namespace and workload name.

## Step 3: Run nightshift Commands

```bash
# Start the nightshift API (runs on :8080 by default)
cd agents/nightshift && python -m nightshift.api
# or
nightshift-api --port 8080

# Check API status
curl -s http://localhost:8080/status | python3 -m json.tool

# List all schedules
curl -s http://localhost:8080/schedules | python3 -m json.tool

# Add a schedule
curl -s -X POST http://localhost:8080/schedules \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "<namespace>",
    "workload": "<workload_name>",
    "kind": "<Deployment|StatefulSet>",
    "scaleDown": "<cron_expression>",
    "scaleUp": "<cron_expression>",
    "minReplicas": <min_replicas>,
    "timezone": "<timezone>"
  }'

# Remove a schedule
curl -s -X DELETE http://localhost:8080/schedules/<namespace>/<workload_name>
```

## Step 4: Savings Estimate Calculation

Given: current replicas = R, node cost/hr = C, hours scaled down/day = H

```
Daily savings   = R × C × H
Monthly savings = Daily savings × 30
Annual savings  = Monthly savings × 12
```

Show the result as:

> At <R> replicas × $<C>/hr scaled down <H> hrs/day:
> Monthly savings: $<monthly> | Annual: $<annual>

## Step 5: For "Add Schedule" — Show the Resulting State

After adding, fetch and display the schedule to confirm:

```bash
curl -s http://localhost:8080/schedules/<namespace>/<workload_name> | python3 -m json.tool
```

Remind the user: the `nightshift-worker` process must be running for schedules to execute.
