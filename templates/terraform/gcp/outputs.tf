output "cluster_name" {
  description = "Name of the GKE Autopilot cluster"
  value       = google_container_cluster.main.name
}

output "artifact_registry_url" {
  description = "Full URL of the Artifact Registry Docker repository (use as image prefix)"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.main.repository_id}"
}

output "sa_email" {
  description = "Email address of the GCP service account used for workload identity"
  value       = google_service_account.app.email
}

output "kubeconfig_command" {
  description = "gcloud CLI command to update local kubeconfig for this cluster"
  value       = "gcloud container clusters get-credentials ${var.cluster_name} --region ${var.region} --project ${var.project_id}"
}

output "secret_name" {
  description = "Full resource name of the Secret Manager secret"
  value       = google_secret_manager_secret.main.name
}
