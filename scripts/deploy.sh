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
#   7. Hand off to scripts/frontend-deploy.sh: build the Nuxt
#      frontend, sync to S3, invalidate CloudFront, print the
#      demo URL.
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

echo "==> Building and deploying the frontend"
# Same build + sync + invalidate + report sequence as `make frontend-deploy`.
# The script loads .env itself, so calling it here is safe.
"${repo_root}/scripts/frontend-deploy.sh" "Deployment complete."
