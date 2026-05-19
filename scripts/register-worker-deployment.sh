#!/usr/bin/env bash
# Register the Lambda worker as a Temporal Worker Deployment Version.
#
# Without this step, Temporal Cloud has no route to the Lambda and
# workflows started against the Lambda task queue are never picked up.
#
# Three CLI calls are made (idempotent across runs):
#   1. `temporal worker deployment create` — only on first run.
#   2. `temporal worker deployment create-version` — every new build ID.
#   3. `temporal worker deployment set-current-version` — every new build ID.
#
# Safe to invoke independently of deploy.sh; loads .env via lib/env.sh.

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
infra_dir="${repo_root}/infra"

# shellcheck disable=SC1091
source "${repo_root}/scripts/lib/env.sh"
load_env

command -v temporal >/dev/null || {
  echo "error: temporal CLI not found (brew install temporal)" >&2
  exit 1
}

# Deployment name is hardcoded to match cmd/worker/main.go:31
# (defaultDeploymentName). The Lambda worker registers itself under this
# name at runtime, so Temporal Cloud expects the same string here.
deployment_name="image-processing"

function_arn="$(tofu -chdir="${infra_dir}" output -raw worker_lambda_function_arn 2>/dev/null || true)"
invoker_role_arn="$(tofu -chdir="${infra_dir}" output -raw worker_lambda_invoker_role_arn 2>/dev/null || true)"

if [[ -z "${invoker_role_arn}" || "${invoker_role_arn}" == "null" ]]; then
  echo "==> Skipping: worker Lambda invoker role is not provisioned (Lambda runtime disabled)."
  exit 0
fi

if [[ -z "${TEMPORAL_CLOUD_EXTERNAL_ID:-}" ]]; then
  echo "==> Skipping: TEMPORAL_CLOUD_EXTERNAL_ID is unset (Lambda runtime disabled)."
  exit 0
fi

build_id="$(git -C "${repo_root}" rev-parse --short HEAD)"

temporal_auth_args=(
  --address "${TEMPORAL_ADDRESS}"
  --namespace "${TEMPORAL_NAMESPACE}"
)

if [[ -n "${TEMPORAL_TLS_CERT:-}" && -n "${TEMPORAL_TLS_KEY:-}" ]]; then
  cert_path="${TEMPORAL_TLS_CERT}"
  key_path="${TEMPORAL_TLS_KEY}"
  [[ "${cert_path}" != /* ]] && cert_path="${repo_root}/${cert_path}"
  [[ "${key_path}" != /* ]] && key_path="${repo_root}/${key_path}"
  temporal_auth_args+=(
    --tls-cert-path "${cert_path}"
    --tls-key-path "${key_path}"
  )
fi

echo "==> Ensuring deployment '${deployment_name}' exists"
if ! temporal worker deployment describe \
  "${temporal_auth_args[@]}" \
  --name "${deployment_name}" >/dev/null 2>&1; then
  temporal worker deployment create \
    "${temporal_auth_args[@]}" \
    --name "${deployment_name}"
fi

echo "==> Creating deployment version (build ${build_id})"
# create-version is idempotent in spirit but the CLI returns an error
# when the version already exists for this build ID. The set-current
# step below is what actually flips traffic to this version, so we
# tolerate the duplicate here.
temporal worker deployment create-version \
  "${temporal_auth_args[@]}" \
  --deployment-name "${deployment_name}" \
  --build-id "${build_id}" \
  --aws-lambda-function-arn "${function_arn}" \
  --aws-lambda-assume-role-arn "${invoker_role_arn}" \
  --aws-lambda-assume-role-external-id "${TEMPORAL_CLOUD_EXTERNAL_ID}" \
  || echo "    (assuming version already exists, continuing)"

echo "==> Setting current version to ${build_id}"
temporal worker deployment set-current-version \
  "${temporal_auth_args[@]}" \
  --deployment-name "${deployment_name}" \
  --build-id "${build_id}"
