variable "project_id" {
  description = "GCP project ID to deploy resources into"
  type        = string
  default     = "{{PROJECT_ID}}"
}

variable "region" {
  description = "GCP region to deploy resources into (e.g. us-central1, europe-west1)"
  type        = string
  default     = "{{REGION}}"
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
  description = "Name of the GKE Autopilot cluster"
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

variable "enable_cloud_sql" {
  description = "Set to true to provision a Cloud SQL PostgreSQL 16 instance alongside GKE"
  type        = bool
  default     = false
}

variable "enable_pubsub" {
  description = "Set to true to provision Pub/Sub topic and subscription for the service"
  type        = bool
  default     = false
}
