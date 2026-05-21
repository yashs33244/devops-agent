# syntax=docker/dockerfile:1.10
# =============================================================================
# Rust — Multi-Stage Production Dockerfile
# Template variables (replace before use):
#   {{PORT}}        — port the app listens on (default: 8080)
#   {{BINARY_NAME}} — name of the compiled binary (matches Cargo.toml [package] name)
#
# Builds a fully static musl binary. The final image is
# gcr.io/distroless/static-debian12:nonroot — only CA certs + tzdata, nothing else.
#
# To obtain real image digests run:
#   docker pull rust:1.82-alpine --no-trunc
#   docker pull gcr.io/distroless/static-debian12:nonroot --no-trunc
# =============================================================================

# =============================================================================
# Stage 1 — builder
# Compiles a fully static binary via the x86_64-unknown-linux-musl target.
# musl libc is statically linked so the binary runs without any shared libraries.
# =============================================================================
FROM rust:1.82-alpine@sha256:0000000000000000000000000000000000000000000000000000000000000009 AS builder
# ^ replace with actual digest from: docker pull rust:1.82-alpine --no-trunc

WORKDIR /app

# Install musl-dev so the musl target has the C headers it needs.
RUN apk add --no-cache musl-dev

# Add the musl target to the toolchain.
RUN rustup target add x86_64-unknown-linux-musl

# -------------------------------------------------------------------------
# Dependency caching trick:
# Copy only the manifests and create a stub src/main.rs, build once to
# pre-compile and cache all dependencies. Then overwrite with real source.
# This means the dep-compile layer is only invalidated when Cargo.toml /
# Cargo.lock change, not on every source edit.
# -------------------------------------------------------------------------
COPY Cargo.toml Cargo.lock ./

# Create a minimal stub binary so `cargo build` succeeds with manifests only.
RUN mkdir -p src && echo 'fn main() {}' > src/main.rs

RUN --mount=type=cache,target=/usr/local/cargo/registry \
    cargo build --release --target x86_64-unknown-linux-musl \
    && rm -f target/x86_64-unknown-linux-musl/release/{{BINARY_NAME}}

# Copy real source and rebuild (only application code is recompiled, deps are cached).
COPY src ./src

RUN --mount=type=cache,target=/usr/local/cargo/registry \
    cargo build --release --target x86_64-unknown-linux-musl

# Verify the binary exists and strip it to further reduce size.
RUN set -eux; \
    BIN="target/x86_64-unknown-linux-musl/release/{{BINARY_NAME}}"; \
    [ -f "$BIN" ] || (echo "Binary not found: $BIN" && exit 1); \
    strip "$BIN"; \
    cp "$BIN" /out-binary

# =============================================================================
# Stage 2 — runner (final distroless image)
# gcr.io/distroless/static-debian12:nonroot is the smallest usable base:
#   - No libc required (musl binary is fully static)
#   - No shell, no package manager, no OS utilities
#   - Includes CA certificates and tzdata
# The pre-configured nonroot user has uid=65532 / gid=65532.
#
# HEALTHCHECK note:
#   distroless images have no shell so HEALTHCHECK CMD cannot execute.
#   Use Kubernetes liveness/readiness probes instead, e.g.:
#
#     livenessProbe:
#       httpGet:
#         path: /healthz
#         port: {{PORT}}
#       initialDelaySeconds: 5
#       periodSeconds: 15
#     readinessProbe:
#       httpGet:
#         path: /readyz
#         port: {{PORT}}
#       initialDelaySeconds: 3
#       periodSeconds: 10
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot@sha256:0000000000000000000000000000000000000000000000000000000000000006 AS runner
# ^ replace with actual digest from: docker pull gcr.io/distroless/static-debian12:nonroot --no-trunc

WORKDIR /

# Copy only the stripped static binary.
COPY --from=builder --chown=nonroot:nonroot /out-binary /{{BINARY_NAME}}

EXPOSE {{PORT}}

# Execute the binary directly — no shell, no entrypoint wrapper.
ENTRYPOINT ["/{{BINARY_NAME}}"]
