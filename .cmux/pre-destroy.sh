#!/usr/bin/env bash
set -euo pipefail

slug="${CMUX_FEATURE_SLUG:-?}"
echo "[pre-destroy] tearing down worktree ${slug}"

# Tear down the compose stack this worktree may have started via `make app-up`
# / `make infra-up`. Both invoke docker-compose from this worktree root with no
# `-p`, so the project name defaults to this directory's basename (the slug).
# pre-destroy runs with cwd = the worktree being destroyed, so `down` here
# targets exactly that project and never touches the main worktree's stack.
#
# `docker-compose down` is idempotent: it exits 0 even when the stack was never
# started (it only warns "No resource found to remove"). So a non-zero status
# means a genuine container-runtime error — we let it propagate to block the
# close. /cmux:close-workspace preserves the worktree when the hook fails, so
# you can fix the runtime and re-run (the merge step becomes a no-op and
# cleanup resumes). The generated compose.override.yaml is removed together
# with the worktree, so there is nothing else to clean up here.
if command -v docker-compose >/dev/null 2>&1 && [[ -f compose.yaml ]]; then
  echo "[pre-destroy] removing compose stack (project ${slug})"
  docker-compose down --remove-orphans
else
  echo "[pre-destroy] docker-compose unavailable or compose.yaml missing; nothing to tear down"
fi

echo "[pre-destroy] done"
