---
name: "Per-worktree compose port isolation"
description: "One generator writes a gitignored compose.override.yaml fanning host ports off a base port; the Makefile parses it back to drive host-mode dev"
type: project
---

# Per-worktree compose port isolation

Parallel worktrees each run their own stack without
colliding on host ports. A single generator writes a
**gitignored** `compose.override.yaml` at the worktree
root, remapping every host-published port off a base
port.

Offset scheme (base = the workspace's assigned port):

| Service       | Host port  | Container port |
| ------------- | ---------- | -------------- |
| frontend      | `base`     | 3000           |
| temporal gRPC | `base + 1` | 7233           |
| temporal UI   | `base + 2` | 8233           |
| backend       | `base + 3` | 8000           |
| moto          | `base + 4` | 5000           |

`make worktree-ports` owns the generator and takes its
base from `CASPER_PORT`; `.casper.json` wires it as the
workspace `setup` script. The cmux entry point,
`.cmux/post-create.sh`, delegates to the same target
with `CMUX_PORT` as the base. With no base port in the
environment, generation is skipped with a log message
rather than failing.

**Why:** without the fan-out, every worktree binds the
same host ports (3000/7233/8233/8000/4566) and the
second `make app-up` fails.

**How to apply:**

- Each `ports:` list in the override carries the compose
  `!override` YAML tag. Compose appends multi-value list
  fields when merging, so an untagged list *adds* the
  remapped mapping while keeping the colliding base one
  (e.g. `7233:7233`). `!override` replaces the list.
  Verified on Docker Compose v5.x with
  `docker-compose config`.
- Only the host (left) side moves. Internal
  service-to-service ports (`temporal:7233`, `moto:5000`)
  stay fixed, so the containerized stack keeps working.
- The Makefile parses the override back with `sed -nE`
  after the `.env` / `.env.local` includes, making it the
  source of truth for the endpoint banner (`show_urls`)
  and for host-mode `make dev`. It force-exports
  `TEMPORAL_ADDRESS` and `AWS_ENDPOINT_URL` at the
  remapped gRPC and Moto ports (overriding the fixed
  values in `.env.local`), sets `PORT` per recipe
  (`backend` = base+3, `frontend` = base), and points the
  Nuxt dev proxy at `NUXT_DEV_API_TARGET` /
  `NUXT_DEV_IMAGES_TARGET`. Absent an override, the
  conventional defaults apply (3000/7233/8233/8000/4566).
- Every host-mode consumer of a remapped port needs its
  own force-export. Parsing the ports alone does not
  reach the Go binaries, which read `AWS_ENDPOINT_URL`
  and `TEMPORAL_ADDRESS` from the environment.
- Both force-exports are unconditional rather than gated
  on `DEV_TARGETS`; deploy targets stay safe one layer
  down, because `load_env` in `scripts/lib/env.sh` unsets
  `AWS_ENDPOINT_URL` (with the other dev-overlay vars)
  and re-sources `.env`. So `deploy`, `frontend-deploy`
  and `teardown` reach real AWS and Temporal Cloud even
  from a port-remapped worktree.
- The backend listen port derives from `PORT` (default
  `:8000`) in `cmd/backend/main.go`. The worker's `:8001`
  health port is out of scope — it is not published and a
  bind failure there is non-fatal.
- `make dev` (host processes, hot reload) and `make app-up`
  (full container stack, prod parity) stay separate
  targets. Both need the remapped ports, and neither
  replaces the other.
- Teardown is per worktree: Casper runs `make app-down`,
  cmux runs `.cmux/pre-destroy.sh`. The generated
  `compose.override.yaml` disappears with the worktree and
  needs no explicit cleanup.

Related notes:
[Worktree env symlinks](worktree_env_symlinks.md) for the env
files a new worktree also needs, and
[Recursive Make helper variables need unexport](makefile_unexport_helpers.md)
for the `unexport override_port` line that the `.env` includes'
bare `export` makes necessary.
