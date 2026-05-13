---
name: "Backend run-mode detection"
description: "`cmd/backend` picks HTTP vs Lambda bootstrap from `AWS_ENDPOINT_URL` presence; no separate `RUN_MODE` variable"
type: project
---

# Backend run-mode detection

`cmd/backend` is one binary with two bootstrap
paths:

- **HTTP server** on `:8000` for local dev
  (LocalStack).
- **Lambda handler** for prod (behind API
  Gateway).

The selection signal is the **presence of
`AWS_ENDPOINT_URL`**, not a dedicated
variable. If `AWS_ENDPOINT_URL` is set
(LocalStack endpoint), boot as HTTP; otherwise
boot as Lambda. A `RUN_MODE` env var existed
briefly in early scaffolding but was removed
because it duplicated the signal carried by
`AWS_ENDPOINT_URL` and could contradict it.

```go
if os.Getenv("AWS_ENDPOINT_URL") != "" {
    http.ListenAndServe(":8000", mux)
} else {
    lambda.Start(httpadapter.New(mux).ProxyWithContext)
}
```

**Why:** there is no real reason to run the
Lambda runtime locally for this demo — the HTTP
handlers are identical in both modes, only the
adapter (e.g. `aws-lambda-go-api-proxy`)
differs. Ruling out "Lambda against LocalStack"
and "HTTP against real AWS" lets a single
variable carry both meanings. One source of
truth, no risk of `RUN_MODE` and
`AWS_ENDPOINT_URL` disagreeing.

**How to apply:**

- Do not reintroduce a `RUN_MODE`-style
  variable. If a third mode ever appears, prefer
  extending the existing signal (e.g. detect
  Lambda via `AWS_LAMBDA_FUNCTION_NAME`, which
  the Lambda runtime sets automatically) over
  adding a new user-facing knob.
- In Tofu (prod), do not set `AWS_ENDPOINT_URL`
  on the Lambda env — leaving it unset is what
  selects Lambda mode.
- For local dev, `AWS_ENDPOINT_URL` is already
  in `.env.example` pointing to LocalStack, so
  HTTP mode is the default with no extra setup.
