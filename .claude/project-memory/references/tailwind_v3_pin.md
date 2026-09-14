---
name: "Frontend Tailwind stays on the v3 LTS line"
description: "tailwindcss is pinned to the ^3.4.x v3-lts line because @nuxtjs/tailwindcss hard-pins tailwindcss ~3.4.17; never document or assume Tailwind v4"
type: project
---

# Frontend Tailwind stays on the v3 LTS line

`frontend/package.json` pins `tailwindcss` to the `^3.4.x` `v3-lts`
line. The frontend styling, docs, and README describe Tailwind as v3 —
never as v4.

**Why:** the Nuxt integration `@nuxtjs/tailwindcss` (v6.14.0) hard-pins
`tailwindcss: ~3.4.17` as a dependency. Bumping `tailwindcss` to v4
conflicts with that pin, and v4's CSS-first configuration is
incompatible with the `tailwind.config` / PostCSS setup the module
expects.

**How to apply:** during a dependency-update pass, bump every other
frontend dependency freely, but leave `tailwindcss` on the v3 line.
Tailwind v4 becomes reachable only once `@nuxtjs/tailwindcss` ships a
release that accepts it — or once the project drops that module and
wires the `@tailwindcss/postcss` plugin directly.
