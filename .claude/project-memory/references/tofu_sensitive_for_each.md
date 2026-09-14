---
name: "OpenTofu for_each rejects sensitive values"
description: "for_each keys become resource addresses, so they must be non-sensitive; count tolerates them; nonsensitive() the presence booleans"
type: feedback
---

# OpenTofu for_each rejects sensitive values

A `for_each` argument must carry no sensitive mark, anywhere in the
value. `count` has no such restriction and accepts a sensitive
expression.

**Why:** `for_each` keys become resource instance addresses, which
OpenTofu prints in plans and writes into state, so it refuses any
value derived from a variable declared `sensitive = true`. A `count`
number never surfaces the value itself, so it is allowed.

**How to apply:** when a map keyed for `for_each` needs to carry
secret material, split it in two — a non-sensitive map of metadata
that drives `for_each`, and a sensitive map of the values, indexed by
`each.key` inside the resource body. A sensitive value is perfectly
valid as a resource *argument*; only the instance keys are
constrained. The sensitive map may list every key statically, since
only the keys selected by the `for_each` map are ever read.

Presence booleans inherit the mark too: `var.some_secret != ""` is
itself sensitive, so gating map entries on one keeps the whole map
sensitive. Wrap such booleans in `nonsensitive(...)` — whether a
secret was supplied is not itself a secret. `nonsensitive()` only
strips the mark and never changes the value, so locals shared with
other modules stay safe to pass through. It errors when applied to an
already-unmarked value, so use it only on expressions that are
statically derived from a `sensitive` variable.

Verify marks empirically with `issensitive()` in `tofu console`
rather than reasoning about them; it needs no AWS credentials, only
`-var` values for the required inputs.
