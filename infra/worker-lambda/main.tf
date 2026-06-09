# --- Lambda package -------------------------------------------------------
#
# The zip is produced by `make worker-lambda-zip` (cross-compiles cmd/worker
# to a linux/arm64 `bootstrap` binary and zips it). Referenced directly via
# filename + filebase64sha256 — no archive_file data source.

locals {
  worker_zip = "${path.module}/../../build/worker.zip"
  # filebase64sha256() reads the file at expression-eval time, which happens
  # even when this module is instantiated with count = 0 at the root (the
  # default ECS path). Guard the read with fileexists() so `tofu plan`
  # succeeds without the zip on disk. `filename` itself is just a string —
  # it doesn't touch disk until apply, and the AWS provider's "exactly one
  # of filename/image_uri/s3_bucket" rule rejects a null value at validate
  # time, so we leave it unconditionally set.
  worker_zip_exists   = fileexists(local.worker_zip)
  create_invoker_role = length(var.temporal_cloud_aws_account_ids) > 0 && var.temporal_cloud_external_id != ""

  # Temporal Worker Deployment name. The optional suffix lets us roll onto a
  # fresh deployment name when the existing one is wedged on the Temporal Cloud
  # side. Surfaced to the worker via WORKER_DEPLOYMENT_NAME and to the
  # registration script via the `deployment_name` output — keeping both in sync.
  deployment_name = "${var.name_prefix}-worker-lambda${var.deployment_name_suffix != "" ? "-${var.deployment_name_suffix}" : ""}"
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/aws/lambda/${var.name_prefix}-worker"
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

resource "aws_iam_role" "worker_execution" {
  name               = "${var.name_prefix}-worker-lambda-execution"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

# CloudWatch Logs perms (CreateLogGroup/Stream + PutLogEvents) ride along
# via the managed policy — no need to inline them.
resource "aws_iam_role_policy_attachment" "worker_logs" {
  role       = aws_iam_role.worker_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "worker_task" {
  # Reads: visitor uploads, the preloaded sample pool, and read-back of
  # derived artifacts (GenerateDescription + ApplyWatermark both fetch
  # the resized variant before processing it).
  statement {
    sid     = "ImagesBucketRead"
    actions = ["s3:GetObject"]
    resources = [
      "${var.images_bucket_arn}/uploads/*",
      "${var.images_bucket_arn}/samples/*",
      "${var.images_bucket_arn}/pipelines/*",
    ]
  }

  # Writes: derived artifacts only — resized and watermarked variants.
  statement {
    sid     = "ImagesBucketWritePipelines"
    actions = ["s3:PutObject"]
    resources = [
      "${var.images_bucket_arn}/pipelines/*",
    ]
  }

  statement {
    sid = "ImagesTableRW"
    actions = [
      "dynamodb:PutItem",
      "dynamodb:Query",
    ]
    resources = [var.images_table_arn]
  }

  # Secrets the Lambda fetches at deploy time (env injection) and could
  # re-fetch at runtime. Anthropic is always present; TLS secrets only when
  # mTLS is enabled.
  statement {
    sid     = "ReadDeploymentSecrets"
    actions = ["secretsmanager:GetSecretValue"]
    resources = concat(
      [var.anthropic_secret_arn],
      var.temporal_tls_enabled ? [
        var.temporal_tls_cert_secret_id,
        var.temporal_tls_key_secret_id,
      ] : [],
    )
  }

}

resource "aws_iam_role_policy" "worker_task" {
  name   = "worker-task"
  role   = aws_iam_role.worker_execution.id
  policy = data.aws_iam_policy_document.worker_task.json
}

# --- Secret values pulled in to inject as env vars ------------------------

data "aws_secretsmanager_secret_version" "anthropic_api_key" {
  secret_id = var.anthropic_secret_arn
}

data "aws_secretsmanager_secret_version" "temporal_tls_cert" {
  count     = var.temporal_tls_enabled ? 1 : 0
  secret_id = var.temporal_tls_cert_secret_id
}

data "aws_secretsmanager_secret_version" "temporal_tls_key" {
  count     = var.temporal_tls_enabled ? 1 : 0
  secret_id = var.temporal_tls_key_secret_id
}

# --- Lambda function ------------------------------------------------------

resource "aws_lambda_function" "worker" {
  function_name = "${var.name_prefix}-worker"
  role          = aws_iam_role.worker_execution.arn

  reserved_concurrent_executions = var.worker_lambda_max_instances

  filename         = local.worker_zip
  source_code_hash = local.worker_zip_exists ? filebase64sha256(local.worker_zip) : null

  # Go custom runtime — handler must be named `bootstrap` (al2023).
  runtime       = "provided.al2023"
  handler       = "bootstrap"
  architectures = ["arm64"]
  package_type  = "Zip"

  # Measured peak ~89 MB leaves ~11x headroom at 1024 MB; not lower because
  # Lambda CPU scales with memory and resize activities are CPU-bound.
  memory_size = 1024
  timeout     = 600
  publish     = true

  environment {
    # Lambda has no `secrets:` block like ECS, so all values surface as env
    # vars. PEM blobs go through aws_secretsmanager_secret_version (mirrors
    # the backend Lambda pattern).
    variables = merge(
      {
        HOME                = "/tmp"
        TEMPORAL_ADDRESS    = var.temporal_address
        TEMPORAL_NAMESPACE  = var.temporal_namespace
        TEMPORAL_TASK_QUEUE = var.temporal_task_queue
        IMAGES_BUCKET       = var.images_bucket_name
        IMAGES_TABLE        = var.images_table_name
        # Read by Go via WORKER_DEPLOYMENT_NAME (cmd/worker/main.go:31). The
        # registration script consumes the same value via the Tofu output
        # `worker_lambda_deployment_name`, so changes here flow there
        # automatically — no string drift across sites.
        WORKER_DEPLOYMENT_NAME           = local.deployment_name
        WORKER_MAX_CONCURRENT_ACTIVITIES = tostring(var.worker_max_concurrent_activities)
        ANTHROPIC_API_KEY                = data.aws_secretsmanager_secret_version.anthropic_api_key.secret_string
      },
      var.temporal_tls_enabled ? {
        TEMPORAL_TLS_CERT = data.aws_secretsmanager_secret_version.temporal_tls_cert[0].secret_string
        TEMPORAL_TLS_KEY  = data.aws_secretsmanager_secret_version.temporal_tls_key[0].secret_string
      } : {},
    )
  }

  depends_on = [aws_cloudwatch_log_group.worker]
}

# --- Invoker role (Temporal Cloud assumes this to invoke the Lambda) ------
#
# Only created when the account-ID list is non-empty and the external ID is
# set. Skipping it keeps the module usable for local-only dev or manual
# invocation.

data "aws_iam_policy_document" "invoker_assume" {
  count = local.create_invoker_role ? 1 : 0

  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "AWS"
      identifiers = [for id in var.temporal_cloud_aws_account_ids : "arn:aws:iam::${id}:role/wci-lambda-invoke"]
    }

    condition {
      test     = "StringEquals"
      variable = "sts:ExternalId"
      values   = [var.temporal_cloud_external_id]
    }
  }
}

resource "aws_iam_role" "worker_invoker" {
  count = local.create_invoker_role ? 1 : 0

  name               = "${var.name_prefix}-worker-invoker"
  assume_role_policy = data.aws_iam_policy_document.invoker_assume[0].json
}

data "aws_iam_policy_document" "invoker_invoke" {
  count = local.create_invoker_role ? 1 : 0

  statement {
    sid     = "InvokeWorker"
    actions = ["lambda:InvokeFunction", "lambda:GetFunction"]
    # Cover the function itself and any published version / alias.
    resources = [
      aws_lambda_function.worker.arn,
      "${aws_lambda_function.worker.arn}:*",
    ]
  }
}

resource "aws_iam_role_policy" "worker_invoker" {
  count = local.create_invoker_role ? 1 : 0

  name   = "invoke-worker"
  role   = aws_iam_role.worker_invoker[0].id
  policy = data.aws_iam_policy_document.invoker_invoke[0].json
}
