---
name: "Dev mode: host processes + Docker infra split"
description: "`make dev` runs Go + Nuxt on host with Docker infra; `make app-up` brings the full stack up in Docker for parity mode"
type: project
---

# Dev mode: host processes + Docker infra split

Two ways to bring up the stack, both via
`compose.yaml`:

- **`make dev`** (default, daily workflow):
  brings up only the infra services in Docker
  (`temporal`, `localstack`, `init`) via
  `docker-compose up -d temporal localstack
  init`, then runs `cmd/backend`, `cmd/worker`,
  and the Nuxt dev server on the host via
  `make -j backend worker frontend`. Stopping
  is Ctrl-C for the host processes, then
  `make infra-down` to stop Docker.
- **`make app-up`** (parity mode): brings up
  the full stack in Docker, including the
  `worker`, `backend`, and `frontend` app
  containers (built from local Dockerfiles).
  `make app-down` tears it down completely.

`compose.yaml` defines six services:
three infra (`temporal`, `localstack`, `init`)
and three app (`worker`, `backend`, `frontend`).
The Make targets pick subsets.

**Why:** the user wants hot reload, fast Go
rebuilds, and attachable debuggers on the
daily path — hence `make dev` running the apps
on the host. `make app-up` exists for parity
testing (validating the prod-bound worker
image, smoke-testing the Dockerfile, demo on a
machine without local toolchains).

**How to apply:**

- Keep `compose.yaml` containing both
  infra and app services. `infra-up` uses
  explicit service names (`temporal localstack
  init`) to avoid bringing app containers up
  by accident.
- Do not collapse `make dev` and `make app-up`
  into one target — they serve different use
  cases (hot reload vs. container parity).
- `infra-down` uses `docker-compose stop`
  (keeps containers around); `app-down` uses
  `docker-compose down` (removes containers
  and network). Symmetric with the
  temporal-patterns-in-action reference
  Makefile.
- GHCR image push is handled by GitHub
  Actions, not a Make target — see the
  workflow under `.github/workflows/`.
