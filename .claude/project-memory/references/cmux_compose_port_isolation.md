---
name: "cmux per-workspace compose port isolation"
type: project
description: ".cmux/post-create.sh generates a gitignored compose.override.yaml remapping host ports off CMUX_PORT so parallel worktrees don't collide"
---

# cmux per-workspace compose port isolation

When a cmux isolated workspace is created,
`.cmux/post-create.sh` generates a **gitignored**
`compose.override.yaml` in the worktree root. It remaps every
host-published port off the per-workspace `CMUX_PORT` base
(exported by cmux into the workspace shell) so several worktrees
can each run `make app-up` at once without host-port collisions.

Offset scheme (base = `$CMUX_PORT`):

- frontend (webui): `CMUX_PORT`
- temporal gRPC: `CMUX_PORT+1`
- temporal UI: `CMUX_PORT+2`
- backend: `CMUX_PORT+3`
- moto: `CMUX_PORT+4` (container port 5000)

**Why:** parallel cmux worktrees otherwise all try to bind the
same host ports (3000/7233/8233/8000/4566) and the second
`make app-up` fails. cmux assigns each workspace a unique
`CMUX_PORT`, so fanning the stack's ports out from that base
keeps every worktree's stack reachable simultaneously.

**How to apply:**

- The override must use the docker compose `!override` YAML tag
  on each `ports:` list. Compose merges/appends multi-value list
  fields by default, so a plain override would *keep* the
  colliding base mapping (e.g. `7233:7233`) in addition to the
  new one. `!override` replaces the list. Verified on Docker
  Compose v5.x via `docker-compose config`.
- Only host-side (left) ports move. Internal
  container-to-container ports (`temporal:7233`, `moto:5000`)
  are unchanged, so the containerized stack keeps working.
- This covers the full-Docker `make app-up` case only. It does
  NOT address host-mode dev (`make dev`), which reads `.env` and
  would need its own port adjustments — deliberately out of
  scope. See [[worktree_env_symlinks]] and [[dev_mode_split]].
- `compose.yaml`'s temporal command no longer passes
  `--ui-port 8233` (redundant — 8233 is the
  `temporal server start-dev` default UI port). The
  `8233:8233` base mapping stays; the override remaps the host
  side. Keep moto's internal port at the default — see
  [[feedback_moto_default_port]].
- If `CMUX_PORT` is unset (hook run outside cmux),
  generation is skipped with a log message rather than failing.

## Teardown counterpart: `.cmux/pre-destroy.sh`

`/cmux:close-workspace` and `/cmux:cancel-workspace` run
`.cmux/pre-destroy.sh` (cwd = the worktree being destroyed) before
removing the worktree. It runs `docker-compose down --remove-orphans`
to tear down the per-worktree stack started by `make app-up` /
`make infra-up`.

**Why / how to apply:**

- The project name defaults to the worktree dir basename (the slug)
  for both `up` and `down` because neither passes `-p`, so the `down`
  hits exactly this worktree's stack and never the main one.
- The hook lets a non-zero `docker-compose down` PROPAGATE (no
  `|| true`). `down` is idempotent (exits 0 when nothing is running),
  so non-zero means a real container-runtime error; failing the hook
  makes `/cmux:close-workspace` preserve the worktree so the user can
  fix the runtime and re-run (merge step becomes a no-op).
- The generated `compose.override.yaml` needs no explicit cleanup —
  it is removed together with the worktree.
