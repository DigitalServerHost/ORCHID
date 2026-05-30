# Machine-Code / Memory-Operation Performance PoC — Evidence Package v0.1

**Originator:** Teppei Oohira / 大平鉄兵  
**Purpose:** Transferable reproduction package for Kevin's technical review.

## What this package proves

This package reproduces a narrow baseline observation:

> On the same x86-64 execution target and for the same integer matrix multiplication output, changing the execution ordering from a locality-hostile access pattern (`I-J-K`) to a locality-favorable pattern (`I-K-J`) can yield a large execution-time difference.

The original recorded fair-run evidence observed:

- minimum speedup: **1.653x**
- median speedup: **1.692x**
- maximum speedup: **1.723x**
- mean speedup: **1.691x**

## What this package does not prove

This package does **not** by itself prove:

- a new machine-code format;
- the memory-operation instruction model described in the concept report;
- a general performance result for all workloads;
- a universal 2x performance increase;
- a finished implementation or commercial product.

It is the first executable evidence layer: existing hardware already exhibits a substantial recoverable delta when execution and memory-access structure are changed.

## Relation to the larger concept

The primary concept is not an assembler-language proposal. The prototype source retains the historical filename `locality_assembler.py` only because it emits the two x86-64 comparison kernels.

The larger question is whether:

1. current machine-code execution formats fail to preserve useful operation and memory-role structure;
2. memory placement, movement, reuse and preparation can be elevated into explicit execution-control objects;
3. a redesigned machine-code format and a memory-operation model can compound recoverable performance gains;
4. because the existing recorded delta is 1.691x, reaching 2.0x requires only an additional ~18.3% over that reference point, subject to non-overlap verification.

## Requirements

- Linux on x86-64
- Python 3.10+
- GCC

## Run

```bash
chmod +x run_demo.sh
./run_demo.sh
```

Generated outputs:

```text
build/flat.S
build/locality.S
build/locality_benchmark_fair
evidence/reproduced/fair_timing_result_current_environment.txt
evidence/reproduced/fair_summary_current_environment.txt
```

## Measurement design

- Same mathematical output is verified before timing.
- The benchmark alternates `flat-first` and `locality-first`.
- A 64 MiB buffer is traversed between timed executions to reduce cache-state bias.
- Eight paired timing runs are recorded.
- A serious next evaluation should additionally collect PMU/perf counters and compare against optimized compiler or library baselines.

## Next technical gate

Construct a controlled four-way comparison:

| Condition | Machine-code / execution-format redesign | Memory-operation redesign |
|---|---:|---:|
| Baseline | No | No |
| Format-only | Yes | No |
| Memory-only | No | Yes |
| Combined | Yes | Yes |

The hypothesis survives only if the `Combined` path produces an additional, reproducible and mechanistically explainable performance delta beyond the current locality-only evidence.
