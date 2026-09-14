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

resource "aws_cloudfront_origin_access_control" "images" {
  name                              = "${local.name_prefix}-images-oac"
  description                       = "OAC for the images bucket"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# --- CloudFront Function: strip /images/ prefix at viewer-request -----------
#
# The bundle uses relative /images/{key} URLs so the gallery loads same-origin.
# This function rewrites those to the raw S3 key (pipelines/..., samples/...)
# before CloudFront hits the images origin.

resource "aws_cloudfront_function" "images_rewrite" {
  name    = "${local.name_prefix}-images-rewrite"
  runtime = "cloudfront-js-2.0"
  publish = true
  comment = "Strip /images/ prefix so the S3 origin sees the raw key"
  code    = <<-EOT
    function handler(event) {
      var req = event.request;
      req.uri = req.uri.replace(/^\/images\//, '/');
      return req;
    }
  EOT
}

# --- CloudFront Function: SPA fallback for the frontend origin --------------
#
# Nuxt deep links such as /pipelines/{id} are client-rendered, so the S3
# frontend bucket holds no object for them and answers 403. The fallback
# CANNOT be expressed as a `custom_error_response`: CloudFront applies those
# distribution-wide, across every cache behavior. A 404 from the backend on
# /api/pipelines/{unknown} would reach the browser as 200 + an HTML page,
# and a missing /images/... key (403 from S3 + OAC) would land an HTML page
# inside an <img>. Rewriting at viewer-request keeps the fallback attached to
# the default cache behavior — that is, to /* only.

resource "aws_cloudfront_function" "spa_fallback" {
  name    = "${local.name_prefix}-spa-fallback"
  runtime = "cloudfront-js-2.0"
  publish = true
  comment = "Serve Nuxt's SPA fallback document for client-rendered routes"
  code    = <<-EOT
    function handler(event) {
      var req = event.request;

      // "/" is resolved by the distribution's default root object.
      if (req.uri === '/') {
        return req;
      }

      // Anything with a file extension is a real object in the bucket
      // (/_nuxt/*.js, /favicon.ico, /200.html, ...): serve it as-is.
      if (req.uri.split('/').pop().indexOf('.') !== -1) {
        return req;
      }

      // 200.html is Nuxt's lightweight SPA fallback, with no prerendered
      // route baked in; the router renders the real route in the browser.
      // Serving index.html here would hydrate the home page instead.
      req.uri = '/200.html';
      return req;
    }
  EOT
}

# --- CloudFront distribution ----------------------------------------------
#
# Three origins, three behaviours:
#   /api/*    → API Gateway, origin-driven caching, forward viewer headers
#   /images/* → S3 images bucket + OAC, /images/ prefix stripped
#   /*        → S3 frontend bucket + OAC, with SPA fallback to /200.html

locals {
  s3_origin_id     = "s3-frontend"
  api_origin_id    = "api-gateway"
  images_origin_id = "s3-images"

  # AWS-managed policies, referenced by their well-known IDs.
  managed_cache_policy_caching_optimized   = "658327ea-f89d-4fab-a63d-7e88639e58f6"
  managed_origin_request_policy_all_viewer = "b689b0a8-53d0-40ab-baf2-68738e2966ac"

  # API Gateway URLs look like https://<id>.execute-api.<region>.amazonaws.com.
  # CloudFront origins want a bare host name.
  api_origin_host = trimsuffix(
    trimprefix(aws_apigatewayv2_api.backend.api_endpoint, "https://"),
    "/",
  )

  cloudfront_aliases = local.use_custom_domain ? [local.cert_fqdn] : []

  # Public entry point of the demo. Also fed to the backend as ALLOWED_ORIGIN
  # and printed by the deploy scripts via the `demo_url` output.
  demo_url = (
    local.use_custom_domain
    ? "https://${local.cert_fqdn}"
    : "https://${aws_cloudfront_distribution.demo.domain_name}"
  )
}

# --- CloudFront cache policy: honor origin Cache-Control on /api/* ----------
#
# The backend sets max-age / s-maxage / stale-while-revalidate / stale-if-error
# on selected endpoints (e.g. /api/stats). Managed-CachingDisabled would force
# CloudFront to ignore those directives, so we ship a minimal custom policy
# that lets the origin pick the TTL. Endpoints without Cache-Control still go
# uncached because default_ttl is 0.
#
# Cache key contains nothing viewer-specific: the API is anonymous, GETs are
# idempotent, and /api/stats has no query strings that vary the response.
# Headers like Authorization are still forwarded to Lambda via the existing
# AllViewerExceptHostHeader origin request policy.

resource "aws_cloudfront_cache_policy" "api" {
  name        = "${local.name_prefix}-api-origin-cache"
  comment     = "Honor origin Cache-Control on /api/*; minimal cache key"
  min_ttl     = 0
  default_ttl = 0
  max_ttl     = 31536000

  parameters_in_cache_key_and_forwarded_to_origin {
    enable_accept_encoding_brotli = true
    enable_accept_encoding_gzip   = true

    cookies_config {
      cookie_behavior = "none"
    }

    headers_config {
      header_behavior = "none"
    }

    query_strings_config {
      query_string_behavior = "none"
    }
  }
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

  origin {
    origin_id                = local.images_origin_id
    domain_name              = aws_s3_bucket.images.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.images.id
  }

  # Default: serve the static site from S3, with the SPA fallback scoped to
  # this behavior only. See frontend/Caddyfile for the matching local
  # compose-stack rule.
  default_cache_behavior {
    target_origin_id       = local.s3_origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    # AWS-managed "CachingOptimized" policy.
    cache_policy_id = local.managed_cache_policy_caching_optimized

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_fallback.arn
    }
  }

  # /api/* → Lambda, never cached, forward all viewer signals.
  ordered_cache_behavior {
    path_pattern           = "/api/*"
    target_origin_id       = local.api_origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    # Origin Cache-Control drives the TTL via the custom policy above.
    # AllViewerExceptHostHeader keeps forwarding the full request envelope
    # to Lambda regardless of cache-key shape.
    cache_policy_id          = aws_cloudfront_cache_policy.api.id
    origin_request_policy_id = local.managed_origin_request_policy_all_viewer
  }

  # /images/* → S3 images bucket. CloudFront Function strips the
  # /images/ prefix so the origin sees the raw key (pipelines/..., samples/...).
  ordered_cache_behavior {
    path_pattern           = "/images/*"
    target_origin_id       = local.images_origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    # CachingOptimized — derived images are immutable per key.
    cache_policy_id = local.managed_cache_policy_caching_optimized

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.images_rewrite.arn
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = !local.use_custom_domain
    acm_certificate_arn            = local.use_custom_domain ? aws_acm_certificate_validation.cf[0].certificate_arn : null
    ssl_support_method             = local.use_custom_domain ? "sni-only" : null
    # Enforce TLS 1.2 (2021 policy) regardless of cert source. The default
    # CloudFront certificate supports TLSv1.2_2021, so there is no reason
    # to fall back to the legacy `TLSv1` SSL policy.
    minimum_protocol_version = "TLSv1.2_2021"
  }
}

# --- S3 bucket policies: grant CloudFront OAC read access -----------------
#
# Same document for both buckets, differing only in the resource ARN.

data "aws_iam_policy_document" "oac_read" {
  for_each = {
    frontend = aws_s3_bucket.frontend.arn
    images   = aws_s3_bucket.images.arn
  }

  statement {
    sid       = "AllowCloudFrontOACRead"
    actions   = ["s3:GetObject"]
    resources = ["${each.value}/*"]

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
  policy = data.aws_iam_policy_document.oac_read["frontend"].json
}

resource "aws_s3_bucket_policy" "images" {
  bucket = aws_s3_bucket.images.id
  policy = data.aws_iam_policy_document.oac_read["images"].json
}
