# aws-image-processing-demo

Image-processing burst pipeline demonstrating
Temporal Cloud + AWS for AWS architects and
developers.

See [README.md](README.md) for installation, usage,
configuration, and architecture. Tech stack, module
layout, and build commands are derivable from
`go.mod`, the directory tree, and the `Makefile` —
not duplicated here.

## Agents

Use the following agents (from the
[skillbox](https://github.com/alexandreroman/skillbox)
plugin) for all code tasks:

- **code-writer** — for ANY task that writes,
  modifies, or refactors code. This includes
  one-line fixes, import changes, visibility
  tweaks, and adding assertions. Never use
  the Edit or Write tools directly on source
  files — always delegate to this agent.
- **code-reviewer** — for read-only code review
  before merging or when investigating issues.

## Memory

At the start of every conversation, read
`.claude/project-memory/MEMORY.md` to load
project context from previous conversations.

Use the **project-memory** skill (from the
[skillbox](https://github.com/alexandreroman/skillbox)
plugin) proactively — without being asked — whenever
the conversation reveals project decisions, deadlines,
team context, external references, workflow preferences,
or corrective feedback worth persisting across
conversations.

**Important:** Always use the **project-memory**
skill to persist information. Never use the built-in
auto-memory system (`~/.claude/projects/.../memory/`)
for project decisions or context — it is local and
not shared with the team.

## Conventions

- Line length limits for readability:
  - Text / Markdown: 80 columns max
  - Code: 120 columns max
- Follow standard Markdown conventions: blank line
  before and after headings, blank line before and
  after lists, fenced code blocks with a language tag
- Always use the latest LTS or stable version of
  languages, frameworks, and libraries. Check the
  official documentation or use available tools
  (e.g. context7) to verify current versions before
  choosing a dependency.

## Project-specific rules

These are invariants that are easy to violate
because they are not obvious from the code alone.

- **Workflow determinism:** never iterate Go maps
  directly inside workflow code. Use the canonical
  ordered slice `manifest.SizeNames`. Use
  `workflow.Now()` / `workflow.GetLogger()` /
  `workflow.Sleep()`, never the `time` or `log`
  equivalents.
- **`ProcessImage` workflows are top-level, not
  Temporal children of `LaunchPipelines`.** They are
  launched via a starter activity that calls
  `client.ExecuteWorkflow`, so the launcher returns
  as soon as every start is acknowledged.
- **Worker mode is detected at runtime**, not via
  build flags: presence of `AWS_LAMBDA_FUNCTION_NAME`
  switches the single Go binary into Lambda mode
  (using `go.temporal.io/sdk/contrib/aws/lambdaworker`);
  otherwise it long-polls.
- **All backend API routes are prefixed with `/api`**
  so CloudFront can dispatch by path (`/api/*` → API
  Gateway, `/images/*` → S3 images bucket via OAC,
  `/*` → S3 frontend bucket). The `/healthz` liveness
  probe is the deliberate exception — both backend
  (`:8000`) and worker (`:8001`) expose it at the
  root for container orchestrators, and neither is
  reachable through CloudFront.
- **No upload path.** The bucket is pre-seeded with
  curated samples under `samples/` (kept
  indefinitely); `workflows/start` rejects any key
  outside that prefix. Derived artifacts live under
  `pipelines/{pipelineId}/...` and expire after
  30 days.
- **Anthropic API direct, not Bedrock.** Keeps local
  dev simple (Moto Server does not mock Bedrock).
- **`internal/awsclient` honors `AWS_ENDPOINT_URL`**
  so the same code path runs against Moto Server and
  real AWS.
- **Env split:** `.env` is the canonical deploy-shaped
  configuration. `.env.local` is an opt-in dev overlay
  layered on top by host-mode dev targets only.
  Deploy targets load only `.env`. The compose stack
  (`make app-up`) is self-contained and does NOT read
  either file beyond `ANTHROPIC_API_KEY` (compose-time
  interpolation).
- **Worker runtime selection** happens per burst at
  the API layer in AWS-deployed environments only.
  Tofu sets `WORKER_TASK_QUEUE_ECS` and
  `WORKER_TASK_QUEUE_LAMBDA` on the deployed backend
  Lambda; the backend advertises both via
  `GET /api/runtimes` and the UI shows a selector. In
  local dev those vars are unset, the API returns
  `[]`, and the single worker polls a fixed queue.
