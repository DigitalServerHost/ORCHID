# -*- coding: utf-8 -*-
"""Result Aggregator and Summary Generator for Project ORCHID Locality PoC.

This script parses timing logs emitted by the fair_harness benchmark runner,
calculates speedup metrics (minimum, median, maximum, mean), and writes a
structured summary output.

Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
Maintainer/Project Lead: Kevin West (@westkevin12)
License: GNU GPLv3
"""

from pathlib import Path
import re
import statistics
import sys


def parse_and_summarize(input_path: Path, output_path: Path) -> str:
    """Parses speedup values from benchmark log files and outputs a statistical summary.

    Args:
        input_path: Path to the raw fair_harness benchmark console output.
        output_path: Path where the statistical summary should be written.

    Returns:
        A multiline string summarizing the speedup statistics.

    Raises:
        ValueError: If no speedup samples are matched in the raw input file.
    """
    source = input_path.read_text(encoding="utf-8")
    
    # Extract speedup patterns (e.g., "speedup=2.245x") using regex
    values = [float(v) for v in re.findall(r"speedup=([0-9.]+)x", source)]
    
    if not values:
        raise ValueError(f"No speedup timing samples found in input file: {input_path}")

    # Calculate summary metrics
    summary = (
        f"speedup_min={min(values):.3f}x\n"
        f"speedup_median={statistics.median(values):.3f}x\n"
        f"speedup_max={max(values):.3f}x\n"
        f"speedup_mean={statistics.mean(values):.3f}x\n"
    )

    output_path.write_text(summary, encoding="utf-8")

    # Generate dynamic JSON endpoints for Shields.io dynamic badges
    import json
    json_path = output_path.parent / "speedups.json"
    json_path.write_text(
        json.dumps({
            "min": f"{min(values):.3f}x",
            "median": f"{statistics.median(values):.3f}x",
            "max": f"{max(values):.3f}x",
            "mean": f"{statistics.mean(values):.3f}x"
        }, indent=2),
        encoding="utf-8"
    )
    return summary


def main() -> int:
    """Main CLI entry point.

    Returns:
        System status code (0 for success).
    """
    if len(sys.argv) != 3:
        print("Usage: python -m orchid.aggregator <INPUT_LOG_PATH> <OUTPUT_SUMMARY_PATH>", file=sys.stderr)
        return 1

    input_file = Path(sys.argv[1])
    output_file = Path(sys.argv[2])

    try:
        results = parse_and_summarize(input_file, output_file)
        print(results, end="")
    except Exception as e:
        print(f"ERROR: Analysis failed: {e}", file=sys.stderr)
        return 2

    return 0


if __name__ == "__main__":
    sys.exit(main())
