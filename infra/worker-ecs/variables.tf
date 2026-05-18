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
