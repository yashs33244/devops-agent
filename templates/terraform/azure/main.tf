##############################################################################
# Data sources
##############################################################################

data "azurerm_client_config" "current" {}

##############################################################################
# Resource Group
##############################################################################

resource "azurerm_resource_group" "main" {
  name     = local.resource_group_name
  location = var.location

  tags = local.common_tags
}

##############################################################################
# AKS Cluster
##############################################################################

resource "azurerm_kubernetes_cluster" "main" {
  name                = var.cluster_name
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = var.cluster_name
  kubernetes_version  = "1.31"

  azure_policy_enabled = true

  # SystemAssigned identity — AKS manages its own service principal lifecycle
  identity {
    type = "SystemAssigned"
  }

  default_node_pool {
    name                 = "system"
    vm_size              = local.node_size
    auto_scaling_enabled = true
    min_count            = var.node_min
    max_count            = var.node_max
    os_disk_size_gb      = 50

    node_labels = {
      Service     = var.service_name
      Environment = var.environment
    }
  }

  # Key Vault secrets provider — CSI driver for secret injection
  key_vault_secrets_provider {
    secret_rotation_enabled  = true
    secret_rotation_interval = "2m"
  }

  # KEDA via workload autoscaler profile
  workload_autoscaler_profile {
    keda_enabled = true
  }

  # Workload identity federation support
  workload_identity_enabled = true
  oidc_issuer_enabled       = true

  network_profile {
    network_plugin    = "azure"
    load_balancer_sku = "standard"
  }

  tags = local.common_tags
}

##############################################################################
# Azure Container Registry
##############################################################################

resource "azurerm_container_registry" "main" {
  name                = replace("${var.service_name}${var.environment}", "-", "")
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = local.acr_sku
  admin_enabled       = false

  tags = local.common_tags
}

# Grant AKS kubelet identity AcrPull on the registry
resource "azurerm_role_assignment" "aks_acr_pull" {
  principal_id                     = azurerm_kubernetes_cluster.main.kubelet_identity[0].object_id
  role_definition_name             = "AcrPull"
  scope                            = azurerm_container_registry.main.id
  skip_service_principal_aad_check = true
}

##############################################################################
# Key Vault (RBAC model — no legacy access policies)
##############################################################################

resource "azurerm_key_vault" "main" {
  name                       = "${var.service_name}-${var.environment}-kv"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  sku_name                   = "standard"
  purge_protection_enabled   = local.is_prod
  soft_delete_retention_days = local.is_prod ? 90 : 7

  rbac_authorization_enabled = true

  tags = local.common_tags
}

# Assign the current Terraform principal Key Vault Secrets Officer so it can
# write the placeholder secret below. Remove or restrict this in production.
resource "azurerm_role_assignment" "terraform_kv_officer" {
  principal_id         = data.azurerm_client_config.current.object_id
  role_definition_name = "Key Vault Secrets Officer"
  scope                = azurerm_key_vault.main.id
}

# NOTE: Do NOT store real secrets in Terraform state.
# This placeholder makes the secret name/URI available before deployment.
# Overwrite the value via the Azure Portal, CLI, or CI pipeline.
resource "azurerm_key_vault_secret" "main_placeholder" {
  name         = "${var.service_name}-app-secret"
  value        = "REPLACE_ME"
  key_vault_id = azurerm_key_vault.main.id

  tags = local.common_tags

  lifecycle {
    # Prevent Terraform from overwriting a secret populated externally
    ignore_changes = [value]
  }

  depends_on = [azurerm_role_assignment.terraform_kv_officer]
}

##############################################################################
# Workload Identity — user-assigned managed identity + federated credential
##############################################################################

resource "azurerm_user_assigned_identity" "app" {
  name                = "${var.service_name}-${var.environment}-app"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  tags = local.common_tags
}

# Allow the workload identity to read Key Vault secrets
resource "azurerm_role_assignment" "app_kv_reader" {
  principal_id         = azurerm_user_assigned_identity.app.principal_id
  role_definition_name = "Key Vault Secrets User"
  scope                = azurerm_key_vault.main.id
}

# Federate the managed identity to the Kubernetes service account
resource "azurerm_federated_identity_credential" "app" {
  name                = "${var.service_name}-${var.environment}-federated"
  resource_group_name = azurerm_resource_group.main.name
  audience            = ["api://AzureADTokenExchange"]
  issuer              = azurerm_kubernetes_cluster.main.oidc_issuer_url
  parent_id           = azurerm_user_assigned_identity.app.id
  subject             = "system:serviceaccount:${var.service_name}:${var.service_name}"
}

##############################################################################
# PostgreSQL Flexible Server (conditional — enable_postgres=true)
##############################################################################

resource "azurerm_postgresql_flexible_server" "main" {
  count               = var.enable_postgres ? 1 : 0
  name                = "${var.service_name}-${var.environment}-pg"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  version    = "16"
  sku_name   = local.is_prod ? "GP_Standard_D4s_v3" : "B_Standard_B1ms"
  storage_mb = local.is_prod ? 131072 : 32768

  # Password should be injected at plan time via an environment variable or
  # fetched from Key Vault. Never commit a real password here.
  administrator_login    = replace(var.service_name, "-", "_")
  administrator_password = "CHANGE_ME_BEFORE_APPLY"

  backup_retention_days        = local.is_prod ? 14 : 7
  geo_redundant_backup_enabled = local.is_prod

  zone = "1"

  authentication {
    active_directory_auth_enabled = true
    password_auth_enabled         = true
    tenant_id                     = data.azurerm_client_config.current.tenant_id
  }

  tags = local.common_tags
}
