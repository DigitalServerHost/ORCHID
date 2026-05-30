# ======================================================================
# 🌸 Project ORCHID: Unified Command Dashboard Makefile
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Project Lead & Maintainer: Kevin West (@westkevin12)
# License: GNU GPLv3
# ======================================================================

.PHONY: help setup test build dist docker-up clean

# Default target listing all options
help:
	@echo "🌸 Project ORCHID: Unified Tooling Interface Daemon Commands"
	@echo "======================================================================"
	@echo "Available commands:"
	@echo "  make setup      - Onboard fresh workspace, install python SDK in editable mode"
	@echo "  make test       - Run Go tests, Python packaged simulator & timing loops"
	@echo "  make build      - Compile Go concurrent execution daemon executable binary"
	@echo "  make dist       - Build distributable Python SDK packages (wheel and tarball)"
	@echo "  make docker-up  - Run all microservices in completely isolated Docker containers"
	@echo "  make clean      - Delete compile outputs, caching files, and speedup traces"
	@echo "======================================================================"

# Bootstrap environment
setup:
	@chmod +x scripts/*.sh
	@./scripts/setup.sh

# Run comprehensive sweeps across all subsystems
test:
	@echo "[TEST] Running concurrent Go scheduler tests..."
	@go test -v ./scheduler/...
	@echo ""
	@echo "[TEST] Running locality timing benchmark..."
	@./scripts/run_locality.sh
	@echo ""
	@echo "[TEST] Running parallel banked memory simulator..."
	@./scripts/run_simulator.sh

# Compile Go daemon core binary executable
build:
	@echo "[BUILD] Compiling Go scheduler executable daemon..."
	@mkdir -p build
	@go build -o build/orchid-daemon ./scheduler/...
	@echo "✓ Successfully compiled Go binary at: build/orchid-daemon"

# Build Python SDK distributable packages
dist:
	@echo "[DIST] Building Python wheel and source distribution packages..."
	@uv build
	@echo "✓ Successfully built package assets in dist/"

# Run compose-orchestrated isolated services
docker-up:
	@echo "[DOCKER] Building and launching multi-service container orchestration..."
	@docker compose up --build

# Clean transient files and build caches
clean:
	@echo "[CLEAN] Purging dynamic targets..."
	@rm -rf build/
	@rm -rf dist/
	@rm -rf locality/build/
	@rm -rf evidence/current/*
	@rm -rf evidence/reproduced/*
	@find . -type d -name "__pycache__" -exec rm -rf {} +
	@find . -type d -name "*.egg-info" -exec rm -rf {} +
	@find . -type d -name ".pytest_cache" -exec rm -rf {} +
	@echo "✓ Workspace successfully cleaned!"
