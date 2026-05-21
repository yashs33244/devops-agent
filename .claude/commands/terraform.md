# /terraform — Generate Terraform for a Service

Generate cloud infrastructure Terraform modules for a service, then automatically format and validate them.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Service name** — lowercase, hyphens only (e.g. `payment-api`)
2. **Cloud provider** — `aws`, `azure`, or `gcp` (always ask — never assume)
3. **Region** — e.g. `us-east-1`, `eastus`, `us-central1`
4. **Environment** — `dev`, `staging`, or `prod`
5. **Use case** — `web_app`, `microservice`, `batch_job`, `data_pipeline`, or `scheduled_task`

If `.tf` files already exist in `workspace/<service_name>/terraform/`, show a diff of what would change and ask for confirmation before overwriting.

## Step 2: Generate Terraform

```bash
python3 tools/terraform_gen.py \
  --cloud <cloud> \
  --service <service_name> \
  --use-case <use_case> \
  --region <region> \
  --env <environment>
```

## Step 3: Format and Validate Automatically

Run these immediately after generation — do not skip:

```bash
terraform -chdir=workspace/<service_name>/terraform fmt
terraform -chdir=workspace/<service_name>/terraform validate -backend=false
```

If validate fails, diagnose and fix the error before reporting back.

## Step 4: Report Back

Tell the user:

- All generated `.tf` file paths
- Result of `terraform fmt` (any files reformatted?)
- Result of `terraform validate` (pass or fail with error details)
- Resources that will be created (VPC, cluster, database, registry, etc.)
- Estimated monthly cost at dev scale vs prod scale
- Next step: `terraform init && terraform plan` command to run manually
