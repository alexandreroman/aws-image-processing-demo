---
name: "Backend run-mode detection"
description: "`cmd/backend` picks HTTP vs Lambda bootstrap from `AWS_ENDPOINT_URL` presence; there is no `RUN_MODE` variable"
type: project
---

# Backend run-mode detection

`cmd/backend` is one binary with two bootstrap paths, and
the selection signal is the **presence of
`AWS_ENDPOINT_URL`**: set (pointing at Moto) boots the
HTTP server on `PORT` (default `:8000`); unset boots the
Lambda handler behind API Gateway.

**Why:** the HTTP handlers are identical in both modes —
only the adapter differs — so one variable can carry both
meanings with no risk of two knobs disagreeing.

**How to apply:**

- Run-mode selection stays derived from
  `AWS_ENDPOINT_URL`. A dedicated `RUN_MODE`-style
  variable has no place here; a third mode should extend
  the existing signal (e.g. `AWS_LAMBDA_FUNCTION_NAME`,
  which the Lambda runtime sets automatically) instead of
  adding a user-facing knob.
- In Tofu, leave `AWS_ENDPOINT_URL` off the Lambda env —
  its absence is what selects Lambda mode.
- `AWS_ENDPOINT_URL` ships in `.env.local.example`, so
  host-mode dev gets HTTP mode with no extra setup.
