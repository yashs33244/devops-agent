# Remote state backend — Azure Blob Storage
#
# IMPORTANT: Run scripts/bootstrap-backend.sh to create the storage account
# and container before running `terraform init`.
#
# The backend block does not support interpolation, so values must be set
# as literals here or via a backend config file
# (terraform init -backend-config=backend.hcl).
#
# Example backend.hcl:
#   resource_group_name  = "<service_name>-<environment>-tfstate-rg"
#   storage_account_name = "<service_name><environment>tfstate"
#   container_name       = "tfstate"
#   key                  = "terraform.tfstate"

terraform {
  backend "azurerm" {
    # resource_group_name  = "<service_name>-<environment>-tfstate-rg"
    # storage_account_name = "<service_name><environment>tfstate"
    # container_name       = "tfstate"
    key                  = "terraform.tfstate"
  }
}
