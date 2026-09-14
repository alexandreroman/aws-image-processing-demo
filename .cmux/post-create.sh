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

# Generate a per-workspace compose override so parallel worktrees don't
# collide on host-published ports. cmux assigns each workspace a unique
# CMUX_PORT; `make worktree-ports` fans the stack's exposed ports out from
# that base and owns the one copy of that logic (it is also the Casper entry
# point, which passes CASPER_PORT directly).
if [[ -n "${CMUX_PORT:-}" ]]; then
  CASPER_PORT="${CMUX_PORT}" make worktree-ports
else
  echo "[post-create] CMUX_PORT unset; skipping compose.override.yaml"
fi

if [[ -f frontend/package.json ]]; then
  if command -v pnpm >/dev/null 2>&1; then
    echo "[post-create] installing frontend deps"
    pnpm -C frontend install --frozen-lockfile
  else
    echo "[post-create] warning: pnpm not on PATH; skipping frontend install"
  fi
fi

echo "[post-create] done"
