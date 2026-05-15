# Project memory

This file indexes memories captured by the **project-memory**
skill. Each entry below is a one-line pointer to a memory file
in this directory.

<!-- Add entries below as `- [Title](file.md) — one-line hook` -->

- [AWS resource naming (S3 bucket and DynamoDB table)](references/images_bucket_naming.md) — fixed `-local` names in dev; Tofu-generated with prefix `aws-image-processing-demo-` in AWS; neither is a user knob
- [Commit message convention](references/commit_message_convention.md) — imperative subject, capitalized, no Conventional Commits prefix (no fix:/chore:/refactor:)
- [Backend run-mode detection](references/backend_run_mode_detection.md) — `cmd/backend` picks HTTP vs Lambda from `AWS_ENDPOINT_URL` presence; do not reintroduce `RUN_MODE`
- [Dev mode: host processes + Docker infra split](references/dev_mode_split.md) — `make dev` runs Go + Nuxt on host with infra in Docker; `make app-up` brings the full stack up in Docker
- [Triggering a workflow from the Temporal CLI](references/workflow_cli.md) — launch a single `ProcessImage` workflow via `temporal workflow start` for debug or scripted invocation
- [IaC provider versions in infra/](references/iac_provider_versions.md) — AWS ~> 6.0 and Cloudflare ~> 5.0; v5 uses `cloudflare_dns_record` with `content` and FQDN `name`
- [Known prod gap: frontend image display](references/known_prod_image_display_gap.md) — gallery images are broken in prod (bundle hardcodes localhost, bucket private, no CF origin); user deferred fix on 2026-05-14
- [Local AWS emulator: Moto Server](references/local_aws_emulator.md) — uses `motoserver/moto` (LocalStack 2026 is Pro-licensed); host 4566 → container 5000
- [Moto: keep the default listening port](references/feedback_moto_default_port.md) — never change moto's internal port; keep it at the default (5000)
- [No per-image notifications](references/no_per_image_notifications.md) — never emit a toast per processed image; at most one end-of-burst toast; errors may still toast individually
- [No workflow.GetVersion in ProcessImage workflows](references/workflow_no_versioning.md) — workflows are short-lived; rollouts ship code directly without versioning gates
- [Worktree env symlinks](references/worktree_env_symlinks.md) — new git worktrees need both `.env` and `.env.local` symlinked from the main worktree, otherwise `make dev` is broken
