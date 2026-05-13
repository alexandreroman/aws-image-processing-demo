#!/usr/bin/env bash
# Build cmd/backend as a Lambda-compatible binary.
#
# Output: dist/backend/bootstrap (consumed by
# infra/backend.tf via archive_file).
#
# Target runtime: provided.al2023 (Lambda
# custom runtime, AMD64).

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${repo_root}/dist/backend"
mkdir -p "${out_dir}"

echo "Building cmd/backend → ${out_dir}/bootstrap"

(
  cd "${repo_root}"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o "${out_dir}/bootstrap" \
      ./cmd/backend
)

echo "Built $(du -h "${out_dir}/bootstrap" | cut -f1) bootstrap"
