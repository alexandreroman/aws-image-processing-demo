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
  (`temporal`, `moto`, `init`) via
  `docker-compose up -d temporal moto init`,
  then runs `cmd/backend`, `cmd/worker`, and
  the Nuxt dev server on the host via
  `make -j backend worker frontend`. Single-
  origin: the Nuxt dev server on `:3000`
  proxies `/api/*` to `localhost:8000` and
  `/images/*` to Moto via `nitro.devProxy`
  (see `frontend/nuxt.config.ts`). Stopping is
  Ctrl-C for the host processes, then
  `make infra-down` to stop Docker.
- **`make app-up`** (parity mode): brings up
  the full stack in Docker, including the
  `worker`, `backend`, and a Caddy-fronted
  `frontend` container (built from
  `frontend/Dockerfile`). Caddy serves the
  Nuxt SSG bundle on `:3000`, reverse-proxies
  `/api/*` to `backend:8000`, and `/images/*`
  to `moto:5000/<dev-bucket>` — same-origin
  from the browser, mirroring the prod
  CloudFront topology. The compose stack is
  self-contained: it does not consult
  `.env.local`, only pulls `ANTHROPIC_API_KEY`
  from `.env` (compose-time interpolation).
  `make app-down` tears it down completely.

`compose.yaml` defines six services: three
infra (`temporal`, `moto`, `init`) and three
app (`worker`, `backend`, `frontend`). The Make
targets pick subsets.

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
  explicit service names (`temporal moto
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
