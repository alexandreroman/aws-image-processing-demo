---
name: "No workflow.GetVersion in ProcessImage workflows"
description: "Workflow code in this demo intentionally skips workflow.GetVersion; rollouts rely on workflows being short-lived."
type: project
---

# No workflow.GetVersion in ProcessImage workflows

ProcessImage and LaunchPipelines do not use
`workflow.GetVersion` for in-place workflow code changes.
Rollouts (e.g. the recent starter-activity change,
the partial-manifest persistence step) simply ship the
new code.

**Why:** ProcessImage workflows complete in seconds, so
the window where an in-flight execution could replay
against newer workflow code is very small and acceptable
for a demo. Adding versioning would be ceremony with no
real payoff here.

**How to apply:** When adding, reordering, or removing
activity calls in workflows under `internal/workflows/`,
do not introduce `workflow.GetVersion` gates. Match the
existing rollout style — change the code directly. Only
revisit this if a workflow grows long-lived (minutes or
longer) or the demo turns into a production system.
