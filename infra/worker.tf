# Both worker runtimes are always provisioned — each polls its own Temporal
# task queue so they can run side by side. The backend's runtime registry
# routes a workflow start request to the matching queue based on the
# `runtime` field in the POST body.

# Resolve the worker image's tag to its content-addressable digest. Pinning the
# ECS task definition to `repo@sha256:...` instead of `repo:latest` makes Tofu
# produce a new task definition revision whenever GHCR publishes a new manifest
# under the same tag, which is what triggers ECS to roll the service. Without
# this, the tag string never changes and the service never picks up new images.
data "docker_registry_image" "worker" {
  name = var.worker_image
}

locals {
  # Strip any `:tag` suffix from `var.worker_image` (the registry/repo portion
  # has no `:` for ghcr.io, but the regex tolerates `host:port/repo` too) and
  # append the resolved digest. Final shape: `<repo>@sha256:<hex>`.
  worker_image_pinned = "${regex("^[^:@]+(?:/[^:@]+)*", var.worker_image)}@${data.docker_registry_image.worker.sha256_digest}"
}

# Both runtimes run the very same worker binary, so their task roles must grant
# exactly the same access. The document lives here, next to the bucket and the
# table it grants on, so the two runtimes cannot drift apart.
data "aws_iam_policy_document" "worker_task" {
  # Reads: the preloaded sample pool — the only source of originals, there is
  # no upload path — plus read-back of derived artifacts, since
  # GenerateDescription and ApplyWatermark both fetch the resized variant
  # before processing it.
  statement {
    sid     = "ImagesBucketRead"
    actions = ["s3:GetObject"]
    resources = [
      "${aws_s3_bucket.images.arn}/samples/*",
      "${aws_s3_bucket.images.arn}/pipelines/*",
    ]
  }

  # Writes: derived artifacts only — resized and watermarked variants.
  # Originals under `samples/` stay read-only for the worker. Deletes are
  # handled by S3 lifecycle rules; the worker never deletes.
  statement {
    sid     = "ImagesBucketWritePipelines"
    actions = ["s3:PutObject"]
    resources = [
      "${aws_s3_bucket.images.arn}/pipelines/*",
    ]
  }

  # The worker only ever writes manifest rows (internal/activities/store.go).
  # Reading them back is the backend's job — see the scoped dynamodb:Query
  # grant in backend.tf.
  statement {
    sid       = "ImagesTableWrite"
    actions   = ["dynamodb:PutItem"]
    resources = [aws_dynamodb_table.images.arn]
  }
}

module "worker_ecs" {
  source = "./worker-ecs"

  name_prefix          = local.name_prefix
  aws_region           = var.aws_region
  temporal_address     = var.temporal_address
  temporal_namespace   = var.temporal_namespace
  temporal_task_queue  = local.worker_task_queue_ecs
  temporal_tls_enabled = local.temporal_tls_enabled
  worker_image         = local.worker_image_pinned
  log_retention_days   = local.log_retention_days

  worker_max_concurrent_activities = var.worker_max_concurrent_activities

  images_bucket_name = aws_s3_bucket.images.bucket
  images_table_name  = aws_dynamodb_table.images.name
  task_policy_json   = data.aws_iam_policy_document.worker_task.json

  anthropic_secret_arn         = aws_secretsmanager_secret.demo["anthropic-api-key"].arn
  temporal_tls_cert_secret_arn = local.temporal_tls_enabled ? aws_secretsmanager_secret.demo["temporal-tls-cert"].arn : ""
  temporal_tls_key_secret_arn  = local.temporal_tls_enabled ? aws_secretsmanager_secret.demo["temporal-tls-key"].arn : ""

  # The flag is deliberately unmarked (see `secrets.tf`): the module feeds it to
  # a CloudWatch alarm `for_each`, and OpenTofu rejects sensitive instance keys.
  autoscaling_enabled      = local.temporal_metrics_enabled
  autoscaling_max_capacity = var.worker_ecs_max_instances

  subnet_ids = aws_subnet.public[*].id
  vpc_id     = aws_vpc.main.id
}

module "worker_otel_collector" {
  count  = local.temporal_metrics_enabled ? 1 : 0
  source = "./worker-otel-collector"

  name_prefix         = local.name_prefix
  aws_region          = var.aws_region
  temporal_namespace  = var.temporal_namespace
  temporal_task_queue = local.worker_task_queue_ecs
  log_retention_days  = local.log_retention_days

  cluster_id = module.worker_ecs.cluster_id
  subnet_ids = aws_subnet.public[*].id
  vpc_id     = aws_vpc.main.id

  temporal_metrics_api_key_secret_arn = aws_secretsmanager_secret.demo["temporal-metrics-api-key"].arn
}

module "worker_lambda" {
  source = "./worker-lambda"

  # The module reads the secret values back (Lambda has no `secrets:` block)
  # to inject them as env vars. Those reads key off the secret ARN alone, so
  # on a first apply they can run ahead of the writes and fail with "can't
  # find the specified secret value". Ordering the module after the writes
  # closes that race — the same guard secrets.tf applies to its own reads.
  depends_on = [aws_secretsmanager_secret_version.demo]

  name_prefix          = local.name_prefix
  temporal_address     = var.temporal_address
  temporal_namespace   = var.temporal_namespace
  temporal_task_queue  = local.worker_task_queue_lambda
  temporal_tls_enabled = local.temporal_tls_enabled
  log_retention_days   = local.log_retention_days

  worker_max_concurrent_activities = var.worker_max_concurrent_activities
  worker_lambda_max_instances      = var.worker_lambda_max_instances

  images_bucket_name = aws_s3_bucket.images.bucket
  images_table_name  = aws_dynamodb_table.images.name
  task_policy_json   = data.aws_iam_policy_document.worker_task.json

  anthropic_secret_arn        = aws_secretsmanager_secret.demo["anthropic-api-key"].arn
  temporal_tls_cert_secret_id = local.temporal_tls_enabled ? aws_secretsmanager_secret.demo["temporal-tls-cert"].id : ""
  temporal_tls_key_secret_id  = local.temporal_tls_enabled ? aws_secretsmanager_secret.demo["temporal-tls-key"].id : ""

  temporal_cloud_aws_account_ids = var.temporal_cloud_aws_account_ids
  temporal_cloud_external_id     = var.temporal_cloud_external_id

  deployment_name_suffix = var.worker_lambda_deployment_suffix
}
