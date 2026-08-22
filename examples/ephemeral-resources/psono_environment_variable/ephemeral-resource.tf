ephemeral "psono_environment_variable" "database_password" {
  secret_id = psono_environment_variable.database_password.secret_id
  name      = psono_environment_variable.database_password.name
}

# Terraform 1.11+ and recent Kubernetes providers can transfer the value
# without persisting it in plan or state.
resource "kubernetes_secret_v1" "database" {
  metadata {
    name      = "database-credentials"
    namespace = "production"
  }

  data_wo = {
    DATABASE_PASSWORD = ephemeral.psono_environment_variable.database_password.value
  }
  data_wo_revision = psono_environment_variable.database_password.rotation_version
}
