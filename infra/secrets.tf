locals {
  # Temporal Cloud TLS material is optional; both PEM blobs must be set for
  # mTLS to be wired in.
  temporal_tls_enabled = var.temporal_tls_cert_pem != "" && var.temporal_tls_key_pem != ""

  # Every Secrets Manager entry the stack owns, keyed by the name fragment
  # that goes between the project prefix and the random suffix. Optional
  # entries drop out of the map when their inputs are empty.
  #
  # Trade-off worth stating plainly: only the ECS worker consumes these as
  # container `secrets:`, where the value never lands in the task definition.
  # Lambda has no such mechanism, so `backend.tf` and the worker-lambda module
  # read the values back and inject them as plaintext function env vars —
  # visible to anyone holding lambda:GetFunctionConfiguration. Secrets Manager
  # still buys central rotation and keeps the values out of the Tofu-rendered
  # task definitions and out of CloudWatch logs.
  secrets = merge(
    {
      "anthropic-api-key" = {
        description = "Anthropic API key used by the worker GenerateDescription activity"
        value       = var.anthropic_api_key
      }
    },
    var.temporal_metrics_api_key != "" ? {
      "temporal-metrics-api-key" = {
        description = "Temporal Cloud Metrics Read-Only API key used by the ADOT collector"
        value       = var.temporal_metrics_api_key
      }
    } : {},
    local.temporal_tls_enabled ? {
      "temporal-tls-cert" = {
        description = "Temporal Cloud mTLS client certificate (PEM)"
        value       = var.temporal_tls_cert_pem
      }
      "temporal-tls-key" = {
        description = "Temporal Cloud mTLS client private key (PEM)"
        value       = var.temporal_tls_key_pem
      }
    } : {},
  )
}

resource "aws_secretsmanager_secret" "demo" {
  for_each = local.secrets

  name        = "${local.name_prefix}-${each.key}-${random_id.suffix.hex}"
  description = each.value.description
  # Demo choice: 0 = delete immediately on `tofu destroy`, no 7-30 day
  # recovery window. Do NOT carry this over to production deployments.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "demo" {
  for_each = local.secrets

  secret_id     = aws_secretsmanager_secret.demo[each.key].id
  secret_string = each.value.value
}

# Lambda has no `secrets:` block like ECS, so we read the PEM values back
# here to inject them as backend Lambda env vars (see `backend.tf`).
# `depends_on` is what keeps the read ordered after the write: without it the
# data source only depends on the secret itself and a first apply can read an
# empty secret ("can't find the specified secret value").
data "aws_secretsmanager_secret_version" "temporal_tls_cert" {
  count = local.temporal_tls_enabled ? 1 : 0

  secret_id  = aws_secretsmanager_secret.demo["temporal-tls-cert"].id
  depends_on = [aws_secretsmanager_secret_version.demo]
}

data "aws_secretsmanager_secret_version" "temporal_tls_key" {
  count = local.temporal_tls_enabled ? 1 : 0

  secret_id  = aws_secretsmanager_secret.demo["temporal-tls-key"].id
  depends_on = [aws_secretsmanager_secret_version.demo]
}

# Transitional: the four secrets used to be individually declared resources.
# Without these the next apply would try to create a secret whose name is
# still taken by the one it is about to destroy. Safe to delete once every
# deployment has applied this change.
moved {
  from = aws_secretsmanager_secret.anthropic_api_key
  to   = aws_secretsmanager_secret.demo["anthropic-api-key"]
}

moved {
  from = aws_secretsmanager_secret_version.anthropic_api_key
  to   = aws_secretsmanager_secret_version.demo["anthropic-api-key"]
}

moved {
  from = aws_secretsmanager_secret.temporal_metrics_api_key[0]
  to   = aws_secretsmanager_secret.demo["temporal-metrics-api-key"]
}

moved {
  from = aws_secretsmanager_secret_version.temporal_metrics_api_key[0]
  to   = aws_secretsmanager_secret_version.demo["temporal-metrics-api-key"]
}

moved {
  from = aws_secretsmanager_secret.temporal_tls_cert[0]
  to   = aws_secretsmanager_secret.demo["temporal-tls-cert"]
}

moved {
  from = aws_secretsmanager_secret_version.temporal_tls_cert[0]
  to   = aws_secretsmanager_secret_version.demo["temporal-tls-cert"]
}

moved {
  from = aws_secretsmanager_secret.temporal_tls_key[0]
  to   = aws_secretsmanager_secret.demo["temporal-tls-key"]
}

moved {
  from = aws_secretsmanager_secret_version.temporal_tls_key[0]
  to   = aws_secretsmanager_secret_version.demo["temporal-tls-key"]
}
