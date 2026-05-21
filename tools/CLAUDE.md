# tools/

Ten Python CLI tools that implement the DevOps pipeline. Each tool does exactly one thing and writes output to a predictable path under `workspace/<service>/`.

## What's Here

| Script | Purpose | Key output path |
|--------|---------|----------------|
| `workflow.py` | Full pipeline orchestrator — calls all tools in sequence | (delegates) |
| `dockerize.py` | Language detection + Dockerfile generation + build test | `workspace/<svc>/Dockerfile` |
| `terraform_gen.py` | Terraform file generation from templates | `workspace/<svc>/terraform/` |
| `helm_gen.py` | Helm chart generation + lint + unittest | `workspace/<svc>/helm/` |
| `cicd_setup.py` | GitHub Actions CI + CD workflow creation | `workspace/<svc>/.github/workflows/` |
| `secrets_manager.py` | Secret scanning + ESO manifest generation | `workspace/<svc>/secrets/` |
| `local_test.py` | LocalStack / Azurite / GCP emulator validation | stdout + exit code |
| `cost_optimize.py` | KEDA scale-to-zero HTTPScaledObject applier | `workspace/<svc>/helm/templates/keda.yaml` |
| `test_runner.py` | Runs ALL test layers (Dockerfile/Terraform/Helm/Actions/Integration) | stdout + exit code |
| `emulators/` | Docker Compose files for LocalStack, Azurite, GCP emulators | — |

## How to Use / Run

```bash
# Full pipeline (recommended entry point)
python3 tools/workflow.py \
  --service payment-api \
  --repo https://github.com/org/payment-api \
  --cloud aws

# Individual tools
python3 tools/dockerize.py --path workspace/payment-api --service payment-api
python3 tools/terraform_gen.py --cloud aws --service payment-api --use-case web_app --region us-east-1 --env dev
python3 tools/helm_gen.py --service payment-api --cloud aws --port 8080
python3 tools/cicd_setup.py --repo-path workspace/payment-api --cloud aws --service payment-api
python3 tools/secrets_manager.py --repo-path workspace/payment-api --service payment-api --cloud aws --output-dir workspace/payment-api/secrets --helm-dir workspace/payment-api/helm
python3 tools/local_test.py --cloud aws --terraform-dir workspace/payment-api/terraform --service payment-api
python3 tools/cost_optimize.py --terraform-dir workspace/payment-api/terraform --platform eks
python3 tools/test_runner.py --service payment-api --repo-path workspace/payment-api --terraform-dir workspace/payment-api/terraform --helm-dir workspace/payment-api/helm --cloud aws

# Get help for any tool
python3 tools/<name>.py --help

# Start cloud emulators before local_test.py
docker compose -f tools/emulators/localstack.yml up -d   # AWS
docker compose -f tools/emulators/azurite.yml up -d      # Azure
docker compose -f tools/emulators/gcp-emulators.yml up -d # GCP
```

## Key Details

- **Execution order** (enforced by `workflow.py`): dockerize → secrets_manager → terraform_gen → helm_gen → cicd_setup → test_runner → local_test → cost_optimize
- All tools accept `--help` and return JSON-serialisable exit codes
- `test_runner.py` supports `--only dockerfile,terraform` to run a subset and `--fail-fast` to stop on first failure
- **Never skip test_runner** — the CLAUDE.md root rule: "All tests must pass before proceeding"
- Tools write to `workspace/<service>/` — never directly into `services/` (those are the reference copies)
- `secrets_manager.py` generates `workspace/<service>/secrets/github-secrets.md` with the exact GitHub secrets to configure

## Related

- `templates/` — all Jinja2 templates consumed by these tools
- `workspace/` — generated output lives here (git-ignored for secrets safety)
- `services/` — reference service implementations used for integration testing
- Root `CLAUDE.md` — full workflow specification with all tool flags
