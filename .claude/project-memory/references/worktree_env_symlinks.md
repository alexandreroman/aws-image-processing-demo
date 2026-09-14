---
name: "Worktree env symlinks"
description: "cmux worktrees need .env and .env.local symlinked from the main worktree; Casper copies them automatically"
type: project
---

# Worktree env symlinks

Secrets live outside the repo in the main worktree's
`.env` and `.env.local`, so every new worktree needs both
files present at its root:

```bash
git worktree add .worktrees/<slug> <branch>
cd .worktrees/<slug>
ln -s ../../.env .env
ln -s ../../.env.local .env.local
```

Both are required:

- `.env` carries Temporal Cloud credentials, the Anthropic
  API key, and the AWS region; every Make target loads it.
- `.env.local` carries the dev overlay (Moto endpoint,
  fixed bucket/table names, local Temporal dev server);
  only host-mode dev targets load it. Without it, the host
  backend has no `AWS_ENDPOINT_URL` and cannot reach Moto,
  which breaks `make dev` end to end.

**Why:** keeping the files in the main worktree means they
survive `rm -rf` of any single worktree and are not
duplicated per branch. Symlinks let each worktree see them
as project-root-local files without copying.

**How to apply:** this is a manual step on the cmux path
only. `.casper.json` lists both files in `copyFiles`, so
Casper workspaces are seeded automatically, while
`.cmux/post-create.sh` symlinks only `ca.pem` / `ca.key`.
After `/cmux:new-workspace` or a bare `git worktree add`,
add the two `ln -s` commands yourself. See
[Per-worktree compose port isolation](worktree_compose_port_isolation.md)
for the port fan-out a new worktree also gets.
