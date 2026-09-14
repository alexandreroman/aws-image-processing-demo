# --- Worker security group ------------------------------------------------
#
# No ingress (the worker exposes no public ports); all egress allowed so the
# task can reach Temporal Cloud, Anthropic, S3, DynamoDB and GHCR.

resource "aws_security_group" "worker" {
  name        = "${var.name_prefix}-worker"
  description = "Egress-only SG for the Temporal worker task"
  vpc_id      = var.vpc_id

  egress {
    description = "Allow all egress"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.name_prefix}-worker"
  }
}

# --- ECS cluster + log group ----------------------------------------------

resource "aws_ecs_cluster" "worker" {
  name = "${var.name_prefix}-cluster"
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/ecs/${var.name_prefix}-worker"
  retention_in_days = var.log_retention_days
}

# --- IAM: task execution role (ECS agent pulls image + reads secrets) -----

data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "worker_execution" {
  name               = "${var.name_prefix}-worker-ecs-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

resource "aws_iam_role_policy_attachment" "worker_execution_managed" {
  role       = aws_iam_role.worker_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Secrets the execution role must be able to read for `secrets:` injection.
data "aws_iam_policy_document" "worker_secrets_read" {
  statement {
    actions = ["secretsmanager:GetSecretValue"]
    resources = concat(
      [var.anthropic_secret_arn],
      var.temporal_tls_enabled ? [
        var.temporal_tls_cert_secret_arn,
        var.temporal_tls_key_secret_arn,
      ] : [],
    )
  }
}

resource "aws_iam_role_policy" "worker_execution_secrets" {
  name   = "secrets-read"
  role   = aws_iam_role.worker_execution.id
  policy = data.aws_iam_policy_document.worker_secrets_read.json
}

# --- IAM: task role (the worker process itself) ---------------------------

resource "aws_iam_role" "worker_task" {
  name               = "${var.name_prefix}-worker-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

resource "aws_iam_role_policy" "worker_task" {
  name   = "worker-task"
  role   = aws_iam_role.worker_task.id
  policy = var.task_policy_json
}

# --- Task definition ------------------------------------------------------

locals {
  worker_env = [
    { name = "TEMPORAL_ADDRESS", value = var.temporal_address },
    { name = "TEMPORAL_NAMESPACE", value = var.temporal_namespace },
    { name = "TEMPORAL_TASK_QUEUE", value = var.temporal_task_queue },
    { name = "AWS_REGION", value = var.aws_region },
    { name = "IMAGES_BUCKET", value = var.images_bucket_name },
    { name = "IMAGES_TABLE", value = var.images_table_name },
    { name = "WORKER_MAX_CONCURRENT_ACTIVITIES", value = tostring(var.worker_max_concurrent_activities) },
  ]

  worker_secrets = concat(
    [
      {
        name      = "ANTHROPIC_API_KEY"
        valueFrom = var.anthropic_secret_arn
      },
    ],
    var.temporal_tls_enabled ? [
      {
        name      = "TEMPORAL_TLS_CERT"
        valueFrom = var.temporal_tls_cert_secret_arn
      },
      {
        name      = "TEMPORAL_TLS_KEY"
        valueFrom = var.temporal_tls_key_secret_arn
      },
    ] : [],
  )
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.name_prefix}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "1024"
  memory                   = "2048"
  execution_role_arn       = aws_iam_role.worker_execution.arn
  task_role_arn            = aws_iam_role.worker_task.arn

  runtime_platform {
    cpu_architecture        = "ARM64"
    operating_system_family = "LINUX"
  }

  container_definitions = jsonencode([
    {
      name      = "worker"
      image     = var.worker_image
      essential = true

      # default 30s is not enough to drain a Temporal
      # worker with concurrent activities. 120s gives in-flight activities
      # time to complete or heartbeat-fail cleanly.
      stopTimeout = 120

      environment = local.worker_env
      secrets     = local.worker_secrets

      healthCheck = {
        command     = ["CMD", "wget", "-qO-", "http://localhost:8001/healthz"]
        interval    = 10
        timeout     = 3
        retries     = 5
        startPeriod = 10
      }

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.worker.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "worker"
        }
      }
    },
  ])
}

