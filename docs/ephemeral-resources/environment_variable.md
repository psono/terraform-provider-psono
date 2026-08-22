---
page_title: "psono_environment_variable Ephemeral Resource"
---

# psono_environment_variable

Reads one key from a Psono Environment Variables entry without persisting the
value in Terraform plan or state.

```terraform
ephemeral "psono_environment_variable" "database_password" {
  secret_id = var.secret_id
  name      = "DB_PASSWORD"
}
```

The sensitive `value` attribute can only be used in an ephemeral context, such
as another provider's write-only argument.
