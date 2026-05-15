#!/usr/bin/env bash
# Deploy the demo to AWS.
#
# Steps:
#   1. Load .env and validate required vars.
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
infra_dir="${repo_root}/infra"
frontend_dir="${repo_root}/frontend"

# shellcheck disable=SC1091
source "${repo_root}/scripts/lib/env.sh"
load_env

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

echo "==> Uploading sample images"
"${repo_root}/scripts/upload-samples.sh"

echo "==> Building frontend"
pnpm -C "${frontend_dir}" install --frozen-lockfile

# Inject the real images bucket name so the SSG bundle targets the right
# S3 bucket in prod; the dev default lives in frontend/nuxt.config.ts.
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
