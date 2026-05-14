---
name: "Known prod gap: frontend image display"
description: "Image display works only locally; production setup has three combined gaps the user deferred fixing on 2026-05-14."
type: project
---

# Known prod gap: frontend image display

Gallery image display is broken in production. Three independent
gaps combine:

1. `Makefile` target `frontend-deploy` runs `pnpm generate` without
   setting `NUXT_PUBLIC_S3_PUBLIC_URL`, so the production bundle
   ships with the default `http://localhost:4566` baked into
   `frontend/nuxt.config.ts`.
2. The production images bucket is locked down by
   `aws_s3_bucket_public_access_block` in `infra/storage.tf` —
   anonymous GETs are blocked.
3. `infra/frontend.tf` declares only two CloudFront origins
   (`s3-frontend`, `api-gateway`); there is no behavior/origin
   that routes to the images bucket.

The frontend constructs URLs as `${s3PublicUrl}/${bucket}/${key}`
in `frontend/app/components/Gallery.vue`, so any of the three gaps
alone would break image display.

**Why:** local-dev fix landed in commit `5776534` (anonymous-read
bucket policy applied by the `init` service in `compose.yaml`),
but that is dev-only. The user explicitly chose to defer the
production fix on 2026-05-14.

**How to apply:** when the user returns to this, propose the three
candidates discussed and let them pick:

- (a) Add a CloudFront `/images/*` behavior with an OAC pointing
  at the images bucket. Cleanest: same-origin URLs, no presigning,
  no CORS, bucket stays private. Requires deciding the URL scheme
  exposed to the frontend (probably `/images/sessions/...` mapped
  to the bucket prefix).
- (b) Add a `GET /api/images/{bucket}/{key}` proxy in
  `internal/api`. Trivial; loads the Lambda for every thumbnail.
- (c) Per-thumbnail presigned GETs from the backend. Most
  expensive: doubles API calls and adds complexity to
  `Gallery.vue`.

In all three cases, `frontend/nuxt.config.ts` and the deploy step
also need to be reworked so the bundled `s3PublicUrl` (or its
replacement) actually points at the chosen production endpoint.
