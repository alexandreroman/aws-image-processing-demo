#!/usr/bin/env bash
set -euo pipefail

slug="${CMUX_FEATURE_SLUG:-?}"
echo "[post-create] setting up worktree ${slug}"

MAIN="${CMUX_MAIN_WORKTREE:-}"
if [[ -z "${MAIN}" ]]; then
  echo "[post-create] CMUX_MAIN_WORKTREE is unset; cannot locate sibling files" >&2
  exit 1
fi

for name in ca.pem ca.key; do
  [[ -e "${MAIN}/${name}" ]] || continue
  [[ -e "${name}" || -L "${name}" ]] && continue
  ln -s "../../${name}" "${name}"
  echo "[post-create] linked ${name}"
done

if [[ -f frontend/package.json ]]; then
  if command -v pnpm >/dev/null 2>&1; then
    echo "[post-create] installing frontend deps"
    pnpm -C frontend install --frozen-lockfile
  else
    echo "[post-create] warning: pnpm not on PATH; skipping frontend install"
  fi
fi

echo "[post-create] done"
