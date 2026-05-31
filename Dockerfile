# ======================================================================
# 🌸 Project ORCHID: Multi-Stage Hybrid Container Packaging Dockerfile
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Project Lead & Maintainer: Kevin West (@westkevin12)
# License: GNU GPLv3
# ======================================================================

# Base image featuring gcc, golang, and compilation tools
FROM debian:bookworm-slim AS base

# Prevent interactive prompts during installs
ENV DEBIAN_FRONTEND=noninteractive

# Install core build essentials, G7 compiler, and Python Dev headers (needed by Nuitka)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    python3-dev \
    patchelf \
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

# ----------------------------------------------------------------------
# Stage 2: Developer runtime copy (Raw Python)
# ----------------------------------------------------------------------
FROM base AS developer

# Initialize sandboxed virtual environment
RUN uv venv --python 3.10
ENV PATH="/app/.venv/bin:${PATH}"

# Copy the entire Project ORCHID repository
COPY . .

# Default container target (executes full diagnostics setup)
CMD ["bash", "scripts/setup.sh"]

# ----------------------------------------------------------------------
# Stage 3: Hardened Release runtime copy (Compiled C Binary Modules)
# ----------------------------------------------------------------------
FROM base AS release-hardened

# Initialize sandboxed virtual environment
RUN uv venv --python 3.10
ENV PATH="/app/.venv/bin:${PATH}"

# Copy the entire Project ORCHID repository
COPY . .

# Install Nuitka and compile the Python control plane
RUN uv pip install nuitka && \
    python3 -m nuitka --module orchid/assembler.py --no-pyi-file --output-dir=build_nuitka && \
    python3 -m nuitka --module orchid/simulator.py --no-pyi-file --output-dir=build_nuitka && \
    python3 -m nuitka --module orchid/aggregator.py --no-pyi-file --output-dir=build_nuitka && \
    # Remove raw Python files to protect IP
    rm orchid/assembler.py orchid/simulator.py orchid/aggregator.py && \
    # Move compiled shared object binary modules into package namespace
    mv build_nuitka/*.so orchid/ && \
    # Purge compilation cache and packages to shrink image
    rm -rf build_nuitka && \
    uv pip uninstall nuitka -y

# Default container target (executes full diagnostics setup)
CMD ["bash", "scripts/setup.sh"]
