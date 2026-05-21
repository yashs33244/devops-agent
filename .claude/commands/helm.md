# /helm — Generate Helm Chart

Generate a production-ready Helm chart for a service with security contexts, resource limits, and health probes baked in.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Service name** — lowercase, hyphens only (e.g. `payment-api`)
2. **Cloud provider** — `aws`, `azure`, or `gcp`
3. **Container port** — the port the service listens on (e.g. `8080`, `3000`)

If a Helm chart already exists at `workspace/<service_name>/helm/`, show a diff and ask for confirmation before overwriting.

## Step 2: Generate the Helm Chart

```bash
python3 tools/helm_gen.py \
  --service <service_name> \
  --cloud <cloud> \
  --port <port>
```

## Step 3: Lint Automatically

Run immediately after generation — do not skip:

```bash
helm lint workspace/<service_name>/helm/
```

If lint reports errors, fix them before reporting back. Warnings are acceptable but should be noted.

Optionally run a dry-run render to catch template errors:

```bash
helm template <service_name> workspace/<service_name>/helm/ --debug
```

## Step 4: Report Back

Tell the user:

- All generated chart file paths (`Chart.yaml`, `values.yaml`, `templates/`)
- Helm lint result (pass / warnings / errors)
- Security contexts confirmed present:
  - `runAsNonRoot: true`
  - `readOnlyRootFilesystem: true`
  - `allowPrivilegeEscalation: false`
  - `capabilities.drop: [ALL]`
- Resource requests/limits set
- Liveness and readiness probes configured
- Next step: `helm install <service_name> ./helm` command with any required `--set` overrides
