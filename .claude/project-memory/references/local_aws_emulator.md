---
name: "Local AWS emulator: Moto Server"
description: "compose.yaml uses motoserver/moto (not LocalStack) because LocalStack 2026 is Pro-licensed; host port 4566 maps to container 5000"
type: project
---

# Local AWS emulator: Moto Server

This project uses `motoserver/moto` (not LocalStack) as the
local AWS emulator in `compose.yaml`.

LocalStack's 2026 image releases moved to a Pro-licensed
model and exit with code 55 without a
`LOCALSTACK_AUTH_TOKEN`. Moto Server is community/free,
lighter, and supports the AWS APIs this project needs (S3,
DynamoDB).

Compose exposes Moto on host port 4566 (mapped from
container port 5000) so `AWS_ENDPOINT_URL=http://localhost:4566`
in `.env.local` keeps the LocalStack convention. Sibling
services (`init`, `worker`, `backend`) reach it internally
at `http://moto:5000`.

**Why:** keep the demo free for any user, no licensing
friction.

**How to apply:**

- Use `motoserver/moto:latest` in `compose.yaml`. If the
  moto image ever lacks a needed API, the fallback is to
  require a free `LOCALSTACK_AUTH_TOKEN` rather than
  re-pin LocalStack `:latest` blindly.
- Moto listens on its default port (5000) inside the
  container, which is why the compose `command` passes no
  port flag. Keep it that way: pinning the internal URL to
  the default removes a knob that would otherwise drift
  between `compose.yaml`, the healthcheck, sibling
  `AWS_ENDPOINT_URL=http://moto:5000` values, and the
  `init` service's inline `aws --endpoint-url` calls. If a
  change tempts you to move moto to another internal port,
  surface the trade-off and ask first.
- The host-side `4566:5000` mapping is a separate concern
  and may be remapped per worktree — see
  [Per-worktree compose port isolation](worktree_compose_port_isolation.md).
