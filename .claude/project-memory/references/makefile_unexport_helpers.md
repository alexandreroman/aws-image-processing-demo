---
name: "Recursive Make helper variables need unexport"
description: "The root Makefile's bare `export` expands every recursive variable per recipe, so a `$(call ...)` helper needs `unexport` right after its definition"
type: feedback
---

# Recursive Make helper variables need unexport

Every recursive (`=`) variable in the root `Makefile` that exists only to
be invoked through `$(call ...)` carries an `unexport <name>` line
immediately after its definition.

**Why:** the `.env` and `.env.local` includes are each followed by a bare
`export`, which marks *every* variable for the recipe environment. To
build that environment GNU Make has to expand each one — and a
`$(call ...)` helper expanded outside a call site sees `$(1)`, `$(2)` …
as empty. A helper whose body contains `$(warning ...)` therefore fires
it with empty values before every single recipe. `override_port`, which
reads a published host port out of `compose.override.yaml` and warns when
a mapping is absent, prints `compose.override.yaml publishes no host port
for : — falling back to` on each invocation without the guard.
`unexport` is the direct counterpart of the bare `export`: it keeps the
helper out of the child environment entirely.

**How to apply:**

- Write `unexport <name>` on the line following the helper's definition,
  and say in a comment why it is there — the pairing with the bare
  `export` far above is not visible locally.
- Reproduce with a real recipe such as `make help`. `make -n` does not
  surface it: Make builds a child environment only when it actually runs
  a recipe, so dry-run mode hides the symptom entirely.
- Keep the helper recursive. A simply-expanded (`:=`) definition is
  evaluated once at parse time with no arguments, so `$(call ...)` has no
  way to pass any.
- Prefer `unexport` over guarding the body with `$(if $(strip $(1)),…)`.
  The guard silences the message but still ships a junk `override_port=…`
  entry into every recipe's environment, and it obscures the reason the
  guard exists.

See [Per-worktree compose port isolation](worktree_compose_port_isolation.md)
for the ports `override_port` feeds.
