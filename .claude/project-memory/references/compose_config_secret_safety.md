---
name: "Inspecting the rendered compose config safely"
description: "compose.yaml interpolates ANTHROPIC_API_KEY into the worker service, so a bare `docker-compose config` prints the live key on stdout"
type: feedback
---

# Inspecting the rendered compose config safely

Validate compose changes with `docker-compose config -q`. It
resolves and checks the whole model and prints nothing.

`compose.yaml` interpolates `${ANTHROPIC_API_KEY}` from `.env`
into the `worker` service at compose time, so a bare
`docker-compose config` renders the live key in plaintext on
stdout.

**Why:** verifying a compose change is a routine step here, and
the unfiltered command is the obvious one to reach for. Its
output is just as routinely pasted into shared transcripts, CI
logs, and review comments — an audience the key is not scoped
for, and one that forces a rotation once reached.

**How to apply:**

- Default to `docker-compose config -q`. Exit status alone
  answers "is this file valid?", which is what a compose change
  usually needs.
- To read the whole rendered document, add `--no-interpolate`.
  Variable references stay literal (`${ANTHROPIC_API_KEY:?…}`)
  while the merge of `compose.yaml` with any
  `compose.override.yaml` is still fully resolved.
- To read real values — for example which environment variables
  one service receives — name that service:
  `docker-compose config backend`. `worker` is the only service
  carrying the key, so filter there explicitly:
  `docker-compose config worker | grep -v ANTHROPIC_API_KEY`.
- Treat `docker-compose config --environment` the same way: it
  prints the entire interpolation environment, key included.

Related notes:
[Per-worktree compose port isolation](worktree_compose_port_isolation.md)
for the override file a rendered config merges in, and
[Worktree env symlinks](worktree_env_symlinks.md) for the `.env`
the value is interpolated from.
