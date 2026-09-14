# Project memory

This file indexes memories captured by the **project-memory**
skill. Each entry below is a one-line pointer to a memory file
in this directory.

> When a new decision **contradicts** an existing
> memory note, do NOT silently override it.
> Instead: surface the conflict, quote the
> existing memory, explain how the new decision
> differs, and ask for explicit confirmation
> before updating. **Do NOT take any action** —
> no tool calls, no file writes — until confirmed.

> **Note wording** — state permanent facts in the
> present tense. A note read out of context must
> not reveal what it replaces or what just
> happened. Ban narration markers: "now", "no
> longer", "previously / used to", "reverses /
> replaces", "kept", "changed to", "reintroduce",
> "the user asked to". Phrase prohibitions
> positively ("the API is versioned under /v2"),
> not as the negation of a former state. Test:
> remove the note from its context — if a sentence
> only makes sense knowing the prior state,
> rewrite it.

<!-- Add entries below as `- [Title](file.md) — one-line hook` -->

- [AWS resource naming (S3 bucket and DynamoDB table)](references/images_bucket_naming.md) — fixed `-local` dev names, Tofu-generated in AWS
- [Backend run-mode detection](references/backend_run_mode_detection.md) — HTTP vs Lambda chosen by `AWS_ENDPOINT_URL` presence; no `RUN_MODE`
- [Cloudflare provider v5 breaking changes](references/iac_provider_versions.md) — `cloudflare_dns_record`, `content` not `value`, FQDN `name`
- [Commit message convention](references/commit_message_convention.md) — imperative capitalized subject, no Conventional Commits prefix
- [Determinism tests assert every run against manifest.SizeNames](references/workflow_determinism_test_repeats.md) — ~10 runs, expected sequence
- [Dev-only Vue warnings from a stale Vite cache](references/dev_vue_warnings_stale_vite_cache.md) — hydration warnings mean two cached Vue copies
- [Frontend Tailwind setup](references/tailwind_setup.md) — v4 via `@tailwindcss/vite`; theme in `@theme` in `main.css`, no config file
- [Keep WORKER_MAX_CONCURRENT_ACTIVITIES env knob](references/worker_max_concurrent_activities.md) — deliberate burst demo dial; not dead config
- [Local AWS emulator: Moto Server](references/local_aws_emulator.md) — `motoserver/moto` on default internal port 5000, host port 4566
- [No per-image notifications](references/no_per_image_notifications.md) — at most one end-of-burst toast, never one per image; errors may toast
- [No workflow.GetVersion in ProcessImage workflows](references/workflow_no_versioning.md) — short-lived workflows ship code without version gates
- [Operating Temporal Worker Deployments from the CLI](references/temporal_worker_deployment_ops.md) — `--unversioned` first; namespace rate limit
- [Per-worktree compose port isolation](references/worktree_compose_port_isolation.md) — one generator, `CASPER_PORT` and `CMUX_PORT` entry points
- [pnpm dependency build approvals](references/pnpm_build_approvals.md) — approve builds via `allowBuilds` in `frontend/pnpm-workspace.yaml`
- [pnpm invocations run from inside frontend/](references/pnpm_corepack_invocation.md) — `cd frontend && corepack pnpm …`; the pin is read from cwd
- [pnpm minimumReleaseAge exemptions](references/pnpm_minimum_release_age.md) — exempt deps published <24h by name, never globally
- [Recording activity schedule order in Temporal Go tests](references/temporal_test_schedule_order_interceptor.md) — interceptor on ExecuteActivity
- [Recursive Make helper variables need unexport](references/makefile_unexport_helpers.md) — bare `export` expands them per recipe with empty args
- [Temporal metric task_type casing](references/temporal_metric_task_type_casing.md) — values are capitalized; lowercase breaks alarms silently
- [WorkflowExecutionStatus.String() pitfall](references/temporal_status_enum_string.md) — returns CamelCase ("Running"), not SCREAMING_SNAKE
- [Worktree env symlinks](references/worktree_env_symlinks.md) — cmux worktrees need `.env`/`.env.local` symlinked; Casper copies
