#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 "$ROOT/src/parallel_memory_minimal_poc.py" --out-dir "$ROOT/evidence/current"
