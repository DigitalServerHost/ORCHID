
.text
.globl matmul_locality
.type matmul_locality, @function
matmul_locality:
    pushq %r12
    xorl %r8d, %r8d
.Llocal_i:
    cmpl $512, %r8d
    jge .Llocal_done
    xorl %r9d, %r9d
.Llocal_k:
    cmpl $512, %r9d
    jge .Llocal_next_i
    movl %r8d, %eax
    imull $512, %eax
    addl %r9d, %eax
    movl (%rdi,%rax,4), %r11d
    xorl %r10d, %r10d
.Llocal_j:
    cmpl $512, %r10d
    jge .Llocal_next_k
    movl %r9d, %eax
    imull $512, %eax
    addl %r10d, %eax
    movl (%rsi,%rax,4), %r12d
    imull %r11d, %r12d
    movl %r8d, %eax
    imull $512, %eax
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
