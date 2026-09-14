---
name: "pnpm invocations go through corepack"
description: "Frontend pnpm commands run as `corepack pnpm`; the PATH pnpm is a different major and writes a lockfile the image build rejects"
type: feedback
---

# pnpm invocations go through corepack

Every frontend pnpm command runs as `corepack pnpm …`. The `pnpm` on
the shell PATH comes from the mise-managed Node LTS and is a different
major version than the `packageManager` pin (`pnpm@12.4.1`) in
`frontend/package.json`.

**Why:** a pnpm 10 install rewrites `frontend/pnpm-lock.yaml` in its
own format and drops the `configDependencies` and
`packageManagerDependencies` sections pnpm 12 writes. The frontend
image runs `pnpm install --frozen-lockfile` through corepack, which
resolves 12.4.1 and rejects the truncated lockfile with
`ERR_PNPM_FROZEN_LOCKFILE_WITH_OUTDATED_LOCKFILE`. The host install
succeeds either way, so the breakage surfaces only at image build
time — far from the command that caused it.

**How to apply:** run `corepack pnpm -C frontend install|build|lint`
rather than bare `pnpm` whenever dependencies change. The `dev`,
`check` and `frontend` Make targets shell out to bare `pnpm`, which is
safe for running but not for writing the lockfile. After any
dependency change, confirm `sed -n '1,14p' frontend/pnpm-lock.yaml`
still shows `packageManagerDependencies` with pnpm 12.4.1. See
[[pnpm_build_approvals]] and [[pnpm_minimum_release_age]].
