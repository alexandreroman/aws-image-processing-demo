---
name: "Worktree env symlinks"
description: "When creating a git worktree, symlink both .env and .env.local from the main worktree — the dev/deploy paths read both"
type: project
---

# Worktree env symlinks

When adding a new git worktree to this project,
the env files must be symlinked from the main
worktree:

```bash
git worktree add .worktrees/<slug> <branch>
cd .worktrees/<slug>
ln -s ../../.env .env
ln -s ../../.env.local .env.local
```

Both files are required:

- `.env` carries Temporal Cloud creds, the
  Anthropic API key, and the AWS region; loaded
  by every Make target.
- `.env.local` carries the dev overlay (Moto
  endpoint + creds, fixed bucket/table names,
  `NUXT_PUBLIC_API_BASE`); loaded only by host-
  mode dev targets. Missing it breaks
  `make dev` end-to-end:
  - the host backend cannot reach Moto (no
    `AWS_ENDPOINT_URL`),
  - the Nuxt bundle bakes the wrong API base
    and the "Start burst" button silently 404s.

**Why:** secrets live outside the repo in
`../../.env*` so they survive `rm -rf` of any
single worktree and are not duplicated per
branch. Symlinks let each worktree see them as
project-root-local files without copying.

**How to apply:** when `/cmux:new-workspace` or
`git worktree add` is used, do not forget the
two `ln -s` commands. Adding them to a helper
script (e.g. `scripts/setup-worktree.sh`) would
remove the manual step — open question whether
to formalize this. See [[dev_mode_split]] for
the broader run modes that consume these files.
