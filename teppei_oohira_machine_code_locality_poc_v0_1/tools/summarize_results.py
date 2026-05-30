#!/usr/bin/env python3
from pathlib import Path
import statistics, re, sys

if len(sys.argv) != 3:
    raise SystemExit("usage: summarize_results.py INPUT OUTPUT")
source = Path(sys.argv[1]).read_text(encoding="utf-8")
values = [float(v) for v in re.findall(r"speedup=([0-9.]+)x", source)]
if not values:
    raise SystemExit("no speedup samples found")
result = (
    f"speedup_min={min(values):.3f}x\n"
    f"speedup_median={statistics.median(values):.3f}x\n"
    f"speedup_max={max(values):.3f}x\n"
    f"speedup_mean={statistics.mean(values):.3f}x\n"
)
Path(sys.argv[2]).write_text(result, encoding="utf-8")
print(result, end="")
