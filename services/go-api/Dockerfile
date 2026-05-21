# syntax=docker/dockerfile:1.10
# =============================================================================
# Go 1.23 — Multi-Stage Production Dockerfile
# Binary: go-api
# Port:   8080
#
# To obtain real image digests run:
#   docker pull golang:1.23-alpine --no-trunc
#   docker pull gcr.io/distroless/static-debian12:nonroot --no-trunc
# =============================================================================

# =============================================================================
# Stage 1 — builder
# Produces a fully static Linux binary via CGO_ENABLED=0.
# -ldflags="-w -s" strips debug info and symbol table, reducing binary size.
# =============================================================================
FROM golang:1.23-alpine AS builder
# Pin to a digest for reproducible builds: docker pull golang:1.23-alpine --no-trunc

WORKDIR /app

# Cross-compilation flags for a static Linux binary.
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Copy module manifests BEFORE source for cache-efficient dependency download.
# The go mod download layer is only invalidated when go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source after modules are cached.
COPY . .

# Build — strip debug info (-w) and symbol table (-s) to minimise binary size.
RUN go build \
      -ldflags="-w -s" \
      -trimpath \
      -o /out/go-api \
      ./...

# =============================================================================
# Stage 2 — runner (final distroless image)
# gcr.io/distroless/static-debian12:nonroot is the smallest possible base:
#   - No libc (CGO_ENABLED=0 binary needs none)
#   - No shell, no package manager, no OS utilities
#   - CA certificates + tzdata included
# The pre-configured nonroot user has uid=65532 / gid=65532.
#
# HEALTHCHECK note:
#   distroless images have no shell so HEALTHCHECK CMD cannot execute.
#   Use Kubernetes liveness/readiness probes instead, e.g.:
#
#     livenessProbe:
#       httpGet:
#         path: /healthz
#         port: 8080
#       initialDelaySeconds: 5
#       periodSeconds: 15
#     readinessProbe:
#       httpGet:
#         path: /readyz
#         port: 8080
#       initialDelaySeconds: 3
#       periodSeconds: 10
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS runner
# Pin to a digest for reproducible builds: docker pull gcr.io/distroless/static-debian12:nonroot --no-trunc

WORKDIR /

# Copy only the compiled binary — nothing else is needed for a static Go binary.
COPY --from=builder --chown=nonroot:nonroot /out/go-api /go-api

EXPOSE 8080

# Execute the binary directly — no shell wrapper.
ENTRYPOINT ["/go-api"]
