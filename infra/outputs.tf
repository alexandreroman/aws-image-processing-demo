output "frontend_bucket" {
  description = "Name of the S3 bucket that hosts the Nuxt SSG output."
  value       = aws_s3_bucket.frontend.bucket
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID — used by `make frontend-deploy` for invalidation."
  value       = aws_cloudfront_distribution.demo.id
}

output "demo_url" {
  description = "Public URL of the demo: custom domain if enabled, CloudFront hostname otherwise."
  value = var.enable_custom_domain && var.domain_name != "" ? (
    "https://${var.subdomain}.${var.domain_name}"
    ) : (
    "https://${aws_cloudfront_distribution.demo.domain_name}"
  )
}

output "images_bucket" {
  description = "Name of the S3 bucket holding originals + derived images."
  value       = aws_s3_bucket.images.bucket
}

output "worker_lambda_function_arn" {
  description = "ARN of the worker Lambda function. Consumed by scripts/deploy.sh to register the Worker Deployment Version with Temporal Cloud."
  value       = module.worker_lambda.function_arn
}

output "worker_lambda_invoker_role_arn" {
  description = "ARN of the role Temporal Cloud assumes to invoke the worker Lambda. Null when the invoker role is not created. Consumed by scripts/deploy.sh."
  value       = module.worker_lambda.invoker_role_arn
}

output "worker_lambda_deployment_name" {
  description = "Temporal Worker Deployment name registered for the Lambda runtime. Consumed by scripts/register-worker-deployment.sh."
  value       = module.worker_lambda.deployment_name
}
