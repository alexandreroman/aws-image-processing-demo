# syntax=docker/dockerfile:1.7
#
# Two-stage build: produce the Nuxt SSG bundle, then serve it through
# Caddy. Caddy also reverse-proxies /api/* to the backend container —
# same-origin routing that mirrors production (CloudFront → S3 +
# API Gateway, single domain).

FROM node:24-alpine AS build
WORKDIR /app

RUN corepack enable

# Dependency layer.
COPY package.json pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

# Source + build. NUXT_PUBLIC_API_BASE="" makes the bundle issue
# same-origin /api/* requests, which Caddy then routes to backend:8000.
COPY . .
ENV NUXT_PUBLIC_API_BASE="" \
    NUXT_PUBLIC_S3_PUBLIC_URL=""
RUN pnpm generate

# ----- runtime ------------------------------------------------------------
FROM caddy:2-alpine
COPY --from=build /app/.output/public /srv
COPY Caddyfile /etc/caddy/Caddyfile
EXPOSE 3000
