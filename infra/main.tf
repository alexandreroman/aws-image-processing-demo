locals {
  name_prefix = var.project_name

  common_tags = {
    Project   = var.project_name
    ManagedBy = "OpenTofu"
  }

  # Task queues are pinned per runtime; they are not user knobs because the
  # backend looks them up by canonical name (`ecs` / `lambda`).
  worker_task_queue_ecs    = "image-processing-ecs"
  worker_task_queue_lambda = "image-processing-lambda"

  # CloudWatch retention shared by every log group in the stack (backend
  # Lambda, both worker runtimes, ADOT collector). Demo-sized: long enough to
  # debug a burst after the fact, short enough to stay cheap.
  log_retention_days = 14
}

# Random suffix for the resource names that must be unique per account and
# cannot be reused immediately after a destroy: the DynamoDB table and the
# Secrets Manager secrets. The S3 buckets do not need it — they use
# `bucket_prefix` (storage.tf, frontend.tf) and let AWS generate the suffix.
resource "random_id" "suffix" {
  byte_length = 4
}
