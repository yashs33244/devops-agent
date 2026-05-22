# Remote state backend — S3 + DynamoDB locking
#
# IMPORTANT: Run scripts/bootstrap-backend.sh to create the S3 bucket and
# DynamoDB table before running `terraform init`.
#
# The backend block does not support interpolation, so the values below must
# be set as literals or supplied via a backend config file
# (terraform init -backend-config=backend.hcl).
#
# Example backend.hcl:
#   bucket         = "<service_name>-<environment>-tfstate"
#   key            = "terraform.tfstate"
#   region         = "<region>"
#   dynamodb_table = "<service_name>-<environment>-tflock"
#   encrypt        = true

terraform {
  backend "s3" {
    # bucket         = "<service_name>-<environment>-tfstate"
    key     = "terraform.tfstate"
    # region         = "<region>"
    # dynamodb_table = "<service_name>-<environment>-tflock"
    encrypt = true
  }
}
