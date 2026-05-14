---
name: "Moto: keep the default listening port"
description: "Do not change the port motoserver listens on inside its container — keep it at the default (5000)."
type: feedback
---

# Moto: keep the default listening port

Do not change the port `motoserver/moto` listens on
inside its container. Keep it at moto's default
(5000).

**Why:** the user explicitly asked to stay on moto's
default port. Tying the container-internal URL to
the default removes a configuration knob that could
drift between `compose.yaml`, healthchecks, sibling
service env vars (`AWS_ENDPOINT_URL=http://moto:5000`),
and the `init` service's inline `aws --endpoint-url`
calls.

**How to apply:**

- In `compose.yaml`, the moto service stays on
  `-p 5000` (its default). Do not change that flag
  or remove the explicit value silently.
- Sibling services keep referring to moto via
  `http://moto:5000` internally.
- The host-side mapping (`4566:5000`) is a separate
  concern documented in [[local_aws_emulator]] — it
  exists so `AWS_ENDPOINT_URL=http://localhost:4566`
  in `.env` stays compatible with the LocalStack
  convention, and is unrelated to this rule.
- If a future change tempts you to move moto to a
  different internal port, surface the trade-off
  and ask before acting.
