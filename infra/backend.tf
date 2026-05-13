# --- Lambda package -------------------------------------------------------
#
# The deployment artifact is built **outside** of OpenTofu by
# `scripts/build-lambda.sh`, which cross-compiles `cmd/backend` for Linux
# and writes the binary to `dist/backend/bootstrap`. This file just zips it.
# Run `make build-lambda` (or invoke the script directly) before
# `tofu apply`.

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
  # Presign uploads (PUT) and read manifests if needed.
  statement {
    sid     = "ImagesBucketRW"
    actions = ["s3:PutObject", "s3:GetObject"]
    resources = [
      "${aws_s3_bucket.images.arn}/*",
    ]
  }

  statement {
    sid       = "ImagesBucketList"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.images.arn]
  }

  statement {
    sid = "ImagesTableRead"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:Query",
      "dynamodb:Scan",
    ]
    resources = [aws_dynamodb_table.images.arn]
  }

  statement {
    sid       = "ReadAnthropicSecret"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_secretsmanager_secret.anthropic_api_key.arn]
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
    variables = {
      TEMPORAL_ADDRESS    = var.temporal_address
      TEMPORAL_NAMESPACE  = var.temporal_namespace
      TEMPORAL_TASK_QUEUE = var.temporal_task_queue
      IMAGES_BUCKET       = aws_s3_bucket.images.bucket
      IMAGES_TABLE        = aws_dynamodb_table.images.name
    }
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

resource "aws_apigatewayv2_route" "backend_api" {
  api_id    = aws_apigatewayv2_api.backend.id
  route_key = "ANY /api/{proxy+}"
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
