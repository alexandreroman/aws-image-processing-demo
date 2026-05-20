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
}

# Random suffix appended to globally-unique resource names (S3 buckets,
# DynamoDB table) so reruns in fresh accounts don't collide with leftovers.
resource "random_id" "suffix" {
  byte_length = 4
}
