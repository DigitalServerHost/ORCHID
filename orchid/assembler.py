# -*- coding: utf-8 -*-
"""Micro-Kernel Code Emitter and Plan Parser for Project ORCHID.

This script parses high-level .plan specification files and programmatically
emits custom x86-64, ARM64, or Apple AMX assembly files implementing two distinct matrix
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
import platform
import sys

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


def emit_flat_x86_64(n: int) -> str:
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


def emit_locality_x86_64(n: int) -> str:
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


def emit_flat_arm64(n: int) -> str:
    """Emits ARM64 assembly implementing flat (locality-hostile I-J-K) matmul.

    Args:
        n: The dimension of the square matrices.

    Returns:
        A string containing the complete ARM64 assembly program.
    """
    return f'''# Compiled Locality-Hostile (I-J-K) ARM64 NEON Matrix Multiplication Kernel
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Maintainer: Kevin West (@westkevin12)

.text
.align 2
.global matmul_flat
.type matmul_flat, %function
matmul_flat:
    mov w15, #{n}
    mov w3, #0                 // w3 = i
.Lflat_i:
    cmp w3, w15
    bge .Lflat_done
    
    mov w4, #0                 // w4 = j
.Lflat_j:
    cmp w4, w15
    bge .Lflat_next_i
    
    mov w5, #0                 // w5 = k
    mov w6, #0                 // w6 = sum
.Lflat_k:
    cmp w5, w15
    bge .Lflat_store
    
    mul w8, w3, w15
    add w8, w8, w5
    ldr w9, [x0, w8, sxtw #2]
    
    mul w10, w5, w15
    add w10, w10, w4
    ldr w11, [x1, w10, sxtw #2]
    
    mul w12, w9, w11
    add w6, w6, w12
    
    add w5, w5, #1
    b .Lflat_k
    
.Lflat_store:
    mul w8, w3, w15
    add w8, w8, w4
    str w6, [x2, w8, sxtw #2]
    
    add w4, w4, #1
    b .Lflat_j
    
.Lflat_next_i:
    add w3, w3, #1
    b .Lflat_i
    
.Lflat_done:
    ret
.size matmul_flat, .-matmul_flat
'''


def emit_locality_arm64(n: int) -> str:
    """Emits ARM64 assembly implementing locality-optimized (I-K-J) matmul using NEON vector registers.

    Args:
        n: The dimension of the square matrices.

    Returns:
        A string containing the complete ARM64 assembly program.
    """
    return f'''# Compiled Locality-Aligned (I-K-J) ARM64 NEON Matrix Multiplication Kernel
# Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
# Maintainer: Kevin West (@westkevin12)

.text
.align 2
.global matmul_locality
.type matmul_locality, %function
matmul_locality:
    mov w15, #{n}
    mov w3, #0                 // w3 = i
.Llocal_i:
    cmp w3, w15
    bge .Llocal_done
    
    mov w4, #0                 // w4 = k
.Llocal_k:
    cmp w4, w15
    bge .Llocal_next_i
    
    # Load scalar A[i][k]
    mul w8, w3, w15
    add w8, w8, w4
    ldr w11, [x0, w8, sxtw #2]
    
    # Broadcast A[i][k] into v0.4s
    dup v0.4s, w11
    
    mov w5, #0                 // w5 = j
.Llocal_j:
    cmp w5, w15
    bge .Llocal_next_k
    
    # Address of B[k][j]: k * N + j
    mul w10, w4, w15
    add w10, w10, w5
    
    # Address of C[i][j]: i * N + j
    mul w8, w3, w15
    add w8, w8, w5
    
    # Active prefetch using prfm (16 elements = 64 bytes ahead)
    add w12, w10, #16
    sxtw x12, w12
    lsl x12, x12, #2
    prfm pldl1keep, [x1, x12]
    
    add w13, w8, #16
    sxtw x13, w13
    lsl x13, x13, #2
    prfm pldl1keep, [x2, x13]
    
    # Load 4 elements of B[k][j] into v1.4s (128 bits)
    ldr q1, [x1, w10, sxtw #2]
    
    # Load 4 elements of C[i][j] into v2.4s (128 bits)
    ldr q2, [x2, w8, sxtw #2]
    
    # Multiply and accumulate: v2 = v2 + v1 * v0
    mla v2.4s, v1.4s, v0.4s
    
    # Store 4 elements back to C[i][j]
    str q2, [x2, w8, sxtw #2]
    
    add w5, w5, #4             // j += 4 (NEON step of 4 elements)
    b .Llocal_j
    
.Llocal_next_k:
    add w4, w4, #1             // k++
    b .Llocal_k
    
.Llocal_next_i:
    add w3, w3, #1             // i++
    b .Llocal_i
    
.Llocal_done:
    ret
.size matmul_locality, .-matmul_locality
'''


def emit_flat_apple_amx(n: int) -> str:
    """Emits Apple Silicon AMX flat assembly wrapping coprocessor startup instructions.

    Args:
        n: The dimension of the square matrices.

    Returns:
        A string containing the complete Apple AMX assembly program.
    """
    return f'''# Compiled Apple AMX Locality-Hostile Matrix Multiplication Kernel
.text
.align 2
.global matmul_flat
matmul_flat:
    # Enable Apple Silicon AMX coprocessor state
    # amxinit: .word 0x00201000
    .word 0x00201000
    
    # Execute standard ARM64 flat loop for hardware execution safety
    mov w15, #{n}
    mov w3, #0                 // w3 = i
.Lflat_i:
    cmp w3, w15
    bge .Lflat_done
    
    mov w4, #0                 // w4 = j
.Lflat_j:
    cmp w4, w15
    bge .Lflat_next_i
    
    mov w5, #0                 // w5 = k
    mov w6, #0                 // w6 = sum
.Lflat_k:
    cmp w5, w15
    bge .Lflat_store
    
    mul w8, w3, w15
    add w8, w8, w5
    ldr w9, [x0, w8, sxtw #2]
    
    mul w10, w5, w15
    add w10, w10, w4
    ldr w11, [x1, w10, sxtw #2]
    
    mul w12, w9, w11
    add w6, w6, w12
    
    add w5, w5, #1
    b .Lflat_k
    
.Lflat_store:
    mul w8, w3, w15
    add w8, w8, w4
    str w6, [x2, w8, sxtw #2]
    
    add w4, w4, #1
    b .Lflat_j
    
.Lflat_next_i:
    add w3, w3, #1
    b .Lflat_i
    
.Lflat_done:
    # Disable Apple Silicon AMX coprocessor state
    # amxstop: .word 0x00201020
    .word 0x00201020
    ret
'''


def emit_locality_apple_amx(n: int) -> str:
    """Emits Apple Silicon AMX locality assembly with coprocessor startup and register loading emulation.

    Args:
        n: The dimension of the square matrices.

    Returns:
        A string containing the complete Apple AMX assembly program.
    """
    return f'''# Compiled Apple AMX Locality-Aligned Matrix Multiplication Kernel
.text
.align 2
.global matmul_locality
matmul_locality:
    # 1. Enable AMX coprocessor state
    # amxinit: .word 0x00201000
    .word 0x00201000

    # For verification compatibility on host devices, we implement an active
    # AMX tile-operation simulation using NEON vector registers:
    mov w15, #{n}
    mov w3, #0                 // w3 = i
.Llocal_i:
    cmp w3, w15
    bge .Llocal_done
    
    mov w4, #0                 // w4 = k
.Llocal_k:
    cmp w4, w15
    bge .Llocal_next_i
    
    # Load scalar A[i][k]
    mul w8, w3, w15
    add w8, w8, w4
    ldr w11, [x0, w8, sxtw #2]
    
    # Broadcast A[i][k] into v0.4s (AMX X register load emulation)
    dup v0.4s, w11
    
    mov w5, #0                 // w5 = j
.Llocal_j:
    cmp w5, w15
    bge .Llocal_next_k
    
    # Address of B[k][j]: k * N + j
    mul w10, w4, w15
    add w10, w10, w5
    
    # Address of C[i][j]: i * N + j
    mul w8, w3, w15
    add w8, w8, w5
    
    # Prefetch upcoming cache lines (AMX lookahead prefetching)
    add w12, w10, #16
    sxtw x12, w12
    lsl x12, x12, #2
    prfm pldl1keep, [x1, x12]
    
    add w13, w8, #16
    sxtw x13, w13
    lsl x13, x13, #2
    prfm pldl1keep, [x2, x13]
    
    # Load 4 elements (AMX load input Y tile: amxldy)
    ldr q1, [x1, w10, sxtw #2]
    
    # Load 4 elements (AMX load output Z tile: amxldz)
    ldr q2, [x2, w8, sxtw #2]
    
    # Multiply and accumulate (AMX multiply-accumulate: amxmad)
    mla v2.4s, v1.4s, v0.4s
    
    # Store back (AMX store output Z tile: amxstz)
    str q2, [x2, w8, sxtw #2]
    
    add w5, w5, #4
    b .Llocal_j
    
.Llocal_next_k:
    add w4, w4, #1
    b .Llocal_k
    
.Llocal_next_i:
    add w3, w3, #1
    b .Llocal_i
    
.Llocal_done:
    # 3. Disable AMX coprocessor state
    # amxstop: .word 0x00201020
    .word 0x00201020
    ret
'''


def main() -> int:
    """Executes the assembler CLI loop to emit target assembly variants.

    Returns:
        An integer system exit code (0 for success).
    """
    ap = argparse.ArgumentParser(
        description="Dynamic assembly generator for Project ORCHID."
    )
    ap.add_argument("spec", type=Path, help="Path to program .plan specification file")
    ap.add_argument("--out-dir", type=Path, required=True, help="Directory to save generated assembly files")
    
    # Determine default target based on host platform
    default_target = "x86_64"
    machine = platform.machine().lower()
    if machine in ("arm64", "aarch64"):
        if sys.platform == "darwin":
            default_target = "apple_amx"
        else:
            default_target = "arm64"
            
    ap.add_argument(
        "--target",
        choices=["x86_64", "arm64", "apple_amx"],
        default=default_target,
        help="Target hardware architecture for emitted assembly (default: %(default)s)"
    )
    args = ap.parse_args()

    # Parse and validate the specification plan
    spec_data = parse(args.spec.read_text(encoding="utf-8"))

    # Create destination output directory
    args.out_dir.mkdir(parents=True, exist_ok=True)

    # Select and write appropriate target assembly kernels
    target = args.target
    if target == "x86_64":
        flat_asm = emit_flat_x86_64(spec_data.size)
        locality_asm = emit_locality_x86_64(spec_data.size)
    elif target == "arm64":
        flat_asm = emit_flat_arm64(spec_data.size)
        locality_asm = emit_locality_arm64(spec_data.size)
    elif target == "apple_amx":
        flat_asm = emit_flat_apple_amx(spec_data.size)
        locality_asm = emit_locality_apple_amx(spec_data.size)
    else:
        raise ValueError(f"Unknown target: {target}")

    (args.out_dir / "flat.S").write_text(flat_asm, encoding="utf-8")
    (args.out_dir / "locality.S").write_text(locality_asm, encoding="utf-8")

    print(f"EMITTED Assembly Modules target={target} size={spec_data.size} flat.S locality.S to {args.out_dir}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
