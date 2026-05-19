# ADOT Collector ECS Fargate task.
#
# Scrapes Temporal Cloud's OpenMetrics endpoint (metrics.temporal.io) at
# 60 s intervals, drops every series not matching our task queue, and
# republishes the result as CloudWatch metrics in the TemporalDemo/Worker
# namespace via the awsemf exporter. Two CloudWatch alarms in the
# worker-ecs module then drive the ECS worker's step-scaling policies on
# the workflow + activity sum of approximate_backlog_count.
#
# The collector runs as a single-task service (no autoscaling): one
# always-on Fargate task (~$10/month at 0.25 vCPU / 0.5 GB). Zero
# custom code — the autoscaler is entirely defined by ADOT
# configuration plus CloudWatch alarms.

# --- Collector security group ---------------------------------------------

resource "aws_security_group" "collector" {
  name        = "${var.name_prefix}-otel-collector"
  description = "Egress-only SG for the ADOT collector task"
  vpc_id      = var.vpc_id

  egress {
    description = "Allow all egress"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.name_prefix}-otel-collector"
  }
}

# --- Log group ------------------------------------------------------------

resource "aws_cloudwatch_log_group" "collector" {
  name              = "/ecs/${var.name_prefix}-otel-collector"
  retention_in_days = 14
}

# --- Inline ADOT YAML config (read by the container via SSM) --------------
#
# Stored as a SecureString so it can be injected via the ECS task
# definition's `secrets:` block (the AWS OTel Collector image reads the
# full YAML config from AOT_CONFIG_CONTENT). `${env:VAR}` substitution is
# performed by the collector at startup against the container environment.

locals {
  collector_config = <<-EOT
    receivers:
      prometheus:
        config:
          scrape_configs:
            - job_name: temporal-cloud
              scrape_interval: 60s
              scrape_timeout: 30s
              honor_timestamps: true
              scheme: https
              authorization:
                type: Bearer
                credentials: $${env:TEMPORAL_METRICS_API_KEY}
              static_configs:
                - targets: [metrics.temporal.io]
              metrics_path: /v1/metrics
              params:
                namespaces: ['$${env:TEMPORAL_NAMESPACE}']
                metrics: ['temporal_cloud_v1_approximate_backlog_count']

    processors:
      batch: {}
      # Drop every datapoint whose temporal_task_queue label is not ours,
      # to keep CloudWatch cardinality bounded to a single series per
      # task_type. OTTL syntax: "true" means "drop this datapoint".
      filter/task_queue:
        metrics:
          datapoint:
            - 'attributes["temporal_task_queue"] != "$${env:TEMPORAL_TASK_QUEUE}"'

    exporters:
      awsemf:
        namespace: TemporalDemo/Worker
        region: $${env:AWS_REGION}
        log_group_name: /aws/emf/temporal-demo-worker
        dimension_rollup_option: NoDimensionRollup
        metric_declarations:
          - dimensions: [[temporal_task_queue, task_type]]
            metric_name_selectors:
              - temporal_cloud_v1_approximate_backlog_count

    service:
      pipelines:
        metrics:
          receivers: [prometheus]
          processors: [filter/task_queue, batch]
          exporters: [awsemf]
  EOT
}

resource "aws_ssm_parameter" "collector_config" {
  name        = "/otel-collector/${var.name_prefix}/config"
  description = "ADOT Collector YAML config for the Temporal Cloud OpenMetrics scrape pipeline."
  type        = "SecureString"
  value       = local.collector_config
}

# --- IAM: task execution role ---------------------------------------------

data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "execution" {
  name               = "${var.name_prefix}-otel-collector-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "execution_secrets" {
  statement {
    sid       = "ReadTemporalMetricsApiKey"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [var.temporal_metrics_api_key_secret_arn]
  }
  statement {
    sid       = "ReadCollectorConfig"
    actions   = ["ssm:GetParameters"]
    resources = [aws_ssm_parameter.collector_config.arn]
  }
  # The collector config parameter is SecureString and decrypted via the
  # AWS-managed KMS key for SSM; ECS' execution role needs kms:Decrypt
  # scoped via kms:ViaService so it can pull the value at task start.
  statement {
    sid       = "DecryptSsmSecureString"
    actions   = ["kms:Decrypt"]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "kms:ViaService"
      values   = ["ssm.${var.aws_region}.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "execution_secrets" {
  name   = "secrets-read"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution_secrets.json
}

# --- IAM: task role (collector process itself) ----------------------------

resource "aws_iam_role" "task" {
  name               = "${var.name_prefix}-otel-collector-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

data "aws_iam_policy_document" "task" {
  # awsemf publishes both EMF log events and CloudWatch metrics. The log
  # group is created by the exporter on first write, so logs:CreateLogGroup
  # is required in addition to the usual log-stream actions.
  statement {
    sid = "EmfLogs"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
      "logs:DescribeLogGroups",
    ]
    resources = ["*"]
  }

  # cloudwatch:PutMetricData has no resource-level permissions, so we
  # narrow via the cloudwatch:namespace condition key.
  statement {
    sid       = "PutMetrics"
    actions   = ["cloudwatch:PutMetricData"]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "cloudwatch:namespace"
      values   = ["TemporalDemo/Worker"]
    }
  }
}

resource "aws_iam_role_policy" "task" {
  name   = "otel-collector-task"
  role   = aws_iam_role.task.id
  policy = data.aws_iam_policy_document.task.json
}

# --- Task definition ------------------------------------------------------

resource "aws_ecs_task_definition" "collector" {
  family                   = "${var.name_prefix}-otel-collector"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    cpu_architecture        = "ARM64"
    operating_system_family = "LINUX"
  }

  container_definitions = jsonencode([
    {
      name      = "otel-collector"
      image     = var.collector_image
      essential = true

      environment = [
        { name = "AWS_REGION", value = var.aws_region },
        { name = "TEMPORAL_NAMESPACE", value = var.temporal_namespace },
        { name = "TEMPORAL_TASK_QUEUE", value = var.temporal_task_queue },
      ]

      # AOT_CONFIG_CONTENT must be the full YAML body, not a path. We pull
      # it from SSM via `secrets:` because the value is multi-line and
      # would not survive a flat `environment:` entry cleanly.
      secrets = [
        {
          name      = "AOT_CONFIG_CONTENT"
          valueFrom = aws_ssm_parameter.collector_config.arn
        },
        {
          name      = "TEMPORAL_METRICS_API_KEY"
          valueFrom = var.temporal_metrics_api_key_secret_arn
        },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.collector.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "otel-collector"
        }
      }
    },
  ])
}

# --- ECS service ----------------------------------------------------------

resource "aws_ecs_service" "collector" {
  name            = "${var.name_prefix}-otel-collector"
  cluster         = var.cluster_id
  task_definition = aws_ecs_task_definition.collector.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.collector.id]
    assign_public_ip = true
  }

  # Single-task service: replace, don't run two collectors in parallel.
  # Doubling up briefly would publish duplicate datapoints and skew alarms.
  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}
