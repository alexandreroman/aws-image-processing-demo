---
name: "Determinism tests assert every run against manifest.SizeNames"
description: "Compare each of ~10 runs to an expected sequence from the canonical slice; comparing two runs to each other misses map-order regressions"
type: feedback
---

# Determinism tests assert every run against manifest.SizeNames

A test guarding workflow fan-out order builds the expected
sequence from the canonical ordered slice `manifest.SizeNames`
and asserts every run against it, repeating the workflow about
10 times. Comparing two runs to each other and checking only
that they match is not enough.

**Why:** Go randomizes iteration over a small map weakly. A
three-key map yields insertion order roughly 75% of the time —
the runtime picks among three rotations, not all six
permutations — so two consecutive runs of map-ordered code
agree often. Injecting the exact regression this test exists to
catch (ranging a map such as `manifest.SizeWidths` instead of
`manifest.SizeNames`) is caught by a run-twice-and-compare test
in only 7 of 12 trials, and by 10 runs against the expected
sequence in 20 of 20. The repeats cost about 60 ms.

**How to apply:** Build the expected slice by ranging
`manifest.SizeNames`, then loop the execution and require
equality on each iteration; see `TestProcessImageDeterminism` in
`internal/workflows/process_image_test.go`. Any future guard
over a per-size or per-key fan-out follows the same shape. For
how to capture the observed order, see
[Recording schedule order](temporal_test_schedule_order_interceptor.md).
