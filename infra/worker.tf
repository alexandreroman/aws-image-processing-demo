# Both worker runtimes are always provisioned — each polls its own Temporal
# task queue so they can run side by side. The backend's runtime registry
# routes a workflow start request to the matching queue based on the
# `runtime` field in the POST body.

module "worker_ecs" {
  source = "./worker-ecs"

  name_prefix          = local.name_prefix
  aws_region           = var.aws_region
  temporal_address     = var.temporal_address
  temporal_namespace   = var.temporal_namespace
  temporal_task_queue  = var.worker_task_queue_ecs
  temporal_tls_enabled = local.temporal_tls_enabled
  worker_image         = var.worker_image

  images_bucket_arn  = aws_s3_bucket.images.arn
  images_bucket_name = aws_s3_bucket.images.bucket
  images_table_arn   = aws_dynamodb_table.images.arn
  images_table_name  = aws_dynamodb_table.images.name

  anthropic_secret_arn         = aws_secretsmanager_secret.anthropic_api_key.arn
  temporal_tls_cert_secret_arn = local.temporal_tls_enabled ? aws_secretsmanager_secret.temporal_tls_cert[0].arn : ""
  temporal_tls_key_secret_arn  = local.temporal_tls_enabled ? aws_secretsmanager_secret.temporal_tls_key[0].arn : ""

  subnet_ids = aws_subnet.public[*].id
  vpc_id     = aws_vpc.main.id
}

module "worker_lambda" {
  source = "./worker-lambda"

  name_prefix          = local.name_prefix
  aws_region           = var.aws_region
  temporal_address     = var.temporal_address
  temporal_namespace   = var.temporal_namespace
  temporal_task_queue  = var.worker_task_queue_lambda
  temporal_tls_enabled = local.temporal_tls_enabled

  images_bucket_arn  = aws_s3_bucket.images.arn
  images_bucket_name = aws_s3_bucket.images.bucket
  images_table_arn   = aws_dynamodb_table.images.arn
  images_table_name  = aws_dynamodb_table.images.name

  anthropic_secret_arn        = aws_secretsmanager_secret.anthropic_api_key.arn
  temporal_tls_cert_secret_id = local.temporal_tls_enabled ? aws_secretsmanager_secret.temporal_tls_cert[0].id : ""
  temporal_tls_key_secret_id  = local.temporal_tls_enabled ? aws_secretsmanager_secret.temporal_tls_key[0].id : ""

  temporal_cloud_aws_account_ids = var.temporal_cloud_aws_account_ids
  temporal_cloud_external_id     = var.temporal_cloud_external_id
}
