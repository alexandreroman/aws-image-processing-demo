# Sourceable helper: loads .env, scrubs dev-only overlay vars,
# validates required values, and maps them to TF_VAR_* names.
#
# Usage from a deploy/teardown script:
#   source "${repo_root}/scripts/lib/env.sh"
#   load_env
#
# Uses `return 1` on error so sourcing in an interactive shell
# does not kill the session; the deploy scripts run with `set -e`
# so the return still aborts them.

load_env() {
  # Resolve the repo root by walking up from PWD until we find a .env.
  # Works regardless of whether the caller is bash or zsh — BASH_SOURCE
  # isn't set when sourced from a zsh interactive shell.
  local repo_root="${PWD}"
  while [[ ! -f "${repo_root}/.env" && "${repo_root}" != "/" ]]; do
    repo_root="$(dirname "${repo_root}")"
  done
  local env_file="${repo_root}/.env"

  # Protect against dev-overlay leakage: if a parent shell exported
  # values from .env.local (Moto endpoint, fake credentials, fixed
  # bucket/table names), unset them so the AWS SDK falls back to the
  # user's profile / SSO chain and Tofu sees the real resource names.
  unset AWS_ENDPOINT_URL AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY \
        AWS_SESSION_TOKEN IMAGES_BUCKET IMAGES_TABLE

  if [[ ! -f "${env_file}" ]]; then
    echo "error: ${env_file} not found." >&2
    echo "Copy .env.example to .env and fill in the values." >&2
    return 1
  fi

  set -a
  # shellcheck disable=SC1090
  . "${env_file}"
  set +a

  local required=(TEMPORAL_ADDRESS TEMPORAL_NAMESPACE ANTHROPIC_API_KEY)
  local missing=()
  local var _check
  for var in "${required[@]}"; do
    eval "_check=\${${var}:-}"
    if [[ -z "${_check}" ]]; then
      missing+=("${var}")
    fi
  done
  if (( ${#missing[@]} > 0 )); then
    echo "error: missing required variables in ${env_file}:" >&2
    for var in "${missing[@]}"; do
      echo "  - ${var}" >&2
    done
    return 1
  fi

  local mtls_enabled=0
  local cert_set="${TEMPORAL_TLS_CERT:-}"
  local key_set="${TEMPORAL_TLS_KEY:-}"
  if [[ -n "${cert_set}" && -n "${key_set}" ]]; then
    local cert_path="${cert_set}"
    local key_path="${key_set}"
    [[ "${cert_path}" != /* ]] && cert_path="${repo_root}/${cert_path}"
    [[ "${key_path}" != /* ]] && key_path="${repo_root}/${key_path}"
    if [[ ! -r "${cert_path}" ]]; then
      echo "error: TEMPORAL_TLS_CERT not readable: ${cert_path}" >&2
      return 1
    fi
    if [[ ! -r "${key_path}" ]]; then
      echo "error: TEMPORAL_TLS_KEY not readable: ${key_path}" >&2
      return 1
    fi
    TF_VAR_temporal_tls_cert_pem="$(cat "${cert_path}")"
    TF_VAR_temporal_tls_key_pem="$(cat "${key_path}")"
    export TF_VAR_temporal_tls_cert_pem TF_VAR_temporal_tls_key_pem
    mtls_enabled=1
  elif [[ -n "${cert_set}" || -n "${key_set}" ]]; then
    echo "error: TEMPORAL_TLS_CERT and TEMPORAL_TLS_KEY must be set together — provide both or neither." >&2
    return 1
  fi

  export TF_VAR_temporal_address="${TEMPORAL_ADDRESS}"
  export TF_VAR_temporal_namespace="${TEMPORAL_NAMESPACE}"
  export TF_VAR_anthropic_api_key="${ANTHROPIC_API_KEY}"
  # Optional: when unset, Tofu sees var.temporal_metrics_api_key == "" and
  # skips the entire ECS worker autoscaling stack (ADOT collector + alarms
  # + scaling policies).
  [[ -n "${TEMPORAL_METRICS_API_KEY:-}" ]] && export TF_VAR_temporal_metrics_api_key="${TEMPORAL_METRICS_API_KEY}"
  [[ -n "${AWS_REGION:-}" ]] && export TF_VAR_aws_region="${AWS_REGION}"
  [[ -n "${WORKER_IMAGE:-}" ]] && export TF_VAR_worker_image="${WORKER_IMAGE}"
  [[ -n "${WORKER_MAX_CONCURRENT_ACTIVITIES:-}" ]] && export TF_VAR_worker_max_concurrent_activities="${WORKER_MAX_CONCURRENT_ACTIVITIES}"
  [[ -n "${WORKER_LAMBDA_MAX_INSTANCES:-}" ]] && export TF_VAR_worker_lambda_max_instances="${WORKER_LAMBDA_MAX_INSTANCES}"
  [[ -n "${WORKER_ECS_MAX_INSTANCES:-}" ]] && export TF_VAR_worker_ecs_max_instances="${WORKER_ECS_MAX_INSTANCES}"
  [[ -n "${TEMPORAL_CLOUD_EXTERNAL_ID:-}" ]] && export TF_VAR_temporal_cloud_external_id="${TEMPORAL_CLOUD_EXTERNAL_ID}"

  local custom_domain=0
  if [[ -n "${DOMAIN_NAME:-}" ]]; then
    if [[ -z "${CLOUDFLARE_API_TOKEN:-}" || -z "${CLOUDFLARE_ZONE_ID:-}" ]]; then
      echo "error: custom domain requires CLOUDFLARE_API_TOKEN and CLOUDFLARE_ZONE_ID." >&2
      return 1
    fi
    export TF_VAR_domain_name="${DOMAIN_NAME}"
    [[ -n "${SUBDOMAIN:-}" ]] && export TF_VAR_subdomain="${SUBDOMAIN}"
    export TF_VAR_cloudflare_zone_id="${CLOUDFLARE_ZONE_ID}"
    custom_domain=1
  fi

  local banner="Loaded .env (AWS deploy mode)"
  (( mtls_enabled )) && banner+=" (mTLS)"
  (( custom_domain )) && banner+=" (custom domain)"

  # Optional AWS identity check — silently skips if AWS CLI is missing
  # or credentials aren't usable. Helps catch wrong-account deploys.
  local _aws_who
  if _aws_who="$(aws sts get-caller-identity --query '[Account, Arn]' --output text 2>/dev/null)"; then
    local _account _arn _who
    _account="$(printf '%s' "${_aws_who}" | cut -f1)"
    _arn="$(printf '%s' "${_aws_who}" | cut -f2)"
    # `${arn##*/}` extracts the trailing principal name from an assumed-role
    # ARN (e.g. `.../AWSReservedSSO_.../alice@example.com` → `alice@example.com`)
    # or the user name from a long-term-IAM-user ARN.
    _who="${_arn##*/}"
    banner+=" — AWS ${_account} (${_who})"
  fi
  echo "${banner}"
}
