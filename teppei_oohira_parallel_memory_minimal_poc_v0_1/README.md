# Parallel Multi-Memory Minimal PoC v0.1

**Originator:** Teppei Oohira / 大平鉄兵  
**Purpose:** Deterministically expose the scheduling delta between an undifferentiated serialized memory-operation model and an explicitly role-separated parallel multi-memory model under identical logical work and verified identical output.

## Boundary

Commodity DRAM already contains channels and banks capable of parallel activity. This PoC does **not** claim that current physical DRAM is literally a single serial memory bank.

The target of this PoC is the control model:

- `serial_single_memory`: all logical reads and writes are serialized through one undifferentiated memory service.
- `parallel_two_memory_role_split`: independent memory roles are explicitly separated, so independent source reads can progress through distinct services.

The question is not whether multiple banks can exist. The question is whether making memory-role separation an explicit execution-control object removes otherwise serialized waiting.

## Workload

A STREAM-Triad-like memory-dominant kernel:

```text
A[i] = B[i] + scalar * C[i]
```

Three memory actions are required per element:

```text
READ B[i]
READ C[i]
WRITE A[i]
```

## Cases

| Case | Services | Mapping | Purpose |
|---|---:|---|---|
| `serial_single_memory` | 1 | `B,C,A -> bank0` | Serialized undifferentiated baseline |
| `parallel_two_memory_role_split` | 2 | `B,A -> bank0`, `C -> bank1` | Conservative role-separated minimum, targeting ~1.5x |
| `parallel_three_memory_role_split` | 3 | `B -> bank0`, `C -> bank1`, `A -> bank2` | Upper reference with fully independent roles |
| `parallel_two_memory_conflicted_control` | 2 | `B,C,A -> bank0` | Negative control: multiple banks alone are not sufficient |

## Run

```bash
python3 src/parallel_memory_minimal_poc.py --out-dir evidence/current
```

This is an architecture scheduling proof, not a physical DRAM benchmark. Physical validation is a subsequent gate.
