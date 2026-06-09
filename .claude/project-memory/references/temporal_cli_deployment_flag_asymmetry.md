---
name: "temporal worker deployment CLI flag asymmetry + rebind hazard"
description: "delete-version has no --yes; set-current-version does. The rebind path must bound delete-version and fail loud when stranded unversioned."
type: reference
---

# temporal worker deployment CLI flag asymmetry + rebind hazard

Verified facts about the `temporal` CLI (caused a
production failure in
`scripts/register-worker-deployment.sh`):

- `temporal worker deployment delete-version` has
  **no** `--yes` flag — it does not prompt. Passing
  `--yes` errors with "unknown flag: --yes" and,
  under `set -euo pipefail`, aborts the script.
- `temporal worker deployment set-current-version`
  **does** accept `-y/--yes`.
- When a version's Lambda binding is unreachable,
  `delete-version` / `describe-version` /
  `set-current-version --build-id <id>` can HANG and
  eventually return "context deadline exceeded".
  Bound them with the CLI's own `--command-timeout`
  flag (`timeout`/`gtimeout` are not on macOS).

## Rebind danger window

Refreshing a version means delete + recreate (no
update-version command). `delete-version` refuses to
drop the current version and there is no "unset
current", so the rebind necessarily does
`set-current-version --unversioned` FIRST. That opens
a danger window: the deployment is unversioned until
the recreate completes. If the intervening
`delete-version` hangs/fails, the deployment is
stranded `--unversioned` and the Lambda runtime is
dead — restoring the old current also hangs
(Temporal-Cloud-side wedge). The script must not exit
silently or fake success here: print a loud,
actionable error (roll forward with a NEW build id:
new commit → rebuild → redeploy → re-run; or contact
Temporal Cloud support to clear the version) and exit
non-zero. Keep the `--unversioned` step first and do
not reorder it.
