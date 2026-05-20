---
name: "Temporal WorkflowExecutionStatus.String() pitfall"
description: "enumspb.WorkflowExecutionStatus.String() returns CamelCase, not the SCREAMING_SNAKE_CASE enum constant — do not use it for the API status field"
type: feedback
---

# Temporal WorkflowExecutionStatus.String() pitfall

When mapping `enumspb.WorkflowExecutionStatus`
to the string returned by `/api/pipelines/{id}`,
use the explicit switch in
`internal/api/api.go:statusName`. Do **not**
"simplify" it to
`strings.TrimPrefix(s.String(),
"WORKFLOW_EXECUTION_STATUS_")`.

**Why:** Temporal's generated `.String()` returns
the CamelCase variant ("Running", "Completed",
"Failed", "Unspecified") — not the full
SCREAMING_SNAKE_CASE constant name. So
`TrimPrefix` is a no-op and the resulting
status (e.g. "Running") does not match the
frontend's `WorkflowStatus` union ("RUNNING",
"COMPLETED", ...). Every in-flight workflow
then briefly flashes as a red-X "failed" tile
in the Gallery until the cached thumb arrives.

**How to apply:** Keep the explicit
`switch` in `statusName`. If you ever introduce
similar enum-to-API-string conversions for other
Temporal protobuf enums, write the mapping
explicitly — do not trust `.String()`. The same
pitfall applies to other `go.temporal.io/api`
enums (status, event type, retry state, etc.).
