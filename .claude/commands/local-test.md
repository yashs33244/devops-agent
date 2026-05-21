# /local-test — Run Local Emulator Tests

Spin up the appropriate cloud emulator (LocalStack / Azurite / GCP emulators) and run integration tests against it locally — no real cloud credentials needed.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Cloud provider** — `aws`, `azure`, or `gcp`
2. **Service name** — lowercase, hyphens only (e.g. `payment-api`)
3. **Terraform directory** — path to the service's Terraform files (default: `workspace/<service_name>/terraform`)

## Step 2: Start the Right Emulator

Start the emulator that matches the cloud provider. Do not start emulators for other clouds.

**AWS → LocalStack:**
```bash
docker compose -f tools/emulators/localstack.yml up -d
```

**Azure → Azurite:**
```bash
docker compose -f tools/emulators/azurite.yml up -d
```

**GCP → Firestore + Pub/Sub emulators:**
```bash
docker compose -f tools/emulators/gcp-emulators.yml up -d
```

Wait for the emulator to be healthy before proceeding (check `docker ps` and container health status).

## Step 3: Run Local Tests

```bash
python3 tools/local_test.py \
  --cloud <cloud> \
  --terraform-dir <terraform_dir> \
  --service <service_name>
```

For AWS, `local_test.py` uses `tflocal` (the LocalStack-aware Terraform wrapper) automatically.

## Step 4: Report Back

Tell the user:

- Emulator start status (healthy / failed to start)
- Test results: pass or fail with error details
- Any Terraform resources that failed to provision against the emulator
- Whether the emulator is still running (leave it up for further iteration, or offer to stop it)
- Command to stop the emulator when done:
  ```bash
  docker compose -f tools/emulators/<emulator>.yml down
  ```
