variable "aws_region" {
  description = "AWS region for all resources except the ACM cert (us-east-1)."
  type        = string
  default     = "eu-west-3"
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

variable "temporal_task_queue" {
  description = "Temporal task queue served by the worker."
  type        = string
  default     = "image-processing"
}

variable "worker_image" {
  description = "Container image for the Fargate worker."
  type        = string
  default     = "ghcr.io/alexandreroman/aws-image-processing-demo-worker:latest"
}

variable "anthropic_api_key" {
  description = "Anthropic API key — stored in Secrets Manager and injected into the worker."
  type        = string
  sensitive   = true
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
