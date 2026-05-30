#!/usr/bin/env bash
# ======================================================================
# 🌸 Project ORCHID: Parallel Multi-Memory Scheduling Simulation Runner
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Project Lead & Maintainer: Kevin West (@westkevin12)
# License: GNU GPLv3
# ======================================================================

set -euo pipefail

# Force execution target directory to the ORCHID repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

EVIDENCE_DIR="$ROOT_DIR/evidence/current"

echo "======================================================================"
echo " Starting Project ORCHID: Parallel Multi-Memory Scheduling Simulation"
echo " Originator: Teppei Oohira / 大平鉄兵"
echo " Maintainer: Kevin West / @westkevin12"
echo "======================================================================"
echo "Creating unified evidence directory..."
mkdir -p "$EVIDENCE_DIR"

echo "Running memory bank role-mapping simulator..."
python3 -m orchid.simulator --out-dir "$EVIDENCE_DIR"

echo "======================================================================"
echo " Simulation Complete!"
echo " Results Saved to: $EVIDENCE_DIR/"
echo "======================================================================"
