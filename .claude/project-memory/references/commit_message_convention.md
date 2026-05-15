---
name: "commit message convention"
description: "imperative subject line, no Conventional Commits prefix (no fix:/chore:/refactor:)"
type: feedback
---

# commit message convention

Write commit subjects in the imperative mood,
capitalized first letter, **no Conventional
Commits prefix** (no `fix:`, `chore:`,
`refactor:`, `feat:`, scope qualifiers, etc.).
Keep the subject short and descriptive of the
effect, not the type.

**Why:** the project's existing history on
`main` uses plain imperative sentences (e.g.
"Anchor lightbox controls to the image
bounds", "Stabilize pipeline page layout
during initial load", "Keep pipeline charts in
sync under load", "Note that ProcessImage
workflows skip workflow.GetVersion", "Stream
in-flight image previews to the gallery").
Mixing Conventional Commits in would break
the stylistic uniformity. Confirmed on
2026-05-15 after a bulk rewrite of the
`feature/code-review` branch where commits
originally typed `fix:`/`chore:`/`refactor:`
were rewritten to match `main`.

**How to apply:** when writing any commit on
this repo, drop type prefixes and scopes
entirely. Bodies and trailers
(`Co-Authored-By:` etc.) are unaffected — only
the subject line changes. Verify the style by
glancing at the last few `main` commits with
`git log --oneline main -10` before writing
a new subject.
