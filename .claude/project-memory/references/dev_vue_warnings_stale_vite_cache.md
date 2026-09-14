---
name: "Dev-only Vue warnings from a stale Vite module cache"
description: "In `nuxt dev`, the resolveComponent / hoisted-ref / RouterLink hydration warning cluster means two cached Vue copies, not a code bug"
type: feedback
---

# Dev-only Vue warnings from a stale Vite module cache

When `nuxt dev` logs this cluster in the browser console — all four
together, on a plain page load:

- `resolveComponent can only be used in render() or setup().`
- `Hydration node mismatch: rendered on server: {} expected on
  client: RouterLink` at the header `<NuxtLink to="/">`
- `Missing ref owner context. ref cannot be used on hoisted vnodes.`
- `withDirectives can only be used inside render functions.`

— the page is running **two instances of Vue**, and the second one comes
from the browser's HTTP cache, not from the dependency tree. Hard-reload
the page (bypassing cache) before investigating anything else.

**Why:** Vite's dev server serves every module URL that carries a
`?v=<browserHash>` query with `Cache-Control: max-age=31536000,
immutable`. When the dep optimizer re-runs — it prints
`Re-optimizing dependencies because lockfile has changed` after any
install or lockfile edit — the hash changes and the server starts
emitting `?v=<newHash>`. Modules already cached under the old hash stay
reachable through a stale cached importer, so part of the module graph
keeps importing the old `@vue/runtime-core`. Two runtime-core instances
mean `currentRenderingInstance` is null in one of them, which is exactly
what those four warnings report. The hydration mismatch is real but its
cause is the duplicate runtime, so suppressing it or wrapping the link
in `<ClientOnly>` would hide a symptom of a cache artefact.

**How to apply:** confirm before theorising — in the browser, read
`[...new Set(performance.getEntriesByType('resource').map(e => e.name)
.filter(n => /runtime-core/.test(n)))]`. More than one distinct `?v=`
hash proves the duplication; the stale entries report
`transferSize: 0`. Clear it by re-fetching same-origin resources with
`cache: 'reload'` and reloading, or by serving the dev server on a port
the browser has not visited. This is a dev-server artefact only: the
production build bundles a single Vue chunk and both `/` and
`/pipelines/{id}` are clean in `.output`. Related:
[[pnpm_corepack_invocation]].
