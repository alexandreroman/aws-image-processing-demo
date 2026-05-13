# --- Frontend S3 bucket ---------------------------------------------------

resource "aws_s3_bucket" "frontend" {
  bucket_prefix = "${local.name_prefix}-frontend-"
  force_destroy = true

  tags = {
    Name = "${local.name_prefix}-frontend"
  }
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

# --- CloudFront Origin Access Control --------------------------------------

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.name_prefix}-frontend-oac"
  description                       = "OAC for the Nuxt SSG bucket"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# --- CloudFront distribution ----------------------------------------------
#
# Two origins, two behaviours (BRIEF.md §"CloudFront origin routing"):
#   /api/*  → API Gateway, no caching, forward viewer headers
#   /*      → S3 + OAC, with SPA fallback (404 → 200 /index.html)

locals {
  s3_origin_id  = "s3-frontend"
  api_origin_id = "api-gateway"

  # API Gateway URLs look like https://<id>.execute-api.<region>.amazonaws.com.
  # CloudFront origins want a bare host name.
  api_origin_host = replace(
    replace(aws_apigatewayv2_api.backend.api_endpoint, "https://", ""),
    "/", "",
  )

  cloudfront_aliases = (
    var.enable_custom_domain && var.domain_name != ""
    ? ["${var.subdomain}.${var.domain_name}"]
    : []
  )
}

resource "aws_cloudfront_distribution" "demo" {
  enabled             = true
  is_ipv6_enabled     = true
  default_root_object = "index.html"
  comment             = local.name_prefix
  price_class         = "PriceClass_100"

  aliases = local.cloudfront_aliases

  origin {
    origin_id                = local.s3_origin_id
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  origin {
    origin_id   = local.api_origin_id
    domain_name = local.api_origin_host

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  # Default: serve the static site from S3.
  default_cache_behavior {
    target_origin_id       = local.s3_origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    # AWS-managed "CachingOptimized" policy.
    cache_policy_id = "658327ea-f89d-4fab-a63d-7e88639e58f6"
  }

  # /api/* → Lambda, never cached, forward all viewer signals.
  ordered_cache_behavior {
    path_pattern           = "/api/*"
    target_origin_id       = local.api_origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    # Managed policies:
    #   CachingDisabled              4135ea2d-6df8-44a3-9df3-4b5a84be39ad
    #   AllViewerExceptHostHeader    b689b0a8-53d0-40ab-baf2-68738e2966ac
    cache_policy_id          = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
    origin_request_policy_id = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
  }

  # SPA fallback: Nuxt /sessions/[id] is client-rendered, so S3 returns 403
  # for any unknown key. CloudFront rewrites to /index.html so the SPA can
  # take over.
  custom_error_response {
    error_code            = 403
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 10
  }

  custom_error_response {
    error_code            = 404
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 10
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = !(var.enable_custom_domain && var.domain_name != "")
    acm_certificate_arn = (
      var.enable_custom_domain && var.domain_name != ""
      ? aws_acm_certificate_validation.cf[0].certificate_arn
      : null
    )
    ssl_support_method = (
      var.enable_custom_domain && var.domain_name != ""
      ? "sni-only"
      : null
    )
    minimum_protocol_version = (
      var.enable_custom_domain && var.domain_name != ""
      ? "TLSv1.2_2021"
      : "TLSv1"
    )
  }
}

# --- S3 bucket policy: grant CloudFront OAC read access -------------------

data "aws_iam_policy_document" "frontend_oac" {
  statement {
    sid     = "AllowCloudFrontOACRead"
    actions = ["s3:GetObject"]
    resources = [
      "${aws_s3_bucket.frontend.arn}/*",
    ]

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.demo.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  policy = data.aws_iam_policy_document.frontend_oac.json
}
