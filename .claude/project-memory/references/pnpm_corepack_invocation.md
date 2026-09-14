---
name: "pnpm invocations run from inside frontend/"
description: "Frontend pnpm commands run as `cd frontend && corepack pnpm …`; the launcher picks the version from its own cwd, so `-C` is applied too late"
type: feedback
---

# pnpm invocations run from inside frontend/

Every frontend pnpm command enters `frontend/` first and then runs
`corepack pnpm …`. The launcher resolves the `packageManager` pin
(`pnpm@12.4.1` in `frontend/package.json`) from its **own** working
directory, and `-C <dir>` takes effect only afterwards. There is no
`package.json` at the repository root, so a call made from there falls
back to the `pnpm` on the shell PATH — the mise-managed Node LTS build,
a different major.

**Why:** a pnpm 10 install rewrites `frontend/pnpm-lock.yaml` in its
own format and drops the `configDependencies` and
`packageManagerDependencies` sections pnpm 12 writes. The frontend
image runs `pnpm install --frozen-lockfile` through corepack, which
resolves 12.4.1 and rejects the truncated lockfile with
`ERR_PNPM_FROZEN_LOCKFILE_WITH_OUTDATED_LOCKFILE`. The host install
succeeds either way, so the breakage surfaces only at image build
time — far from the command that caused it.

**How to apply:** write `cd frontend && corepack pnpm <cmd>`, or a
`(cd "${frontend_dir}" && corepack pnpm <cmd>)` subshell in a script.
The cwd is what selects the version — prefixing with `corepack` while
keeping `-C frontend` does not help, and measuring settles it:
`pnpm -C frontend --version` and `corepack pnpm -C frontend --version`
both report 10.33.0 from the repository root, while
`(cd frontend && corepack pnpm --version)` reports 12.4.1. The
frontend `Dockerfile` needs no `cd`: its `WORKDIR` already is the
frontend directory. After any dependency change, confirm
`sed -n '1,14p' frontend/pnpm-lock.yaml` still shows
`packageManagerDependencies` with pnpm 12.4.1. See
[[pnpm_build_approvals]] and [[pnpm_minimum_release_age]].
