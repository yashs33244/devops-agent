# nightshift EKS bootstrap

Terraform that provisions the AWS infrastructure `make eks-quickstart`
expects: a VPC, an EKS cluster, a managed node group, the EBS CSI driver
addon (with IRSA), a `gp3` StorageClass, and four ECR repositories. The
in-cluster addons that actually serve traffic — ingress-nginx,
cert-manager, the `letsencrypt-prod` ClusterIssuer — are installed
separately by `make eks-addons-up`, not here.

## Cost warning

This stack runs **~$170/month** while up:

- EKS control plane: $73/mo
- 2× t3.medium on-demand: ~$60/mo
- NAT gateway: $32/mo + data transfer
- EBS volumes for the chart's StatefulSets: ~$5/mo at default sizes

Tear it down with `make eks-cluster-down` when you're not using it.

## State backend

This config has **no backend block** — Terraform falls back to local
state in `terraform.tfstate`. That's the lowest-friction default but it
has a real failure mode: **if you lose this file, you can't
`terraform destroy`**, and the cluster will keep charging you until you
clean it up by hand in the AWS console. Two ways to avoid that:

1. Back the directory up. Local state is still local state — commit it
   to a private repo, sync it to a private S3 prefix, whatever works.
2. Migrate to a real backend before you forget. Create an S3 bucket +
   DynamoDB lock table, drop a `backend.tf` with an `s3` block in this
   directory, then `terraform init -migrate-state`.

## Prerequisites

- `terraform` ≥ 1.7
- `aws` CLI authenticated to the account you want to deploy into
- `kubectl` ≥ 1.28
- `helm` ≥ 3.14
- A domain you control (Cloudflare, Route 53, anywhere) — the chart's UI
  ingress needs a real hostname so cert-manager's HTTP-01 challenge can
  succeed.

## Usage

```bash
cd deploy/eks/terraform
cat > terraform.tfvars <<EOF
domain            = "example.com"
letsencrypt_email = "you@example.com"
EOF

terraform init
terraform apply       # ~15 min. EKS control plane + node group is the slow part.
```

After apply, populate kubeconfig with the command Terraform prints as
`kubeconfig_cmd`:

```bash
aws eks update-kubeconfig --region us-east-1 --name nightshift
```

From here, drop back to the repo root and run `make eks-addons-up`,
create the CNAME it prints, then `make eks-quickstart`. Full walkthrough
in the top-level README.

## What gets created

| Resource | Purpose |
|---|---|
| VPC + 2 public + 2 private subnets across 2 AZs + NAT GW | Network for the cluster. Subnets are tagged so EKS picks them for ELB placement. |
| EKS cluster | Control plane + OIDC provider for IRSA. |
| Managed node group | 2× t3.medium (overridable). |
| EKS addons: vpc-cni, coredns, kube-proxy, aws-ebs-csi-driver | EBS CSI is the load-bearing one — the chart's PVCs need it. |
| IRSA role for EBS CSI controller | Without this, PVCs sit Pending forever. |
| `gp3` StorageClass (default) | EKS only ships `gp2`; the chart references `gp3`. The existing `gp2` default annotation is removed. |
| ECR repos: `nightshift/{nightshift-api,nightshift-worker,nightshift-worker-claude,nightshift-ui}` | Lifecycle policy keeps the most recent 20 tags. |

## What's *not* here

- ingress-nginx, cert-manager, the `letsencrypt-prod` ClusterIssuer —
  installed by `make eks-addons-up`.
- DNS records — you create the CNAME from `<ui_subdomain>.<domain>` to
  the ELB hostname `make eks-addons-up` prints. Bring your own provider.
- The application itself — installed by `make eks-quickstart`.

## Variables

| Name | Required | Default | Notes |
|---|---|---|---|
| `domain` | yes | — | Apex you control. |
| `letsencrypt_email` | yes | — | Email registered with Let's Encrypt. |
| `cluster_name` | no | `nightshift` | |
| `region` | no | `us-east-1` | Must support EKS + gp3. |
| `ui_subdomain` | no | `nightshift` | Final UI host: `<ui_subdomain>.<domain>`. |
| `k8s_version` | no | `1.31` | EKS-supported minor version. |
| `vpc_cidr` | no | `10.0.0.0/16` | |
| `node_instance_types` | no | `["t3.medium"]` | |
| `node_min` / `node_desired` / `node_max` | no | `2` / `2` / `4` | |
| `ecr_repos` | no | the four nightshift images | |

## Teardown

```bash
make eks-cluster-down
```

This runs `terraform destroy`. It will fail if there are LoadBalancer
Services or PVCs that still own AWS resources (ELBs, EBS volumes). Run
`helm uninstall nightshift -n nightshift && kubectl delete ns nightshift`
first, or `make eks-uninstall`.
