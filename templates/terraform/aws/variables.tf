variable "region" {
  description = "AWS region to deploy resources into"
  type        = string
  default     = "{{REGION}}"

  validation {
    condition = contains([
      "us-east-1", "us-east-2", "us-west-1", "us-west-2",
      "eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
      "ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2",
      "ap-south-1", "sa-east-1", "ca-central-1", "af-south-1",
      "me-south-1", "eu-south-1", "eu-north-1", "ap-east-1"
    ], var.region)
    error_message = "The region must be a valid AWS region (e.g. us-east-1, eu-west-1)."
  }
}

variable "service_name" {
  description = "Name of the service being deployed. Used as a prefix for all resource names."
  type        = string
  default     = "{{SERVICE_NAME}}"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]*[a-z0-9]$", var.service_name))
    error_message = "service_name must be lowercase alphanumeric characters and hyphens only, and must start and end with a letter or number."
  }
}

variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "{{CLUSTER_NAME}}"
}

variable "environment" {
  description = "Deployment environment (dev, staging, or prod)"
  type        = string
  default     = "{{ENVIRONMENT}}"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "node_instance_type" {
  description = "EC2 instance type for EKS worker nodes. Defaults to t3.medium for dev and m5.xlarge for prod."
  type        = string
  default     = "t3.medium"
}

variable "node_min" {
  description = "Minimum number of worker nodes in the EKS managed node group"
  type        = number
  default     = 1

  validation {
    condition     = var.node_min >= 1
    error_message = "node_min must be at least 1."
  }
}

variable "node_max" {
  description = "Maximum number of worker nodes in the EKS managed node group"
  type        = number
  default     = 5

  validation {
    condition     = var.node_max >= var.node_min
    error_message = "node_max must be greater than or equal to node_min."
  }
}

variable "enable_rds" {
  description = "Set to true to provision an RDS PostgreSQL instance alongside the EKS cluster"
  type        = bool
  default     = false
}

variable "enable_elasticache" {
  description = "Set to true to provision an ElastiCache Redis cluster alongside the EKS cluster"
  type        = bool
  default     = false
}
