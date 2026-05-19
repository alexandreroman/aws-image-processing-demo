variable "name_prefix" {
  description = "Prefix applied to every resource name created by this module."
  type        = string
}

variable "aws_region" {
  description = "AWS region — surfaced to the worker via the AWS_REGION env var and used in awslogs config."
  type        = string
}

variable "temporal_address" {
  description = "Temporal gRPC endpoint passed to the worker as TEMPORAL_ADDRESS."
  type        = string
}

variable "temporal_namespace" {
  description = "Temporal namespace passed to the worker as TEMPORAL_NAMESPACE."
  type        = string
}

variable "temporal_task_queue" {
  description = "Temporal task queue served by the worker."
  type        = string
}

variable "temporal_tls_enabled" {
  description = "When true, mount TEMPORAL_TLS_CERT / TEMPORAL_TLS_KEY from Secrets Manager."
  type        = bool
}

variable "worker_image" {
  description = "Container image for the Fargate worker."
  type        = string
}

variable "worker_max_concurrent_activities" {
  description = "Maximum number of activities the ECS worker executes concurrently."
  type        = number
}

variable "images_bucket_arn" {
  description = "ARN of the S3 images bucket — used to scope task-role permissions."
  type        = string
}

variable "images_bucket_name" {
  description = "Name of the S3 images bucket — surfaced to the worker as IMAGES_BUCKET."
  type        = string
}

variable "images_table_arn" {
  description = "ARN of the DynamoDB images table — used to scope task-role permissions."
  type        = string
}

variable "images_table_name" {
  description = "Name of the DynamoDB images table — surfaced to the worker as IMAGES_TABLE."
  type        = string
}

variable "anthropic_secret_arn" {
  description = "ARN of the Secrets Manager secret holding the Anthropic API key."
  type        = string
}

variable "temporal_tls_cert_secret_arn" {
  description = "ARN of the Secrets Manager secret holding the Temporal mTLS cert. Empty when TLS is disabled."
  type        = string
}

variable "temporal_tls_key_secret_arn" {
  description = "ARN of the Secrets Manager secret holding the Temporal mTLS key. Empty when TLS is disabled."
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs the ECS service places worker tasks into."
  type        = list(string)
}

variable "vpc_id" {
  description = "VPC ID the worker security group belongs to."
  type        = string
}

variable "autoscaling_enabled" {
  description = <<-EOT
    When false, the autoscaling target, CloudWatch alarms, and scaling
    policies are not provisioned and the worker stays at desired_count = 1.
    When true, the full autoscaling stack is wired in.
  EOT
  type        = bool
  default     = false
}

variable "autoscaling_min_capacity" {
  description = "Minimum desired_count for the ECS worker service (warm capacity, never zero by design)."
  type        = number
  default     = 1
}

variable "autoscaling_max_capacity" {
  description = "Maximum desired_count for the ECS worker service."
  type        = number
  default     = 5
}

variable "scale_out_threshold" {
  description = "Total backlog (workflow + activity) above which the scale-out alarm fires (first step boundary)."
  type        = number
  default     = 10
}

variable "scale_out_step_2_lower" {
  description = "Total backlog (workflow + activity) at which the scale-out step jumps to +2 tasks."
  type        = number
  default     = 30
}

variable "scale_out_step_3_lower" {
  description = "Total backlog (workflow + activity) at which the scale-out step jumps to +3 tasks."
  type        = number
  default     = 60
}

variable "scale_in_threshold" {
  description = "Total backlog (workflow + activity) below which the scale-in alarm fires (sustained for 5 datapoints)."
  type        = number
  default     = 5
}
