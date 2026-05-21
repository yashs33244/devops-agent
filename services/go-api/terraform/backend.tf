# Remote state backend — Google Cloud Storage
#
# IMPORTANT: Run scripts/bootstrap-backend.sh to create the GCS bucket
# before running `terraform init`.
#
# The backend block does not support interpolation, so the bucket name must
# be set as a literal here or supplied via a backend config file
# (terraform init -backend-config=backend.hcl).
#
# Example backend.hcl:
#   bucket = "<service_name>-<environment>-tfstate"
#   prefix = "terraform/state"

terraform {
  backend "gcs" {
    # bucket = "<service_name>-<environment>-tfstate"
    prefix = "terraform/state"
  }
}
