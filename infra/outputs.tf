output "frontend_bucket" {
  description = "Name of the S3 bucket that hosts the Nuxt SSG output."
  value       = aws_s3_bucket.frontend.bucket
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID — used by `make frontend-deploy` for invalidation."
  value       = aws_cloudfront_distribution.demo.id
}

output "cloudfront_domain_name" {
  description = "Default CloudFront hostname (d…cloudfront.net)."
  value       = aws_cloudfront_distribution.demo.domain_name
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

output "images_table" {
  description = "Name of the DynamoDB table storing image manifests."
  value       = aws_dynamodb_table.images.name
}

output "api_gateway_url" {
  description = "Direct API Gateway URL — useful for debugging without going through CloudFront."
  value       = aws_apigatewayv2_stage.backend.invoke_url
}

output "worker_image_repo" {
  description = "Container image reference used by the Fargate task."
  value       = var.worker_image
}
