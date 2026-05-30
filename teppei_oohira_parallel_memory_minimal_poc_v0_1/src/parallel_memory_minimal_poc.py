#!/usr/bin/env python3
"""
Parallel Memory Minimal Proof of Concept
Originator: Teppei Oohira / 大平鉄兵

Purpose:
    Compare the same logical STREAM-like triad workload under:
      1) a single serialized logical memory service;
      2) a two-bank role-separated parallel memory service;
      3) a three-bank upper-reference role-separated service;
      4) a two-bank design with all roles conflicted onto one bank.

Important boundary:
    This is a deterministic architectural scheduling proof, not a physical
    DRAM benchmark. Commodity DRAM already contains channels/banks and hides
    their operation behind controllers/caches. The PoC isolates the proposed
    control-plane difference: undifferentiated serial memory operation versus
    explicit role-separated parallel memory operation.
"""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import argparse
import csv
import json
import math
from typing import Dict, List, Tuple

@dataclass
class Event:
    role: str
    kind: str
    index: int
    bank: int
    earliest: int
    start: int
    end: int

class BankedMemoryScheduler:
    def __init__(self, bank_count: int, service_cycles: int, trace_limit: int = 24) -> None:
        if bank_count < 1 or service_cycles < 1:
            raise ValueError("bank_count and service_cycles must be >= 1")
        self.free_at = [0 for _ in range(bank_count)]
        self.service_cycles = service_cycles
        self.trace_limit = trace_limit
        self.trace: List[Event] = []
        self.requests = [0 for _ in range(bank_count)]

    def access(self, role: str, kind: str, index: int, bank: int, earliest: int = 0) -> int:
        if bank < 0 or bank >= len(self.free_at):
            raise ValueError(f"invalid bank: {bank}")
        start = max(earliest, self.free_at[bank])
        end = start + self.service_cycles
        self.free_at[bank] = end
        self.requests[bank] += 1
        if len(self.trace) < self.trace_limit:
            self.trace.append(Event(role, kind, index, bank, earliest, start, end))
        return end

    @property
    def cycles(self) -> int:
        return max(self.free_at, default=0)

@dataclass
class Result:
    name: str
    description: str
    banks: int
    mapping: Dict[str, int]
    cycles: int
    output_checksum: int
    output: List[int]
    requests: List[int]
    trace: List[Event]

def source_vectors(n: int) -> Tuple[List[int], List[int]]:
    b = [((i * 17 + 3) % 97) - 48 for i in range(n)]
    c = [((i * 29 + 11) % 89) - 44 for i in range(n)]
    return b, c

def run_triad(
    name: str,
    description: str,
    n: int,
    scalar: int,
    bank_count: int,
    mapping: Dict[str, int],
    service_cycles: int,
    compute_cycles: int,
    trace_limit: int,
) -> Result:
    b, c = source_vectors(n)
    a = [0 for _ in range(n)]
    memory = BankedMemoryScheduler(bank_count, service_cycles, trace_limit)

    for i in range(n):
        b_done = memory.access("B", "READ", i, mapping["B"])
        c_done = memory.access("C", "READ", i, mapping["C"])
        computed = max(b_done, c_done) + compute_cycles
        a[i] = b[i] + scalar * c[i]                 # same logical workload/output
        memory.access("A", "WRITE", i, mapping["A"], earliest=computed)

    checksum = sum((i + 1) * value for i, value in enumerate(a))
    return Result(name, description, bank_count, mapping, memory.cycles, checksum, a, memory.requests, memory.trace)

