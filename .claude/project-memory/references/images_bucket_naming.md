---
name: "AWS resource naming (S3 bucket and DynamoDB table)"
description: "Fixed names in dev (LocalStack); OpenTofu-generated with prefix `temporal-aws-autoscaling-demo-` in AWS. Neither is a user-tunable knob."
type: project
---

# AWS resource naming (S3 bucket and DynamoDB table)

Both the S3 images bucket and the DynamoDB
images table follow the same naming pattern.
Env vars are namespaced `IMAGES_*` to reflect
that every resource in this project relates to
image processing.

|                  | Env var         | Dev (LocalStack)                                  | Prod (AWS)                                                                |
|------------------|-----------------|---------------------------------------------------|---------------------------------------------------------------------------|
| S3 bucket        | `IMAGES_BUCKET` | `temporal-aws-autoscaling-demo-images-local`      | OpenTofu-generated via `bucket_prefix = "temporal-aws-autoscaling-demo-"` |
| DynamoDB table   | `IMAGES_TABLE`  | `temporal-aws-autoscaling-demo-images-local`      | OpenTofu-generated with prefix `temporal-aws-autoscaling-demo-`           |

Bucket and table share the same literal dev
name. That is fine because S3 and DynamoDB live
in separate namespaces — there is no collision.
In prod, Tofu generates distinct random
suffixes, so the names diverge automatically.

The Go code reads `IMAGES_BUCKET` and
`IMAGES_TABLE` from the environment in both
modes — the values come from `.env` locally and
from Tofu in AWS. Same Go code in dev and prod.

Neither variable is documented in the README
configuration table: they are not user-tunable
knobs, just process plumbing. They live only in `.env.example` (with
their dev default values) and in the Tofu
modules (which inject the prod values onto the
worker ECS task def and backend Lambda env).

**Why:** the bucket and table both need unique
names per environment to avoid collisions; Tofu
already owns their lifecycle, so it should own
their names too. Exposing them as documented env
vars invites drift (two local envs that step on
each other, hardcoded values in tests, etc.).
Chose fixed local names over a Tofu-managed
local resource for simplicity — LocalStack
accepts any name, no `tofu output → .env`
plumbing needed in `make dev`. The
`IMAGES_*` namespace makes the env-var origin
obvious at a glance.

**How to apply:**

- Local bootstrap (awslocal in docker-compose or
  a setup script) must create both resources
  with the exact dev names listed above.
- The Tofu storage module must use `bucket_prefix`
  (not `bucket`) for S3 and a `name_prefix`-style
  pattern for DynamoDB (or `random_id` suffix);
  expose `images_bucket` and `images_table`
  outputs.
- Tofu must inject those outputs as
  `IMAGES_BUCKET` and `IMAGES_TABLE` on the
  worker ECS task def and the backend Lambda env —
  no hardcoded prod defaults anywhere.
- Do not reintroduce either variable in the
  README config table.
