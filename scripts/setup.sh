#!/usr/bin/env bash
# ======================================================================
# 🌸 Project ORCHID: First-Run Bootstrap & UV Environment Setup
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Project Lead & Maintainer: Kevin West (@westkevin12)
# License: GNU GPLv3
# ======================================================================

set -euo pipefail

# Force execution target directory to the ORCHID repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# Define text colors
COLOR_RESET="\033[0m"
COLOR_INFO="\033[1;34m"
COLOR_SUCCESS="\033[1;32m"
COLOR_WARNING="\033[1;33m"
COLOR_ERROR="\033[1;31m"

echo -e "${COLOR_INFO}======================================================================"
echo -e "         PROJECT ORCHID: DEVELOPER ONBOARDING BOOTSTRAPPER"
echo -e "======================================================================${COLOR_RESET}"

# Step 1: Detect uv installation
echo -e "${COLOR_INFO}[Step 1/4] Detecting Astral 'uv' package manager...${COLOR_RESET}"
if ! command -v uv &> /dev/null; then
    echo -e "${COLOR_WARNING}Astral 'uv' was not found on your system PATH.${COLOR_RESET}"
    echo -e "Please install 'uv' to ensure ultra-fast Python version management:"
    echo -e "  ${COLOR_SUCCESS}curl -LsSf https://astral.sh/uv/install.sh | sh${COLOR_RESET}"
    echo -e "Or install it via your system package manager (pipx, brew, etc.)."
    exit 1
fi

echo -e "${COLOR_SUCCESS}✓ Found 'uv' version: $(uv --version)${COLOR_RESET}"

# Step 2: Enforce/Install Python 3.10+
echo -e "\n${COLOR_INFO}[Step 2/4] Initializing Python >=3.10 runtime environment...${COLOR_RESET}"
uv python install 3.10

# Step 3: Create virtual environment and sync packaged SDK
echo -e "\n${COLOR_INFO}[Step 3/4] Creating virtual environment (.venv) and installing SDK package...${COLOR_RESET}"
if [ -d ".venv" ]; then
    echo -e "${COLOR_WARNING}Virtual environment '.venv' already exists. Synchronizing package...${COLOR_RESET}"
else
    uv venv --python 3.10
    echo -e "${COLOR_SUCCESS}✓ Created virtual environment using Python 3.10.${COLOR_RESET}"
fi

# Enable virtual environment inside shell execution context
export PATH="$ROOT_DIR/.venv/bin:$PATH"

# Install the Python SDK in editable development mode
echo -e "Installing local 'orchid' package in editable dev mode..."
uv pip install -e . --quiet
echo -e "${COLOR_SUCCESS}✓ Package sync complete!${COLOR_RESET}"

# Step 4: Verification run of micro-architectural PoCs
echo -e "\n${COLOR_INFO}[Step 4/4] Running diagnostic test sweeps to verify the workspace...${COLOR_RESET}"

# Run Go concurrent scheduler tests
if command -v go &> /dev/null; then
    echo -e "Running Go concurrent scheduler tests..."
    go test -v ./scheduler/...
    echo -e "${COLOR_SUCCESS}✓ Concurrent Go Scheduler verified successfully!${COLOR_RESET}"
else
    echo -e "${COLOR_WARNING}Go compiler not found. Skipping Go scheduler unit tests.${COLOR_RESET}"
fi

# Run CPU locality-aware memory timing harness
echo -e "\nRunning Locality Timing Harness..."
./scripts/run_locality.sh
echo -e "${COLOR_SUCCESS}✓ Locality Cache Benchmark verified successfully!${COLOR_RESET}"

# Run Parallel banked scheduling simulation
echo -e "\nRunning Parallel bank scheduling simulator..."
./scripts/run_simulator.sh
echo -e "${COLOR_SUCCESS}✓ Parallel Bank Simulation verified successfully!${COLOR_RESET}"

echo -e "\n${COLOR_SUCCESS}======================================================================"
echo -e " SUCCESS: Project ORCHID Developer Environment is lock-and-load ready!"
echo -e "======================================================================"
echo -e "To activate the virtual environment manually in your shell:"
echo -e "  ${COLOR_INFO}source .venv/bin/activate${COLOR_RESET}"
echo -e "\n${COLOR_INFO}Static Analysis & Linting (SonarQube):${COLOR_RESET}"
echo -e "1. Install the ${COLOR_SUCCESS}SonarLint${COLOR_RESET} extension in your IDE for real-time"
echo -e "   feedback on Go, Python, and C files."
echo -e "2. Run a SonarQube local analysis scan using the Sonar Scanner CLI:"
echo -e "   ${COLOR_SUCCESS}sonar-scanner -Dsonar.host.url=YOUR_SONAR_SERVER_URL${COLOR_RESET}"
echo -e "   (Configured properties loaded from ${COLOR_INFO}sonar-project.properties${COLOR_RESET})"
echo -e "======================================================================${COLOR_RESET}"
