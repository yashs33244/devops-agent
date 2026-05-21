##############################################################################
# Data sources
##############################################################################

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

##############################################################################
# VPC
##############################################################################

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.service_name}-${var.environment}"
  cidr = "10.0.0.0/16"

  azs             = slice(data.aws_availability_zones.available.names, 0, 2)
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway     = true
  single_nat_gateway     = local.single_nat_gateway
  one_nat_gateway_per_az = local.one_nat_per_az

  enable_dns_hostnames = true
  enable_dns_support   = true

  # Required tags for EKS subnet auto-discovery
  public_subnet_tags = {
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
    "kubernetes.io/role/elb"                    = "1"
  }

  private_subnet_tags = {
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
    "kubernetes.io/role/internal-elb"           = "1"
  }

  tags = local.common_tags
}

##############################################################################
# EKS
##############################################################################

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = "1.31"

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = true

  enable_irsa = true

  eks_managed_node_groups = {
    main = {
      name           = "${var.service_name}-ng"
      instance_types = [local.node_type]

      min_size     = var.node_min
      max_size     = var.node_max
      desired_size = var.node_min

      labels = {
        Service     = var.service_name
        Environment = var.environment
      }

      tags = local.common_tags
    }
  }

  tags = local.common_tags
}

##############################################################################
# ECR
##############################################################################

resource "aws_ecr_repository" "main" {
  name                 = "${var.service_name}-${var.environment}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = local.common_tags
}

resource "aws_ecr_lifecycle_policy" "main" {
  repository = aws_ecr_repository.main.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep only the last 10 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 10
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

##############################################################################
# IRSA — app workload role
##############################################################################

data "aws_iam_policy_document" "app_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    actions = ["sts:AssumeRoleWithWebIdentity"]

    condition {
      test     = "StringEquals"
      variable = "${replace(module.eks.cluster_oidc_issuer_url, "https://", "")}:sub"
      values   = ["system:serviceaccount:${var.service_name}:${var.service_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(module.eks.cluster_oidc_issuer_url, "https://", "")}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "app" {
  name               = "${var.service_name}-${var.environment}-app"
  assume_role_policy = data.aws_iam_policy_document.app_assume_role.json

  tags = local.common_tags
}

data "aws_iam_policy_document" "app_permissions" {
  # Minimal S3 access — restrict to a service-specific prefix
  statement {
    sid    = "S3Access"
    effect = "Allow"

    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:ListBucket",
    ]

    resources = [
      "arn:aws:s3:::${var.service_name}-${var.environment}-*",
      "arn:aws:s3:::${var.service_name}-${var.environment}-*/*",
    ]
  }

  # Allow the pod to read its own secrets only
  statement {
    sid    = "SecretsManagerAccess"
    effect = "Allow"

    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:DescribeSecret",
    ]

    resources = [
      aws_secretsmanager_secret.main.arn,
    ]
  }
}

resource "aws_iam_role_policy" "app" {
  name   = "${var.service_name}-${var.environment}-app-policy"
  role   = aws_iam_role.app.id
  policy = data.aws_iam_policy_document.app_permissions.json
}

##############################################################################
# Secrets Manager — placeholder secret
##############################################################################

resource "aws_secretsmanager_secret" "main" {
  name        = "${var.service_name}/${var.environment}/app"
  description = "Application secrets for ${var.service_name} (${var.environment}). Actual values must be populated by humans or CI — this resource is a placeholder."

  recovery_window_in_days = local.is_prod ? 30 : 0

  tags = local.common_tags
}

# NOTE: Do NOT store real secrets in Terraform state. This version sets an
# empty placeholder so the secret ARN is available before application
# deployment. Populate actual values via AWS Console, CLI, or CI pipeline.
resource "aws_secretsmanager_secret_version" "main_placeholder" {
  secret_id = aws_secretsmanager_secret.main.id

  secret_string = jsonencode({
    placeholder = "REPLACE_ME"
  })

  lifecycle {
    # Prevent Terraform from overwriting a secret that was set externally
    ignore_changes = [secret_string]
  }
}

##############################################################################
# RDS (conditional — enable_rds=true to provision)
##############################################################################

module "rds" {
  count   = var.enable_rds ? 1 : 0
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.0"

  identifier = "${var.service_name}-${var.environment}"

  engine                = "postgres"
  engine_version        = "16"
  instance_class        = local.is_prod ? "db.m5.large" : "db.t3.micro"
  allocated_storage     = local.is_prod ? 100 : 20
  max_allocated_storage = local.is_prod ? 500 : 100

  db_name  = replace(var.service_name, "-", "_")
  username = replace(var.service_name, "-", "_")

  # Delegate password management to RDS — rotated automatically
  manage_master_user_password = true

  multi_az               = local.is_prod
  db_subnet_group_name   = module.vpc.database_subnet_group_name
  vpc_security_group_ids = [aws_security_group.rds[0].id]

  backup_retention_period = local.is_prod ? 14 : 1
  deletion_protection     = local.is_prod

  skip_final_snapshot = !local.is_prod

  tags = local.common_tags
}

resource "aws_security_group" "rds" {
  count       = var.enable_rds ? 1 : 0
  name        = "${var.service_name}-${var.environment}-rds"
  description = "Allow PostgreSQL access from EKS node security group"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "PostgreSQL from EKS nodes"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.common_tags
}
