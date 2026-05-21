# /status — Monorepo Status Overview

Show a live snapshot of the devops-agent monorepo: agents, services, cluster health, monitoring stack, and recent activity.

## Step 1: No Input Required

Run all checks immediately without asking the user for anything.

## Step 2: Run Status Checks

**List agents with language and purpose:**
```bash
ls -1 agents/
```
For each agent directory, check for a `README.md` or `CLAUDE.md` and extract the one-line description.

**List services with workspace presence:**
```bash
ls -1 services/ 2>/dev/null || echo "No services/ directory found"
ls -1 workspace/ 2>/dev/null || echo "No workspace/ directory found"
```

**Check if kind cluster is running:**
```bash
kind get clusters 2>/dev/null || echo "kind not installed or no clusters"
kubectl get nodes 2>/dev/null || echo "kubectl not reachable"
```

**Check if monitoring stack is up:**
```bash
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "prometheus|grafana|alertmanager" || echo "Monitoring stack not running"
```

**Check LocalStack / emulators:**
```bash
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "localstack|azurite|firestore" || echo "No emulators running"
```

**Recent git activity:**
```bash
git log --oneline -10
git status --short
```

## Step 3: Report Back

Present a structured summary:

**Agents** (`agents/`): list name + one-line purpose for each

**Services in workspace** (`workspace/`): list each service and whether it has Dockerfile / Terraform / Helm chart present

**Cluster**: kind cluster name and node count, or "not running"

**Monitoring**: Prometheus / Grafana status

**Emulators**: which cloud emulators are currently running

**Repo**: last 10 commits and any uncommitted changes
