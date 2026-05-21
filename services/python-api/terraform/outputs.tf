output "cluster_name" {
  description = "Name of the EKS cluster"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "API server endpoint for the EKS cluster"
  value       = module.eks.cluster_endpoint
}

output "cluster_ca" {
  description = "Base64-encoded certificate authority data for the EKS cluster"
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "ecr_repository_url" {
  description = "URI of the ECR repository (use as the image registry)"
  value       = aws_ecr_repository.main.repository_url
}

output "kubeconfig_command" {
  description = "AWS CLI command to update local kubeconfig for this cluster"
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${var.cluster_name}"
}

output "app_role_arn" {
  description = "ARN of the IRSA IAM role assigned to the application workload"
  value       = aws_iam_role.app.arn
}

output "secret_arn" {
  description = "ARN of the Secrets Manager secret for application configuration"
  value       = aws_secretsmanager_secret.main.arn
}
