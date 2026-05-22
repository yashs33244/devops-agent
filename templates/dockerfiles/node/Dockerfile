# syntax=docker/dockerfile:1.10
# =============================================================================
# Node.js 22 — Multi-Stage Production Dockerfile
# Template variables (replace before use):
#   {{PORT}}  — port the app listens on (default: 3000)
#
# To obtain real image digests run:
#   docker pull node:22-alpine --no-trunc
#   docker pull gcr.io/distroless/nodejs22-debian12:nonroot --no-trunc
# =============================================================================

# =============================================================================
# Stage 1 — deps
# Install ONLY production dependencies.
# Isolated stage so the cached layer survives source-code changes and is only
# busted when a lockfile changes.
# =============================================================================
FROM node:22-alpine AS deps
# Pin to a digest for reproducible builds: docker pull node:22-alpine --no-trunc

WORKDIR /app

# Copy all possible lockfiles and the manifest so the correct one is present
# regardless of which package manager the project uses.
COPY package.json package-lock.json* yarn.lock* pnpm-lock.yaml* ./

# Auto-detect the package manager from the lockfile and install production deps.
RUN set -eux; \
    if [ -f yarn.lock ]; then \
      yarn install --frozen-lockfile --production --non-interactive; \
    elif [ -f pnpm-lock.yaml ]; then \
      corepack enable pnpm && pnpm install --frozen-lockfile --prod; \
    else \
      npm ci --only=production --ignore-scripts; \
    fi

# =============================================================================
# Stage 2 — builder
# Full install (devDependencies included) + compile step.
# =============================================================================
FROM node:22-alpine AS builder
# Pin to a digest for reproducible builds: docker pull node:22-alpine --no-trunc

WORKDIR /app

COPY package.json package-lock.json* yarn.lock* pnpm-lock.yaml* ./

# Install all deps for the build (devDependencies needed for TypeScript, etc.).
RUN set -eux; \
    if [ -f yarn.lock ]; then \
      yarn install --frozen-lockfile --non-interactive; \
    elif [ -f pnpm-lock.yaml ]; then \
      corepack enable pnpm && pnpm install --frozen-lockfile; \
    else \
      npm ci; \
    fi

# Copy source AFTER deps — preserves the dep-install cache layer on code changes.
COPY . .

# Build the project (TypeScript compile, webpack bundle, etc.)
RUN set -eux; \
    if [ -f yarn.lock ]; then \
      yarn build; \
    elif [ -f pnpm-lock.yaml ]; then \
      pnpm run build; \
    else \
      npm run build; \
    fi

# =============================================================================
# Stage 3 — runner (final distroless image)
# gcr.io/distroless/nodejs22-debian12:nonroot contains only the Node.js
# runtime — no shell, no package manager, no wget/curl, no OS utilities.
# The pre-configured nonroot user has uid=65532 / gid=65532.
#
# HEALTHCHECK note:
#   distroless images have no shell so `HEALTHCHECK CMD` cannot execute.
#   Configure a Kubernetes liveness/readiness probe instead, e.g.:
#
#     livenessProbe:
#       httpGet:
#         path: /healthz
#         port: {{PORT}}
#       initialDelaySeconds: 15
#       periodSeconds: 20
#     readinessProbe:
#       httpGet:
#         path: /readyz
#         port: {{PORT}}
#       initialDelaySeconds: 5
#       periodSeconds: 10
# =============================================================================
FROM gcr.io/distroless/nodejs22-debian12:nonroot AS runner
# Pin to a digest for reproducible builds: docker pull gcr.io/distroless/nodejs22-debian12:nonroot --no-trunc

WORKDIR /app

# Runtime configuration.
ENV NODE_ENV=production
ENV PORT={{PORT}}

# Copy production node_modules from the dedicated deps stage (not from builder)
# so that devDependencies are never present in the final image.
COPY --from=deps    --chown=nonroot:nonroot /app/node_modules ./node_modules

# Copy compiled artefacts.
COPY --from=builder --chown=nonroot:nonroot /app/dist         ./dist

# Package manifest required by some frameworks for module resolution.
COPY --from=builder --chown=nonroot:nonroot /app/package.json ./package.json

EXPOSE 8080

# CMD must reference the JS file directly — no shell wrapper allowed.
CMD ["dist/index.js"]
