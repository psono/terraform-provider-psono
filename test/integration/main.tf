terraform {
  required_version = ">= 1.11.0"

  required_providers {
    psono = {
      source  = "psono/psono"
      version = "0.0.1"
    }
  }
}

provider "psono" {
  allow_insecure_http = var.allow_insecure_http
}

variable "allow_insecure_http" {
  type    = bool
  default = false
}

variable "secret_id" {
  type = string
}

variable "supplied_name" {
  type = string
}

variable "copied_name" {
  type = string
}

variable "generated_name" {
  type = string
}

variable "write_value" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "write_version" {
  type = number
}

variable "rotation_version" {
  type = number
}

variable "enable_copy" {
  type = bool
}

resource "psono_environment_variable" "supplied" {
  secret_id        = var.secret_id
  name             = var.supplied_name
  value_wo         = var.write_value
  value_wo_version = var.write_version
  deletion_policy  = "Delete"
}

ephemeral "psono_environment_variable" "supplied" {
  count = var.enable_copy ? 1 : 0

  secret_id = var.secret_id
  name      = var.supplied_name

  depends_on = [psono_environment_variable.supplied]
}

resource "psono_environment_variable" "copied" {
  count = var.enable_copy ? 1 : 0

  secret_id        = var.secret_id
  name             = var.copied_name
  value_wo         = ephemeral.psono_environment_variable.supplied[0].value
  value_wo_version = var.write_version
  deletion_policy  = "Delete"
}

resource "psono_environment_variable" "generated" {
  secret_id = var.secret_id
  name      = var.generated_name

  length      = 40
  min_lower   = 5
  min_upper   = 5
  min_numeric = 5
  min_special = 5

  rotation_version = var.rotation_version
  deletion_policy  = "Delete"
}
