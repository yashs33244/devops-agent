output "cluster_name" {
  description = "Name of the AKS cluster"
  value       = azurerm_kubernetes_cluster.main.name
}

output "acr_login_server" {
  description = "Login server hostname for the Azure Container Registry"
  value       = azurerm_container_registry.main.login_server
}

output "resource_group" {
  description = "Name of the resource group containing all deployed resources"
  value       = azurerm_resource_group.main.name
}

output "kubeconfig_command" {
  description = "Azure CLI command to update local kubeconfig for this cluster"
  value       = "az aks get-credentials --resource-group ${azurerm_resource_group.main.name} --name ${var.cluster_name}"
}

output "key_vault_uri" {
  description = "URI of the Azure Key Vault (use as the vault_uri in CSI driver SecretProviderClass)"
  value       = azurerm_key_vault.main.vault_uri
}

output "workload_identity_client_id" {
  description = "Client ID of the user-assigned managed identity for workload identity federation"
  value       = azurerm_user_assigned_identity.app.client_id
}
