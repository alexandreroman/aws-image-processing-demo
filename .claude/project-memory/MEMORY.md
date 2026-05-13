# Project memory

This file indexes memories captured by the **project-memory**
skill. Each entry below is a one-line pointer to a memory file
in this directory.

<!-- Add entries below as `- [Title](file.md) — one-line hook` -->

- [AWS resource naming (S3 bucket and DynamoDB table)](references/images_bucket_naming.md) — fixed `-local` names in dev; Tofu-generated with prefix `temporal-aws-autoscaling-demo-` in AWS; neither is a user knob
- [Backend run-mode detection](references/backend_run_mode_detection.md) — `cmd/backend` picks HTTP vs Lambda from `AWS_ENDPOINT_URL` presence; do not reintroduce `RUN_MODE`
- [Dev mode: host processes + Docker infra split](references/dev_mode_split.md) — `make dev` runs Go + Nuxt on host with infra in Docker; `make app-up` brings the full stack up in Docker
- [Triggering a workflow from the Temporal CLI](references/workflow_cli.md) — launch a single `ProcessImage` workflow via `temporal workflow start` for debug or scripted invocation
- [IaC provider versions in infra/](references/iac_provider_versions.md) — AWS ~> 6.0 and Cloudflare ~> 5.0; v5 uses `cloudflare_dns_record` with `content` and FQDN `name`
- [Local AWS emulator: Moto Server](references/local_aws_emulator.md) — uses `motoserver/moto` (LocalStack 2026 is Pro-licensed); host 4566 → container 5000
