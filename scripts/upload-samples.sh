#!/usr/bin/env bash
# Upload sample images to the demo S3 bucket
# under the samples/ prefix.
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

for f in "${samples[@]}"; do
  name="$(basename -- "${f}")"
  key="samples/${name}"
  aws "${aws_args[@]}" s3 cp "${f}" "s3://${bucket}/${key}" \
    --content-type image/jpeg \
    --only-show-errors
  echo "  uploaded ${key}"
done

echo
echo "Done: ${#samples[@]} image(s) uploaded to s3://${bucket}/samples/"
