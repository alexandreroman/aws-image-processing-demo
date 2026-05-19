---
name: "Cleanup recipe for orphan Temporal Worker Deployments"
description: "Three-step CLI sequence to remove a Temporal Cloud Worker Deployment after a rename, deletion, or manual experiment"
type: reference
---

# Cleanup recipe for orphan Temporal Worker Deployments

`temporal worker deployment delete` is gated by two
constraints: a deployment cannot be deleted while it
has any version, and a version cannot be deleted
while it is the current version (or has active
pollers). The CLI does not expose an "unset current
version" command directly, so use the `--unversioned`
trick on `set-current-version`:

```bash
# 1. Unpin the current version of the deployment.
temporal worker deployment set-current-version \
  --address "$TEMPORAL_ADDRESS" \
  --namespace "$TEMPORAL_NAMESPACE" \
  --tls-cert-path "$PWD/$TEMPORAL_TLS_CERT" \
  --tls-key-path "$PWD/$TEMPORAL_TLS_KEY" \
  --deployment-name <name> --unversioned --yes

# 2. Delete each version (use --skip-drainage when
#    the version still has cached drainage state).
temporal worker deployment delete-version \
  ... --deployment-name <name> --build-id <build-id> \
  --skip-drainage

# 3. Delete the now-empty deployment.
temporal worker deployment delete ... --name <name>
```

When a version refuses to delete because it has
"active pollers", another deployment is probably
still routing tasks to its Lambda. Remove that
upstream routing first (step 1 on the upstream
deployment) so Temporal Cloud stops invoking the
Lambda, then retry.

Useful when:

- Renaming the worker deployment (e.g. the
  `image-processing` → `${name_prefix}-worker-lambda`
  migration left orphans).
- Cleaning up after a manual `temporal worker
  deployment create*` experiment.
- A Lambda autoregistered a deployment under an
  unintended name (mismatch between
  `WORKER_DEPLOYMENT_NAME` env var and the name
  the registration script uses).
