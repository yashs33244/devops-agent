# /deploy — Full Pipeline Deploy

Run the complete DevOps pipeline for a service: Dockerize → Secrets → Terraform → Helm → CI/CD → Tests.

## Step 1: Gather Required Inputs

Ask the user for each of the following (do not proceed until all are provided):

1. **Service name** — lowercase, hyphens only (e.g. `payment-api`)
2. **Repo URL or local path** — GitHub URL or absolute path on disk
3. **Cloud provider** — `aws`, `azure`, or `gcp`
4. **Region** — e.g. `us-east-1`, `eastus`, `us-central1`
5. **Environment** — `dev`, `staging`, or `prod`
6. **Use case** — `web_app`, `microservice`, `batch_job`, `data_pipeline`, or `scheduled_task`
7. **Backing services needed** — any of: `postgres`, `redis`, `s3`, `pubsub`, `queue`, or `none`
8. **Apply scale-to-zero (car-painter)?** — `yes` or `no` (recommended for bursty HTTP)

## Step 2: Run the Full Pipeline

```bash
python3 tools/workflow.py \
  --service <service_name> \
  --repo <repo_url_or_path> \
  --cloud <cloud> \
  --region <region> \
  --env <environment> \
  --use-case <use_case>
```

If scale-to-zero was requested, also run:

```bash
python3 tools/cost_optimize.py \
  --terraform-dir workspace/<service_name>/terraform \
  --platform <eks|aks|gke>
```

## Step 3: Report Back

After the pipeline completes, show the user:

- All generated file paths
- GitHub Actions secrets/variables to add (from `workspace/<service_name>/secrets/github-secrets.md`)
- IRSA / Workload Identity setup steps if cloud credentials were detected
- Next manual commands to run (e.g. `terraform apply`, `helm install`)
- Estimated monthly cost at dev scale, and note for prod sizing
- Any test failures that need attention (do not declare done with failing tests)
