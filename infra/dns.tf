# Custom domain (ACM cert + Cloudflare DNS).
#
# Gated on var.enable_custom_domain so the project can also ship on the
# default *.cloudfront.net hostname. ACM cert lives in us-east-1 — non-
# negotiable AWS constraint for CloudFront (BRIEF.md decision #17).
#
# Cloudflare proxy MUST stay OFF (DNS only) — see BRIEF.md decision #16.
# Two CDNs in cascade break HTTPS validation and add no value.

locals {
  use_custom_domain = var.enable_custom_domain && var.domain_name != ""
  cert_fqdn         = local.use_custom_domain ? "${var.subdomain}.${var.domain_name}" : ""
}

resource "aws_acm_certificate" "cf" {
  count = local.use_custom_domain ? 1 : 0

  provider = aws.us_east_1

  domain_name       = local.cert_fqdn
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

# Deduplicate validation records — ACM occasionally emits the same CNAME
# twice when SAN names overlap. for_each on a map keyed by record name
# collapses duplicates safely.
locals {
  cert_validation_records = local.use_custom_domain ? {
    for dvo in aws_acm_certificate.cf[0].domain_validation_options : dvo.domain_name => {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  } : {}
}

resource "cloudflare_dns_record" "cert_validation" {
  for_each = local.cert_validation_records

  zone_id = var.cloudflare_zone_id

  # Cloudflare v5 expects the FQDN without a trailing dot.
  name    = trimsuffix(each.value.name, ".")
  type    = each.value.type
  content = trimsuffix(each.value.value, ".")
  ttl     = 60
  proxied = false
}

resource "aws_acm_certificate_validation" "cf" {
  count = local.use_custom_domain ? 1 : 0

  provider = aws.us_east_1

  certificate_arn         = aws_acm_certificate.cf[0].arn
  validation_record_fqdns = [for r in cloudflare_dns_record.cert_validation : r.name]
}

resource "cloudflare_dns_record" "alias" {
  count = local.use_custom_domain ? 1 : 0

  zone_id = var.cloudflare_zone_id
  name    = local.cert_fqdn
  type    = "CNAME"
  content = aws_cloudfront_distribution.demo.domain_name
  ttl     = 60
  proxied = false
}
