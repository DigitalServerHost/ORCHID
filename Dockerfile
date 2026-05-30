# ======================================================================
# 🌸 Project ORCHID: Multi-Language Isolated Developer Sandbox Dockerfile
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Project Lead & Maintainer: Kevin West (@westkevin12)
# License: GNU GPLv3
# ======================================================================

# Base image featuring gcc, golang, and basic utilities
FROM debian:bookworm-slim AS base

# Prevent interactive prompts during installs
ENV DEBIAN_FRONTEND=noninteractive

# Install core build essentials, compiler packages, and system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    curl \
    git \
    ca-certificates \
    python3 \
    golang-go \
    && rm -rf /var/lib/apt/lists/*

# Embed Astral 'uv' from its official distribution image
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uv/bin/
ENV PATH="/uv/bin:${PATH}"

WORKDIR /app

# Set uv environment variable to avoid container symbol link constraints
ENV UV_LINK_MODE=copy

# Pre-fetch and register Python 3.10 runtime
RUN uv python install 3.10

# Stage 2: Developer runtime copy
FROM base AS developer

# Initialize sandboxed virtual environment
RUN uv venv --python 3.10
ENV PATH="/app/.venv/bin:${PATH}"

# Copy the entire Project ORCHID repository
COPY . .

# Default container target (executes full diagnostics setup)
CMD ["bash", "scripts/setup.sh"]
