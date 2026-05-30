# -*- coding: utf-8 -*-
"""Parallel Multi-Memory Scheduler Simulator for Project ORCHID.

This script simulates the scheduling behavior of STREAM-Triad memory kernels
(A[i] = B[i] + scalar * C[i]) under different memory bank role mappings.
It deterministically demonstrates the latency delta between serialized single-service
memory models and role-separated parallel memory models under identical logical work.

Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
Maintainer/Project Lead: Kevin West (@westkevin12)
License: GNU GPLv3
"""

from __future__ import annotations
from dataclasses import dataclass
from pathlib import Path
import argparse
import csv
import json
from typing import Dict, List, Tuple

@dataclass
class Event:
    """Represents a scheduled memory access event.

    Attributes:
        role: The memory stream role ('A', 'B', or 'C').
        kind: The operation type ('READ' or 'WRITE').
        index: The vector index being processed.
        bank: The mapped physical memory bank ID.
        earliest: The earliest cycle this operation could logically start.
        start: The actual scheduled start cycle.
        end: The cycle when the operation successfully completes.
    """
    role: str
    kind: str
    index: int
    bank: int
    earliest: int
    start: int
    end: int


class BankedMemoryScheduler:
    """Simulates a physical memory bus controller with multiple independent banks.

    Enforces that each bank can only service a single memory request at a time
    (serialized access per bank, parallel access across distinct banks).
    """

    def __init__(self, bank_count: int, service_cycles: int, trace_limit: int = 24) -> None:
        """Initializes the memory scheduler.

        Args:
            bank_count: The number of simulated physical memory banks.
            service_cycles: The latency penalty in cycles to complete one access.
            trace_limit: Max number of events to record in the trace.

        Raises:
            ValueError: If bank_count or service_cycles is < 1.
        """
        if bank_count < 1 or service_cycles < 1:
            raise ValueError("bank_count and service_cycles must be >= 1")
        self.free_at = [0 for _ in range(bank_count)]
        self.service_cycles = service_cycles
        self.trace_limit = trace_limit
        self.trace: List[Event] = []
        self.requests = [0 for _ in range(bank_count)]

    def access(self, role: str, kind: str, index: int, bank: int, earliest: int = 0) -> int:
        """Schedules a memory access to a specific bank.

        Calculates the start cycle by taking the max of when the data is logically
        ready (earliest) and when the target memory bank becomes free.

        Args:
            role: Memory stream role ('A', 'B', or 'C').
            kind: Operation type ('READ' or 'WRITE').
            index: Element array index.
            bank: Target bank index to request from.
            earliest: Pre-requisite completion cycle (default 0).

        Returns:
            The completion cycle of the memory operation.

        Raises:
            ValueError: If the requested bank ID is out of range.
        """
        if bank < 0 or bank >= len(self.free_at):
            raise ValueError(f"Invalid bank: {bank}")
        
        # Calculate optimal start time based on bank availability and earliest ready constraint
        start = max(earliest, self.free_at[bank])
        end = start + self.service_cycles
        
        # Mark bank as busy until end cycle
        self.free_at[bank] = end
        self.requests[bank] += 1
        
        if len(self.trace) < self.trace_limit:
            self.trace.append(Event(role, kind, index, bank, earliest, start, end))
            
        return end

    @property
    def cycles(self) -> int:
        """Returns the total elapsed cycles to complete all scheduled operations."""
        return max(self.free_at, default=0)


