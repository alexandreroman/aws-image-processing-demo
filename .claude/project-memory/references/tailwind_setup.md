---
name: "Frontend Tailwind setup"
description: "Tailwind v4 is wired through @tailwindcss/vite in nuxt.config.ts, with the theme in an @theme block in app/assets/css/main.css and no tailwind.config file"
type: project
---

# Frontend Tailwind setup

The frontend runs Tailwind CSS v4, wired through the first-party
`@tailwindcss/vite` plugin registered under `vite.plugins` in
`frontend/nuxt.config.ts`. The design tokens — brand colours
(`primary`, `accent`, `iris`, `bg`, `surface*`, `ink`), the `sans` and
`mono` stacks, `shadow-card` / `shadow-glow`, and the three custom
animations with their `@keyframes` — live in an `@theme` block at the
top of `frontend/app/assets/css/main.css`, which opens with
`@import "tailwindcss"`. The styling carries no `tailwind.config`
file and no Nuxt Tailwind module.

**Why:** the Nuxt Tailwind module hard-pins `tailwindcss` to the v3
line as a direct dependency, so it cannot coexist with v4. The Vite
plugin is what the Tailwind guide for Nuxt prescribes, and it keeps
the CSS entry point and the plugin that compiles it side by side.

**How to apply:** declare design tokens as CSS variables inside
`@theme` (`--color-*`, `--font-*`, `--shadow-*`, `--animate-*`),
never in a JS config. Background images have no theme namespace, so
`bg-grid` ships as an `@utility`. `@apply` accepts utilities only,
never a component class declared in `@layer components` — which is
why `.btn-primary-lg` repeats `.btn`'s declarations instead of
applying it by name; doing otherwise fails the build with
`Cannot apply unknown utility class`. Utility renames worth knowing
when editing templates: `bg-linear-*` for gradients, `outline-hidden`
for the focus-ring reset, and `rounded-sm` for the 0.25rem radius.
