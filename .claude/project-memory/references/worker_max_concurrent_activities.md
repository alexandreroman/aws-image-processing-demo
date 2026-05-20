---
name: "Keep WORKER_MAX_CONCURRENT_ACTIVITIES env knob"
description: "do not remove the WORKER_MAX_CONCURRENT_ACTIVITIES tunable as dead config; it is a deliberate demo dial"
type: feedback
---

# Keep WORKER_MAX_CONCURRENT_ACTIVITIES env knob

Do **not** remove `WORKER_MAX_CONCURRENT_ACTIVITIES`
on the grounds that "the default is never
overridden". Keep it wired end-to-end:

- read in `cmd/worker/main.go` (both
  `runLongRunning` and `runLambda`)
- exposed as the `worker_max_concurrent_activities`
  Tofu variable (default 4), plumbed to the
  `worker-ecs` and `worker-lambda` modules
- overlay support in `scripts/lib/env.sh`
- documented in `.env.example` and the README

**Why:** This is a demo project for AWS
architects and Temporal Cloud. The whole point
is being able to dial worker concurrency at
demo time to *show* burst behavior, autoscaling,
backpressure, and worker saturation. The knob is
a feature of the demo narrative, not stale
config. A previous simplification pass deleted
it as "unused" and the user immediately
restored it.

**How to apply:** Treat this env var like a
public API of the demo. When pruning unused
config, skip this one. If a future review flags
it as dead, point reviewers here.
