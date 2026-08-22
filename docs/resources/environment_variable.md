---
page_title: "psono_environment_variable Resource"
---

# psono_environment_variable

Generates or writes one key in a pre-created Psono Environment Variables entry.
The Psono secret must already be assigned to a restricted API key with read and
write permission.

Generated and write-only values are not stored in Terraform state.

```terraform
resource "psono_environment_variable" "database_password" {
  secret_id = "25070c66-8950-4264-9b39-11e6d83312e3"
  name      = "DB_PASSWORD"
  length    = 32

  rotation_version = 1
}
```

If the key already exists on first creation, it is adopted without changing its
value. Increment `rotation_version` to rotate a provider-generated value.

For an externally supplied ephemeral value:

```terraform
ephemeral "random_password" "database" {
  length = 32
}

resource "psono_environment_variable" "database_password" {
  secret_id        = var.secret_id
  name             = "DB_PASSWORD"
  value_wo         = ephemeral.random_password.database.result
  value_wo_version = 1
}
```

Increment `value_wo_version` when `value_wo` should be written again.

## Arguments

- `secret_id`: UUID of the pre-created Environment Variables entry.
- `name`: POSIX environment variable key.
- `value_wo`: Optional write-only value. Omit to generate when absent.
- `value_wo_version`: Write trigger for `value_wo`.
- `rotation_version`: Rotation trigger for generated values. Defaults to `1`.
- `length`: Generated length. Defaults to `32`.
- `lower`, `upper`, `numeric`, `special`: Enabled generated character classes.
- `min_lower`, `min_upper`, `min_numeric`, `min_special`: Required class counts.
- `special_characters`: Allowed special characters.
- `deletion_policy`: `Retain` keeps the key on destroy and is the default;
  `Delete` removes it explicitly.

## Attributes

- `id`: `<secret-id>:<name>`.
- `write_date`: Psono entry modification timestamp.

## Import

```bash
terraform import psono_environment_variable.database_password \
  '25070c66-8950-4264-9b39-11e6d83312e3:DB_PASSWORD'
```
