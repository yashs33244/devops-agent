# ── builder ────────────────────────────────────────────────────
FROM oven/bun:1.1.30-debian AS builder
WORKDIR /app

# Node.js runtime for the next CLI's `#!/usr/bin/env node` shebang.
# better-sqlite3 used to require python3 + build-essential here for
# native compilation; now that the UI uses pg (pure JS), we only need
# nodejs. The smaller toolchain shrinks the build image accordingly.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

COPY package.json ./
# bun.lock is optional — when present, install resolves deterministically;
# when absent, bun resolves package.json and writes a lockfile in-image.
COPY bun.loc[k] ./
RUN bun install

# Put node_modules/.bin on PATH so `next` / `drizzle-kit` resolve
# directly. bun install symlinks binaries here the same way npm does.
ENV PATH="/app/node_modules/.bin:${PATH}"

COPY . .

# `next build`'s "collect page data" phase evaluates route imports
# (including auth/index.ts → better-auth + drizzle-adapter init).
# better-auth requires BETTER_AUTH_SECRET + URL to initialize.
# Provide dummies; the runtime chart-managed Secret + ConfigMap
# override at container start.
#
# DATABASE_URL points at an unreachable Postgres for build — pg's Pool
# defers actual connections until the first query, and `next build`
# does no queries during static analysis. Schema is applied at deploy
# time by the chart's UI migration Helm Job (templates/ui-migrate/job.yaml).
ENV BETTER_AUTH_SECRET=build-placeholder-not-for-runtime \
    BETTER_AUTH_URL=http://localhost:3000 \
    NIGHTSHIFT_API_URL=http://placeholder:8080 \
    TENANT_ID=build \
    TENANT_NAME=build \
    OPENBAO_INTERNAL_URL=http://placeholder:8200 \
    DATABASE_URL=postgres://build:build@localhost:5432/build?sslmode=disable
RUN next build

# ── runner ─────────────────────────────────────────────────────
FROM oven/bun:1.1.30-debian AS runner
WORKDIR /app
ENV NODE_ENV=production

# Node runtime for the `next` / `drizzle-kit` CLI shebangs.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/node_modules       ./node_modules
COPY --from=builder /app/.next              ./.next
COPY --from=builder /app/public             ./public
COPY --from=builder /app/package.json       ./package.json
COPY --from=builder /app/next.config.mjs    ./next.config.mjs
COPY --from=builder /app/db                 ./db
COPY --from=builder /app/drizzle.config.ts  ./drizzle.config.ts

ENV PATH="/app/node_modules/.bin:${PATH}"

RUN chown -R bun:bun /app
USER bun

EXPOSE 3000

# Pods are stateless and don't run migrations — that's the chart's
# UI migration Helm Job. drizzle-kit + drizzle.config.ts ship in
# the image so the migration Job can reuse this same image and
# invoke `bunx drizzle-kit migrate`.
CMD ["next", "start", "-p", "3000", "-H", "0.0.0.0"]
