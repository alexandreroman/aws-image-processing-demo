# AWS Image Processing Demo

A conference and customer demo showcasing
**Temporal Cloud + AWS** through an image-processing
burst pipeline. Built to make durable orchestration,
fan-out/fan-in, and AI integration tangible for AWS
architects and developers.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

> **Status:** scaffold only. The project structure is
> in place; implementation lands sprint by sprint.

## Features

- **Bursty image pipeline** — upload N images, watch
  Temporal fan them out into 8 activities per image
  (resize × 3, describe, watermark × 3, persist).
- **Durable execution** — kill the worker mid-burst;
  Temporal Cloud keeps the workflows alive and a new
  Fargate task resumes where the previous one left off.
- **AI in the loop** — each image is described and
  labeled by Claude Haiku 4.5 vision.
- **Direct-to-S3 uploads** — the backend signs PUT
  URLs; bytes never touch the API.
- **Shareable sessions** — every burst gets a session
  ID, threaded through the URL, workflow IDs, S3
  prefixes, and DynamoDB items.
- **Single-domain deploy** — CloudFront fronts both
  the Nuxt SSG frontend and the API Gateway backend
  under one custom domain managed via Cloudflare DNS.

## Prerequisites

- **Go** 1.26 or newer
- **Node.js** 24 LTS (or newer) and **pnpm** 11 (or newer)
- **Docker** and **Docker Compose**
- **OpenTofu** 1.8 or newer (for AWS deployment)
- **AWS CLI v2** (for AWS deployment)
- **Temporal CLI** — `brew install temporal`
- An **Anthropic API key** — used in both local dev
  and production (LocalStack does not mock Bedrock)

For AWS deployment you also need an AWS account in
`eu-west-3`, plus a Cloudflare account and API token
if you want a custom domain.

## Getting Started

```bash
git clone https://github.com/alexandreroman/aws-image-processing-demo.git
cd aws-image-processing-demo

# Configure secrets
cp .env.example .env
# edit .env and set ANTHROPIC_API_KEY

# Local dev — Temporal dev server and LocalStack
# (S3 + DynamoDB) in Docker; worker, backend, and
# frontend as host processes with hot reload.
# Frontend deps install automatically on first run.
make dev
```

Once the stack is up:

- Frontend — <http://localhost:3000>
- Backend API — <http://localhost:8000/api>
- Temporal UI — <http://localhost:8233>
- LocalStack endpoint — <http://localhost:4566>

Open the frontend, pick a number of images, and click
**Start burst**. You will be redirected to
`/sessions/{sessionId}` where the gallery fills in as
workflows complete.

## Usage

### Run a single workflow from the CLI

Useful for debugging activities without going through
the frontend. Use the `temporal` CLI directly.

```bash
temporal workflow start \
  --type ProcessImage \
  --task-queue image-processing \
  --workflow-id "manual-$(uuidgen)" \
  --input '{"bucket":"aws-image-processing-demo-images-local","key":"samples/dog.jpg"}'
```

The image must already be present in the bucket
(upload it manually with `aws --endpoint-url
http://localhost:4566 s3 cp ...` first).

### Run unit tests

```bash
make test
```

### Deploy to AWS

The worker container image is built and pushed to
GHCR by a GitHub Actions workflow on every push to
`main`. Then, from a clone:

```bash
make deploy
# runs: tofu init, tofu apply (interactive), frontend build,
#       S3 sync, CloudFront invalidation
```

To re-deploy only the frontend (typical iteration):

```bash
make frontend-deploy
```

To tear everything down:

```bash
make teardown
```

## Configuration

All configuration is via environment variables. Copy
`.env.example` to `.env` and adjust.

| Variable                | Description                                   | Default                  |
| ----------------------- | --------------------------------------------- | ------------------------ |
| `TEMPORAL_ADDRESS`      | Temporal frontend address                     | `localhost:7233`         |
| `TEMPORAL_NAMESPACE`    | Temporal namespace                            | `default`                |
| `TEMPORAL_TLS_CERT`     | Path to client cert (Temporal Cloud only)     | (empty)                  |
| `TEMPORAL_TLS_KEY`      | Path to client key (Temporal Cloud only)      | (empty)                  |
| `TEMPORAL_TASK_QUEUE`   | Worker task queue                             | `image-processing`       |
| `AWS_ENDPOINT_URL`      | Override AWS endpoint (set for LocalStack)    | `http://localhost:4566`  |
| `AWS_REGION`            | AWS region                                    | `eu-west-3`              |
| `ANTHROPIC_API_KEY`     | Anthropic API key (used in dev and prod)      | (required)               |
| `CLOUDFLARE_API_TOKEN`  | Cloudflare DNS token (only for `tofu apply`)  | (empty)                  |
| `CLOUDFLARE_ZONE_ID`    | Cloudflare zone ID                            | (empty)                  |

## Architecture

```mermaid
graph TD
    User[Browser] -->|PUT presigned| S3[(S3 images bucket)]
    User -->|POST /api/*| CF[CloudFront]
    CF -->|/api/*| APIGW[API Gateway]
    APIGW --> BE[Backend Lambda]
    CF -->|/*| FE[(S3 Nuxt SSG)]
    BE -->|StartWorkflow / Query| TC[Temporal Cloud]
    BE --> DDB[(DynamoDB metadata)]
    Worker[ECS Fargate worker] -->|long-poll| TC
    Worker --> S3
    Worker --> DDB
    Worker --> Anthropic[Anthropic API]
```

Each image is processed by one `ProcessImage` workflow
with 8 activities, 6 of which run in parallel:

1. Fan-out 3 × `ResizeAndUpload` (small / medium / large)
2. 1 × `GenerateDescription` on the medium size
3. Fan-out 3 × `ApplyWatermark`
4. 1 × `StoreManifest` to DynamoDB

The workflow ID format is `{sessionId}-{i}` so the
Temporal UI can filter a whole burst with a prefix
search.

### Modules

| Module                     | Description                                              |
| -------------------------- | -------------------------------------------------------- |
| `cmd/worker`               | Temporal worker for ECS Fargate                          |
| `cmd/backend`              | Backend service — Lambda or local HTTP server            |
| `internal/workflows`       | `ProcessImage` workflow definition                       |
| `internal/activities`      | Resize, describe, watermark, store activities            |
| `internal/manifest`        | Shared manifest types and canonical size list            |
| `internal/awsclient`       | AWS SDK config (LocalStack-aware)                        |
| `internal/anthropicclient` | Anthropic API wrapper                                    |
| `internal/api`             | HTTP handlers for `/api/uploads/presign`, `/workflows/*` |
| `frontend`                 | Nuxt 4 SSG frontend (Tailwind, pnpm)                     |
| `infra`                    | OpenTofu modules for AWS + Cloudflare DNS                |
| `scripts`                  | Deploy, teardown, and sample-upload helpers              |

## Contributing

Issues and pull requests are welcome.

## License

This project is licensed under the Apache-2.0 License
— see [LICENSE](LICENSE) for details.
