#!/usr/bin/env bash
# ======================================================================
# 🌸 Project ORCHID: Locality-Aware CPU Cache Saturation Benchmark Runner
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Project Lead & Maintainer: Kevin West (@westkevin12)
# License: GNU GPLv3
# ======================================================================

set -euo pipefail

# Force execution target directory to the ORCHID repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

BUILD_DIR="$ROOT_DIR/locality/build"
EVIDENCE_DIR="$ROOT_DIR/evidence/reproduced"

echo "======================================================================"
echo " Starting Project ORCHID: Locality-Aware CPU Cache Saturation Benchmark"
echo " Originator: Teppei Oohira / 大平鉄兵"
echo " Maintainer: Kevin West / @westkevin12"
echo "======================================================================"
echo "Creating build and evidence directories..."
mkdir -p "$BUILD_DIR" "$EVIDENCE_DIR"

echo "----------------------------------------------------------------------"
echo "Step 1: Parsing matmul.plan and generating raw x86-64 assembly..."
python3 -m orchid.assembler "$ROOT_DIR/locality/matmul.plan" --out-dir "$BUILD_DIR"

echo "----------------------------------------------------------------------"
echo "Step 2: Compiling fair_harness.c and generated assembly kernels..."
gcc -O2 -std=c11 -Wall -Wextra -o "$BUILD_DIR/locality_benchmark_fair" \
  "$ROOT_DIR/locality/fair_harness.c" "$BUILD_DIR/flat.S" "$BUILD_DIR/locality.S"

echo "----------------------------------------------------------------------"
echo "Step 3: Running benchmark timing harness (alternating loop patterns)..."
"$BUILD_DIR/locality_benchmark_fair" | tee "$EVIDENCE_DIR/fair_timing_result_current_environment.txt"

echo "----------------------------------------------------------------------"
echo "Step 4: Executing results aggregator to calculate speedup statistics..."
python3 -m orchid.aggregator \
  "$EVIDENCE_DIR/fair_timing_result_current_environment.txt" \
  "$EVIDENCE_DIR/fair_summary_current_environment.txt"

echo "======================================================================"
echo " Benchmark Execution Complete!"
echo " Raw Results Saved To: $EVIDENCE_DIR/fair_timing_result_current_environment.txt"
echo " Summary Saved To:     $EVIDENCE_DIR/fair_summary_current_environment.txt"
echo "======================================================================"
