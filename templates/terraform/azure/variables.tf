variable "location" {
  description = "Azure region (location) to deploy resources into (e.g. eastus, westeurope)"
  type        = string
  default     = "{{LOCATION}}"
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
  description = "Name of the AKS cluster"
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

variable "node_vm_size" {
  description = "Azure VM size for AKS system node pool. Defaults to Standard_B2s for dev, Standard_D4s_v5 for prod."
  type        = string
  default     = "Standard_B2s"
}

variable "node_min" {
  description = "Minimum number of nodes in the AKS default node pool (auto-scaling)"
  type        = number
  default     = 1

  validation {
    condition     = var.node_min >= 1
    error_message = "node_min must be at least 1."
  }
}

variable "node_max" {
  description = "Maximum number of nodes in the AKS default node pool (auto-scaling)"
  type        = number
  default     = 5

  validation {
    condition     = var.node_max >= var.node_min
    error_message = "node_max must be greater than or equal to node_min."
  }
}

variable "enable_postgres" {
  description = "Set to true to provision an Azure PostgreSQL Flexible Server alongside AKS"
  type        = bool
  default     = false
}
