terraform {
  required_version = ">= 1.11.0"
  required_providers {
    psono = {
      source  = "psono/psono"
      version = "~> 0.1"
    }
  }
}

# Prefer PSONO_SERVER_URL, PSONO_API_KEY_ID, and PSONO_API_SECRET_KEY.
provider "psono" {}
