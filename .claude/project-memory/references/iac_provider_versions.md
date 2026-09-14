---
name: "Cloudflare provider v5 breaking changes in infra/"
description: "The v5 DNS resource rename and attribute changes that shape infra/dns.tf"
type: project
---

# Cloudflare provider v5 breaking changes in infra/

`infra/dns.tf` targets the Cloudflare provider v5 API.
Most Cloudflare DNS examples online still use the v4
spellings, which fail here:

- the resource is `cloudflare_dns_record`, not
  `cloudflare_record`
- the record body attribute is `content`, not `value`
- `name` takes the full FQDN, not just the host portion
- `proxied = false` is mandatory — ACM DNS validation
  needs the record unproxied

**How to apply:** verify current stable provider versions
via context7 before touching the constraints in
`infra/providers.tf`. See
[AWS resource naming](images_bucket_naming.md) for the
matching bucket / table naming rules.
