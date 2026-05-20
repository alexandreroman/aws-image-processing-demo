---
name: "Temporal Cloud metric task_type dimension casing"
description: "ADOT-republished CloudWatch metric task_type dimension is capitalized (Workflow/Activity), not lowercase"
type: project
---

# Temporal Cloud metric task_type dimension casing

The ADOT collector scrapes Temporal Cloud's OpenMetrics endpoint
and republishes `temporal_cloud_v1_approximate_backlog_count`
into the `TemporalDemo/Worker` CloudWatch namespace. The
`task_type` dimension values are **capitalized**: `Workflow` and
`Activity` — not `workflow` / `activity`.

**Why:** Temporal Cloud emits the values capitalized at the
OpenMetrics source; the collector passes them through verbatim.
Verified against the live endpoint and CloudWatch metrics in
eu-west-1. A previous lowercase mismatch in
`infra/worker-ecs/main.tf` caused the metric-math expression
`workflow + activity` to return no datapoints, leaving the
backlog alarms permanently in OK (because
`treat_missing_data = "notBreaching"`) and autoscaling never
fired.

**How to apply:** Any CloudWatch alarm, metric math expression,
or dashboard widget filtering on this `task_type` dimension must
use the capitalized values. Watch for regressions in
`infra/worker-ecs/main.tf` and any future
`aws_cloudwatch_metric_alarm` / `aws_cloudwatch_dashboard`
resources that touch this metric.
