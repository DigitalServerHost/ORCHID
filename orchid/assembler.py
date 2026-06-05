# -*- coding: utf-8 -*-
"""Micro-Kernel Code Emitter and Plan Parser for Project ORCHID.

This script parses high-level .plan specification files and programmatically
emits custom x86-64 assembly files implementing two distinct matrix
multiplication layouts: flat (locality-hostile I-J-K) and locality-aligned (I-K-J).

Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
Maintainer/Project Lead: Kevin West (@westkevin12)
License: GNU GPLv3
"""

from __future__ import annotations
from dataclasses import dataclass
from pathlib import Path
import argparse
import re

@dataclass(frozen=True)
class Spec:
    """Represents the benchmark specifications parsed from a .plan file.

    Attributes:
        name: The identifier of the benchmark program.
        size: The matrix size (dimension N of the square N x N matrices).
        repeats: The number of benchmark timings to execute.
    """
    name: str
    size: int
    repeats: int


def parse(text: str) -> Spec:
    """Parses execution specifications from plan file contents.

    Args:
        text: Raw text content from the spec .plan file.

    Returns:
        A validated Spec configuration instance.

    Raises:
        ValueError: If mandatory statements are missing, syntax errors exist,
          or specifications contain invalid values.
    """
    # Normalize lines by stripping whitespace and filtering comments/blanks
    lines = [
        line.strip() for line in text.splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]

    expected_statements = {
        "TYPE I32",
        "COMPUTE C <- A * B",
        "EMIT FLAT ORDER I J K",
        "EMIT LOCALITY ORDER I K J",
        "VERIFY EQUAL",
    }

    # Verify that the file starts with the program declaration header
    header = re.fullmatch(r"PROGRAM\s+([A-Za-z_]\w*)", lines[0])
    if not header:
        raise ValueError("PROGRAM declaration missing or syntactically incorrect")

    # Check for presence of all required statements
    missing = expected_statements.difference(set(lines))
    if missing:
        raise ValueError(f"Required statements missing from spec: {sorted(missing)}")

    # Extract SIZE and BENCH parameters
    size_line = next(line for line in lines if line.startswith("SIZE "))
    repeat_line = next(line for line in lines if line.startswith("BENCH "))

    size_match = re.fullmatch(r"SIZE\s+(\d+)", size_line)
    repeat_match = re.fullmatch(r"BENCH\s+REPEAT\s+(\d+)", repeat_line)

    if not size_match or not repeat_match:
        raise ValueError("SIZE or BENCH REPEAT parameters contain syntax errors")

    n = int(size_match.group(1))
    repeats = int(repeat_match.group(1))

    if n < 2 or repeats < 1:
        raise ValueError("Matrix SIZE must be >= 2 and BENCH REPEAT must be >= 1")

    return Spec(header.group(1), n, repeats)


def emit_flat(n: int) -> str:
    """Emits x86-64 assembly implementing flat (locality-hostile I-J-K) matmul.

    This routine performs standard textbook matrix multiplication where the inner
    loop iterates over index K. The memory stride for Matrix B jumps by an entire
    row on every iteration, leading to frequent cache misses.

    Args:
        n: The dimension of the square matrices.

    Returns:
        A string containing the complete x86-64 assembly program.
    """
    return f'''# Compiled Locality-Hostile (I-J-K) Matrix Multiplication Kernel
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Maintainer: Kevin West (@westkevin12)

.text
.globl matmul_flat
.type matmul_flat, @function
matmul_flat:
    pushq %r12                  # Preserve callee-saved register %r12

    xorl %r8d, %r8d             # %r8d = i (outer loop index)
.Lflat_i:
    cmpl ${n}, %r8d
    jge .Lflat_done             # If i >= n, exit routine

    xorl %r9d, %r9d             # %r9d = j (middle loop index)
.Lflat_j:
    cmpl ${n}, %r9d
    jge .Lflat_next_i           # If j >= n, increment i

    xorl %r10d, %r10d           # %r10d = k (inner loop index)
    xorl %r11d, %r11d           # %r11d = accumulator (sum)
.Lflat_k:
    cmpl ${n}, %r10d
    jge .Lflat_store            # If k >= n, store accumulator in C

    # Address calculation: A[i][k] -> %rax = (i * n + k)
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r10d, %eax
    movl (%rdi,%rax,4), %r12d   # Load A[i][k] value into %r12d

    # Address calculation: B[k][j] -> %rax = (k * n + j) (Strided cache miss target)
    movl %r10d, %eax
    imull ${n}, %eax
    addl %r9d, %eax
    imull (%rsi,%rax,4), %r12d  # %r12d = A[i][k] * B[k][j]

    addl %r12d, %r11d           # Accumulate product into %r11d
    incl %r10d                  # Increment k
    jmp .Lflat_k

.Lflat_store:
    # Address calculation: C[i][j] -> %rax = (i * n + j)
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r9d, %eax
    movl %r11d, (%rdx,%rax,4)   # Store result sum to C[i][j]

    incl %r9d                   # Increment j
    jmp .Lflat_j

.Lflat_next_i:
    incl %r8d                   # Increment i
    jmp .Lflat_i

.Lflat_done:
    popq %r12                   # Restore register %r12
    ret
.size matmul_flat, .-matmul_flat
.section .note.GNU-stack,"",@progbits
'''


