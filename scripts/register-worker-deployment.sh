#!/usr/bin/env bash
# Register the Lambda worker as a Temporal Worker Deployment Version.
#
# Without this step, Temporal Cloud has no route to the Lambda and
# workflows started against the Lambda task queue are never picked up.
#
# Steps (all idempotent across runs):
#   1. `temporal worker deployment create` — only on first run.
#   2. Register/refresh the version for this build ID against the
#      current Lambda ARN (see the rebind note near step 2 below).
#   3. `temporal worker deployment set-current-version` — flips traffic.
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

# Deployment name comes from Tofu (worker-lambda module). The Lambda
# function's WORKER_DEPLOYMENT_NAME env var is set from the same Tofu
# expression, so the live worker and the registered deployment never drift.
# The Go default `defaultDeploymentName` in cmd/worker/main.go
# ("image-processing") only applies to local-dev runs where versioning is off.
deployment_name="$(tofu -chdir="${infra_dir}" output -raw worker_lambda_deployment_name 2>/dev/null || true)"
if [[ -z "${deployment_name}" ]]; then
  echo "error: tofu output worker_lambda_deployment_name is empty — has \`tofu apply\` been run?" >&2
  exit 1
fi

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

# --- Register/refresh this build's version against the current ARN ---
#
# A Worker Deployment Version is keyed by the git short SHA (build_id),
# which must equal the worker binary's compiled-in buildID. But the
# Lambda ARN we register is VERSION-QUALIFIED (…:function:NAME:N), and
# every `tofu apply` republishes the function (publish = true; secrets
# are re-injected as env vars and reserved concurrency churns), bumping
# N. So an infra-only redeploy with an unchanged SHA leaves the version
# keyed by that SHA still pointing at a STALE qualified ARN.
#
# The naive create-version-or-ignore form silently skipped the refresh,
# so Temporal Cloud kept the dead binding: the Lambda task queue lost
# all pollers, Lambda-runtime pipelines hung at the first workflow task,
# and `GET /api/pipelines/{id}` returned 500. There is no update-version
# command, so refreshing means delete + recreate. Do NOT collapse this
# back into create-or-ignore.
#
# create_version / rebind_version registers the version; the explicit
# set-current-version after this block is what flips traffic to it.
create_version() {
  temporal worker deployment create-version \
    "${temporal_auth_args[@]}" \
    --deployment-name "${deployment_name}" \
    --build-id "${build_id}" \
    --aws-lambda-function-arn "${function_arn}" \
    --aws-lambda-assume-role-arn "${invoker_role_arn}" \
    --aws-lambda-assume-role-external-id "${TEMPORAL_CLOUD_EXTERNAL_ID}"
}

rebind_version() {
  # delete-version refuses to drop the deployment's current version and
  # the CLI has no "unset current" command, so first park traffic on the
  # unversioned (pre-versioning) routing, then delete and recreate. These
  # workflows are short-lived, so the brief unversioned window is fine.
  #
  # DANGER WINDOW: this set-current-version --unversioned MUST stay first
  # and the deployment is unversioned until create_version below completes.
  # Do not reorder these steps casually — if the intervening delete-version
  # fails, the deployment is stranded unversioned (handled explicitly below).
  temporal worker deployment set-current-version \
    "${temporal_auth_args[@]}" \
    --deployment-name "${deployment_name}" \
    --unversioned --yes

  # delete-version has no --yes flag (it does not prompt); passing one aborts
  # the script mid-rebind. Bound it with --command-timeout so a version whose
  # Lambda binding is wedged fails fast instead of hanging (no timeout/gtimeout
  # on macOS). On failure the deployment is already unversioned, so surface a
  # loud, actionable error rather than exiting silently or faking success.
  if ! temporal worker deployment delete-version \
    "${temporal_auth_args[@]}" \
    --command-timeout 30s \
    --deployment-name "${deployment_name}" \
    --build-id "${build_id}" \
    --skip-drainage; then
    cat >&2 <<EOF
error: failed to delete worker deployment version '${build_id}'.

The deployment '${deployment_name}' has been left in --unversioned state and
version '${build_id}' could NOT be deleted — it may be wedged on the Temporal
Cloud side (an unreachable Lambda binding can make delete-version time out).

The Lambda runtime will NOT work until this is resolved. To recover, either:
  - roll forward with a NEW build id: make a new commit, rebuild and redeploy
    the worker Lambda, then re-run this script; or
  - contact Temporal Cloud support to clear version '${build_id}'.
EOF
    exit 1
  fi

  create_version
}

echo "==> Registering deployment version (build ${build_id})"
# Read the ARN currently bound to this build ID. describe-version can hang
# when the binding is already broken, so bound it with --command-timeout.
# Tolerate failure so `set -e` does not abort: capture stderr to tell a
# clean "not found" (version absent → create) apart from an error/timeout
# (binding unreadable → rebind, the safe default). On success, recursively
# pull the qualified Lambda ARN out of the JSON without depending on the
# (experimental, casing-prone) field path.
describe_status=0
describe_err=""
describe_json="$(temporal worker deployment describe-version \
  "${temporal_auth_args[@]}" \
  --command-timeout 15s \
  --deployment-name "${deployment_name}" \
  --build-id "${build_id}" \
  -o json 2>/tmp/describe-version.$$.err)" || describe_status=$?
describe_err="$(cat /tmp/describe-version.$$.err 2>/dev/null || true)"
rm -f /tmp/describe-version.$$.err

if (( describe_status == 0 )) && [[ -n "${describe_json}" ]]; then
  arn_pattern='^arn:aws:lambda:[^:]+:[0-9]+:function:[^:]+:[0-9]+$'
  registered_arn="$(printf '%s' "${describe_json}" \
    | jq -r --arg re "${arn_pattern}" \
        '[.. | strings | select(test($re))] | first // empty' 2>/dev/null || true)"

  if [[ -n "${registered_arn}" && "${registered_arn}" == "${function_arn}" ]]; then
    echo "    version in sync with ${function_arn} — no rebind needed"
  else
    echo "    registered ARN '${registered_arn:-unknown}' differs from ${function_arn} — rebinding"
    rebind_version
  fi
elif [[ "${describe_err}" == *"not found"* || "${describe_err}" == *"NotFound"* ]]; then
  echo "    version not found — creating against ${function_arn}"
  create_version
else
  # Describe errored or timed out: the binding may exist but be unreadable.
  # Rebind (delete + recreate) is the safe default — a plain create would
  # fail with "already exists" if the version is in fact still there.
  echo "    describe-version failed (binding unreadable) — rebinding against ${function_arn}"
  rebind_version
fi

echo "==> Setting current version to ${build_id}"
temporal worker deployment set-current-version \
  "${temporal_auth_args[@]}" \
  --deployment-name "${deployment_name}" \
  --build-id "${build_id}" \
  --yes
