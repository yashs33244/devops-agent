# ─── Required ────────────────────────────────────────────────────────────

variable "domain" {
  type        = string
  description = "Apex domain you control. The UI host defaults to <ui_subdomain>.<domain> (e.g. domain=example.com → nightshift.example.com). After apply, point that name at the ELB hostname `make eks-addons-up` prints."

  validation {
    condition     = can(regex("^[a-z0-9.-]+\\.[a-z]{2,}$", var.domain))
    error_message = "domain must be a bare apex like example.com (no scheme, no trailing slash, no subdomain prefix)."
  }
}

variable "letsencrypt_email" {
  type        = string
  description = "Email registered with Let's Encrypt. Used by the cert-manager ClusterIssuer that `make eks-addons-up` installs. Expiry warnings go here."

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.letsencrypt_email))
    error_message = "letsencrypt_email must look like a real email address."
  }
}

# ─── Optional with defaults ──────────────────────────────────────────────

variable "cluster_name" {
  type        = string
  description = "EKS cluster name. Also used as the Name tag on the VPC."
  default     = "nightshift"
}

variable "region" {
  type        = string
  description = "AWS region. Must support EKS + gp3."
  default     = "us-east-1"
}

variable "ui_subdomain" {
  type        = string
  description = "Subdomain for the UI host. Final UI host is <ui_subdomain>.<domain>."
  default     = "nightshift"
}

variable "k8s_version" {
  type        = string
  description = "EKS Kubernetes minor version. Must be one of the EKS-supported versions."
  default     = "1.31"
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR for the new VPC. /16 is plenty; the module carves /20 subnets out of it."
  default     = "10.0.0.0/16"
}

variable "node_instance_types" {
  type        = list(string)
  description = "Instance types for the managed node group. The first that has capacity in the AZ wins (Spot/On-Demand sharing if you use multiple)."
  default     = ["t3.medium"]
}

variable "node_min" {
  type        = number
  description = "Minimum nodes in the managed node group."
  default     = 2
}

variable "node_max" {
  type        = number
  description = "Maximum nodes in the managed node group. ASG scales between min and max as Pods request capacity."
  default     = 4
}

variable "node_desired" {
  type        = number
  description = "Desired node count at apply time. Subsequent scaling is left to the ASG."
  default     = 2
}

variable "ecr_repos" {
  type        = list(string)
  description = <<-DESC
    Private ECR repositories under the `nightshift/` namespace.
    Used by `make eks-quickstart` to push dev/SHA-tagged images for
    self-hosted bring-up. Tagged production releases (`vX.Y.Z`) go
    to ECR Public instead — see `var.ecr_public_repos`.
  DESC
  default = [
    "nightshift-api",
    "nightshift-worker",
    "nightshift-worker-claude",
    "nightshift-ui",
  ]
}

variable "ecr_public_repos" {
  type        = list(string)
  description = <<-DESC
    Public ECR repositories. Pulled anonymously by the community +
    by Argo CD across customer clusters. Release CI pushes here on
    `vX.Y.Z` tags. The OCI helm chart shares the namespace under
    `charts/nightshift`.
  DESC
  default = [
    "nightshift-api",
    "nightshift-worker",
    "nightshift-worker-claude",
    "nightshift-ui",
    "charts/nightshift",
  ]
}

variable "ecr_public_alias" {
  type        = string
  description = <<-DESC
    Custom ECR Public registry alias. URLs become
    `public.ecr.aws/<alias>/<repo>`. Aliases are one-per-account and
    registered via the AWS console (Console → ECR → Public registries
    → "Edit alias"; instant approval in practice). Terraform cannot
    create the alias itself; this var is informational + propagated
    to outputs for CI to consume.
  DESC
  default     = "nightshiftco"
}

variable "tags" {
  type        = map(string)
  description = "Extra tags applied to every resource."
  default     = {}
}
