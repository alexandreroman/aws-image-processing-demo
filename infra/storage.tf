# --- S3 images bucket ------------------------------------------------------
#
# Holds three logical sets of objects (see BRIEF.md):
#   uploads/   — visitor uploads, expire after 7 days
#   sessions/  — derived artifacts (resized + watermarked), expire after 30
#   samples/   — preloaded demo pool, never expires

resource "aws_s3_bucket" "images" {
  bucket_prefix = "${local.name_prefix}-images-"
  force_destroy = true

  tags = {
    Name = "${local.name_prefix}-images"
  }
}

resource "aws_s3_bucket_public_access_block" "images" {
  bucket = aws_s3_bucket.images.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "images" {
  bucket = aws_s3_bucket.images.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "images" {
  bucket = aws_s3_bucket.images.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# SECURITY: `allowed_origins = ["*"]` lets ANY web origin issue presigned
# PUTs against this bucket. That is acceptable for this demo, but it is
# NOT safe for production — combined with our presign role it broadens
# the blast radius of any leaked presigned URL beyond the demo frontend.
#
# Why it is wide open today: locking CORS to `https://${subdomain}.${domain}`
# requires knowing the public hostname at bucket-creation time, but the
# bucket is referenced by the worker task definition which is provisioned
# before the CloudFront distribution exists, so there is a chicken-and-egg
# cycle in a single `tofu apply`.
#
# Post-deploy hardening (run after the first successful apply):
#   1. Set `allowed_origins` to the CloudFront / custom-domain URL only
#      (e.g. ["https://demo.example.com"]).
#   2. Drop `GET` and `HEAD` from `allowed_methods` — only `PUT` is needed
#      for the upload flow; downloads go through CloudFront, not S3 CORS.
#   3. Re-apply; nothing else depends on the wide-open rule.
resource "aws_s3_bucket_cors_configuration" "images" {
  bucket = aws_s3_bucket.images.id

  cors_rule {
    allowed_methods = ["PUT", "GET", "HEAD"]
    allowed_origins = ["*"]
    allowed_headers = ["*"]
    expose_headers  = ["ETag"]
    max_age_seconds = 3000
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "images" {
  bucket = aws_s3_bucket.images.id

  rule {
    id     = "expire-uploads"
    status = "Enabled"

    filter {
      prefix = "uploads/"
    }

    expiration {
      days = 7
    }
  }

  rule {
    id     = "expire-sessions"
    status = "Enabled"

    filter {
      prefix = "sessions/"
    }

    expiration {
      days = 30
    }
  }

  # samples/ rule is intentionally absent: preloaded pool, kept indefinitely.
}

# --- DynamoDB image manifests ---------------------------------------------
#
# Composite key (sessionId, imageId) — matches the LocalStack init schema
# in compose.yaml so the same Go code path works against both backends.

resource "aws_dynamodb_table" "images" {
  name         = "${local.name_prefix}-images-${random_id.suffix.hex}"
  billing_mode = "PAY_PER_REQUEST"

  hash_key  = "sessionId"
  range_key = "imageId"

  attribute {
    name = "sessionId"
    type = "S"
  }

  attribute {
    name = "imageId"
    type = "S"
  }

  point_in_time_recovery {
    enabled = false
  }

  tags = {
    Name = "${local.name_prefix}-images"
  }
}
