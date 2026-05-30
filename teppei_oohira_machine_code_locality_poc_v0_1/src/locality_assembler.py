#!/usr/bin/env python3
from __future__ import annotations
from dataclasses import dataclass
from pathlib import Path
import argparse
import re

@dataclass(frozen=True)
class Spec:
    name: str
    size: int
    repeats: int

def parse(text: str) -> Spec:
    lines = [line.strip() for line in text.splitlines()
             if line.strip() and not line.strip().startswith("#")]
    expected = {
        "TYPE I32",
        "COMPUTE C <- A * B",
        "EMIT FLAT ORDER I J K",
        "EMIT LOCALITY ORDER I K J",
        "VERIFY EQUAL",
    }
    header = re.fullmatch(r"PROGRAM\s+([A-Za-z_][A-Za-z0-9_]*)", lines[0])
    if not header:
        raise ValueError("PROGRAM declaration missing")
    missing = expected.difference(set(lines))
    if missing:
        raise ValueError(f"required statements missing: {sorted(missing)}")
    size_line = next(line for line in lines if line.startswith("SIZE "))
    repeat_line = next(line for line in lines if line.startswith("BENCH "))
    size_match = re.fullmatch(r"SIZE\s+([0-9]+)", size_line)
    repeat_match = re.fullmatch(r"BENCH\s+REPEAT\s+([0-9]+)", repeat_line)
    if not size_match or not repeat_match:
        raise ValueError("SIZE or BENCH REPEAT syntax error")
    n, repeats = int(size_match.group(1)), int(repeat_match.group(1))
    if n < 2 or repeats < 1:
        raise ValueError("invalid SIZE or REPEAT")
    return Spec(header.group(1), n, repeats)

def emit_flat(n: int) -> str:
    return f'''
.text
.globl matmul_flat
.type matmul_flat, @function
matmul_flat:
    pushq %r12
    xorl %r8d, %r8d
.Lflat_i:
    cmpl ${n}, %r8d
    jge .Lflat_done
    xorl %r9d, %r9d
.Lflat_j:
    cmpl ${n}, %r9d
    jge .Lflat_next_i
    xorl %r10d, %r10d
    xorl %r11d, %r11d
.Lflat_k:
    cmpl ${n}, %r10d
    jge .Lflat_store
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r10d, %eax
    movl (%rdi,%rax,4), %r12d
    movl %r10d, %eax
    imull ${n}, %eax
    addl %r9d, %eax
    imull (%rsi,%rax,4), %r12d
    addl %r12d, %r11d
    incl %r10d
    jmp .Lflat_k
.Lflat_store:
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r9d, %eax
    movl %r11d, (%rdx,%rax,4)
    incl %r9d
    jmp .Lflat_j
.Lflat_next_i:
    incl %r8d
    jmp .Lflat_i
.Lflat_done:
    popq %r12
    ret
.size matmul_flat, .-matmul_flat
.section .note.GNU-stack,"",@progbits
'''

def emit_locality(n: int) -> str:
    return f'''
.text
.globl matmul_locality
.type matmul_locality, @function
matmul_locality:
    pushq %r12
    xorl %r8d, %r8d
.Llocal_i:
    cmpl ${n}, %r8d
    jge .Llocal_done
    xorl %r9d, %r9d
.Llocal_k:
    cmpl ${n}, %r9d
    jge .Llocal_next_i
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r9d, %eax
    movl (%rdi,%rax,4), %r11d
    xorl %r10d, %r10d
.Llocal_j:
    cmpl ${n}, %r10d
    jge .Llocal_next_k
    movl %r9d, %eax
    imull ${n}, %eax
    addl %r10d, %eax
    movl (%rsi,%rax,4), %r12d
    imull %r11d, %r12d
    movl %r8d, %eax
    imull ${n}, %eax
    addl %r10d, %eax
    addl %r12d, (%rdx,%rax,4)
    incl %r10d
    jmp .Llocal_j
.Llocal_next_k:
    incl %r9d
    jmp .Llocal_k
.Llocal_next_i:
    incl %r8d
    jmp .Llocal_i
.Llocal_done:
    popq %r12
    ret
.size matmul_locality, .-matmul_locality
.section .note.GNU-stack,"",@progbits
'''

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("spec", type=Path)
    ap.add_argument("--out-dir", type=Path, required=True)
    args = ap.parse_args()
    s = parse(args.spec.read_text(encoding="utf-8"))
    args.out_dir.mkdir(parents=True, exist_ok=True)
    (args.out_dir / "flat.S").write_text(emit_flat(s.size), encoding="utf-8")
    (args.out_dir / "locality.S").write_text(emit_locality(s.size), encoding="utf-8")
    print(f"EMITTED size={s.size} flat.S locality.S")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
