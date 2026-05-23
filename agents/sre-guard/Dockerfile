# syntax=docker/dockerfile:1.7
# ---------------------------------------------------------------------------
# Stage 1: builder — install Python dependencies
# ---------------------------------------------------------------------------
FROM python:3.12-slim-bookworm AS builder

WORKDIR /app

# Install build tools and uv for fast installs
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml ./
COPY src/ ./src/

# Install into a prefix we can copy wholesale
RUN pip install --no-cache-dir --prefix=/install ".[dev]"

# ---------------------------------------------------------------------------
# Stage 2: runner — distroless Python (nonroot)
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/python3-debian12:nonroot

WORKDIR /app

# Copy installed packages from builder
COPY --from=builder /install /usr/local
COPY --from=builder /app/src /app/src

# Copy default config
COPY config/sre-guard.yaml /etc/sre-guard/sre-guard.yaml

# Copy CLI
COPY cli/sreguard.py /app/cli/sreguard.py

EXPOSE 8888

# distroless nonroot user (UID 65532)
USER nonroot

# PYTHONPATH so sre_guard package is importable
ENV PYTHONPATH="/app/src:/usr/local/lib/python3.12/site-packages"

# Health check — hits the daemon's own /health endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/python3", "-c", \
         "import urllib.request; urllib.request.urlopen('http://localhost:8888/health')"]

CMD ["/usr/local/bin/python3", "-m", "sre_guard.daemon"]
