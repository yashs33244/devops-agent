##############################################################################
# Enable required GCP APIs
##############################################################################

resource "google_project_service" "container" {
  project            = var.project_id
  service            = "container.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "artifactregistry" {
  project            = var.project_id
  service            = "artifactregistry.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "sqladmin" {
  project            = var.project_id
  service            = "sqladmin.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "secretmanager" {
  project            = var.project_id
  service            = "secretmanager.googleapis.com"
  disable_on_destroy = false
}

##############################################################################
# GKE Autopilot cluster
##############################################################################

resource "google_container_cluster" "main" {
  name     = var.cluster_name
  project  = var.project_id
  location = var.region

  # Autopilot manages node provisioning and scaling automatically
  enable_autopilot = true

  release_channel {
    channel = "REGULAR"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  resource_labels = local.common_labels

  depends_on = [google_project_service.container]
}

##############################################################################
# Artifact Registry — Docker repository
##############################################################################

resource "google_artifact_registry_repository" "main" {
  project       = var.project_id
  location      = var.region
  repository_id = "${var.service_name}-${var.environment}"
  description   = "Docker images for ${var.service_name} (${var.environment})"
  format        = "DOCKER"

  labels = local.common_labels

  depends_on = [google_project_service.artifactregistry]
}

##############################################################################
# Service Account for the application workload
##############################################################################

resource "google_service_account" "app" {
  project      = var.project_id
  account_id   = "${var.service_name}-${var.environment}-app"
  display_name = "${var.service_name} (${var.environment}) app workload SA"
}

# Allow the SA to pull images from Artifact Registry
resource "google_project_iam_member" "app_artifact_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.app.email}"
}

# Allow the SA to access Secret Manager secrets
resource "google_project_iam_member" "app_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.app.email}"
}

# Bind the GCP SA to the Kubernetes SA via Workload Identity
resource "google_service_account_iam_binding" "workload_identity_user" {
  service_account_id = google_service_account.app.name
  role               = "roles/iam.workloadIdentityUser"

  members = [
    "serviceAccount:${var.project_id}.svc.id.goog[${var.service_name}/${var.service_name}]",
  ]
}

##############################################################################
# Secret Manager — placeholder secret
##############################################################################

resource "google_secret_manager_secret" "main" {
  project   = var.project_id
  secret_id = "${var.service_name}-${var.environment}-app"

  replication {
    auto {}
  }

  labels = local.common_labels

  depends_on = [google_project_service.secretmanager]
}

# NOTE: Do NOT store real secrets in Terraform state.
# This placeholder makes the secret name available before deployment.
# Overwrite the value via gcloud CLI or CI pipeline.
resource "google_secret_manager_secret_version" "main_placeholder" {
  secret      = google_secret_manager_secret.main.id
  secret_data = "REPLACE_ME"

  lifecycle {
    # Prevent Terraform from overwriting a secret populated externally
    ignore_changes = [secret_data]
  }
}

##############################################################################
# Cloud SQL — PostgreSQL 16 (conditional — enable_cloud_sql=true)
##############################################################################

resource "google_sql_database_instance" "main" {
  count            = var.enable_cloud_sql ? 1 : 0
  project          = var.project_id
  name             = "${var.service_name}-${var.environment}-pg"
  database_version = "POSTGRES_16"
  region           = var.region

  settings {
    tier              = local.sql_tier
    availability_type = local.is_prod ? "REGIONAL" : "ZONAL"
    disk_autoresize   = true
    disk_size         = local.is_prod ? 100 : 10

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = local.is_prod
      backup_retention_settings {
        retained_backups = local.is_prod ? 14 : 3
      }
    }

    insights_config {
      query_insights_enabled = local.is_prod
    }

    user_labels = local.common_labels
  }

  deletion_protection = local.is_prod

  depends_on = [google_project_service.sqladmin]
}

##############################################################################
# Pub/Sub topic + subscription (conditional — enable_pubsub=true)
##############################################################################

resource "google_pubsub_topic" "main" {
  count   = var.enable_pubsub ? 1 : 0
  project = var.project_id
  name    = "${var.service_name}-${var.environment}-events"

  labels = local.common_labels
}

resource "google_pubsub_subscription" "main" {
  count   = var.enable_pubsub ? 1 : 0
  project = var.project_id
  name    = "${var.service_name}-${var.environment}-events-sub"
  topic   = google_pubsub_topic.main[0].name

  ack_deadline_seconds       = 30
  message_retention_duration = local.is_prod ? "604800s" : "86400s" # 7d prod, 1d dev

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  labels = local.common_labels
}
