---
name: "Triggering a workflow from the Temporal CLI"
description: "Launch a single ProcessImage workflow for debug using `temporal workflow start`"
type: project
---

# Triggering a workflow from the Temporal CLI

To trigger a single `ProcessImage` workflow
without going through the frontend (for debug,
CI smoke tests, or demo automation), use the
`temporal` CLI directly:

```bash
temporal workflow start \
  --type ProcessImage \
  --task-queue image-processing \
  --workflow-id "manual-$(uuidgen)" \
  --input '{"bucket":"aws-image-processing-demo-images-local","key":"samples/dog.jpg"}'
```

Project-specific values:

- Workflow type: `ProcessImage` (defined in
  `internal/workflows`).
- Task queue: `image-processing`.
- Input shape: JSON
  `{"bucket": "<bucket-name>", "key": "<s3-key>"}`.

**Pre-condition:** the image must already be in
S3. The CLI does not upload — that is the job
of the frontend (via presigned URL) or a manual
`aws s3 cp` for debug, e.g.

```bash
aws --endpoint-url http://localhost:4566 s3 cp \
  ./samples/dog.jpg \
  s3://aws-image-processing-demo-images-local/uploads/dog.jpg
```

**How to apply:**

- For scripted invocation, shell out to
  `temporal workflow start` from a script in
  `scripts/`.
- The CLI's Temporal connection is configured
  via `temporal env set …` for Temporal Cloud;
  locally it defaults to `localhost:7233`.
- Keep the workflow input shape stable across
  the frontend, the CLI, and any future
  scripts — they all post the same
  `{bucket, key}` payload.
