# nightshift-worker-claude — Python image for the reference LLM worker.
#
# Adapted from cr0n-a's Dockerfile. The worker drives the Claude Agent
# SDK, which requires the @anthropic-ai/claude-code CLI on PATH (see
# https://docs.anthropic.com/en/docs/claude-code/sdk).
#
# This image is worker-only — there is no API surface to serve. Pods
# launched by the nightshift-api Workers service run `worker.py` as a
# K8s Job. See cmd/nightshift-worker-claude/README.md for env-var
# contract + which MCP tools require chunks 15-17 to function.

FROM ghcr.io/astral-sh/uv:python3.13-trixie-slim

ENV DEBIAN_FRONTEND=noninteractive

# Apt deps:
#   - cairo / pango / pangoft2 / gdk-pixbuf / shared-mime-info: weasyprint
#     runtime (used by report_generators.create_pdf).
#   - fonts-dejavu / fonts-liberation: PDF / DOCX text rendering.
#   - git / curl / ca-certificates: pulled in for downstream MCP tools that
#     git-clone or fetch references.
RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends \
    git curl ca-certificates gnupg \
    libcairo2 libpango-1.0-0 libpangoft2-1.0-0 \
    libgdk-pixbuf-2.0-0 libffi-dev shared-mime-info \
    fonts-dejavu fonts-liberation && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Node 22 + Claude Code CLI (required by claude-agent-sdk).
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y -qq nodejs && \
    apt-get clean && rm -rf /var/lib/apt/lists/* && \
    npm install -g @anthropic-ai/claude-code

# Non-root user 65532 — matches the chunk-9 hardened pod SecurityContext
# (runAsUser=65532). The launcher applies the SecurityContext to the
# pod regardless; baking the same uid here lets `docker run` work too.
RUN useradd -u 65532 -m -s /bin/bash nightshift

WORKDIR /opt/nightshift-worker-claude

ENV UV_NO_DEV=1
ENV UV_COMPILE_BYTECODE=1
ENV PATH="/opt/nightshift-worker-claude/.venv/bin:$PATH"

COPY pyproject.toml ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --no-install-project

COPY . /opt/nightshift-worker-claude

RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync

# Per-pod ephemeral workspace; SDK transcripts get symlinked from
# <workspace>/.claude/projects to NS_SESSION_STATE_DIR by worker.py
# when the chunk-13 mount is present.
RUN mkdir -p /home/nightshift/workspace && \
    chown -R nightshift:nightshift /home/nightshift /opt/nightshift-worker-claude

USER nightshift
WORKDIR /home/nightshift

CMD ["python", "/opt/nightshift-worker-claude/worker.py"]
