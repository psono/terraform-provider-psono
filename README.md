# Psono Terraform Provider

The Psono Terraform Provider generates, reads, updates, rotates, and removes
keys inside pre-created Psono Environment Variables entries. It uses restricted
API keys and performs all decryption and encryption locally.

It intentionally does not create vault entries, datastores, or folders and does
not require an unrestricted API key.

## Requirements

- Terraform 1.11 or newer
- A pre-created Psono Environment Variables entry
- A restricted API key with the entry assigned
- Read permission for all operations
- Write permission for resource create, update, rotation, and deletion
- `allow_insecure_access` disabled

## Provider configuration

Prefer environment variables so credentials do not appear in Terraform files:

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

`PSONO_CA_BUNDLE` can contain an additional PEM CA bundle. TLS verification
cannot be disabled.

## Generate on demand

```terraform
resource "psono_environment_variable" "database_password" {
  secret_id = var.psono_environment_variables_secret_id
  name      = "DB_PASSWORD"

  length      = 32
  min_lower   = 4
  min_upper   = 4
  min_numeric = 4
  min_special = 4

  rotation_version = 1
}
```

On initial creation, the provider preserves an existing key or generates it if
missing. Increment `rotation_version` to rotate it. The generated value is never
returned by the managed resource and never enters Terraform state.

Destroy retains the Psono key by default, which prevents an adopted value from
being deleted unexpectedly. Set `deletion_policy = "Delete"` when Terraform
should explicitly remove the key.

## Supply a write-only value

```terraform
ephemeral "random_password" "database" {
  length = 32
}

resource "psono_environment_variable" "database_password" {
  secret_id        = var.psono_environment_variables_secret_id
  name             = "DB_PASSWORD"
  value_wo         = ephemeral.random_password.database.result
  value_wo_version = 1
}
```

Increment `value_wo_version` to write a replacement. Terraform discards
`value_wo` after the operation.

## Read without state

```terraform
ephemeral "psono_environment_variable" "database_password" {
  secret_id = psono_environment_variable.database_password.secret_id
  name      = psono_environment_variable.database_password.name
}
```

Use its sensitive `value` only with another write-only argument. For continuous
Kubernetes synchronization, use `psono-kubernetes-operator` instead; it updates
Kubernetes independently from Terraform runs.

## Development

```bash
make test
make build
```

### Integration test

The Compose integration test uses a real Psono Environment Variables entry. Use
a dedicated entry and a restricted API key with read and write permission. The
test creates uniquely named keys, verifies write-only and generated values,
rotates them, and removes them before exiting.

Export credentials locally. Do not commit them or paste them into logs or
support requests:

```bash
export PSONO_SERVER_URL='https://psono.example.com/server'
export PSONO_API_KEY_ID='...'
export PSONO_API_SECRET_KEY='...'
export PSONO_SECRET_ID='25070c66-8950-4264-9b39-11e6d83312e3'

make test-compose
```

Set `PSONO_CA_BUNDLE` when the server uses a private certificate authority.
If the test server deliberately uses unencrypted HTTP, also set
`PSONO_ALLOW_INSECURE_HTTP=true`; production provider configurations should
leave `allow_insecure_http` disabled.
`PSONO_TEST_PREFIX` can optionally change the environment-variable key prefix;
the test always appends a random 128-bit suffix. It checks that its generated
key names do not already exist and performs a direct cleanup attempt if
Terraform fails before destroy completes.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
