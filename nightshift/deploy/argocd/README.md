# Argo CD bootstrap for nightshift

A reference for running nightshift via Argo CD on EKS. Customer
deployments use this — the customers repo declares per-customer
`Application`s; this directory is what stands the controller up
in the cluster.

## What lives here

```
deploy/argocd/
  values.yaml             argo-cd chart values: ingress + cluster
                          options + namespace ConfigMaps the chart
                          needs to know about.
  ecr-token-refresh.yaml  CronJob that re-creates the
                          `argocd-repo-server-ecr-public` Secret
                          every 6h. ECR Public tokens expire in 12h
                          and Argo CD doesn't natively re-auth
                          helm OCI repos.
```

## One-time install

1. Pre-reqs: an EKS cluster with IAM-for-ServiceAccount and the
   `nightshiftco` ECR Public alias registered.

2. Install Argo CD via its upstream chart:

   ```bash
   helm repo add argo https://argoproj.github.io/argo-helm
   helm repo update
   kubectl create namespace argocd
   helm install argocd argo/argo-cd \
     --namespace argocd \
     --version 7.7.5 \
     -f deploy/argocd/values.yaml
   ```

3. Apply the ECR Public token refresher (uses IRSA — see
   `ecr-token-refresh.yaml` for the SA + IAM role expected):

   ```bash
   kubectl apply -n argocd -f deploy/argocd/ecr-token-refresh.yaml
   ```

4. Bootstrap the customers App-of-Apps. From the customers repo
   checkout:

   ```bash
   kubectl apply -n argocd -f customers/argocd/root.yaml
   ```

   That's the only Application that's ever manually applied. From
   here on, the customers repo is the source of truth — Argo CD
   reconciles itself + every customer namespace from it.

## How releases flow into customer clusters

```
nightshift git tag vX.Y.Z
        │
        ▼ release CI
public.ecr.aws/nightshiftco/charts/nightshift:vX.Y.Z   (chart)
public.ecr.aws/nightshiftco/nightshift-{api,worker,*}  (images)
        │
        ▼ Argo CD repo-server polls the OCI registry every ~3 min
        │  resolves each Application's targetRevision against the
        │  latest matching chart
        │
        ▼ if newer chart matches the semver range, Argo applies
Customer namespace upgraded. No PR. No human in the loop.
```

To **decline a release** for a customer, edit the Application's
`spec.source.targetRevision` from `0.x.x` (or whatever range) to
a pin (`0.3.7`). Commit. Argo CD stops upgrading that namespace
until you widen the range again. The git diff is the audit trail.

## Why ECR Public, not private

- Anonymous pulls — Argo CD doesn't need IAM trust to a customer
  account just to read the chart. The chart is open-source anyway.
- Same artifact for community + customers. No drift between what
  open-source users can self-host and what customers receive.
- Storage is free up to 50 GB; egress is free.

The private ECR repos still exist for `make eks-quickstart` SHA-
tagged dev images. Production releases don't go there.

## ECR Public alias caveat

The `nightshiftco` alias is one per AWS account, registered via
the AWS Console (Console → ECR → Public registries → "Edit
alias"). Terraform cannot create or claim the alias. If the alias
is renamed, every `targetRevision` URL across all customers needs
to update.
