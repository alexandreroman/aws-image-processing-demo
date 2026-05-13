# temporal-aws-demo

Image-processing burst pipeline demonstrating
Temporal Cloud + AWS for AWS architects and
developers.

See [README.md](README.md) for installation, usage,
configuration, and architecture.

## Tech stack

- Go (Temporal SDK, AWS SDK v2, Anthropic SDK)
- Nuxt 4 SSG, Tailwind, pnpm
- Temporal Cloud (workflow orchestration)
- AWS — ECS Fargate, Lambda, API Gateway, S3,
  DynamoDB, CloudFront
- OpenTofu (IaC) — AWS provider in `eu-west-3` +
  alias in `us-east-1` for ACM, plus Cloudflare DNS
- LocalStack + Temporal CLI dev server for local dev

## Build & run

```bash
make dev               # infra in Docker + worker/backend/frontend on host
make test              # unit tests
make deploy            # tofu init + apply + frontend-deploy
make frontend-deploy   # build Nuxt + sync to S3 + invalidate CloudFront
make teardown          # tofu destroy + cleanup
```

## Modules

- `cmd/` — entry points: `worker` (Fargate),
  `backend` (Lambda + local HTTP).
- `internal/workflows` — `ProcessImage` workflow.
- `internal/activities` — resize, describe, watermark,
  store.
- `internal/manifest` — shared types; **always**
  iterate `manifest.SizeNames`, never the maps.
- `internal/awsclient` — single AWS config that
  honors `AWS_ENDPOINT_URL` so the same code runs
  against LocalStack and real AWS.
- `internal/anthropicclient` — Anthropic API wrapper.
- `internal/api` — HTTP handlers for the three
  endpoints under `/api/*`.
- `frontend/` — Nuxt 4 SSG, two pages: `/` and
  `/sessions/[id]`.
- `infra/` — OpenTofu modules (network, storage,
  worker, backend, frontend, dns).

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

- **Workflow determinism:** never iterate Go maps
  directly inside workflow code. Use the canonical
  ordered slice `manifest.SizeNames`. Use
  `workflow.Now()` / `workflow.GetLogger()` /
  `workflow.Sleep()`, never the `time` or `log`
  equivalents.
- **All backend routes are prefixed with `/api`.**
  This is what makes CloudFront path-based routing
  work cleanly (`/api/*` → API Gateway, `/*` → S3).
- **S3 prefix convention:** uploads under
  `uploads/` (flat), derived artifacts under
  `sessions/{sessionId}/...`. Lifecycle rules expire
  `uploads/` after 7 days and `sessions/` after 30.
- **Anthropic API direct, not Bedrock.** Keeps local
  dev simple (LocalStack does not mock Bedrock).
