#!/usr/bin/env bash
# Deploy the demo to AWS.
#
# Steps:
#   1. Load .env if present, then check required
#      env vars.
#   2. Build the Lambda bootstrap binary.
#   3. tofu init && tofu apply.
#   4. Build the Nuxt frontend.
#   5. Sync to S3 and invalidate CloudFront.
#   6. Print the demo URL.
#
# Set INTERACTIVE=0 to skip the tofu approval
# prompt (useful in CI). Default is interactive.

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${repo_root}/.env"
infra_dir="${repo_root}/infra"
frontend_dir="${repo_root}/frontend"

if [[ -f "${env_file}" ]]; then
  echo "Loading environment from ${env_file}"
  set -a
  # shellcheck disable=SC1090
  . "${env_file}"
  set +a
fi

required_vars=(
  ANTHROPIC_API_KEY
  CLOUDFLARE_API_TOKEN
  CLOUDFLARE_ZONE_ID
  TEMPORAL_ADDRESS
  TEMPORAL_NAMESPACE
)

missing=()
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    missing+=("${var}")
  fi
done

if (( ${#missing[@]} > 0 )); then
  echo "error: missing required environment variables:" >&2
  for var in "${missing[@]}"; do
    echo "  - ${var}" >&2
  done
  echo "Set them in ${env_file} or export them." >&2
  exit 1
fi

interactive="${INTERACTIVE:-1}"
tofu_apply_args=()
if [[ "${interactive}" == "0" ]]; then
  tofu_apply_args+=(-auto-approve)
fi

echo "==> Building Lambda bootstrap"
"${repo_root}/scripts/build-lambda.sh"

echo "==> Provisioning infra with OpenTofu"
tofu -chdir="${infra_dir}" init
tofu -chdir="${infra_dir}" apply "${tofu_apply_args[@]}"

echo "==> Building frontend"
pnpm -C "${frontend_dir}" install --frozen-lockfile

# Inject the real images bucket name so the static site targets the
# right S3 bucket in prod (the local dev bucket name is hardcoded
# elsewhere — see ControlPanel.vue).
NUXT_PUBLIC_API_BASE="" \
NUXT_PUBLIC_SAMPLES_BUCKET="$(tofu -chdir="${infra_dir}" output -raw images_bucket)" \
  pnpm -C "${frontend_dir}" generate

frontend_bucket="$(tofu -chdir="${infra_dir}" output -raw frontend_bucket)"
distribution_id="$(tofu -chdir="${infra_dir}" output -raw cloudfront_distribution_id)"

echo "==> Syncing frontend to s3://${frontend_bucket}"
aws s3 sync "${frontend_dir}/.output/public/" \
  "s3://${frontend_bucket}/" --delete

echo "==> Invalidating CloudFront (${distribution_id})"
aws cloudfront create-invalidation \
  --distribution-id "${distribution_id}" \
  --paths '/*' >/dev/null

demo_url="$(tofu -chdir="${infra_dir}" output -raw demo_url)"
echo
echo "Deployment complete."
echo "Demo URL: ${demo_url}"
