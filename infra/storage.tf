# --- S3 images bucket ------------------------------------------------------
#
# Holds two logical sets of objects:
#   pipelines/  — derived artifacts (resized + watermarked), expire after 30
#   samples/    — preloaded demo pool, never expires

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

resource "aws_s3_bucket_lifecycle_configuration" "images" {
  bucket = aws_s3_bucket.images.id

  rule {
    id     = "expire-pipelines"
    status = "Enabled"

    filter {
      prefix = "pipelines/"
    }

    expiration {
      days = 30
    }
  }

  # samples/ rule is intentionally absent: preloaded pool, kept indefinitely.
}

# --- DynamoDB image manifests ---------------------------------------------
#
# Composite key (pipelineId, imageId) — matches the LocalStack init schema
# in compose.yaml so the same Go code path works against both backends.

resource "aws_dynamodb_table" "images" {
  name         = "${local.name_prefix}-images-${random_id.suffix.hex}"
  billing_mode = "PAY_PER_REQUEST"

  hash_key  = "pipelineId"
  range_key = "imageId"

  attribute {
    name = "pipelineId"
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
