# --- ECS cluster + log group ----------------------------------------------

resource "aws_ecs_cluster" "worker" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "disabled"
  }
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/ecs/${local.name_prefix}-worker"
  retention_in_days = 14
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
  name               = "${local.name_prefix}-worker-execution"
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
      [aws_secretsmanager_secret.anthropic_api_key.arn],
      local.temporal_tls_enabled ? [
        aws_secretsmanager_secret.temporal_tls_cert[0].arn,
        aws_secretsmanager_secret.temporal_tls_key[0].arn,
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
  name               = "${local.name_prefix}-worker-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

data "aws_iam_policy_document" "worker_task" {
  # Reads: visitor uploads, the preloaded sample pool, and read-back of
  # derived artifacts (GenerateDescription + ApplyWatermark both fetch
  # the resized variant before processing it).
  statement {
    sid     = "ImagesBucketRead"
    actions = ["s3:GetObject"]
    resources = [
      "${aws_s3_bucket.images.arn}/uploads/*",
      "${aws_s3_bucket.images.arn}/samples/*",
      "${aws_s3_bucket.images.arn}/pipelines/*",
    ]
  }

  # Writes: derived artifacts only — resized and watermarked variants.
  # Originals under `uploads/` and `samples/` are read-only for the worker.
  # Deletes are handled by S3 lifecycle rules; the worker never deletes.
  statement {
    sid     = "ImagesBucketWritePipelines"
    actions = ["s3:PutObject"]
    resources = [
      "${aws_s3_bucket.images.arn}/pipelines/*",
    ]
  }

  statement {
    sid = "ImagesTableRW"
    actions = [
      "dynamodb:PutItem",
      "dynamodb:Query",
    ]
    resources = [aws_dynamodb_table.images.arn]
  }
}

resource "aws_iam_role_policy" "worker_task" {
  name   = "worker-task"
  role   = aws_iam_role.worker_task.id
  policy = data.aws_iam_policy_document.worker_task.json
}

# --- Task definition ------------------------------------------------------

locals {
  worker_env = [
    { name = "TEMPORAL_ADDRESS", value = var.temporal_address },
    { name = "TEMPORAL_NAMESPACE", value = var.temporal_namespace },
    { name = "TEMPORAL_TASK_QUEUE", value = var.temporal_task_queue },
    { name = "AWS_REGION", value = var.aws_region },
    { name = "IMAGES_BUCKET", value = aws_s3_bucket.images.bucket },
    { name = "IMAGES_TABLE", value = aws_dynamodb_table.images.name },
    { name = "WORKER_MAX_CONCURRENT_ACTIVITIES", value = "16" },
  ]

  worker_secrets = concat(
    [
      {
        name      = "ANTHROPIC_API_KEY"
        valueFrom = aws_secretsmanager_secret.anthropic_api_key.arn
      },
    ],
    local.temporal_tls_enabled ? [
      {
        name      = "TEMPORAL_TLS_CERT"
        valueFrom = aws_secretsmanager_secret.temporal_tls_cert[0].arn
      },
      {
        name      = "TEMPORAL_TLS_KEY"
        valueFrom = aws_secretsmanager_secret.temporal_tls_key[0].arn
      },
    ] : [],
  )
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${local.name_prefix}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "1024"
  memory                   = "2048"
  execution_role_arn       = aws_iam_role.worker_execution.arn
  task_role_arn            = aws_iam_role.worker_task.arn

  runtime_platform {
    cpu_architecture        = "X86_64"
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
        command     = ["CMD", "wget", "-qO-", "http://localhost:8000/healthz"]
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
  name            = "${local.name_prefix}-worker"
  cluster         = aws_ecs_cluster.worker.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.public[*].id
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

  # Worker tasks expose a /healthz liveness probe (see the container
  # healthCheck above); ECS will replace tasks that fail it. Graceful
  # shutdown still goes through SIGTERM and the stopTimeout above.
  enable_execute_command = false
}
