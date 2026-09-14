#!/usr/bin/env bash
# Rebuild the Nuxt frontend, sync to S3, and invalidate CloudFront.
#
# Runs `tofu init` so the Tofu outputs are readable, then skips
# `tofu apply`: assumes the infra is already provisioned and pulls
# bucket/distribution names from Tofu outputs. Use this for fast
# iteration on the frontend only.

set -euo pipefail

# Optional $1: the closing banner, so scripts/deploy.sh can reuse this script
# for its own last steps without announcing a frontend-only deployment.
completion_message="${1:-Frontend deployment complete.}"

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
infra_dir="${repo_root}/infra"
frontend_dir="${repo_root}/frontend"

# shellcheck disable=SC1091
source "${repo_root}/scripts/lib/env.sh"
load_env

echo "==> Initializing OpenTofu"
tofu -chdir="${infra_dir}" init

echo "==> Building frontend"
# pnpm resolves the `packageManager` pin from its own working directory, so run it from
# inside frontend/ (in a subshell, to leave this script's cwd alone) and via corepack.
(cd "${frontend_dir}" && corepack pnpm install --frozen-lockfile)

(
  cd "${frontend_dir}"
  NUXT_PUBLIC_API_BASE="" \
  NUXT_PUBLIC_S3_PUBLIC_URL="" \
    corepack pnpm generate
)

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
echo "${completion_message}"
echo "Demo URL: ${demo_url}"
