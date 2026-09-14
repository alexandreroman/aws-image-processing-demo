---
name: "pnpm dependency build approvals"
description: "Frontend build-script approvals live in frontend/pnpm-workspace.yaml as an allowBuilds map; the pnpm field in package.json is inert"
type: project
---

# pnpm dependency build approvals

The frontend pins pnpm 12, which reads dependency build-script
approvals only from an `allowBuilds` map in
`frontend/pnpm-workspace.yaml`. A `"pnpm"` field in `package.json`
(including `onlyBuiltDependencies`) is inert there and triggers
`[WARN] The "pnpm" field in package.json is no longer read by pnpm`.

**Why:** pnpm 12 turns unapproved build scripts into a hard
`ERR_PNPM_IGNORED_BUILDS` failure instead of a warning, so a missing
or misplaced approval breaks `pnpm install` outright — including
inside the frontend image, which is why the dependency layer copies
`pnpm-workspace.yaml` alongside `package.json` and the lockfile.

**How to apply:** when a dependency needs a postinstall build (native
binaries such as `esbuild`, `@parcel/watcher`, `unrs-resolver`), add
it to `allowBuilds` in `frontend/pnpm-workspace.yaml` with the value
`true`. Keep `package.json` free of a `"pnpm"` section.
