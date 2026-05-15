# syntax=docker/dockerfile:1.7
#
# Multi-stage build producing two artifacts: `worker` and `backend`.
# Build a single target with:
#   docker build --target worker  -t demo-worker  .
#   docker build --target backend -t demo-backend .
#
# The build stage is alpine (musl) and the runtime stage is
# distroless-static (debian-derived, no glibc). This mix is safe
# because CGO_ENABLED=0 produces a fully static binary with no
# dynamic linker dependency — neither libc nor ld-linux is invoked
# at runtime, so the build libc never matters.

FROM golang:1.26-alpine AS build
WORKDIR /src

# Module cache layer.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Source.
COPY . .

ENV CGO_ENABLED=0 \
    GOOS=linux

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    go build -trimpath -ldflags="-s -w" -o /out/backend ./cmd/backend

# ----- worker -------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS worker
COPY --from=build /out/worker /worker
# Bring in a static wget binary so docker-compose / ECS can run an
# HTTP healthcheck. Distroless static ships no shell or userland.
COPY --from=busybox:1.37-musl /bin/wget /usr/bin/wget
USER nonroot:nonroot
EXPOSE 8000
ENTRYPOINT ["/worker"]

# ----- backend ------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS backend
COPY --from=build /out/backend /backend
# Bring in a static wget binary so docker-compose / ECS can run an
# HTTP healthcheck. Distroless static ships no shell or userland.
COPY --from=busybox:1.37-musl /bin/wget /usr/bin/wget
USER nonroot:nonroot
EXPOSE 8000
ENTRYPOINT ["/backend"]
