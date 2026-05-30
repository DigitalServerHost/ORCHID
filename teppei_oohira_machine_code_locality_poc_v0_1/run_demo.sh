#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD="$ROOT/build"
OUT="$ROOT/evidence/reproduced"
mkdir -p "$BUILD" "$OUT"

python3 "$ROOT/src/locality_assembler.py" "$ROOT/specs/matmul.plan" --out-dir "$BUILD"
gcc -O2 -std=c11 -Wall -Wextra -o "$BUILD/locality_benchmark_fair" \
  "$ROOT/tools/fair_harness.c" "$BUILD/flat.S" "$BUILD/locality.S"

"$BUILD/locality_benchmark_fair" | tee "$OUT/fair_timing_result_current_environment.txt"
python3 "$ROOT/tools/summarize_results.py" \
  "$OUT/fair_timing_result_current_environment.txt" \
  "$OUT/fair_summary_current_environment.txt"
