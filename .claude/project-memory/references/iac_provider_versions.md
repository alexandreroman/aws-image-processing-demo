---
name: "IaC provider versions in infra/"
description: "AWS provider ~> 6.0 and Cloudflare provider ~> 5.0; v5 renamed cloudflare_record to cloudflare_dns_record (uses content not value, FQDN names)"
type: project
---

# IaC provider versions in infra/

`infra/providers.tf` pins `hashicorp/aws ~> 6.0` and
`cloudflare/cloudflare ~> 5.0`. Both providers have
current-stable major bumps that ship breaking changes
we already accommodate.

**Why:** As of 2026-05, AWS v6 and Cloudflare v5 are
the GA lines. Verify current stable versions via
context7 before changing constraints.

**How to apply:**
- Cloudflare v5 breaking changes that already shape
  `infra/dns.tf`:
  - resource renamed `cloudflare_record` →
    `cloudflare_dns_record`
  - attribute renamed `value` → `content`
  - `name` requires the full FQDN, not just the host
    portion
  - `proxied = false` stays mandatory (ACM DNS
    validation needs the unproxied record)
- AWS v6 used here without surprises — default tags
  in the provider block plus
  `aws_s3_bucket_lifecycle_configuration` with
  `filter { prefix = "..." }` blocks.
- See [[images_bucket_naming]] for the matching bucket
  / table naming rules.
