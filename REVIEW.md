# Code review — recent `main` activity (`main~10..main`)

Scope: 10 commits on `main`, ending at `fd5377b`. Themes:
pipeline duration metric + animated Running indicator,
worker compute right-sizing, backlog alarm casing fix,
and a docs restructure. Read-only review — no source
files modified.

Overall this is a clean batch. One real bug worth
flagging in the duration aggregation (drift from a
wall-clock `time.Now()` mixed with Temporal server
times), plus a small handful of minor issues and dead
code left over from the 739f7dc refactor.

## 1 — Pipeline duration metric + animated Running indicator

### 1.1 — [minor] `pipelineTiming` mixes Temporal server time with backend wall clock

**File:** `internal/api/api.go:421-444`

```go
resp.CompletedAt, resp.DurationMs = pipelineTiming(
    resp.CreatedAt, latestClose, resp.Summary.Running, len(resp.Workflows), time.Now(),
)
```

`createdAt` and `latestClose` are derived from
`exec.GetStartTime()` / `exec.GetCloseTime()` —
**Temporal server time**. The in-flight branch then
computes `now.Sub(createdAt)` where `now` is the
**backend wall clock** (`time.Now()` passed by the
caller).

If the backend is running on a host whose clock is
even a few hundred ms behind the Temporal frontend
clock, the live duration response can be negative or
jump backwards across requests. In Lambda this is
usually fine (NTP is good), but on a developer laptop
or a misconfigured ECS task it surfaces as visible
jitter.

The post-completion branch is internally consistent
(`latestClose - createdAt`, both Temporal server
times), so this only affects the in-flight value.

**Fix:** either drop the in-flight `durationMs`
entirely (the frontend doesn't display it anymore —
see 1.3), or document that the in-flight value is
best-effort and accept it. Cheapest: just return
`nil, nil` while running, since the frontend now
shows "Running" instead of a counting number.

### 1.2 — [minor] `createdAt` is "earliest ProcessImage start", not pipeline start

**File:** `internal/api/api.go:350-356`, README.md:236-243

`resp.CreatedAt` is set to the minimum
`exec.GetStartTime()` across the `ProcessImage`
workflows the visibility API has already indexed. For
small bursts this is within a few ms of the launcher
fire time, but:

- the field is named `createdAt` on the wire and
  rendered as the burst's start time;
- if visibility hasn't indexed the earliest-started
  workflow yet, the synthesized RUNNING placeholders
  (lines 410-419) contribute no `StartTime`, so
  `createdAt` will retroactively shift earlier on
  subsequent polls as visibility catches up. The
  resulting `durationMs` is monotonically *increasing*
  for the right reason but with a moving zero point —
  duration can jump up between polls in the first
  second or two of a burst.

**Fix:** if a pipeline start timestamp is wanted as
the duration anchor, prefer the launcher workflow's
own `StartTime` (already fetched as
`fetchPipelineWorkflowIDs`), which is established
before any child is scheduled.

### 1.3 — [minor] Dead branch in `formatSeconds`

**File:** `frontend/app/pages/pipelines/[id].vue:13-15`

```ts
function formatSeconds(ms: number | null): string {
  return ms === null ? '…' : `${(ms / 1000).toFixed(1)}s`;
}
```

After 739f7dc, `formatSeconds` is only called in the
`v-if="durationMs !== null"` branch (line 83-92), so
the `null` case is unreachable. The "…" placeholder is
now rendered by the separate `key="loading"` branch.

**Fix:** narrow the type to `number` and drop the null
guard, or inline as a one-liner.

### 1.4 — [minor] Backend still emits in-flight `durationMs` that nothing consumes

**File:** `internal/api/api.go:441-443`

After 739f7dc, the frontend only reads
`pipeline.durationMs` once `isDone` is true. The
server keeps returning a fresh-each-poll
`now.Sub(createdAt)` value on every in-flight
response. Harmless, but it's a footgun for anyone
who later reuses the field thinking it ticks
monotonically — see 1.1 about clock-skew jitter.

**Fix:** stop populating `DurationMs` while running,
or rename it to make the "frozen on completion"
contract explicit in the field name.

### 1.5 — [nit] `isDone` and the previous `summary` watcher are equivalent

**File:** `frontend/app/composables/usePipeline.ts:40-45,106-110`

```ts
const isDone = computed<boolean>(() => {
  const p = pipeline.value;
  if (!p) return false;
  if (p.completedAt) return true;
  return p.summary.total > 0 && p.summary.running === 0;
});
```

The `p.completedAt` short-circuit is redundant: the
backend only sets `completedAt` when `running == 0 &&
total > 0` (`pipelineTiming` line 437), which the
fall-through already handles. Not wrong, just two
expressions of the same condition. Keep if you want
the contract documented in code; otherwise drop.

### 1.6 — [nit] Inline `pr-5` on the running indicator visually centers under a scrollbar that may not exist

**File:** `frontend/app/pages/pipelines/[id].vue:97`

```html
class="absolute inset-0 flex items-center justify-center gap-2 pr-5"
```

