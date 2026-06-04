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
# Stage 3: Builder stage to generate assembly and compile Go daemon
# ----------------------------------------------------------------------
FROM base AS release-builder

WORKDIR /app
COPY . .

# Generate assembly kernels from planning specs
RUN python3 -c \
    "import sys; from orchid.assembler import main; sys.exit(main())" \
    locality/matmul.plan --out-dir cmd/orchid-daemon

# Compile Go daemon binary
RUN go build -o /app/orchid-daemon ./cmd/orchid-daemon

# ----------------------------------------------------------------------
# Stage 4: Hardened Release runtime copy (Zero-Dependency distroless)
# ----------------------------------------------------------------------
FROM gcr.io/distroless/base-debian12:nonroot AS release-hardened

WORKDIR /app

# Copy the compiled Go daemon executable
COPY --from=release-builder /app/orchid-daemon /app/orchid-daemon

# Default container target (executes full sweeps diagnostics)
CMD ["/app/orchid-daemon", "--mode", "all"]
