---
page_title: "Psono Provider"
description: |-
  The Psono provider manages keys inside pre-created Psono Environment Variables entries.
---

# Psono Provider

The Psono provider generates, reads, updates, rotates, and removes keys inside
pre-created Psono Environment Variables entries. It uses restricted API keys
and performs all encryption and decryption locally.

Use environment variables to configure credentials without placing them in
Terraform configuration:

```bash
export PSONO_SERVER_URL='https://psono.example.com/server'
export PSONO_API_KEY_ID='...'
export PSONO_API_SECRET_KEY='...'
```

```terraform
terraform {
  required_version = ">= 1.11.0"

  required_providers {
    psono = {
      source = "psono/psono"
    }
  }
}

provider "psono" {}
```

The restricted API key needs read access to every assigned entry. Resource
writes, rotation, and deletion additionally require write access.
