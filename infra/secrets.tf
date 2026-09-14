locals {
  # Temporal Cloud TLS material is optional; both PEM blobs must be set for
  # mTLS to be wired in. `nonsensitive` only strips the mark, never changes the
  # value: whether the material was supplied is not itself a secret, and the
  # flag gates the `for_each` keys below, which OpenTofu refuses to derive from
  # a sensitive value.
  temporal_tls_enabled = nonsensitive(var.temporal_tls_cert_pem != "" && var.temporal_tls_key_pem != "")

  # Same reasoning: the presence of a metrics API key selects resources, and is
  # not secret material itself.
  temporal_metrics_enabled = nonsensitive(var.temporal_metrics_api_key != "")

  # Every Secrets Manager entry the stack owns, keyed by the name fragment that
  # goes between the project prefix and the random suffix. Optional entries drop
  # out of the map when their inputs are empty.
  #
  # The entries are split across two maps so that no sensitive value ever
  # reaches `for_each`, whose keys become resource instance addresses:
  # `secret_descriptions` is free of secret material and drives `for_each`,
  # while `secret_values` stays sensitive and is only ever indexed by
  # `each.key` — a sensitive value is perfectly fine as a resource argument.
  #
  # Trade-off worth stating plainly: only the ECS worker consumes these as
  # container `secrets:`, where the value never lands in the task definition.
  # Lambda has no such mechanism, so `backend.tf` and the worker-lambda module
  # read the values back and inject them as plaintext function env vars —
  # visible to anyone holding lambda:GetFunctionConfiguration. Secrets Manager
  # still buys central rotation and keeps the values out of the Tofu-rendered
  # task definitions and out of CloudWatch logs.
  secret_descriptions = merge(
    {
      "anthropic-api-key" = "Anthropic API key used by the worker GenerateDescription activity"
    },
    local.temporal_metrics_enabled ? {
      "temporal-metrics-api-key" = "Temporal Cloud Metrics Read-Only API key used by the ADOT collector"
    } : {},
    local.temporal_tls_enabled ? {
      "temporal-tls-cert" = "Temporal Cloud mTLS client certificate (PEM)"
      "temporal-tls-key"  = "Temporal Cloud mTLS client private key (PEM)"
    } : {},
  )

  # Listed in full on purpose: only the keys that `secret_descriptions` selects
  # are ever read, so unset optional inputs simply stay untouched here.
  secret_values = {
    "anthropic-api-key"        = var.anthropic_api_key
    "temporal-metrics-api-key" = var.temporal_metrics_api_key
    "temporal-tls-cert"        = var.temporal_tls_cert_pem
    "temporal-tls-key"         = var.temporal_tls_key_pem
  }
}

resource "aws_secretsmanager_secret" "demo" {
  for_each = local.secret_descriptions

  name        = "${local.name_prefix}-${each.key}-${random_id.suffix.hex}"
  description = each.value
  # Demo choice: 0 = delete immediately on `tofu destroy`, no 7-30 day
  # recovery window. Do NOT carry this over to production deployments.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "demo" {
  for_each = local.secret_descriptions

  secret_id     = aws_secretsmanager_secret.demo[each.key].id
  secret_string = local.secret_values[each.key]
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
