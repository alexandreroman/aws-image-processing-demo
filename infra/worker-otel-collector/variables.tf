variable "name_prefix" {
  description = "Prefix applied to every resource name created by this module."
  type        = string
}

variable "aws_region" {
  description = "AWS region — exposed to the collector container and used in awslogs/awsemf config."
  type        = string
}

variable "temporal_namespace" {
  description = "Temporal Cloud namespace to scrape (passed as a query parameter to the OpenMetrics endpoint)."
  type        = string
}

variable "temporal_task_queue" {
  description = "Temporal task queue to keep in the metric stream (the ECS worker queue)."
  type        = string
}

variable "cluster_id" {
  description = "ARN/id of the existing ECS cluster the collector task runs in."
  type        = string
}

variable "cluster_name" {
  description = "Name of the existing ECS cluster (only used for tagging / log clarity)."
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs the ECS service places the collector task into."
  type        = list(string)
}

variable "vpc_id" {
  description = "VPC ID the collector security group belongs to."
  type        = string
}

variable "temporal_metrics_api_key_secret_arn" {
  description = <<-EOT
    ARN of the Secrets Manager secret holding the Temporal Cloud Metrics
    Read-Only API key. The collector injects it as TEMPORAL_METRICS_API_KEY
    and uses it as a Bearer token to scrape metrics.temporal.io.
  EOT
  type        = string
}

variable "collector_image" {
  description = <<-EOT
    Container image for the ADOT Collector. Pinned to a specific public ECR
    tag so changes are explicit. Update by bumping this default to a newer
    release of github.com/aws-observability/aws-otel-collector.
  EOT
  type        = string
  default     = "public.ecr.aws/aws-observability/aws-otel-collector:v0.47.0"
}
