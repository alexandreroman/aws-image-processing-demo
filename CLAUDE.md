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
  - Exception: files under `.claude/project-memory/`
    follow the project-memory skill's own
    150-character cap for frontmatter `description:`
    fields and `MEMORY.md` index lines
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
because they are not obvious from the code alone. The
README is the reference for everything else — setup,
configuration, and architecture — so the pointers
below are deliberate, not summaries to expand.

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
  as soon as every start is acknowledged. Comments,
  docs, and tests must never call them children.
- **No upload path.** The bucket is pre-seeded with
  curated samples under `samples/` at the repo root
  (kept indefinitely); `workflows/start` rejects any
  key outside that prefix. Derived artifacts live
  under `pipelines/{pipelineId}/...` and expire after
  30 days.
- **`internal/awsclient` honors `AWS_ENDPOINT_URL`**
  so the same code path runs against Moto Server and
  real AWS.
- **`make dev` and `make app-up` are deliberately
  separate**: host processes with hot reload against
  Docker infra, versus the whole stack in Docker.
  Never collapse them into one target.
- **Worker mode is detected at runtime**, not via build
  flags — see [Deployment](README.md#deployment).
- **All backend API routes are prefixed with `/api`**,
  with `/healthz` at the root as the deliberate
  exception — see
  [Request flow](README.md#request-flow).
- **Env split:** `.env` is canonical, `.env.local` is a
  host-mode-only dev overlay, and the compose stack
  reads neither — see
  [Configuration](README.md#configuration).
- **Worker runtime selection is per burst**, and only
  in AWS-deployed environments — see
  [Per-burst runtime selection](README.md#per-burst-runtime-selection).
- **Anthropic API direct, not Bedrock** — see
  [Prerequisites](README.md#prerequisites).