@dataclass
class Result:
    """Holds the complete execution results of a simulated STREAM-Triad case.

    Attributes:
        name: Name identifier of the execution case.
        description: Informative description of the case goals.
        banks: The count of available memory banks.
        mapping: Dictionary mapping roles 'A', 'B', and 'C' to bank IDs.
        cycles: Total cycles elapsed for the whole vector.
        output_checksum: Mathematical checksum of vector calculations.
        output: The resulting computed array.
        requests: The count of total requests serviced per bank.
        trace: A subset list of scheduled access events.
    """
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
    """Generates deterministic pseudo-random input streams B and C.

    Args:
        n: The length of the vectors.

    Returns:
        A tuple of integer arrays (B, C).
    """
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
    """Executes a full STREAM-Triad memory scheduling simulation run.

    Args:
        name: Unique name of the simulation case.
        description: Description of the mapping case.
        n: Number of elements in the vector.
        scalar: Constant multiply scaling factor.
        bank_count: Total physical memory banks available.
        mapping: Dict mapping roles 'A', 'B', and 'C' to bank IDs.
        service_cycles: Latency cycles per memory access request.
        compute_cycles: Arithmetic computation delay cycles.
        trace_limit: Trace event limit.

    Returns:
        A validated Result object.
    """
    b, c = source_vectors(n)
    a = [0 for _ in range(n)]
    memory = BankedMemoryScheduler(bank_count, service_cycles, trace_limit)

    for i in range(n):
        # Parallel-eligible read requests
        b_done = memory.access("B", "READ", i, mapping["B"])
        c_done = memory.access("C", "READ", i, mapping["C"])
        
        # Computation can only start when both input streams are fully read
        computed = max(b_done, c_done) + compute_cycles
        
        a[i] = b[i] + scalar * c[i]                 # Pure logical math work
        
        # Write results back to memory substrate (cannot start until computed)
        memory.access("A", "WRITE", i, mapping["A"], earliest=computed)

    # Compute deterministic validation checksum
    checksum = sum((i + 1) * value for i, value in enumerate(a))
    return Result(
        name, description, bank_count, mapping, memory.cycles,
        checksum, a, memory.requests, memory.trace
    )


def main() -> int:
    """Main CLI entry point for the parallel memory scheduling simulator."""
    ap = argparse.ArgumentParser(
        description="Deterministic serial-vs-parallel memory-operation PoC for Project ORCHID."
    )
    ap.add_argument("--n", type=int, default=16384, help="number of triad elements")
    ap.add_argument("--scalar", type=int, default=3, help="STREAM-Triad scalar value")
    ap.add_argument("--service-cycles", type=int, default=100, help="memory latency occupancy cycles")
    ap.add_argument("--compute-cycles", type=int, default=1, help="triad ALU arithmetic delay")
    ap.add_argument("--trace-limit", type=int, default=18, help="limit of events to trace")
    ap.add_argument("--out-dir", type=Path, default=Path("evidence/current"), help="directory for evidence files")
    args = ap.parse_args()

    if args.n < 1 or args.compute_cycles < 0:
        raise SystemExit("ERROR: N must be >= 1 and compute-cycles must be >= 0")

    # Define the simulation cases testing the control plane hypothesis
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

    # Verify arithmetic correctness across all mapping variants
    baseline = results[0]
    for result in results[1:]:
        if result.output != baseline.output or result.output_checksum != baseline.output_checksum:
            raise AssertionError(f"ERROR: Logical calculation mismatch in case: {result.name}")

    # Generate results summaries
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

    # Write data outputs
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

    # Construct and print text execution summary
    summary = [
        "======================================================================",
        "           PROJECT ORCHID: PARALLEL MEMORY MINIMAL POC",
        "           Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵",
        "           Maintainer: Kevin West (@westkevin12)",
        "======================================================================",
        f"workload: A[i] = B[i] + {args.scalar} * C[i] | total_elements={args.n}",
        f"latency_cycles: service={args.service_cycles} | compute={args.compute_cycles}",
        f"verification: identical outputs validated | checksum={baseline.output_checksum}",
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
        "======================================================================",
    ]
    
    txt_path.write_text("\n".join(summary) + "\n", encoding="utf-8")
    print("\n".join(summary))
    print(f"\nWROTE {csv_path}")
    print(f"WROTE {json_path}")
    print(f"WROTE {trace_path}")
    return 0


if __name__ == "__main__":
    import sys
    sys.exit(main())