# --- ECS service ----------------------------------------------------------

resource "aws_ecs_service" "worker" {
  name            = "${var.name_prefix}-worker"
  cluster         = aws_ecs_cluster.worker.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.worker.id]
    assign_public_ip = true
  }

  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 200

  # Auto-rollback a bad task definition (image pull failure, crash loop,
  # etc.) instead of leaving the service stuck at 0 healthy tasks.
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  # The autoscaling target below manages desired_count at runtime;
  # ignoring it here stops Tofu from snapping the count back to 1 on
  # every apply.
  lifecycle {
    ignore_changes = [desired_count]
  }
}

# --- Application Auto Scaling --------------------------------------------
#
# Step scaling on a CloudWatch metric emitted by the ADOT Collector ECS task
# (see infra/worker-otel-collector), which scrapes Temporal Cloud's
# OpenMetrics endpoint and republishes
# temporal_cloud_v1_approximate_backlog_count in the TemporalDemo/Worker
# namespace, dimensioned by `temporal_task_queue` and `task_type`. Each
# alarm uses Metric Math to sum the two task_type series (Workflow +
# Activity) for our task queue, since a worker pulls from both.
#
# End-to-end reactivity is bounded by Temporal Cloud's 3-minute aggregation
# latency plus the collector's 60 s scrape interval; expect a ~4–5 min
# reaction time. Meaningful for sustained or repeated load, not single
# short bursts.

locals {
  # Autoscaling tuning. Deliberately locals rather than module variables:
  # the root module never overrides them, and WORKER_ECS_MAX_INSTANCES
  # (var.autoscaling_max_capacity) is the one knob meant to be turned.
  autoscaling_min_capacity = 1  # warm capacity, never zero by design
  scale_out_threshold      = 10 # total backlog that fires scale-out (first step boundary)
  scale_out_step_2_lower   = 30 # total backlog at which the step jumps to +2 tasks
  scale_out_step_3_lower   = 60 # total backlog at which the step jumps to +3 tasks
  scale_in_threshold       = 5  # total backlog below which scale-in fires

  # Metric-query id → `task_type` dimension value. The capitalisation is NOT
  # cosmetic: ADOT republishes Temporal Cloud's task_type label capitalised,
  # so a lowercase dimension here matches no datapoint and both alarms sit
  # in INSUFFICIENT_DATA forever — silently, with no scaling at all.
  backlog_task_types = {
    workflow = "Workflow"
    activity = "Activity"
  }

  # The two alarms watch the very same metric math and differ only in the
  # threshold, the comparison, how long the condition must hold, and which
  # scaling policy they trigger.
  backlog_alarms = {
    high = {
      name_suffix         = "backlog-high"
      description         = "Triggers scale-out when the Temporal task queue backlog exceeds the threshold."
      threshold           = local.scale_out_threshold
      comparison_operator = "GreaterThanThreshold"
      evaluation_periods  = 1
      policy_arn          = one(aws_appautoscaling_policy.scale_out[*].arn)
    }
    low = {
      name_suffix         = "backlog-low"
      description         = "Triggers scale-in when the backlog stays below the threshold for the configured window."
      threshold           = local.scale_in_threshold
      comparison_operator = "LessThanThreshold"
      evaluation_periods  = 5
      policy_arn          = one(aws_appautoscaling_policy.scale_in[*].arn)
    }
  }
}

resource "aws_appautoscaling_target" "worker" {
  count = var.autoscaling_enabled ? 1 : 0

  service_namespace  = "ecs"
  resource_id        = "service/${aws_ecs_cluster.worker.name}/${aws_ecs_service.worker.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = local.autoscaling_min_capacity
  max_capacity       = var.autoscaling_max_capacity
}

