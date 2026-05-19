#!/usr/bin/env bash
# Sync sample images to the demo S3 bucket under
# the samples/ prefix. Uses `aws s3 sync` so files
# that are already present and unchanged are
# skipped — the first deploy uploads everything,
# subsequent deploys are no-ops unless a sample
# was added or modified locally.
#
# Local vs. AWS detection:
#   - If AWS_ENDPOINT_URL is set, target
#     LocalStack and use IMAGES_BUCKET (default:
#     aws-image-processing-demo-images-local).
#   - Otherwise, read images_bucket from Tofu.

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
samples_dir="${repo_root}/frontend/public/sample-images"
infra_dir="${repo_root}/infra"

if [[ ! -d "${samples_dir}" ]]; then
  echo "error: ${samples_dir} does not exist" >&2
  exit 1
fi

shopt -s nullglob
samples=("${samples_dir}"/*.jpg)
shopt -u nullglob

if (( ${#samples[@]} == 0 )); then
  echo "error: no .jpg files found in ${samples_dir}" >&2
  exit 1
fi

aws_args=()
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
  bucket="${IMAGES_BUCKET:-aws-image-processing-demo-images-local}"
  aws_args+=(--endpoint-url "${AWS_ENDPOINT_URL}")
  echo "Target: LocalStack (${AWS_ENDPOINT_URL})"
else
  bucket="$(tofu -chdir="${infra_dir}" output -raw images_bucket)"
  echo "Target: AWS"
fi

echo "Bucket: s3://${bucket}"
echo "Samples: ${#samples[@]} file(s)"
echo

echo "Syncing samples to s3://${bucket}/samples/ (skips unchanged)"
aws "${aws_args[@]}" s3 sync "${samples_dir}/" "s3://${bucket}/samples/" \
  --exclude '*' \
  --include '*.jpg' \
  --no-progress

echo
echo "Done: ${#samples[@]} local sample(s) reconciled with s3://${bucket}/samples/"
