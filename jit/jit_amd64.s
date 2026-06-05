#include "textflag.h"

// func callJIT(codePtr, a, b, c unsafe.Pointer)
// System V AMD64 ABI expects arguments in: RDI, RSI, RDX
TEXT ·callJIT(SB), NOSPLIT, $0-32
    MOVQ codePtr+0(FP), AX
    MOVQ a+8(FP), DI
    MOVQ b+16(FP), SI
    MOVQ c+24(FP), DX
    CALL AX
    RET

// func cpuid(leaf, subleaf uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
    MOVL leaf+0(FP), AX
    MOVL subleaf+4(FP), CX
    CPUID
    MOVL AX, eax+8(FP)
    MOVL BX, ebx+12(FP)
    MOVL CX, ecx+16(FP)
    MOVL DX, edx+20(FP)
    RET
