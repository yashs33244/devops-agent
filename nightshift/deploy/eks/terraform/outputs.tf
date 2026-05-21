output "cluster_arn" {
  description = "EKS cluster ARN. Consumed by Makefile EKS_CLUSTER_ARN."
  value       = module.eks.cluster_arn
}

output "cluster_name" {
  description = "EKS cluster name."
  value       = module.eks.cluster_name
}

output "region" {
  description = "AWS region the cluster lives in. Consumed by Makefile EKS_REGION."
  value       = var.region
}

output "account_id" {
  description = "AWS account id. Consumed by Makefile EKS_ACCOUNT."
  value       = data.aws_caller_identity.current.account_id
}

output "ecr_registry" {
  description = "Private ECR registry hostname. Consumed by Makefile EKS_REGISTRY (without the `/nightshift` namespace suffix). Used by `make eks-quickstart` for SHA-tagged dev images."
  value       = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.region}.amazonaws.com"
}

output "ecr_public_registry" {
  description = "ECR Public registry hostname. Release CI pushes vX.Y.Z-tagged images and the helm chart here. Anonymous pulls."
  value       = "public.ecr.aws/${var.ecr_public_alias}"
}

output "ecr_public_alias" {
  description = "ECR Public alias (must be registered out-of-band via the AWS Console — one-per-account)."
  value       = var.ecr_public_alias
}

output "ui_host" {
  description = "Hostname the chart binds the UI ingress to. Consumed by Makefile EKS_UI_HOST. Operator must CNAME this to the ELB hostname `make eks-addons-up` prints."
  value       = local.ui_host
}

output "letsencrypt_email" {
  description = "Email registered with the cert-manager ClusterIssuer."
  value       = var.letsencrypt_email
}

output "kubeconfig_cmd" {
  description = "Shell command to populate ~/.kube/config with this cluster's context. Run after apply."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name}"
}
