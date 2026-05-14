---
name: "No per-image notifications"
description: "Do not emit a UI toast or notification for each processed image; a single end-of-burst signal is acceptable"
type: feedback
---

# No per-image notifications

Do not display a toast (or any equivalent
notification) for each processed image. One
notification per completed image clutters the
UI when bursts contain dozens of images.

**Why:** the demo is designed around bursts of
many images at once; stacking one toast per
completion creates visual noise that masks
useful information (errors, summary metrics).
The pipeline page already shows live progress
through the gallery and metrics cards, so
per-image confirmation is redundant.

**How to apply:** in
`frontend/app/pages/pipelines/[id].vue` (and
anywhere else that watches workflow completion),
do not call `toast.success(...)` per workflow.
If a completion signal is desired, emit at most
one toast when the whole burst transitions to
done — never one per item. Errors may still be
toasted individually since they are rare and
actionable.
