variable "aws_region" {
  description = "AWS region for all resources except the ACM cert (us-east-1)."
  type        = string
  default     = "eu-west-1"
}

variable "project_name" {
  description = "Name prefix used for every resource and as the Project tag value."
  type        = string
  default     = "aws-image-processing-demo"
}

variable "domain_name" {
  description = <<-EOT
    Cloudflare zone (e.g. "alexandre.dev"). When empty, the custom domain is
    skipped and the CloudFront default *.cloudfront.net hostname is used.
  EOT
  type        = string
  default     = ""
}

variable "subdomain" {
  description = "Subdomain part of the demo URL (e.g. \"demo\" for demo.example.com)."
  type        = string
  default     = "demo"
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for the demo domain. Set via TF_VAR_cloudflare_zone_id or .env."
  type        = string
  sensitive   = true
  default     = ""
}

variable "temporal_address" {
  description = "Temporal Cloud gRPC endpoint, e.g. \"xxx.tmprl.cloud:7233\"."
  type        = string
}

variable "temporal_namespace" {
  description = "Temporal namespace to connect to."
  type        = string
}

variable "worker_image" {
  description = "Container image for the Fargate worker."
  type        = string
  default     = "ghcr.io/alexandreroman/aws-image-processing-demo-worker:latest"
}

variable "worker_max_concurrent_activities" {
  description = "Maximum number of activities a worker executes concurrently (applies to both ECS and Lambda runtimes)."
  type        = number
  default     = 4
}

variable "worker_lambda_max_instances" {
  description = "Caps the maximum number of Lambda worker instances (one per concurrent execution). Use -1 (the AWS API sentinel) to disable the reservation and fall back to unreserved account concurrency."
  type        = number
  default     = 10
}

variable "worker_ecs_max_instances" {
  description = "Caps the maximum number of ECS worker instances (the service's desired_count) when autoscaling is enabled (TEMPORAL_METRICS_API_KEY set). Ignored otherwise — the service stays at desired_count = 1."
  type        = number
  default     = 5
}

variable "worker_lambda_deployment_suffix" {
  description = "Optional suffix for the Lambda Temporal Worker Deployment name. See module worker-lambda var.deployment_name_suffix."
  type        = string
  default     = ""
}

variable "temporal_cloud_aws_account_ids" {
  description = <<-EOT
    AWS account IDs of the Temporal Cloud Lambda invoker cells. The default
    is the full list of 5 cells published in Temporal's CloudFormation
    template; the trust policy admits the precise `wci-lambda-invoke` role
    in each. You normally do not need to override this.
  EOT
  type        = list(string)
  default = [
    "902542641901",
    "160190466495",
    "819232936619",
    "829909441867",
    "354116250941",
  ]
}

variable "temporal_cloud_external_id" {
  description = "External ID Temporal Cloud presents when assuming the Lambda invoker role."
  type        = string
  sensitive   = true
  default     = ""
}

variable "anthropic_api_key" {
  description = "Anthropic API key — stored in Secrets Manager and injected into the worker."
  type        = string
  sensitive   = true
}

variable "temporal_metrics_api_key" {
  description = <<-EOT
    Temporal Cloud service-account API key with the Metrics Read-Only
    role. Stored in Secrets Manager and injected into the ADOT collector
    ECS task so it can scrape metrics.temporal.io. Leave empty to disable
    ECS worker autoscaling — the collector, alarms, and scaling policies
    are then not provisioned and the worker stays at desired_count = 1.
  EOT
  type        = string
  sensitive   = true
  default     = ""
}

variable "temporal_tls_cert_pem" {
  description = <<-EOT
    Temporal Cloud client certificate (PEM). Optional — leave empty for local
    Temporal dev server. When set together with temporal_tls_key_pem, the
    worker mounts both as Secrets Manager-backed env vars.
  EOT
  type        = string
  sensitive   = true
  default     = ""
}

variable "temporal_tls_key_pem" {
  description = "Temporal Cloud client private key (PEM). See temporal_tls_cert_pem."
  type        = string
  sensitive   = true
  default     = ""
}

variable "enable_custom_domain" {
  description = <<-EOT
    Gates ACM certificate + Cloudflare DNS records + CloudFront alias config.
    Set false to ship a demo on the default *.cloudfront.net hostname.
  EOT
  type        = bool
  default     = true
}
