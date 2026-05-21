provider "aws" {
  region = var.region
}

# Kubernetes provider is configured against the cluster this module
# creates. exec-based auth avoids storing a long-lived kubeconfig in
# state. The aws CLI must be on PATH at apply time.
provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name, "--region", var.region]
  }
}
