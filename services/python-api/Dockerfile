# syntax=docker/dockerfile:1.10
# Python 3.12 — Multi-Stage Production Dockerfile for python-api

# =============================================================================
# Stage 1 — builder
# =============================================================================
FROM python:3.12-slim-bookworm AS builder

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

# Install dependencies only (not the project itself — flat layout)
COPY pyproject.toml requirements*.txt ./
RUN pip install --no-cache-dir fastapi==0.115.5 "uvicorn[standard]==0.32.1" prometheus-client==0.21.0 httpx==0.28.0

# Copy application source after deps
COPY . .

# =============================================================================
# Stage 2 — runner
# Use slim image (distroless lacks uvicorn in PATH for CMD execution)
# =============================================================================
FROM python:3.12-slim-bookworm AS runner

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PORT=8000

# Copy installed packages from builder
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin/uvicorn /usr/local/bin/uvicorn

# Copy application source
COPY --from=builder /app/main.py .

# Run as non-root
RUN useradd -u 65532 -r -s /bin/false nonroot && chown -R nonroot:nonroot /app
USER nonroot

EXPOSE 8000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
