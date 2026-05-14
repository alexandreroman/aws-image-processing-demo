locals {
  # Temporal Cloud TLS material is optional; when both PEM blobs are non-empty
  # we surface them as additional Secrets Manager entries and inject them as
  # container `secrets:` (rather than env vars) so the values never appear in
  # task definitions or CloudWatch logs.
  temporal_tls_enabled = var.temporal_tls_cert_pem != "" && var.temporal_tls_key_pem != ""
}

resource "aws_secretsmanager_secret" "anthropic_api_key" {
  name        = "${local.name_prefix}-anthropic-api-key-${random_id.suffix.hex}"
  description = "Anthropic API key used by the worker GenerateDescription activity"
  # Demo choice: 0 = delete immediately on `tofu destroy`, no 7-30 day
  # recovery window. Do NOT carry this over to production deployments.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "anthropic_api_key" {
  secret_id     = aws_secretsmanager_secret.anthropic_api_key.id
  secret_string = var.anthropic_api_key
}

resource "aws_secretsmanager_secret" "temporal_tls_cert" {
  count = local.temporal_tls_enabled ? 1 : 0

  name        = "${local.name_prefix}-temporal-tls-cert-${random_id.suffix.hex}"
  description = "Temporal Cloud mTLS client certificate (PEM)"
  # Demo choice: 0 = delete immediately on `tofu destroy`, no 7-30 day
  # recovery window. Do NOT carry this over to production deployments.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "temporal_tls_cert" {
  count = local.temporal_tls_enabled ? 1 : 0

  secret_id     = aws_secretsmanager_secret.temporal_tls_cert[0].id
  secret_string = var.temporal_tls_cert_pem
}

resource "aws_secretsmanager_secret" "temporal_tls_key" {
  count = local.temporal_tls_enabled ? 1 : 0

  name        = "${local.name_prefix}-temporal-tls-key-${random_id.suffix.hex}"
  description = "Temporal Cloud mTLS client private key (PEM)"
  # Demo choice: 0 = delete immediately on `tofu destroy`, no 7-30 day
  # recovery window. Do NOT carry this over to production deployments.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "temporal_tls_key" {
  count = local.temporal_tls_enabled ? 1 : 0

  secret_id     = aws_secretsmanager_secret.temporal_tls_key[0].id
  secret_string = var.temporal_tls_key_pem
}

# Lambda has no `secrets:` block like ECS, so we read the PEM values back
# here to inject them as backend Lambda env vars (see `backend.tf`).
data "aws_secretsmanager_secret_version" "temporal_tls_cert" {
  count = local.temporal_tls_enabled ? 1 : 0

  secret_id  = aws_secretsmanager_secret.temporal_tls_cert[0].id
  depends_on = [aws_secretsmanager_secret_version.temporal_tls_cert]
}

data "aws_secretsmanager_secret_version" "temporal_tls_key" {
  count = local.temporal_tls_enabled ? 1 : 0

  secret_id  = aws_secretsmanager_secret.temporal_tls_key[0].id
  depends_on = [aws_secretsmanager_secret_version.temporal_tls_key]
}