def emit_locality(n: int) -> str:
    """Emits x86-64 assembly implementing AVX-512 locality-optimized (I-K-J) matmul.

    This routine performs loop-ordered matrix multiplication where the inner
    loop iterates over index J in strides of 16 using AVX-512 register sets.
    Contiguous memory streams from B are loaded into %zmm registers, multiplied by
    the broadcasted scalar of A, and accumulated directly into C.

    Args:
        n: The dimension of the square matrices.

    Returns:
        A string containing the complete x86-64 assembly program.
    """
    return f'''# Compiled Locality-Aligned (I-K-J) AVX-512 Vector Matrix Multiplication Kernel
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Maintainer: Kevin West (@westkevin12)

.text
.globl matmul_locality
.type matmul_locality, @function
matmul_locality:
    pushq %r12                  # Preserve callee-saved register %r12

    xorl %r8d, %r8d             # %r8d = i (outer loop index)
.Llocal_i:
    cmpl ${n}, %r8d
    jge .Llocal_done            # If i >= n, exit routine

    xorl %r9d, %r9d             # %r9d = k (middle loop index)
.Llocal_k:
    cmpl ${n}, %r9d
    jge .Llocal_next_i          # If k >= n, increment i

    # Cache load: A[i][k] is constant for the entire inner loop
    # Address calculation: A[i][k] -> %rax = (i * n + k)
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r9d, %eax
    movl (%rdi,%rax,4), %r11d   # Load constant scalar A[i][k] into %r11d

    # Broadcast scalar A[i][k] from %r11d into AVX-512 register %zmm0
    vpbroadcastd %r11d, %zmm0

    xorl %r10d, %r10d           # %r10d = j (inner loop index)
.Llocal_j:
    cmpl ${n}, %r10d
    jge .Llocal_next_k          # If j >= n, increment k

    # Contiguous Address calculation: B[k][j] -> %rax = (k * n + j)
    movl %r9d, %eax
    imull ${n}, %eax
    addl %r10d, %eax
    
    # Active prefetch of upcoming Matrix B cache line (16 elements = 64 bytes ahead)
    prefetcht0 64(%rsi,%rax,4)
    
    # Load 16 dense 32-bit integers from B[k][j] into %zmm1
    vmovdqu32 (%rsi,%rax,4), %zmm1

    # Multiply B[k][j] by broadcasted A[i][k] -> %zmm1 = %zmm1 * %zmm0
    vpmulld %zmm0, %zmm1, %zmm1

    # Contiguous Address calculation: C[i][j] -> %rax = (i * n + j)
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r10d, %eax
    
    # Active prefetch of upcoming Matrix C cache line (16 elements = 64 bytes ahead)
    prefetcht0 64(%rdx,%rax,4)
    
    # Load 16 dense 32-bit integers from C[i][j] into %zmm2
    vmovdqu32 (%rdx,%rax,4), %zmm2

    # Accumulate: C[i][j] += A[i][k] * B[k][j]
    vpaddd %zmm1, %zmm2, %zmm2

    # Store 16 elements back to C[i][j]
    vmovdqu32 %zmm2, (%rdx,%rax,4)

    addl $16, %r10d             # Increment j by 16 (linear forward step of 16 elements)
    jmp .Llocal_j

.Llocal_next_k:
    incl %r9d                   # Increment k
    jmp .Llocal_k

.Llocal_next_i:
    incl %r8d                   # Increment i
    jmp .Llocal_i

.Llocal_done:
    popq %r12                   # Restore register %r12
    ret
.size matmul_locality, .-matmul_locality
.section .note.GNU-stack,"",@progbits
'''


def main() -> int:
    """Executes the assembler CLI loop to emit both assembly variants.

    Returns:
        An integer system exit code (0 for success).
    """
    ap = argparse.ArgumentParser(
        description="Dynamic x86-64 assembly generator for Project ORCHID."
    )
    ap.add_argument("spec", type=Path, help="Path to program .plan specification file")
    ap.add_argument("--out-dir", type=Path, required=True, help="Directory to save generated assembly files")
    args = ap.parse_args()

    # Parse and validate the specification plan
    spec_data = parse(args.spec.read_text(encoding="utf-8"))

    # Create destination output directory
    args.out_dir.mkdir(parents=True, exist_ok=True)

    # Write assembly kernels
    (args.out_dir / "flat.S").write_text(emit_flat(spec_data.size), encoding="utf-8")
    (args.out_dir / "locality.S").write_text(emit_locality(spec_data.size), encoding="utf-8")

    print(f"EMITTED Assembly Modules size={spec_data.size} flat.S locality.S to {args.out_dir}")
    return 0


if __name__ == "__main__":
    import sys
    sys.exit(main())