The `pr-5` compensates for the reserved scrollbar
gutter (`lg:[scrollbar-gutter:stable]` added on the
aside in the same commit). On viewports where the
aside isn't tall enough to require a scrollbar, the
running indicator is shifted ~20 px left of center.

**Fix:** drop the `pr-5` — the gutter is symmetric
enough that horizontal centering will read correctly
with or without the scrollbar.

## 2 — Worker compute right-sizing

No findings.

The Lambda 2048→1024 MB change has a clear measurement
backing it (~89 MB peak, ~11× headroom) and keeps the
CPU/memory ratio adequate for the resize/watermark
activities. ECS Fargate ARM64 is safe: `ci.yml:148-153`
publishes a multi-arch `:latest` via
`docker buildx imagetools create`, so the ECS task
definition's image reference will resolve to the arm64
manifest at pull time.

## 3 — Backlog alarm casing fix

No findings.

The fix matches the documented Temporal Cloud →
ADOT → CloudWatch behavior captured in
`.claude/project-memory/references/temporal_metric_task_type_casing.md`.
Both `backlog_high` and `backlog_low` are updated;
the surrounding comment is also corrected. Worth
verifying once in CloudWatch that datapoints now
appear under the alarms, since the prior failure
mode was silent.

## 4 — README / CLAUDE restructure + Lambda Serverless Workers callout

### 4.1 — [minor] CLAUDE.md is now an invariant-only file; README is the source of truth

**File:** `CLAUDE.md` (post-5e05e4b)

The slimmed CLAUDE.md is good and the invariants list
is accurate against the current code. One nit: the
file now relies on `README.md`, `go.mod`, the
directory tree, and the `Makefile` being current — if
any of those drift, agents lose the only context they
have. Worth periodically diffing CLAUDE.md invariants
against the code to catch silent rot.

The invariants themselves all check out against the
current tree:

- `ProcessImage` top-level — confirmed (`api.go`
  starts workflows directly).
- Worker mode detection via `AWS_LAMBDA_FUNCTION_NAME`
  — confirmed (`cmd/worker/main.go`).
- `/api` prefix + `/healthz` exception — confirmed
  (`cmd/worker/main.go`, `internal/api/api.go`).
- No upload path; `samples/` guard — confirmed
  (handler rejects non-`samples/` keys).
- Anthropic direct, not Bedrock — confirmed.
- `internal/awsclient` honors `AWS_ENDPOINT_URL` —
  confirmed (`internal/awsclient/awsclient.go:31`).

### 4.2 — [nit] `TEMPORAL_TASK_QUEUE` removal from `.env.example` is safe but unannounced

**File:** `.env.example` (removed), `compose.yaml:129,168`,
`cmd/worker/main.go:32,83`

`TEMPORAL_TASK_QUEUE` is still read by
`cmd/worker/main.go` and explicitly set in
`compose.yaml`. Removing it from `.env.example` is
fine because `defaultTaskQueue = "image-processing"`
matches the compose value. But anyone who had a
custom queue in their local `.env` (e.g. running two
demos in parallel against the same Temporal Cloud
namespace) will silently fall back to the default
without warning.

**Fix:** either add a note in the README's local-dev
section that the variable still exists as a worker
override, or leave a comment in `.env.example` saying
"override `TEMPORAL_TASK_QUEUE` if you need to."

### 4.3 — [nit] `.env.local.example` AWS credential block leaves dead state on existing developer machines

**File:** `.env.local.example:11-14`

The removal of `AWS_ACCESS_KEY_ID=test` /
`AWS_SECRET_ACCESS_KEY=test` is correct — Moto
accepts anything. But existing checkouts have those
two lines already populated in `.env.local`, which now
silently override real CLI-profile creds for any
`make dev` invocation. If a developer ever points
`AWS_ENDPOINT_URL` at real AWS by mistake, those
`test` creds will fail; better than the opposite, but
worth a one-line release note in the README's "Local
development" section.

## Looks good

- **Sequence guard in `usePipeline.refresh`** is the
  right call. Out-of-order responses overwriting fresh
  state with stale snapshots is a real failure mode
  under bursty backends, and the monotonic `seq`
  approach is the standard fix.
- **`scrollbar-gutter:stable`** on the aside (739f7dc)
  is a quietly correct fix to a real layout-shift bug
  during the Transition fade — easy to miss, easy to
  get right.
- **Backlog alarm fix** (bd5b70c) including the
  capitalized comment correction at lines 256-258
  shows good follow-through: a casing fix that only
  touches the alarm filters without updating the
  rationale comment would have left a future
  reviewer confused.
- **Lambda memory_size comment** (`infra/worker-lambda/main.tf:135-136`)
  explicitly notes "not lower because Lambda CPU
  scales with memory and resize activities are
  CPU-bound" — exactly the rationale someone needs
  to resist over-correcting later.
- **Test coverage for `pipelineTiming`** with both
  the all-completed and some-running cases is a nice
  small addition; the function would otherwise be
  invisible to unit tests.
- **CLAUDE.md slimming** with the explicit "these are
  invariants that are easy to violate because they
  are not obvious from the code alone" framing is a
  good editorial principle that should keep the file
  from accreting stale facts again.
