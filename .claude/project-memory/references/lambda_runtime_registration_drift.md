---
name: "Lambda runtime breaks when worker deployment registration drifts"
description: "Infra-only Lambda redeploys silently break Temporal Cloud task delivery; symptom is HTTP 500 on /api/pipelines/{id}"
type: project
---

# Lambda runtime breaks when worker deployment registration drifts

Incident (2026-06-09): pipelines started on the **Lambda** runtime
hung at the first workflow task and `GET /api/pipelines/{id}`
returned **HTTP 500** (`context deadline exceeded`). The ECS runtime
was unaffected.

Diagnostic chain (all read-only AWS + `temporal` CLI against
`alex.a2dd6` in `eu-west-1`):

- Launcher workflow `image-pipeline-{id}` stuck RUNNING with
  `historyLength: 2`, `pendingWorkflowTask` SCHEDULED, never picked up.
- Task queue `image-processing-lambda` had **0 pollers**;
  `image-processing-ecs` had 2 active pollers.
- Worker Lambda CloudWatch logs empty; **no `AssumeRole` on the
  `*-worker-invoker` role** in CloudTrail → Temporal Cloud was not
  invoking the Lambda at all. `temporal worker deployment
  describe-version` itself timed out.
- `handlePipeline` resolves the launcher via **`QueryWorkflow`**
  (`fetchPipelineWorkflowIDs`, internal/api/api.go). A query needs a
  live poller to answer; with none, it blocks until the backend
  Lambda's 10s timeout → 500. Empty gallery has the same root (the
  launcher never started the `ProcessImage` workflows).

**Why:** `scripts/register-worker-deployment.sh` keys the Temporal
Worker Deployment Version by **git short SHA** (`build_id`) and
registers a **version-qualified Lambda ARN**
(`worker_lambda_function_arn` = `...:worker:N`, see infra/outputs.tf).
Each `tofu apply` republishes the Lambda (`publish = true`) bumping
`:N`, but if the git HEAD is unchanged, `create-version` returns
"already exists" and is skipped, so the registered ARN/binding is
**never refreshed**. Confirmed by `currentVersionChangedTime` staying
at the prior date while the live Lambda had newer published versions.

**How to apply:** When the Lambda runtime is "down" (500s on pipeline
status, empty gallery) but ECS works, check pollers with
`temporal task-queue describe --task-queue image-processing-lambda`
and the deployment with `temporal worker deployment describe --name
aws-image-processing-demo-worker-lambda` (compare
`currentVersionChangedTime` against the Lambda's `LastModified`).
Remediation: re-register against the current Lambda ARN — needs a new
build id (new commit) or delete+recreate the version (see
[temporal_worker_deployment_cleanup.md](temporal_worker_deployment_cleanup.md)),
then terminate the stuck launcher workflows. Proper fix: make the
deploy script re-key/refresh the binding on the Lambda ARN/version,
not only on the git SHA. Workaround for users: select the **ECS**
runtime in the UI.
