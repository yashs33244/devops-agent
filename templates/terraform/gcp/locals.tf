locals {
  # GCP labels must be lowercase; no uppercase letters allowed
  common_labels = {
    service     = var.service_name
    environment = var.environment
    managed_by  = "devops-agent"
    terraform   = "true"
  }

  is_prod = var.environment == "prod"

  # Cloud SQL tier: prod gets a small-standard instance; dev uses a micro
  sql_tier = local.is_prod ? "db-g1-small" : "db-f1-micro"
}
