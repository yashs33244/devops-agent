locals {
  common_tags = {
    Service     = var.service_name
    Environment = var.environment
    ManagedBy   = "devops-agent"
    Terraform   = "true"
  }

  is_prod = var.environment == "prod"

  # Use larger instance types in prod; fall back to the variable value if explicitly overridden
  node_type = local.is_prod ? "m5.xlarge" : "t3.medium"

  # Single NAT gateway for dev (cost saving); one per AZ for prod (HA)
  single_nat_gateway = !local.is_prod
  one_nat_per_az     = local.is_prod
}
