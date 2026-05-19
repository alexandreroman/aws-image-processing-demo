# --- Lambda package -------------------------------------------------------
#
# The zip is produced by `make worker-autoscaler-lambda-zip` (cross-compiles
# cmd/worker-autoscaler to a linux/arm64 `bootstrap` binary and zips it).
# Same pattern as infra/worker-lambda: filebase64sha256 is guarded by
# fileexists() so `tofu plan` succeeds when the artifact is absent.

locals {
  zip_path   = "${path.module}/../../build/worker-autoscaler.zip"
  zip_exists = fileexists(local.zip_path)
}

resource "aws_cloudwatch_log_group" "autoscaler" {
  name              = "/aws/lambda/${var.name_prefix}-worker-autoscaler"
  retention_in_days = 14
}

# --- IAM: Lambda execution role ------------------------------------------

data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "autoscaler" {
  name               = "${var.name_prefix}-worker-autoscaler"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy_attachment" "autoscaler_logs" {
  role       = aws_iam_role.autoscaler.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "autoscaler_task" {
  # cloudwatch:PutMetricData has no resource-level permissions — narrowing is
  # done by IAM condition keys (cloudwatch:namespace) rather than by ARN.
  statement {
    sid       = "PutBacklogMetrics"
    actions   = ["cloudwatch:PutMetricData"]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "cloudwatch:namespace"
      values   = ["TemporalDemo/Worker"]
    }
  }

  # Secrets read at deploy time and injected as env vars (mirrors the
  # worker-lambda pattern — Lambda has no `secrets:` block like ECS).
  dynamic "statement" {
    for_each = var.temporal_tls_enabled ? [1] : []
    content {
      sid     = "ReadTemporalTLSSecrets"
      actions = ["secretsmanager:GetSecretValue"]
      resources = [
        var.temporal_tls_cert_secret_id,
        var.temporal_tls_key_secret_id,
      ]
    }
  }

  dynamic "statement" {
    for_each = var.temporal_tls_enabled ? [1] : []
    content {
      sid       = "DecryptSecretsManagerKMS"
      actions   = ["kms:Decrypt"]
      resources = ["*"]
      condition {
        test     = "StringEquals"
        variable = "kms:ViaService"
        values   = ["secretsmanager.${var.aws_region}.amazonaws.com"]
      }
    }
  }
}

resource "aws_iam_role_policy" "autoscaler_task" {
  name   = "autoscaler-task"
  role   = aws_iam_role.autoscaler.id
  policy = data.aws_iam_policy_document.autoscaler_task.json
}

# --- Secret values pulled in to inject as env vars ------------------------

data "aws_secretsmanager_secret_version" "temporal_tls_cert" {
  count     = var.temporal_tls_enabled ? 1 : 0
  secret_id = var.temporal_tls_cert_secret_id
}

data "aws_secretsmanager_secret_version" "temporal_tls_key" {
  count     = var.temporal_tls_enabled ? 1 : 0
  secret_id = var.temporal_tls_key_secret_id
}

# --- Lambda function ------------------------------------------------------

resource "aws_lambda_function" "autoscaler" {
  function_name = "${var.name_prefix}-worker-autoscaler"
  role          = aws_iam_role.autoscaler.arn

  filename         = local.zip_path
  source_code_hash = local.zip_exists ? filebase64sha256(local.zip_path) : null

  runtime       = "provided.al2023"
  handler       = "bootstrap"
  architectures = ["arm64"]
  package_type  = "Zip"

  memory_size = 128
  # One DescribeTaskQueue round-trip + one PutMetricData; 10 s leaves slack
  # for cold starts on provided.al2023 (~300 ms) without masking real hangs.
  timeout = 10

  environment {
    variables = merge(
      {
        HOME                = "/tmp"
        TEMPORAL_ADDRESS    = var.temporal_address
        TEMPORAL_NAMESPACE  = var.temporal_namespace
        TEMPORAL_TASK_QUEUE = var.temporal_task_queue
      },
      var.temporal_tls_enabled ? {
        TEMPORAL_TLS_CERT = data.aws_secretsmanager_secret_version.temporal_tls_cert[0].secret_string
        TEMPORAL_TLS_KEY  = data.aws_secretsmanager_secret_version.temporal_tls_key[0].secret_string
      } : {},
    )
  }

  depends_on = [aws_cloudwatch_log_group.autoscaler]
}

# --- EventBridge Scheduler ------------------------------------------------
#
# EventBridge Scheduler (not the legacy CloudWatch Events rule) is the only
# AWS-native scheduler that supports sub-minute rates — the design needs
# 30 s cadence, and `rate(30 seconds)` is rejected by the legacy
# `aws_cloudwatch_event_rule` resource.

data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name               = "${var.name_prefix}-worker-autoscaler-scheduler"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume.json
}

data "aws_iam_policy_document" "scheduler_invoke" {
  statement {
    sid       = "InvokeAutoscaler"
    actions   = ["lambda:InvokeFunction"]
    resources = [aws_lambda_function.autoscaler.arn, "${aws_lambda_function.autoscaler.arn}:*"]
  }
}

resource "aws_iam_role_policy" "scheduler_invoke" {
  name   = "invoke-autoscaler"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler_invoke.json
}

resource "aws_scheduler_schedule" "autoscaler" {
  name        = "${var.name_prefix}-worker-autoscaler"
  description = "Polls Temporal task queue backlog and publishes CloudWatch metrics for ECS worker autoscaling."

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression = var.schedule_expression

  target {
    arn      = aws_lambda_function.autoscaler.arn
    role_arn = aws_iam_role.scheduler.arn

    # Drop missed invocations rather than catching up — the autoscaler is a
    # gauge poller, stale data is worse than a gap.
    retry_policy {
      maximum_retry_attempts = 0
    }
  }
}
