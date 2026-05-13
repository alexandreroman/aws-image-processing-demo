locals {
  name_prefix = var.project_name

  common_tags = {
    Project   = var.project_name
    ManagedBy = "OpenTofu"
  }
}

# Random suffix appended to globally-unique resource names (S3 buckets,
# DynamoDB table) so reruns in fresh accounts don't collide with leftovers.
resource "random_id" "suffix" {
  byte_length = 4
}
