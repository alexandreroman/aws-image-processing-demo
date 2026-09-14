---
name: "Operating Temporal Worker Deployments from the CLI"
description: "Delete/rebind order, CLI flag asymmetry, the namespace-wide create-version rate limit, and how to judge Lambda-runtime health"
type: reference
---

# Operating Temporal Worker Deployments from the CLI

Hard-won facts about `temporal worker deployment …`
against Temporal Cloud. They are not in the CLI help and
each one has already cost a live outage.

## Deletion / rebind order

A deployment cannot be deleted while it has any version,
and a version cannot be deleted while it is the current
version. The CLI has no "unset current version" command,
so `set-current-version --unversioned` plays that role and
**must come first**:

```bash
# 1. Unpin the current version.
temporal worker deployment set-current-version \
  --deployment-name <name> --unversioned --yes

# 2. Delete each version (--skip-drainage when the version
#    still holds cached drainage state).
temporal worker deployment delete-version \
  --deployment-name <name> --build-id <build-id> \
  --skip-drainage --command-timeout 30s

# 3. Delete the now-empty deployment.
temporal worker deployment delete --name <name>
```

Never reorder the `--unversioned` step. Refreshing a
version means delete + recreate (there is no
`update-version`), so a rebind runs the same sequence.

When a version refuses to delete for "active pollers",
another deployment is still routing tasks to its Lambda.
Unpin that upstream deployment first, then retry.

## Flag asymmetry and hangs

- `delete-version` has **no** `--yes` flag — it does not
  prompt. Passing `--yes` fails with "unknown flag" and,
  under `set -euo pipefail`, aborts the caller mid-rebind.
- `set-current-version` **does** accept `-y/--yes`.
- When a version's Lambda binding is unreachable,
  `delete-version`, `describe-version` and
  `set-current-version --build-id <id>` hang and end in
  "context deadline exceeded". Bound them with the CLI's
  own `--command-timeout` (`timeout`/`gtimeout` are absent
  on macOS).

## Rebind danger window

Between step 1 and the recreate, the deployment is
unversioned and the Lambda runtime is dead. If the
intervening `delete-version` hangs, the deployment is
stranded on `--unversioned`, and restoring the previous
current version hangs too. Any script doing this must fail
loudly and exit non-zero — never silently or with a faked
success — printing the roll-forward path: new commit →
rebuild → redeploy → re-run, or contact Temporal Cloud
support to clear the version.

## Rate limit: one attempt, then stop

`create-version` is rate-limited **per namespace**, and the
limit does not drain under retry:

- Repeated describe / delete / set-current / create attempts
  trip "too many requests issued to Worker Deployment …",
  which then blocks even a brand-new deployment's first
  `create-version`, and stays tripped well past 10 minutes
  of retries. Make one attempt, then stop and wait.
- Rolling onto a fresh deployment name does not dodge it —
  the limit is per namespace, not per deployment.
- Recovery is a single deploy with a **fresh build id** (new
  commit). `set-current-version` onto a fresh version
  succeeds even while the previous current version is
  wedged, so it sidesteps the wedge without deleting it;
  the wedged version becomes an orphan to clean up later
  with the sequence above.

## Judging Lambda-runtime health

**0 pollers at rest is normal for the Lambda runtime** — it
is invoked on demand and runs no long-poller. Judge health
by `currentVersionBuildID` matching the live build and by
tasks actually progressing, never by idle poller count.
