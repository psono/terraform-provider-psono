#!/bin/sh
set -eu

: "${PSONO_SERVER_URL:?PSONO_SERVER_URL is required}"
: "${PSONO_API_KEY_ID:?PSONO_API_KEY_ID is required}"
: "${PSONO_SECRET_ID:?PSONO_SECRET_ID is required}"

if [ ! -s /run/secrets/psono_api_secret_key ]; then
    echo "PSONO_API_SECRET_KEY is required" >&2
    exit 1
fi
PSONO_API_SECRET_KEY="$(cat /run/secrets/psono_api_secret_key)"
export PSONO_API_SECRET_KEY

export TF_CLI_CONFIG_FILE=/test/terraform.rc
export TF_IN_AUTOMATION=1
export TF_INPUT=0

prefix_base="${PSONO_TEST_PREFIX:-PSONO_TF_TEST}"
if ! printf '%s\n' "$prefix_base" | grep -Eq '^[A-Za-z_][A-Za-z0-9_]*$'; then
    echo "PSONO_TEST_PREFIX must be a POSIX environment-variable prefix" >&2
    exit 1
fi
suffix="$(date +%s)_$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
prefix="${prefix_base}_${suffix}"

export TF_VAR_secret_id="$PSONO_SECRET_ID"
export TF_VAR_allow_insecure_http="${PSONO_ALLOW_INSECURE_HTTP:-false}"
export TF_VAR_supplied_name="${prefix}_SUPPLIED"
export TF_VAR_copied_name="${prefix}_COPIED"
export TF_VAR_generated_name="${prefix}_GENERATED"
export TF_VAR_write_version=1
export TF_VAR_rotation_version=1
export TF_VAR_enable_copy=false

initial_value="compose_${suffix}_initial"
replacement_value="compose_${suffix}_replacement"
export TF_VAR_write_value="$initial_value"

initialized=0
claimed=0
destroyed=0
terraform_pid=""

run_terraform() {
    terraform "$@" &
    terraform_pid=$!
    set +e
    wait "$terraform_pid"
    result=$?
    set -e
    terraform_pid=""
    return "$result"
}

terminate() {
    signal="$1"
    trap - INT TERM
    if [ -n "$terraform_pid" ]; then
        child_pid="$terraform_pid"
        terraform_pid=""
        kill "-$signal" "$child_pid" >/dev/null 2>&1 || true
        remaining=10
        while kill -0 "$child_pid" >/dev/null 2>&1 && [ "$remaining" -gt 0 ]; do
            sleep 1
            remaining=$((remaining - 1))
        done
        if kill -0 "$child_pid" >/dev/null 2>&1; then
            kill -KILL "$child_pid" >/dev/null 2>&1 || true
        fi
        wait "$child_pid" >/dev/null 2>&1 || true
    fi
    if [ "$signal" = "INT" ]; then
        exit 130
    fi
    exit 143
}

cleanup_key() {
    key="$1"
    if ! psono-integration-verify delete --secret-id "$PSONO_SECRET_ID" --name "$key" >/dev/null 2>&1; then
        echo "WARNING: failed to clean up Psono test key $key" >&2
    fi
}

cleanup() {
    status=$?
    trap - EXIT INT TERM
    set +e
    if [ "$claimed" -eq 1 ]; then
        if [ "$initialized" -eq 1 ] && [ "$destroyed" -eq 0 ]; then
            if ! timeout -k 10s 120s terraform destroy -auto-approve -input=false >/dev/null 2>&1; then
                echo "WARNING: Terraform cleanup did not complete; attempting direct key cleanup" >&2
            fi
        fi
        cleanup_key "$TF_VAR_copied_name"
        cleanup_key "$TF_VAR_supplied_name"
        cleanup_key "$TF_VAR_generated_name"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'terminate INT' INT
trap 'terminate TERM' TERM

cp /test/main.tf ./main.tf
run_terraform init -input=false
initialized=1

psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_supplied_name"
psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_copied_name"
psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_generated_name"
claimed=1

run_terraform apply -auto-approve -input=false
EXPECTED_VALUE="$initial_value" psono-integration-verify present --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_supplied_name"
psono-integration-verify present --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_generated_name" --length 40 --min-lower 5 --min-upper 5 --min-numeric 5 --min-special 5
generated_hash="$(psono-integration-verify hash --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_generated_name")"

export TF_VAR_enable_copy=true
run_terraform apply -auto-approve -input=false
EXPECTED_VALUE="$initial_value" psono-integration-verify present --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_copied_name"

if grep -F "$initial_value" terraform.tfstate >/dev/null; then
    echo "The initial write-only value was stored in Terraform state" >&2
    exit 1
fi

export TF_VAR_write_value="$replacement_value"
export TF_VAR_write_version=2
export TF_VAR_rotation_version=2
export TF_VAR_enable_copy=false
run_terraform apply -auto-approve -input=false

EXPECTED_VALUE="$replacement_value" psono-integration-verify present --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_supplied_name"
psono-integration-verify changed --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_generated_name" --previous-hash "$generated_hash" --length 40 --min-lower 5 --min-upper 5 --min-numeric 5 --min-special 5
psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_copied_name"

export TF_VAR_enable_copy=true
run_terraform apply -auto-approve -input=false
EXPECTED_VALUE="$replacement_value" psono-integration-verify present --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_copied_name"

if grep -F "$initial_value" terraform.tfstate >/dev/null || grep -F "$replacement_value" terraform.tfstate >/dev/null; then
    echo "A write-only value was stored in Terraform state" >&2
    exit 1
fi

run_terraform destroy -auto-approve -input=false
destroyed=1
psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_supplied_name"
psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_copied_name"
psono-integration-verify absent --secret-id "$PSONO_SECRET_ID" --name "$TF_VAR_generated_name"
claimed=0

echo "Real Psono Terraform provider integration test succeeded"
