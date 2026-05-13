---
name: "Local AWS emulator: Moto Server"
description: "compose.yaml uses motoserver/moto (not LocalStack) because LocalStack 2026 became Pro-licensed; host port 4566 maps to container 5000"
type: project
---

# Local AWS emulator: Moto Server

This project uses `motoserver/moto` (not LocalStack)
as the local AWS emulator in `compose.yaml`.

LocalStack's 2026 image releases moved to a
Pro-licensed model and exit with code 55 without a
`LOCALSTACK_AUTH_TOKEN`. Moto Server is community/free,
lighter, and supports the AWS APIs we need (S3,
DynamoDB).

Compose exposes Moto on host port 4566 (mapped from
container port 5000) so `AWS_ENDPOINT_URL=http://localhost:4566`
in `.env` is unchanged. The container-internal URL
used by sibling services (`init`, `worker`, `backend`)
is `http://moto:5000`.

**Why:** keep the demo free for any user, no
licensing friction.

**How to apply:** Use `motoserver/moto:latest` in
`compose.yaml`. If the moto image's API surface ever
lacks a needed feature, the fallback is to require a
free `LOCALSTACK_AUTH_TOKEN` rather than re-pin
LocalStack `:latest` blindly. See related
[[dev_mode_split]].
