---
name: "pnpm minimumReleaseAge exemptions"
description: "pnpm 12 rejects lockfile entries published within 24h; targeted exemptions live in minimumReleaseAgeExclude in frontend/pnpm-workspace.yaml"
type: project
---

# pnpm minimumReleaseAge exemptions

pnpm 12 enforces a `minimumReleaseAge` supply-chain policy — a 24-hour
window, applied by default and set nowhere in the repository. An
install fails with `ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION` when a
lockfile entry is younger than that window. Exemptions are listed by
package name under `minimumReleaseAgeExclude` in
`frontend/pnpm-workspace.yaml`.

**Why:** the rejection is about publication age, not package contents,
and it blocks `pnpm install`, `pnpm build` and the frontend image
build at once. Disabling the policy globally (`minimumReleaseAge=0`,
or an `.npmrc` override) drops the check for every entry in the
lockfile, so a named exemption is the narrow form that keeps the
protection everywhere else.

**How to apply:** when an install fails on release age, add only the
named package to `minimumReleaseAgeExclude` — never the global
opt-out. An entry is removable once its version clears the window;
left in place it grants that one package a permanent bypass. A fresh
transitive dependency of a newly bumped library is the usual trigger,
so expect this during dependency-update passes. See
[[pnpm_build_approvals]] and [[pnpm_corepack_invocation]].
