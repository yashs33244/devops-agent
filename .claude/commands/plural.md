# /plural — GitOps Multi-Cloud Fleet Deployment

Plural is a GitOps multi-cloud deployment platform that manages applications across multiple Kubernetes clusters using a GitOps workflow with promotion pipelines (dev → staging → prod). Lives at `agents/plural/`.

## Step 1: Ask What to Do

Offer the following options:

1. Deploy an application to a cluster
2. List current deployments
3. Promote to next environment (dev → staging → prod)
4. Rollback a deployment
5. Add a new cluster to the fleet
6. View fleet status (all clusters + sync state)

## Step 2: Collect Operation-Specific Inputs

**For "deploy application":** ask for:
- Application name
- Target cluster (list available with `plural clusters list`)
- Values overrides (key=value pairs, or path to a values file)

**For "promote":** ask for:
- Application name
- From environment: `dev` / `staging`
- To environment: `staging` / `prod`
- Confirm: show current image/chart version before promoting

**For "rollback":** ask for:
- Application name
- Target cluster
- Revision to roll back to (or `previous` for one step back)

**For "add cluster":** ask for:
- Cluster name
- Cloud provider: `aws` / `azure` / `gcp`
- Kubeconfig context name

**For "view fleet status":** no additional input needed.

## Step 3: Run plural Commands

```bash
# Install / verify plural CLI
plural version

# List all clusters in the fleet
plural clusters list

# List all applications and their sync status
plural deployments list

# View fleet status (all clusters + all apps)
plural fleet status

# Deploy an application
plural deploy <app_name> \
  --cluster <cluster_name> \
  --set key1=value1 \
  --set key2=value2

# Deploy with a values file
plural deploy <app_name> \
  --cluster <cluster_name> \
  --values /path/to/values-override.yaml

# Promote to next environment
plural pipeline promote <app_name> \
  --from <from_env> \
  --to <to_env>

# Rollback to previous revision
plural rollback <app_name> --cluster <cluster_name> --revision previous

# Add a new cluster
plural clusters add <cluster_name> \
  --cloud <cloud> \
  --context <kubeconfig_context>
```

## Step 4: Show Fleet Status Table

For fleet status, display results in this format:

| Cluster | Cloud | Environment | App | Sync Status | Version |
|---------|-------|-------------|-----|-------------|---------|
| prod-eks | aws | prod | payment-api | Synced | v1.4.2 |
| staging-gke | gcp | staging | payment-api | OutOfSync | v1.4.3 |

Highlight any **OutOfSync** or **Degraded** entries and suggest next steps.

## Step 5: For "Promote" — Show Diff Before Confirming

Before executing a promotion, display:

> Promoting `<app_name>` from `<from_env>` to `<to_env>`
> Image: `registry/app:v1.4.2` → `registry/app:v1.4.3`
> Chart: `1.2.0` → `1.2.1`

Ask: "Confirm promotion? (yes / no)" — only proceed on explicit yes.

## Step 6: For "Add Cluster" — Check bin/ Scripts

```bash
# Plural helper scripts live in agents/plural/bin/
ls agents/plural/bin/

# Common setup scripts
bash agents/plural/bin/bootstrap-cluster.sh --cloud <cloud> --name <cluster_name>
bash agents/plural/bin/register-cluster.sh --context <kubeconfig_context>
```
