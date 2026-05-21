# /test — Run the Full TDD Test Suite

Run all test layers (Dockerfile, Terraform, Helm, GitHub Actions, Integration) for one or more services and fix any failures.

## Step 1: Determine Scope

Ask the user:

1. **Which service(s) to test** — provide a service name, a comma-separated list, or `all` to test every service under `workspace/`
2. **Specific layers only?** — optional: `dockerfile`, `terraform`, `helm`, `actions`, `integration` (leave blank for all layers)
3. **Fail fast?** — stop on first failure (`yes`) or run all layers and report all failures (`no`, default)

If `all` is selected, list all directories under `workspace/` and confirm before running.

## Step 2: Run Tests

For each service:

```bash
python3 tools/test_runner.py \
  --service <service_name> \
  --repo-path workspace/<service_name> \
  --terraform-dir workspace/<service_name>/terraform \
  --helm-dir workspace/<service_name>/helm \
  --cloud <cloud>
```

Add `--only <layers>` if the user specified specific layers. Add `--fail-fast` if requested.

Individual layer commands for reference:
```bash
hadolint workspace/<service_name>/Dockerfile
terraform -chdir=workspace/<service_name>/terraform validate -backend=false
helm lint workspace/<service_name>/helm/
act push --dry-run
```

## Step 3: Fix Failures Automatically

If any test layer fails:
- Diagnose the root cause from the error output
- Apply the fix directly to the relevant file
- Re-run the failing test to confirm it now passes
- Do NOT declare the command done while any test is still failing

## Step 4: Report Back

Show a pass/fail summary table:

| Service | Dockerfile | Terraform | Helm | Actions | Integration |
|---------|-----------|-----------|------|---------|-------------|
| `<name>` | PASS/FAIL | PASS/FAIL | PASS/FAIL | PASS/FAIL | PASS/FAIL |

- List any fixes applied with file paths changed
- Confirm: "All tests passing — pipeline is ready to proceed" or list remaining failures
