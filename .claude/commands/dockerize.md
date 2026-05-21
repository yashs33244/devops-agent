# /dockerize — Dockerize a Service

Generate an optimised, security-hardened multi-stage Dockerfile for a service and verify the build succeeds.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Path to service** — absolute or repo-relative path (e.g. `workspace/payment-api` or `/home/user/repos/payment-api`)
2. **Service name** — lowercase, hyphens only (e.g. `payment-api`)

If a `Dockerfile` already exists at that path, **do NOT overwrite it**. Instead:
- Show a diff of what would change
- Ask for explicit confirmation before proceeding

## Step 2: Run Dockerize Tool

```bash
python3 tools/dockerize.py --path <path> --service <service_name>
```

## Step 3: Lint and Scan

After generation, automatically run:

```bash
hadolint <path>/Dockerfile
trivy image --severity HIGH,CRITICAL <service_name>:latest
```

If `hadolint` or `trivy` are not installed, note this and skip gracefully — do not fail the whole command.

## Step 4: Report Back

Tell the user:

- Whether the Dockerfile was **created** or **already existed** (and what was changed)
- The detected language/runtime (Node.js, Python, Go, Java, etc.)
- Build result: success or failure with error output
- Any HIGH/CRITICAL CVEs found by Trivy
- Any hadolint warnings (especially DL rules about pinned versions)
- Path to the generated Dockerfile
