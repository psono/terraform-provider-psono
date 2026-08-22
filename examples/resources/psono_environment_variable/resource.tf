resource "psono_environment_variable" "database_password" {
  secret_id = var.psono_environment_variables_secret_id
  name      = "DB_PASSWORD"

  length      = 32
  min_lower   = 4
  min_upper   = 4
  min_numeric = 4
  min_special = 4

  # Increment explicitly to rotate a generated value.
  rotation_version = 1
}

variable "psono_environment_variables_secret_id" {
  type = string
}
