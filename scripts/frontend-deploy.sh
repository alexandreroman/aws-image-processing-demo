#!/usr/bin/env bash
# Rebuild the Nuxt frontend, sync to S3, and invalidate CloudFront.
#
# Skips `tofu apply`: assumes the infra is already provisioned and
# pulls bucket/distribution names from Tofu outputs. Use this for
# fast iteration on the frontend only.

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
infra_dir="${repo_root}/infra"
frontend_dir="${repo_root}/frontend"

# shellcheck disable=SC1091
source "${repo_root}/scripts/lib/env.sh"
load_env

echo "==> Building frontend"
pnpm -C "${frontend_dir}" install --frozen-lockfile

# Inject the real images bucket name so the SSG bundle targets the right
# S3 bucket in prod; the dev default lives in frontend/nuxt.config.ts.
NUXT_PUBLIC_API_BASE="" \
NUXT_PUBLIC_S3_PUBLIC_URL="" \
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
echo "Frontend deployment complete."
echo "Demo URL: ${demo_url}"
