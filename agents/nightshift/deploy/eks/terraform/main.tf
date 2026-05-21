data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

locals {
  azs = slice(data.aws_availability_zones.available.names, 0, 2)

  ui_host = "${var.ui_subdomain}.${var.domain}"

  common_tags = merge(var.tags, {
    "managed-by"   = "terraform"
    "module"       = "nightshift-eks"
    "cluster-name" = var.cluster_name
  })
}

# ─── VPC ─────────────────────────────────────────────────────────────────
# Two AZs, public + private subnets, single NAT gateway. The subnet tags
# the EKS module needs (`kubernetes.io/role/elb` etc.) are applied by the
# vpc module when `enable_nat_gateway` + the subnet inputs are set, but
# we still tag explicitly so internal/external load-balancer placement is
# deterministic.
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.13"

  name = var.cluster_name
  cidr = var.vpc_cidr

  azs             = local.azs
  private_subnets = [for i, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, i)]
  public_subnets  = [for i, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, i + 8)]

  enable_nat_gateway     = true
  single_nat_gateway     = true
  enable_dns_hostnames   = true
  enable_dns_support     = true
  map_public_ip_on_launch = false

  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
  }

  tags = local.common_tags
}

# ─── EKS cluster ─────────────────────────────────────────────────────────
# OIDC provider auto-created (used for IRSA — required by aws-ebs-csi).
# Public + private endpoints so the operator can talk to the API server
# from their laptop without a bastion. Restrict `cluster_endpoint_public_access_cidrs`
# in production.
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.31"

  cluster_name    = var.cluster_name
  cluster_version = var.k8s_version

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access  = true
  cluster_endpoint_private_access = true

  enable_cluster_creator_admin_permissions = true

  cluster_addons = {
    coredns                = {}
    kube-proxy             = {}
    vpc-cni                = {}
    aws-ebs-csi-driver     = {
      service_account_role_arn = module.ebs_csi_irsa.iam_role_arn
    }
  }

  eks_managed_node_groups = {
    default = {
      ami_type       = "AL2023_x86_64_STANDARD"
      instance_types = var.node_instance_types
      min_size       = var.node_min
      max_size       = var.node_max
      desired_size   = var.node_desired

      labels = {
        "nightshift.io/role" = "default"
      }
    }
  }

  tags = local.common_tags
}

# ─── IRSA for the EBS CSI driver ─────────────────────────────────────────
# The managed addon needs an IAM role assumable by the CSI controller's
# ServiceAccount. Without this, PVCs sit in Pending forever.
module "ebs_csi_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  role_name             = "${var.cluster_name}-ebs-csi"
  attach_ebs_csi_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }

  tags = local.common_tags
}

# ─── gp3 StorageClass ────────────────────────────────────────────────────
# EKS ships only `gp2` by default. The chart references `gp3` explicitly
# (`postgres.storage.storageClassName=gp3`, etc.), so we create it. Marked
# default so any chart values that omit `storageClassName` also land on
# gp3. The existing gp2 default annotation is removed via local-exec
# (kubernetes_storage_class doesn't support patching pre-existing SCs).
resource "kubernetes_storage_class_v1" "gp3" {
  metadata {
    name = "gp3"
    annotations = {
      "storageclass.kubernetes.io/is-default-class" = "true"
    }
  }

  storage_provisioner    = "ebs.csi.aws.com"
  reclaim_policy         = "Delete"
  volume_binding_mode    = "WaitForFirstConsumer"
  allow_volume_expansion = true

  parameters = {
    type      = "gp3"
    encrypted = "true"
    fsType    = "ext4"
  }

  depends_on = [module.eks]
}

resource "null_resource" "unset_gp2_default" {
  triggers = {
    cluster_name = module.eks.cluster_name
  }

  provisioner "local-exec" {
    command = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name} --kubeconfig /tmp/kubeconfig-${module.eks.cluster_name} >/dev/null && KUBECONFIG=/tmp/kubeconfig-${module.eks.cluster_name} kubectl annotate storageclass gp2 storageclass.kubernetes.io/is-default-class- --overwrite || true"
  }

  depends_on = [kubernetes_storage_class_v1.gp3]
}

# ─── ECR repositories ────────────────────────────────────────────────────
# Private repos for `make eks-quickstart` SHA-tagged images. Production
# tagged releases (`vX.Y.Z`) push to the ECR Public repos below instead.
# Lifecycle policy keeps the most recent 20 tags so the repo doesn't
# grow unbounded as `make eks-quickstart` is re-run.
resource "aws_ecr_repository" "this" {
  for_each = toset(var.ecr_repos)

  name                 = "nightshift/${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = local.common_tags
}

# ─── ECR Public repositories ─────────────────────────────────────────────
# Anonymous-pull artifacts under `public.ecr.aws/<alias>/<repo>`. The
# alias itself is registered out-of-band (AWS Console; one-per-account).
# ECR Public is a single global service — its API only lives in
# us-east-1, so we use a region-pinned alias provider rather than the
# default `aws` provider so this works regardless of var.aws_region.
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

resource "aws_ecrpublic_repository" "this" {
  provider = aws.us_east_1
  for_each = toset(var.ecr_public_repos)

  repository_name = each.key

  catalog_data {
    description = "nightshift ${each.key}"
    about_text  = "https://github.com/nightshiftco/nightshift"
  }
}

resource "aws_ecr_lifecycle_policy" "this" {
  for_each = aws_ecr_repository.this

  repository = each.value.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep last 20 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 20
      }
      action = { type = "expire" }
    }]
  })
}
