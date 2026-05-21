locals {
  common_tags = {
    Service     = var.service_name
    Environment = var.environment
    ManagedBy   = "devops-agent"
    Terraform   = "true"
  }

  is_prod = var.environment == "prod"

  # VM size: use prod-grade nodes in prod, burstable for dev/staging
  node_size = local.is_prod ? "Standard_D4s_v5" : "Standard_B2s"

  # ACR SKU: Standard provides geo-replication and content trust in prod
  acr_sku = local.is_prod ? "Standard" : "Basic"

  # Resource group name derived consistently from service + environment
  resource_group_name = "${var.service_name}-${var.environment}-rg"
}