def main() -> int:
    ap = argparse.ArgumentParser(description="Deterministic serial-vs-parallel memory-operation PoC.")
    ap.add_argument("--n", type=int, default=16384, help="number of triad elements")
    ap.add_argument("--scalar", type=int, default=3)
    ap.add_argument("--service-cycles", type=int, default=100, help="memory service occupancy per request")
    ap.add_argument("--compute-cycles", type=int, default=1, help="triad arithmetic delay after both reads")
    ap.add_argument("--trace-limit", type=int, default=18)
    ap.add_argument("--out-dir", type=Path, default=Path("evidence/current"))
    args = ap.parse_args()
    if args.n < 1 or args.compute_cycles < 0:
        raise SystemExit("n must be >= 1 and compute-cycles must be >= 0")

    cases = [
        ("serial_single_memory",
         "One logical memory service; B read, C read and A write serialize.",
         1, {"B": 0, "C": 0, "A": 0}),
        ("parallel_two_memory_role_split",
         "Two independent services; B read and A write share bank 0, C read uses bank 1.",
         2, {"B": 0, "C": 1, "A": 0}),
        ("parallel_three_memory_role_split",
         "Three independent services; B read, C read and A write have distinct banks.",
         3, {"B": 0, "C": 1, "A": 2}),
        ("parallel_two_memory_conflicted_control",
         "Two banks exist but all data roles are placed on bank 0; negative control.",
         2, {"B": 0, "C": 0, "A": 0}),
    ]
    results = [
        run_triad(name, desc, args.n, args.scalar, bank_count, mapping,
                  args.service_cycles, args.compute_cycles, args.trace_limit)
        for name, desc, bank_count, mapping in cases
    ]
    baseline = results[0]
    for result in results[1:]:
        if result.output != baseline.output or result.output_checksum != baseline.output_checksum:
            raise AssertionError(f"output mismatch: {result.name}")

    args.out_dir.mkdir(parents=True, exist_ok=True)
    csv_path = args.out_dir / "results.csv"
    json_path = args.out_dir / "results.json"
    txt_path = args.out_dir / "summary.txt"
    trace_path = args.out_dir / "trace_first_events.csv"

    rows = []
    for result in results:
        speedup = baseline.cycles / result.cycles
        utilization = [
            req * args.service_cycles / result.cycles for req in result.requests
        ]
        rows.append({
            "case": result.name,
            "banks_available": result.banks,
            "mapping": json.dumps(result.mapping, sort_keys=True),
            "cycles": result.cycles,
            "speedup_vs_serial": round(speedup, 6),
            "requests_per_bank": json.dumps(result.requests),
            "utilization_per_bank": json.dumps([round(v, 6) for v in utilization]),
            "checksum": result.output_checksum,
        })

    with csv_path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
        writer.writeheader()
        writer.writerows(rows)

    json_path.write_text(json.dumps({
        "workload": {
            "formula": "A[i] = B[i] + scalar * C[i]",
            "n": args.n,
            "scalar": args.scalar,
            "memory_operations_per_element": ["READ B", "READ C", "WRITE A"],
            "service_cycles_per_memory_operation": args.service_cycles,
            "compute_cycles_after_reads": args.compute_cycles,
        },
        "baseline": baseline.name,
        "results": rows,
        "verification": {
            "same_logical_output": True,
            "checksum": baseline.output_checksum,
        },
    }, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    with trace_path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(["case", "role", "kind", "index", "bank", "earliest", "start", "end"])
        for result in results:
            for ev in result.trace:
                writer.writerow([result.name, ev.role, ev.kind, ev.index, ev.bank, ev.earliest, ev.start, ev.end])

    summary = [
        "PARALLEL MEMORY MINIMAL POC",
        f"workload=A[i]=B[i]+{args.scalar}*C[i] elements={args.n}",
        f"service_cycles={args.service_cycles} compute_cycles={args.compute_cycles}",
        f"VERIFY same_output checksum={baseline.output_checksum}",
        "",
        f"{'case':42} {'cycles':>14} {'speedup':>12} {'requests/bank':>20}",
    ]
    for row in rows:
        summary.append(
            f"{row['case']:42} {row['cycles']:14d} {row['speedup_vs_serial']:11.3f}x {row['requests_per_bank']:>20}"
        )
    summary += [
        "",
        "INTERPRETATION",
        "- serial_single_memory is the undifferentiated one-service baseline.",
        "- parallel_two_memory_role_split is the conservative proposed minimum: independent source access is parallelized while output still shares one bank.",
        "- parallel_three_memory_role_split shows the upper reference when input and output roles have independent services.",
        "- parallel_two_memory_conflicted_control proves that merely having multiple banks gives no benefit unless roles/requests are separated correctly.",
        "",
        "BOUNDARY",
        "- This is a deterministic architectural scheduling proof, not a physical DRAM benchmark.",
        "- Physical validation requires an implementation substrate exposing independent memory service paths or hardware/FPGA/simulator support.",
    ]
    txt_path.write_text("\n".join(summary) + "\n", encoding="utf-8")
    print("\n".join(summary))
    print(f"\nWROTE {csv_path}")
    print(f"WROTE {json_path}")
    print(f"WROTE {trace_path}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