resource "aws_cloudwatch_metric_alarm" "backlog" {
  for_each = var.autoscaling_enabled ? local.backlog_alarms : {}

  alarm_name        = "${var.name_prefix}-worker-${each.value.name_suffix}"
  alarm_description = each.value.description

  evaluation_periods  = each.value.evaluation_periods
  datapoints_to_alarm = each.value.evaluation_periods
  threshold           = each.value.threshold
  comparison_operator = each.value.comparison_operator
  # Missing data is treated as "not breaching" so a scrape outage does not
  # spuriously scale the service in either direction.
  treat_missing_data = "notBreaching"

  dynamic "metric_query" {
    for_each = local.backlog_task_types

    content {
      id          = metric_query.key
      return_data = false

      metric {
        namespace   = "TemporalDemo/Worker"
        metric_name = "temporal_cloud_v1_approximate_backlog_count"
        period      = 60
        stat        = "Maximum"
        dimensions = {
          temporal_task_queue = var.temporal_task_queue
          task_type           = metric_query.value
        }
      }
    }
  }

  metric_query {
    id          = "total"
    expression  = "workflow + activity"
    label       = "Total backlog"
    return_data = true
  }

  alarm_actions = [each.value.policy_arn]
}

resource "aws_appautoscaling_policy" "scale_out" {
  count = var.autoscaling_enabled ? 1 : 0

  name               = "${var.name_prefix}-worker-scale-out"
  policy_type        = "StepScaling"
  service_namespace  = aws_appautoscaling_target.worker[0].service_namespace
  resource_id        = aws_appautoscaling_target.worker[0].resource_id
  scalable_dimension = aws_appautoscaling_target.worker[0].scalable_dimension

  step_scaling_policy_configuration {
    adjustment_type         = "ChangeInCapacity"
    cooldown                = 30
    metric_aggregation_type = "Maximum"

    # Step boundaries are *relative to the alarm threshold* (scale_out_threshold).
    # E.g. with threshold=10: 10..30 → +1, 30..60 → +2, ≥60 → +3.
    step_adjustment {
      metric_interval_lower_bound = 0
      metric_interval_upper_bound = local.scale_out_step_2_lower - local.scale_out_threshold
      scaling_adjustment          = 1
    }
    step_adjustment {
      metric_interval_lower_bound = local.scale_out_step_2_lower - local.scale_out_threshold
      metric_interval_upper_bound = local.scale_out_step_3_lower - local.scale_out_threshold
      scaling_adjustment          = 2
    }
    step_adjustment {
      metric_interval_lower_bound = local.scale_out_step_3_lower - local.scale_out_threshold
      scaling_adjustment          = 3
    }
  }
}

resource "aws_appautoscaling_policy" "scale_in" {
  count = var.autoscaling_enabled ? 1 : 0

  name               = "${var.name_prefix}-worker-scale-in"
  policy_type        = "StepScaling"
  service_namespace  = aws_appautoscaling_target.worker[0].service_namespace
  resource_id        = aws_appautoscaling_target.worker[0].resource_id
  scalable_dimension = aws_appautoscaling_target.worker[0].scalable_dimension

  step_scaling_policy_configuration {
    adjustment_type         = "ChangeInCapacity"
    cooldown                = 120
    metric_aggregation_type = "Maximum"

    step_adjustment {
      metric_interval_upper_bound = 0
      scaling_adjustment          = -1
    }
  }
}

# Transitional: the two backlog alarms used to be separate resources. Without
# these the next apply would upsert the new alarm and then delete it again
# under its old address, silently leaving the service unscaled. Safe to
# delete once every deployment has applied this change.
moved {
  from = aws_cloudwatch_metric_alarm.backlog_high[0]
  to   = aws_cloudwatch_metric_alarm.backlog["high"]
}

moved {
  from = aws_cloudwatch_metric_alarm.backlog_low[0]
  to   = aws_cloudwatch_metric_alarm.backlog["low"]
}
