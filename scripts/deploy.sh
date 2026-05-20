#!/usr/bin/env bash
# Deploy the demo to AWS.
#
# Steps:
#   1. Load .env and validate required vars.
#   2. Build the Lambda bootstrap binary.
#   3. Build the worker Lambda zip (build/worker.zip).
#   4. tofu init && tofu apply.
#   5. Register the Temporal Worker Deployment version for the
#      Lambda runtime so Temporal Cloud can route workflows to it.
#   6. Upload sample images.
#   7. Build the Nuxt frontend.
#   8. Sync to S3 and invalidate CloudFront.
#   9. Print the demo URL.
#
# The ECS worker autoscaler is now an ADOT Collector ECS task
# scraping Temporal Cloud's OpenMetrics endpoint — there is no
# Lambda artifact to build for it.
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

echo "==> Building worker Lambda zip"
make -C "${repo_root}" worker-lambda-zip

echo "==> Provisioning infra with OpenTofu"
tofu -chdir="${infra_dir}" init
tofu -chdir="${infra_dir}" apply "${tofu_apply_args[@]}"

echo "==> Registering Temporal Worker Deployment version"
"${repo_root}/scripts/register-worker-deployment.sh"

echo "==> Uploading sample images"
"${repo_root}/scripts/upload-samples.sh"

echo "==> Building frontend"
pnpm -C "${frontend_dir}" install --frozen-lockfile

NUXT_PUBLIC_API_BASE="" \
NUXT_PUBLIC_S3_PUBLIC_URL="" \
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
