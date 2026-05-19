variable "name_prefix" {
  description = "Prefix applied to every resource name created by this module."
  type        = string
}

variable "aws_region" {
  description = "AWS region — used to scope kms:ViaService in the task policy."
  type        = string
}

variable "temporal_address" {
  description = "Temporal gRPC endpoint passed to the autoscaler as TEMPORAL_ADDRESS."
  type        = string
}

variable "temporal_namespace" {
  description = "Temporal namespace passed to the autoscaler as TEMPORAL_NAMESPACE."
  type        = string
}

variable "temporal_task_queue" {
  description = "Temporal task queue the autoscaler polls for backlog stats (the ECS worker queue)."
  type        = string
}

variable "temporal_tls_enabled" {
  description = "When true, surface TEMPORAL_TLS_CERT / TEMPORAL_TLS_KEY env vars read from Secrets Manager."
  type        = bool
}

variable "temporal_tls_cert_secret_id" {
  description = <<-EOT
    Secret ID (ARN or name) of the Secrets Manager secret holding the Temporal
    mTLS cert. Empty when TLS is disabled.
  EOT
  type        = string
}

variable "temporal_tls_key_secret_id" {
  description = <<-EOT
    Secret ID (ARN or name) of the Secrets Manager secret holding the Temporal
    mTLS key. Empty when TLS is disabled.
  EOT
  type        = string
}

variable "schedule_expression" {
  description = "EventBridge Scheduler expression driving the polling cadence. 30 s is the design default."
  type        = string
  default     = "rate(30 seconds)"
}
