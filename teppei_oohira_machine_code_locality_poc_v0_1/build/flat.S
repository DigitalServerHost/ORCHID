
.text
.globl matmul_flat
.type matmul_flat, @function
matmul_flat:
    pushq %r12
    xorl %r8d, %r8d
.Lflat_i:
    cmpl $512, %r8d
    jge .Lflat_done
    xorl %r9d, %r9d
.Lflat_j:
    cmpl $512, %r9d
    jge .Lflat_next_i
    xorl %r10d, %r10d
    xorl %r11d, %r11d
.Lflat_k:
    cmpl $512, %r10d
    jge .Lflat_store
    movl %r8d, %eax
    imull $512, %eax
    addl %r10d, %eax
    movl (%rdi,%rax,4), %r12d
    movl %r10d, %eax
    imull $512, %eax
    addl %r9d, %eax
    imull (%rsi,%rax,4), %r12d
    addl %r12d, %r11d
    incl %r10d
    jmp .Lflat_k
.Lflat_store:
    movl %r8d, %eax
    imull $512, %eax
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
