# /secrets — Discover and Scaffold Secrets

Scan a service repo for secrets and environment variables, then generate the appropriate ESO manifests, Sealed Secrets, or IRSA annotations.

## Step 1: Gather Required Inputs

Ask the user for:

1. **Repo path** — absolute or repo-relative path to the service source (e.g. `workspace/payment-api`)
2. **Service name** — lowercase, hyphens only (e.g. `payment-api`)
3. **Cloud provider** — `aws`, `azure`, or `gcp`

## Step 2: Run Secrets Discovery

```bash
python3 tools/secrets_manager.py \
  --repo-path <repo_path> \
  --service <service_name> \
  --cloud <cloud> \
  --output-dir workspace/<service_name>/secrets \
  --helm-dir workspace/<service_name>/helm
```

## Step 3: Interactive Secret Confirmation

For each detected secret, present it to the user and confirm:
- What the secret is (name, current source)
- Recommended handling strategy using the decision tree:
  - Cloud credential → IRSA / Workload Identity (never static keys)
  - Dynamic/rotatable → External Secrets Operator + cloud secrets manager
  - Static, changes rarely → Sealed Secrets (encrypted in Git)
  - Dev only → K8s native Secret (warn: not for prod)
- Ask: "Does this look correct, or do you want to override the strategy?"

Do not proceed to write manifests until the user has confirmed each secret's strategy.

## Step 4: Report Back

Tell the user:

- Number of secrets discovered and their names (not values)
- Which secrets will use IRSA/Workload Identity vs ESO vs Sealed Secrets
- Generated manifest paths under `workspace/<service_name>/secrets/`
- Any cloud secrets manager setup steps needed (e.g. create secret in AWS Secrets Manager)
- The ESO ClusterSecretStore + ExternalSecret YAML paths if applicable
- Reminder: never commit `.env` files or raw secret values to Git
