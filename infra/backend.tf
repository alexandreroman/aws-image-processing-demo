# --- Lambda package -------------------------------------------------------
#
# The deployment artifact is built **outside** of OpenTofu by
# `scripts/build-lambda.sh`, which cross-compiles `cmd/backend` for Linux
# and writes the binary to `dist/backend/bootstrap`. This file just zips it.
# The script is invoked by `scripts/deploy.sh` (i.e. `make deploy`) before
# `tofu apply`; run it directly if applying Tofu by hand.

data "archive_file" "backend" {
  type        = "zip"
  source_file = "${path.module}/../dist/backend/bootstrap"
  output_path = "${path.module}/../dist/backend/bootstrap.zip"
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

resource "aws_iam_role" "backend" {
  name               = "${local.name_prefix}-backend"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy_attachment" "backend_logs" {
  role       = aws_iam_role.backend.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "backend" {
  statement {
    sid       = "ImagesTableRead"
    actions   = ["dynamodb:Query"]
    resources = [aws_dynamodb_table.images.arn]
  }
}

resource "aws_iam_role_policy" "backend" {
  name   = "backend"
  role   = aws_iam_role.backend.id
  policy = data.aws_iam_policy_document.backend.json
}

# --- Lambda function ------------------------------------------------------

resource "aws_cloudwatch_log_group" "backend" {
  name              = "/aws/lambda/${local.name_prefix}-backend"
  retention_in_days = 14
}

resource "aws_lambda_function" "backend" {
  function_name = "${local.name_prefix}-backend"
  role          = aws_iam_role.backend.arn

  filename         = data.archive_file.backend.output_path
  source_code_hash = data.archive_file.backend.output_base64sha256

  # Go custom runtime — handler must be named `bootstrap` (al2023).
  runtime       = "provided.al2023"
  handler       = "bootstrap"
  architectures = ["x86_64"]
  package_type  = "Zip"

  memory_size = 256
  timeout     = 29 # API Gateway HTTP API caps at 30s

  environment {
    # Temporal Cloud mTLS material (when configured) is injected from
    # Secrets Manager via the data sources below. Lambda has no native
    # `secrets:` equivalent to the ECS task definition, so the PEM blobs
    # are surfaced as env vars at deploy time — matching the env-var
    # contract the worker uses and `temporalclient.Dial` reads.
    variables = merge(
      {
        TEMPORAL_ADDRESS         = var.temporal_address
        TEMPORAL_NAMESPACE       = var.temporal_namespace
        WORKER_TASK_QUEUE_ECS    = local.worker_task_queue_ecs
        WORKER_TASK_QUEUE_LAMBDA = local.worker_task_queue_lambda
        IMAGES_BUCKET            = aws_s3_bucket.images.bucket
        IMAGES_TABLE             = aws_dynamodb_table.images.name
        # Pin CORS to the only origin that legitimately calls this API.
        # Without this, the handler falls back to "*" and the API
        # advertises itself to arbitrary origins.
        ALLOWED_ORIGIN = local.use_custom_domain ? "https://${var.subdomain}.${var.domain_name}" : "https://${aws_cloudfront_distribution.demo.domain_name}"
      },
      local.temporal_tls_enabled ? {
        TEMPORAL_TLS_CERT = data.aws_secretsmanager_secret_version.temporal_tls_cert[0].secret_string
        TEMPORAL_TLS_KEY  = data.aws_secretsmanager_secret_version.temporal_tls_key[0].secret_string
      } : {},
    )
  }

  depends_on = [
    aws_cloudwatch_log_group.backend,
    aws_iam_role_policy_attachment.backend_logs,
  ]
}

# --- API Gateway HTTP API ------------------------------------------------

resource "aws_apigatewayv2_api" "backend" {
  name          = "${local.name_prefix}-backend"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_integration" "backend" {
  api_id                 = aws_apigatewayv2_api.backend.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.backend.invoke_arn
  integration_method     = "POST"
  payload_format_version = "2.0"
}

# `{proxy+}` requires at least one path segment, so /api alone would 404.
# A dedicated route covers the exact /api path; the proxy route handles
# everything below it.
resource "aws_apigatewayv2_route" "backend_api" {
  for_each = toset(["ANY /api", "ANY /api/{proxy+}"])

  api_id    = aws_apigatewayv2_api.backend.id
  route_key = each.value
  target    = "integrations/${aws_apigatewayv2_integration.backend.id}"
}

resource "aws_apigatewayv2_stage" "backend" {
  api_id      = aws_apigatewayv2_api.backend.id
  name        = "$default"
  auto_deploy = true
}

resource "aws_lambda_permission" "apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.backend.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.backend.execution_arn}/*/*"
}
