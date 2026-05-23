# /opensre — SRE Runbook Automation

OpenSRE automates SRE runbooks: it runs pre-defined response playbooks in reaction to alerts or manual triggers, executing steps like pod restarts, scale-ups, log collection, notifications, and rollbacks. Lives at `agents/opensre/`. Install with `pip install -e agents/opensre/` then use the `opensre` CLI.

## Step 1: Ask What to Do

Offer the following options:

1. Run a runbook
2. List available runbooks
3. Create a new runbook
4. Test a runbook (dry-run)
5. View runbook execution history

## Step 2: Collect Operation-Specific Inputs

**For "run runbook":** ask for:
- Runbook name (or show list from `opensre list`)
- Target service or incident description
- Namespace (if K8s-related)

**For "create runbook":** ask for:
- Runbook name (slug format)
- Trigger condition: alert name (e.g. `HighErrorRate`) or `manual`
- Steps (pick from: `restart_pod`, `scale_up`, `notify_slack`, `check_logs`, `rollback_deployment`, `run_script`, `wait`, `custom`)
- Approval required before execution? (`yes` / `no`)

**For "test runbook":** ask for runbook name and a target service — runs with `--dry-run` flag.

**For "view history":** ask for optional filter (runbook name or service name).

## Step 3: Run opensre Commands

```bash
# Install (once)
pip install -e agents/opensre/ --quiet

# List all runbooks
opensre list

# Run a runbook against a service
opensre run <runbook_name> --service <service_name> --namespace <namespace>

# Dry-run (test without executing)
opensre run <runbook_name> --service <service_name> --dry-run

# View execution history
opensre history
opensre history --runbook <runbook_name>
opensre history --service <service_name>

# Create a runbook interactively
opensre create --name <runbook_name>
```

## Step 4: For "Create Runbook" — Generate YAML Spec

Build and display the runbook manifest:

```yaml
apiVersion: opensre.io/v1
kind: Runbook
metadata:
  name: <runbook_name>
spec:
  trigger:
    type: <alert|manual>
    alertName: <alert_name>       # only if type: alert
  requireApproval: <true|false>
  steps:
    - name: check-logs
      action: check_logs
      params:
        namespace: <namespace>
        selector: app=<service_name>
        tail: 100
    - name: restart-pod
      action: restart_pod
      params:
        namespace: <namespace>
        deployment: <service_name>
    - name: notify
      action: notify_slack
      params:
        channel: "#incidents"
        message: "Runbook <runbook_name> executed for <service_name>"
```

Ask: "Apply this runbook? (yes / no)"

```bash
opensre apply -f /tmp/<runbook_name>.yaml
```

## Step 5: Show Execution Output

After running, display results in this format:

**Runbook:** `<runbook_name>`
**Service:** `<service_name>`
**Status:** PASSED / FAILED / DRY-RUN

| Step | Action | Result |
|------|--------|--------|
| check-logs | check_logs | OK |
| restart-pod | restart_pod | OK |

Show any step failures with the error message and suggest remediation.
