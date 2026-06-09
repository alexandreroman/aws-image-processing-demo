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
- [Local AWS emulator: Moto Server](references/local_aws_emulator.md) — uses `motoserver/moto` (LocalStack 2026 is Pro-licensed); host 4566 → container 5000
- [Moto: keep the default listening port](references/feedback_moto_default_port.md) — never change moto's internal port; keep it at the default (5000)
- [No per-image notifications](references/no_per_image_notifications.md) — never emit a toast per processed image; at most one end-of-burst toast; errors may still toast individually
- [No workflow.GetVersion in ProcessImage workflows](references/workflow_no_versioning.md) — workflows are short-lived; rollouts ship code directly without versioning gates
- [Worktree env symlinks](references/worktree_env_symlinks.md) — new git worktrees need both `.env` and `.env.local` symlinked from the main worktree, otherwise `make dev` is broken
- [Cleanup recipe for orphan Temporal Worker Deployments](references/temporal_worker_deployment_cleanup.md) — `set-current-version --unversioned` first, then `delete-version --skip-drainage`, then `delete`
- [Temporal WorkflowExecutionStatus.String() pitfall](references/temporal_status_enum_string.md) — `.String()` returns CamelCase ("Running"), not the SCREAMING_SNAKE constant; keep the explicit `statusName` switch
- [Keep WORKER_MAX_CONCURRENT_ACTIVITIES env knob](references/worker_max_concurrent_activities.md) — deliberate demo dial for burst/autoscaling/backpressure; do not prune as dead config
- [Temporal Cloud metric task_type dimension casing](references/temporal_metric_task_type_casing.md) — ADOT-republished `task_type` values are capitalized (`Workflow`/`Activity`); lowercase breaks backlog alarms silently
- [Lambda runtime breaks when worker deployment registration drifts](references/lambda_runtime_registration_drift.md) — infra-only Lambda redeploys leave Temporal Cloud pointing at a stale version; symptom is HTTP 500 on /api/pipelines/{id}, empty gallery, 0 pollers on image-processing-lambda
- [temporal CLI delete-version has no --yes (set-current-version does)](references/temporal_cli_deployment_flag_asymmetry.md) — rebind must bound delete-version with --command-timeout and fail loud when stranded --unversioned; never reorder the --unversioned step
