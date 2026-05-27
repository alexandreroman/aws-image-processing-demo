variable "name_prefix" {
  description = "Prefix applied to every resource name created by this module."
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
  description = "When true, surface TEMPORAL_TLS_CERT / TEMPORAL_TLS_KEY env vars read from Secrets Manager."
  type        = bool
}

variable "worker_max_concurrent_activities" {
  description = "Maximum number of activities the Lambda worker executes concurrently."
  type        = number
}

variable "worker_lambda_max_instances" {
  description = "Caps the maximum number of Lambda worker instances (one per concurrent execution). Use -1 (the AWS API sentinel) to disable the reservation and fall back to unreserved account concurrency."
  type        = number
  default     = 10
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
  description = <<-EOT
    ARN of the Secrets Manager secret holding the Anthropic API key. Used both
    to scope the IAM policy and to read the value via aws_secretsmanager_secret_version
    (the data source's secret_id accepts an ARN).
  EOT
  type        = string
}

variable "temporal_tls_cert_secret_id" {
  description = <<-EOT
    Secret ID (ARN or name) of the Secrets Manager secret holding the Temporal mTLS cert.
    Empty when TLS is disabled. Used both for IAM scoping and for reading the value
    via aws_secretsmanager_secret_version.
  EOT
  type        = string
}

variable "temporal_tls_key_secret_id" {
  description = <<-EOT
    Secret ID (ARN or name) of the Secrets Manager secret holding the Temporal mTLS key.
    Empty when TLS is disabled. Used both for IAM scoping and for reading the value
    via aws_secretsmanager_secret_version.
  EOT
  type        = string
}

variable "temporal_cloud_aws_account_ids" {
  description = <<-EOT
    List of AWS account IDs whose `wci-lambda-invoke` role is trusted to invoke
    this Lambda. Defaulted by the root module; empty list disables invoker-role
    creation.
  EOT
  type        = list(string)
}

variable "temporal_cloud_external_id" {
  description = "External ID Temporal Cloud must present when assuming the invoker role."
  type        = string
  sensitive   = true
  default     = ""
}
