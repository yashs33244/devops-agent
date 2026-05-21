# syntax=docker/dockerfile:1.10
# =============================================================================
# Python 3.12 — Multi-Stage Production Dockerfile
# Template variables (replace before use):
#   {{PORT}}         — port the app listens on (default: 8000)
#   {{MAIN_FILE}}    — uvicorn entry point module, e.g. main (resolves to main:app)
#
# To obtain real image digests run:
#   docker pull python:3.12-slim-bookworm --no-trunc
#   docker pull gcr.io/distroless/python3-debian12:nonroot --no-trunc
# =============================================================================

# =============================================================================
# Stage 1 — builder
# Installs dependencies using uv (astral-sh), a fast Rust-based Python package
# manager. Supports both requirements.txt and pyproject.toml workflows.
# =============================================================================
FROM python:3.12-slim-bookworm AS builder
# Pin to a digest for reproducible builds: docker pull python:3.12-slim-bookworm --no-trunc

WORKDIR /app

# Prevent .pyc files and enable unbuffered output during install.
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    # uv: install into the system interpreter so packages land in the standard path
    UV_SYSTEM_PYTHON=1 \
    UV_NO_CACHE=1

# Install uv via pip — avoids needing curl/wget in the slim base image.
# Pin the version for reproducible builds.
RUN pip install --no-cache-dir uv==0.5.26

# Copy dependency specs BEFORE source to maximise cache reuse.
# Only these files invalidate the install layer.
COPY pyproject.toml uv.lock* requirements*.txt ./

# Install dependencies with uv.
# - pyproject.toml: installs the package itself (. means project root).
# - requirements.txt fallback: plain file-based install.
RUN set -eux; \
    if [ -f pyproject.toml ]; then \
      uv pip install --system --no-cache .; \
    else \
      uv pip install --system --no-cache -r requirements.txt; \
    fi

# Copy application source after deps to preserve the install cache layer.
COPY . .

# =============================================================================
# Stage 2 — runner (final distroless image)
# gcr.io/distroless/python3-debian12:nonroot contains the Python 3.x interpreter
# but NO shell, NO pip, NO OS package manager, NO curl/wget.
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
#       initialDelaySeconds: 15
#       periodSeconds: 20
#     readinessProbe:
#       httpGet:
#         path: /readyz
#         port: {{PORT}}
#       initialDelaySeconds: 5
#       periodSeconds: 10
# =============================================================================
FROM gcr.io/distroless/python3-debian12:nonroot AS runner
# Pin to a digest for reproducible builds: docker pull gcr.io/distroless/python3-debian12:nonroot --no-trunc

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PORT={{PORT}}

# Copy installed site-packages from builder so the runtime finds all packages.
COPY --from=builder --chown=nonroot:nonroot /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages

# Copy application source.
COPY --from=builder --chown=nonroot:nonroot /app .

EXPOSE {{PORT}}

# uvicorn entry point — adjust {{MAIN_FILE}} to match your app (e.g. main:app).
# The Python interpreter in distroless is at /usr/bin/python3.
CMD ["uvicorn", "{{MAIN_FILE}}:app", "--host", "0.0.0.0", "--port", "{{PORT}}"]
