#!/usr/bin/env bash
# Destroy the AWS infrastructure provisioned by
# scripts/deploy.sh.
#
# Steps:
#   1. Empty the images and frontend S3 buckets
#      (Tofu cannot delete non-empty buckets).
#   2. tofu destroy.
#
# Set INTERACTIVE=0 to skip the destroy prompt
# (useful in CI). Default is interactive.

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${repo_root}/.env"
infra_dir="${repo_root}/infra"

if [[ -f "${env_file}" ]]; then
  echo "Loading environment from ${env_file}"
  set -a
  # shellcheck disable=SC1090
  . "${env_file}"
  set +a
fi

interactive="${INTERACTIVE:-1}"
tofu_destroy_args=()
if [[ "${interactive}" == "0" ]]; then
  tofu_destroy_args+=(-auto-approve)
fi

# Empty a bucket only if it currently exists in
# state. tofu output -raw returns non-zero when the
# output is missing, which is fine — we just skip.
empty_bucket() {
  local output_name="$1"
  local bucket
  if ! bucket="$(tofu -chdir="${infra_dir}" output -raw "${output_name}" 2>/dev/null)"; then
    echo "Skipping ${output_name}: output not available"
    return 0
  fi
  if [[ -z "${bucket}" ]]; then
    echo "Skipping ${output_name}: output is empty"
    return 0
  fi
  echo "Emptying s3://${bucket}"
  aws s3 rm "s3://${bucket}" --recursive >/dev/null || true
}

echo "==> Emptying S3 buckets"
empty_bucket images_bucket
empty_bucket frontend_bucket

echo "==> Destroying infra with OpenTofu"
tofu -chdir="${infra_dir}" destroy "${tofu_destroy_args[@]}"

echo
echo "Teardown complete."
