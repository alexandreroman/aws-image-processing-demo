---
name: "Go image builds resolve modules during go build"
description: "The root Dockerfile build stage has no `go mod download` layer; `tool` directives pull Air and Hugo into a ~370-module list."
type: feedback
---

# Go image builds resolve modules during go build

The root `Dockerfile` build stage copies the source once and lets
`go build` fetch modules for `./cmd/worker` and `./cmd/backend`. It
carries no `COPY go.mod go.sum` + `go mod download` prefetch layer, so
the project-rules Go container reference is deliberately not followed
on this point.

**Why:** `go mod download` with no arguments fetches the whole module
build list. `go.mod` declares `tool github.com/air-verse/air`, and Air
pulls `github.com/gohugoio/hugo` and its large dependency tree, so the
build list is ~370 modules against the 66 the two binaries import. That
extra surface is what trips flaky `proxy.golang.org` reads in CI. The
layer also buys nothing there: the `type=gha` cache backend does not
export BuildKit `--mount=type=cache` mounts, so even a cache-hit layer
leaves the module cache empty for the later `go build`.

**How to apply:** keep the build stage at a single `COPY . .` followed
by the `go build` RUN carrying both cache mounts (`id=gobuild`,
`id=gomod`). If a prefetch layer is ever warranted, give `go mod
download` explicit module arguments instead of none.
