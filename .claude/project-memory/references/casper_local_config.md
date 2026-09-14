---
name: "Casper local-config support"
description: "How Casper workspaces isolate ports and run the demo, mirroring cmux"
type: reference
---

# Casper local-config support

`.casper.json` (committed) configures Casper workspaces:

- `copyFiles` seeds each new workspace with `.env`,
  `.env.local`, `ca.pem`, `ca.key`.
- Scripts: `setup=make worktree-ports`, `run=make app-up`,
  `dev=make dev`, `stop`/`teardown=make app-down`.

Port isolation shares the SAME mechanism as cmux: both write the
gitignored `compose.override.yaml` with an identical fan-out from a
base port (frontend base:3000, temporal grpc base+1:7233, temporal
ui base+2:8233, backend base+3:8000, moto base+4:5000). cmux uses
`.cmux/post-create.sh` (from `CMUX_PORT`); Casper uses
`make worktree-ports` (from `CASPER_PORT`). Only the generated-file
header comment differs. Keep the two fan-outs byte-compatible.

The Makefile parses `compose.override.yaml` back (via `sed -nE`)
after the `.env`/`.env.local` includes, so it is the source of
truth for the endpoint banner (`show_urls` macro) and for host-side
`make dev`: `TEMPORAL_ADDRESS` and `AWS_ENDPOINT_URL` are
force-exported to the remapped gRPC and Moto ports (overriding the
fixed values in `.env.local`), `PORT` is set per-recipe
(`backend`=base+3, `frontend`=base), and Nuxt dev-proxy targets
come from `NUXT_DEV_API_TARGET` / `NUXT_DEV_IMAGES_TARGET`. No
override present → conventional defaults (3000/7233/8233/8000/4566).

Every host-mode consumer of a remapped port needs its own
force-export; parsing the ports alone does not reach the Go
binaries, which read `AWS_ENDPOINT_URL` and `TEMPORAL_ADDRESS` from
the environment.

Both force-exports are unconditional rather than gated on
`DEV_TARGETS`, and the deploy targets stay safe one layer down:
`load_env` in `scripts/lib/env.sh` unsets `AWS_ENDPOINT_URL`
(alongside the other dev-overlay vars) and re-sources `.env`, so
`deploy`, `frontend-deploy` and `teardown` reach real AWS and
Temporal Cloud even from a port-remapped worktree.

Backend listen port derives from `PORT` (default `:8000`) in
`cmd/backend/main.go`. Worker `:8001` health port is out of scope
(not published, bind failure non-fatal). Never touch the cmux
`.cmux/*` scripts when changing Casper support.
