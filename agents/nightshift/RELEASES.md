# Releases

Nightshift is published as versioned, immutable artifacts on a public container registry. Once you point an Argo CD installation at the
chart, you get every new release automatically. 

## What ships in a release

Every `vX.Y.Z` git tag in this repo produces:

- **Four images** at `public.ecr.aws/nightshiftco/<image>:vX.Y.Z`
  (anonymous pull; no AWS account required):
  - `nightshift-api`
  - `nightshift-worker`
  - `nightshift-worker-claude`
  - `nightshift-ui`
- **The helm chart** at
  `oci://public.ecr.aws/nightshiftco/charts/nightshift:vX.Y.Z`,
  with `chart.version == chart.appVersion == vX.Y.Z`. The chart's
  default `image.tag` is `.Chart.AppVersion`, so the chart and
  images are version-locked — installing chart `0.4.0` always pulls
  image `0.4.0`.
- **A GitHub Release** at
  github.com/nightshiftco/nightshift/releases/tag/vX.Y.Z with
  auto-generated notes between tags.

The chart in ECR Public is the source of truth that downstream consumers reconcile against.

## Recommended: Argo CD with a semver range

If you're running nightshift on EKS or any Kubernetes cluster, the
clean pull-based pattern is:

```yaml
# nightshift-app.yaml (create this once)
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nightshift
  namespace: argocd
spec:
  project: default
  source:
    repoURL: public.ecr.aws/nightshiftco/charts
    chart: nightshift
    # Auto-track every 0.x release. To freeze on a specific version,
    # change to e.g. "0.4.7" (exact pin) or "0.4.x" (patches only).
    targetRevision: 0.x.x
    helm:
      values: |
        # your values here, see deploy/charts/nightshift/values.yaml
        tenant:
          name: my-tenant
        ui:
          enabled: true
          # ... whatever else you override
  destination:
    server: https://kubernetes.default.svc
    namespace: nightshift
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

Apply it once:

```bash
kubectl apply -f nightshift-app.yaml
```

Argo CD's repo-server polls the OCI registry every ~3 minutes and
resolves `targetRevision: 0.x.x` to the newest matching chart. When
we push a new tag, your cluster picks it up on the next poll. No
human in the loop.

## Argo CD prerequisites

You need Argo CD installed in your cluster. The standard install:

```bash
helm repo add argo https://argoproj.github.io/argo-helm
helm install argocd argo/argo-cd --namespace argocd --create-namespace
```

You also need Argo CD's repo-server able to **pull from ECR Public
anonymously**. The default install supports anonymous OCI helm pulls
out of the box — the chart at `public.ecr.aws/nightshiftco/charts/nightshift`
is publicly readable; no auth Secret is required. If you've locked
down the repo-server's outbound network, allow `public.ecr.aws`.

A reference Argo CD bootstrap (with values + an optional ECR auth
token-refresh CronJob, only needed if you're authenticating to ECR
Public for some reason) lives in [deploy/argocd/](deploy/argocd/).

## Pinning or declining a release

Editing one field in your `Application`'s YAML is all it takes:

| Goal | `targetRevision` |
|---|---|
| Always latest 0.x | `0.x.x` |
| Latest 0.4 patch | `0.4.x` |
| Pin exactly | `0.4.7` |
| Hold below a known-bad version | `<0.5.0` |
| Pre-release channel | `0.x.x-rc.x` |

Commit. Argo CD stops upgrading until you widen the range. The git
diff is the audit trail for which version is in production.

## Without Argo CD

If you don't want Argo CD, the chart is plain helm — install or
upgrade by version:

```bash
helm upgrade --install nightshift \
  oci://public.ecr.aws/nightshiftco/charts/nightshift \
  --version 0.4.0 \
  --namespace nightshift --create-namespace \
  -f my-values.yaml
```

You're then responsible for noticing new releases and re-running
helm. Either subscribe to GitHub Releases for this repo or poll the
chart registry:

```bash
helm search repo --version-list \
  oci://public.ecr.aws/nightshiftco/charts/nightshift
```

## Forking + running your own release pipeline

Most operators don't need this — pull from our public registry. If
you're forking and want to publish your own artifacts:

- Set up an ECR Public registry alias for your fork
- Replace `public.ecr.aws/nightshiftco` with your alias in
  `deploy/charts/nightshift/values.yaml` and the GitHub Actions
  workflow at `.github/workflows/release.yml`
- Provide the workflow with an AWS IAM role that can push to your
  ECR Public via OIDC. The role's permissions:
  `ecr-public:*`, `sts:GetServiceBearerToken`. Trust policy
  scoped to your fork on `refs/tags/v*`.
- Set the `AWS_ACCOUNT_ID` repo secret.
- Tag `vX.Y.Z` — CI builds and publishes.

Reference: see [deploy/argocd/README.md](deploy/argocd/README.md)
for an end-to-end EKS bootstrap including IAM role creation and
the optional ECR token-refresh path.

## Versioning policy

- We follow semver. `MAJOR.MINOR.PATCH`.
- Pre-1.0 (current): MINOR bumps may include breaking chart-value
  or proto changes; PATCH bumps are bug fixes only.
- Post-1.0: MAJOR is reserved for breaking changes.

Read the GitHub Release notes before widening a `targetRevision`
range across a MINOR boundary.

## What if a release is bad?

1. Pin every consumer's `targetRevision` to the last known-good
   version. Git diff is the rollback audit trail.
2. Open an issue at github.com/nightshiftco/nightshift/issues with
   reproduction.
3. We cut a `vX.Y.Z+1` patch. Consumers widen the range when they're
   ready.

We don't unpublish releases — published charts and images stay in
ECR Public so anyone holding a pin keeps working.
