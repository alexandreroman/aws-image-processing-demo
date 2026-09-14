---
name: "AWS resource naming (S3 bucket and DynamoDB table)"
description: "Fixed names in dev; OpenTofu-generated with prefix `aws-image-processing-demo-` in AWS. Neither is a user-tunable knob."
type: project
---

# AWS resource naming (S3 bucket and DynamoDB table)

Both the S3 images bucket and the DynamoDB images table
follow the same naming pattern. Env vars are namespaced
`IMAGES_*` because every resource in this project relates
to image processing.

|                | Env var         | Dev (Moto)                                 | Prod (AWS)                                                            |
| -------------- | --------------- | ------------------------------------------ | --------------------------------------------------------------------- |
| S3 bucket      | `IMAGES_BUCKET` | `aws-image-processing-demo-images-local`   | OpenTofu-generated via `bucket_prefix = "aws-image-processing-demo-"`   |
| DynamoDB table | `IMAGES_TABLE`  | `aws-image-processing-demo-images-local`   | OpenTofu-generated with prefix `aws-image-processing-demo-`             |

Bucket and table share the same literal dev name. S3 and
DynamoDB live in separate namespaces, so there is no
collision. In prod, Tofu generates distinct random
suffixes and the names diverge automatically.

The Go code reads `IMAGES_BUCKET` and `IMAGES_TABLE` from
the environment in both modes — values come from
`.env.local` locally and from Tofu in AWS. Same Go code in
dev and prod.

**Why:** the bucket and table both need unique names per
environment to avoid collisions, and Tofu already owns
their lifecycle, so it should own their names too. Fixed
local names beat a Tofu-managed local resource for
simplicity: Moto accepts any name, so `make dev` needs no
`tofu output → .env.local` plumbing.

**How to apply:**

- The local bootstrap (the `init` service in
  `compose.yaml`) must create both resources with the
  exact dev names above.
- The Tofu storage module must use `bucket_prefix` (not
  `bucket`) for S3 and a prefix/`random_id` pattern for
  DynamoDB, and expose `images_bucket` / `images_table`
  outputs.
- Tofu injects those outputs as `IMAGES_BUCKET` and
  `IMAGES_TABLE` on the worker ECS task def and the
  backend Lambda env — no hardcoded prod defaults
  anywhere.
- Both variables belong in the README's dev-overlay
  (`.env.local`) table only. They are process plumbing,
  not deploy-time knobs, so keep them out of the canonical
  `.env` configuration table.
