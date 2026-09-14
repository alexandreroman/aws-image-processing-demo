---
name: "Recording activity schedule order in Temporal Go tests"
description: "Record ordering with a worker interceptor on ExecuteActivity; the activity-started listener runs on activity goroutines, unordered"
type: feedback
---

# Recording activity schedule order in Temporal Go tests

A test that asserts the order in which a workflow issues its
activities records that order with a worker interceptor: an
`interceptor.WorkerInterceptor` whose outbound interceptor
overrides `ExecuteActivity`, installed through
`env.SetWorkerOptions`. `SetOnActivityStartedListener` is
unsuitable for ordering assertions.

**Why:** Interception happens on the workflow goroutine, so the
recorded sequence is the order in which the workflow emitted its
commands — which is exactly what determinism means. The started
listener instead fires on each activity's own goroutine, so the
start order of a parallel fan-out varies run to run even when
the workflow code is correct; a listener-based test fails
against correct code within a handful of runs (observed on run 2
of 20). Throttling the fan-out is not an escape hatch either:
the test environment ignores
`MaxConcurrentActivityExecutionSize` in the options passed to
`SetWorkerOptions`, so activities cannot be serialized that way.

**How to apply:** Model new ordering tests on `scheduleRecorder`
in `internal/workflows/process_image_test.go`. The interceptor
receives typed activity inputs as `args ...any`, so read the
size name or key with a type switch on `args[0]` (for example
`activities.ResizeInput`) rather than decoding JSON. Guard the
recorded slice with a mutex and hand back a copy.
Pair it with the repeat-count rule in
[Determinism tests](workflow_determinism_test_repeats.md).
