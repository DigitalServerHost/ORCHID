/**
 * @file jit_amd64.go
 * @brief AMD64 machine instruction emitter for flat and locality matrix multiplication.
 * 
 * License: GNU GPLv3
 */

package jit

import (
	"unsafe"
)

// callJIT is the external assembler routing stub.
func callJIT(codePtr, a, b, c unsafe.Pointer)

// cpuid is the Go-native assembly helper to query CPU capability flags.
func cpuid(leaf, subleaf uint32) (eax, ebx, ecx, edx uint32)

/**
 * @brief Checks if the host processor supports the AVX-512 foundation feature.
 * 
 * @return true if AVX-512 foundation is supported, false otherwise.
 */
func hasAVX512F() bool {
	eax, _, _, _ := cpuid(0, 0)
	if eax < 7 {
		return false
	}
	_, ebx, _, _ := cpuid(7, 0)
	return (ebx & (1 << 16)) != 0
}

/**
 * @brief Checks if the host processor supports the AVX2 vector instructions.
 * 
 * @return true if AVX2 is supported, false otherwise.
 */
func hasAVX2() bool {
	eax, _, _, _ := cpuid(0, 0)
	if eax < 7 {
		return false
	}
	_, ebx, _, _ := cpuid(7, 0)
	return (ebx & (1 << 5)) != 0
}

/**
 * @struct amd64Kernel
 * @brief Implements Kernel interface for memory-resident AMD64 machine code blocks.
 */
type amd64Kernel struct {
	code []byte ///< Slice holding the JIT-allocated and marked executable byte segment
}

/**
 * @brief Executes the JIT-compiled matrix multiplication kernel.
 * 
 * @param a Pointer to matrix A.
 * @param b Pointer to matrix B.
 * @param c Pointer to output matrix C.
 */
func (k *amd64Kernel) Execute(a, b, c unsafe.Pointer) {
	callJIT(unsafe.Pointer(&k.code[0]), a, b, c)
}

/**
 * @brief Deallocates the JIT-compiled executable memory block.
 * 
 * @return nil on success, or error if munmap failed.
 */
func (k *amd64Kernel) Free() error {
	if k.code == nil {
		return nil
	}
	err := munmapJIT(k.code)
	k.code = nil
	return err
}

/**
 * @brief Compiles flat matrix multiplication for target size n.
 * 
 * @param n Size of the matrix (N x N).
 * @return Compiled Kernel object or error.
 */
func CompileFlat(n int) (Kernel, error) {
	// Template for matmul_flat scalar
	template := []byte{
		0x45, 0x31, 0xc0,                         // 0: xor %r8d, %r8d
		0x41, 0x81, 0xf8, 0x00, 0x02, 0x00, 0x00, // 3: cmp $512, %r8d
		0x7d, 0x5e,                               // 10: jge .Ldone
		0x45, 0x31, 0xc9,                         // 12: xor %r9d, %r9d
		0x41, 0x81, 0xf9, 0x00, 0x02, 0x00, 0x00, // 15: cmp $512, %r9d
		0x7d, 0x4d,                               // 22: jge .Lnext_i
		0x45, 0x31, 0xd2,                         // 24: xor %r10d, %r10d
		0x31, 0xc9,                               // 27: xor %ecx, %ecx
		0x41, 0x81, 0xfa, 0x00, 0x02, 0x00, 0x00, // 29: cmp $512, %r10d
		0x7d, 0x2b,                               // 36: jge .Lstore
		0x44, 0x89, 0xc0,                         // 38: mov %r8d, %eax
		0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 41: imul $512, %eax, %eax
		0x44, 0x01, 0xd0,                         // 47: add %r10d, %eax
		0x44, 0x8b, 0x1c, 0x87,                   // 50: mov (%rdi,%rax,4), %r11d
		0x44, 0x89, 0xd0,                         // 54: mov %r10d, %eax
		0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 57: imul $512, %eax, %eax
		0x44, 0x01, 0xc8,                         // 63: add %r9d, %eax
		0x8b, 0x04, 0x86,                         // 66: mov (%rsi,%rax,4), %eax
		0x44, 0x0f, 0xaf, 0xd8,                   // 69: imul %eax, %r11d
		0x44, 0x01, 0xd9,                         // 73: add %r11d, %ecx
		0x41, 0xff, 0xc2,                         // 76: inc %r10d
		0xeb, 0xcc,                               // 79: jmp .L3
		0x44, 0x89, 0xc0,                         // 81: mov %r8d, %eax
		0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 84: imul $512, %eax, %eax
		0x44, 0x01, 0xc8,                         // 90: add %r9d, %eax
		0x89, 0x0c, 0x82,                         // 93: mov %ecx, (%rdx,%rax,4)
		0x41, 0xff, 0xc1,                         // 96: inc %r9d
		0xeb, 0xaa,                               // 99: jmp .L2
		0x41, 0xff, 0xc0,                         // 101: inc %r8d
		0xeb, 0x99,                               // 104: jmp .L1
		0xc3,                                     // 106: ret
	}

	code, err := mmapJIT(len(template))
	if err != nil {
		return nil, err
	}
	copy(code, template)

	val := uint32(n)
	writeUint32(code, 6, val)
	writeUint32(code, 18, val)
	writeUint32(code, 32, val)
	writeUint32(code, 43, val)
	writeUint32(code, 59, val)
	writeUint32(code, 86, val)

	err = mprotectRX(code)
	if err != nil {
		_ = munmapJIT(code)
		return nil, err
	}

	return &amd64Kernel{code: code}, nil
}

/**
 * @brief Compiles locality-optimized matrix multiplication for target size n.
 * 
 * Decides whether to compile to AVX-512, AVX2, or scalar assembly pathways
 * depending on runtime CPU detection.
 * 
 * @param n Size of the matrix (N x N).
 * @return Compiled Kernel object or error.
 */
func CompileLocality(n int) (Kernel, error) {
	if hasAVX512F() {
		// Emit vectorized AVX-512 kernel (16-way strides)
		template := []byte{
			0x45, 0x31, 0xc0,                         // 0: xor %r8d, %r8d
			0x41, 0x81, 0xf8, 0x00, 0x02, 0x00, 0x00, // 3: cmp $512, %r8d
			0x0f, 0x8d, 0x84, 0x00, 0x00, 0x00,       // 10: jge 94 <matmul_locality+0x94>
			0x45, 0x31, 0xc9,                         // 16: xor %r9d, %r9d
			0x41, 0x81, 0xf9, 0x00, 0x02, 0x00, 0x00, // 19: cmp $512, %r9d
			0x7d, 0x70,                               // 26: jge 8c <matmul_locality+0x8c>
			0x44, 0x89, 0xc0,                         // 28: mov %r8d, %eax
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 31: imul $512, %eax, %eax
			0x44, 0x01, 0xc8,                         // 37: add %r9d, %eax
			0x44, 0x8b, 0x1c, 0x87,                   // 40: mov (%rdi,%rax,4), %r11d
			0x62, 0xd2, 0x7d, 0x48, 0x7c, 0xc3,       // 44: vpbroadcastd %r11d, %zmm0
			0x45, 0x31, 0xd2,                         // 50: xor %r10d, %r10d
			0x41, 0x81, 0xfa, 0x00, 0x02, 0x00, 0x00, // 53: cmp $512, %r10d
			0x7d, 0x49,                               // 60: jge 87 <matmul_locality+0x87>
			0x44, 0x89, 0xc8,                         // 62: mov %r9d, %eax
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 65: imul $512, %eax, %eax
			0x44, 0x01, 0xd0,                         // 71: add %r10d, %eax
			0x0f, 0x18, 0x4c, 0x86, 0x40,             // 74: prefetcht0 0x40(%rsi,%rax,4)
			0x62, 0xf1, 0x7e, 0x48, 0x6f, 0x0c, 0x86, // 79: vmovdqu32 (%rsi,%rax,4), %zmm1
			0x62, 0xf2, 0x75, 0x48, 0x40, 0xc8,       // 86: vpmulld %zmm0, %zmm1, %zmm1
			0x44, 0x89, 0xc0,                         // 92: mov %r8d, %eax
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 95: imul $512, %eax, %eax
			0x44, 0x01, 0xd0,                         // 101: add %r10d, %eax
			0x0f, 0x18, 0x4c, 0x82, 0x40,             // 104: prefetcht0 0x40(%rdx,%rax,4)
			0x62, 0xf1, 0x7e, 0x48, 0x6f, 0x14, 0x82, // 109: vmovdqu32 (%rdx,%rax,4), %zmm2
			0x62, 0xf1, 0x6d, 0x48, 0xfe, 0xd1,       // 116: vpaddd %zmm1, %zmm2, %zmm2
			0x62, 0xf1, 0x7e, 0x48, 0x7f, 0x14, 0x82, // 122: vmovdqu32 %zmm2, (%rdx,%rax,4)
			0x41, 0x83, 0xc2, 0x10,                   // 129: add $16, %r10d
			0xeb, 0xae,                               // 133: jmp 35 <matmul_locality+0x35>
			0x41, 0xff, 0xc1,                         // 135: inc %r9d
			0xeb, 0x87,                               // 138: jmp 13 <matmul_locality+0x13>
			0x41, 0xff, 0xc0,                         // 140: inc %r8d
			0xe9, 0x6f, 0xff, 0xff, 0xff,             // 143: jmp 3 <matmul_locality+0x3>
			0xc3,                                     // 148: ret
		}

		code, err := mmapJIT(len(template))
		if err != nil {
			return nil, err
		}
		copy(code, template)

		val := uint32(n)
		writeUint32(code, 6, val)
		writeUint32(code, 22, val)
		writeUint32(code, 33, val)
		writeUint32(code, 56, val)
		writeUint32(code, 67, val)
		writeUint32(code, 97, val)

		err = mprotectRX(code)
		if err != nil {
			_ = munmapJIT(code)
			return nil, err
		}

		return &amd64Kernel{code: code}, nil
	} else if hasAVX2() {
		// Emit vectorized AVX2 kernel (8-way strides)
		template := []byte{
			0x45, 0x31, 0xc0,                         // 0: xor %r8d, %r8d
			0x41, 0x81, 0xf8, 0x00, 0x02, 0x00, 0x00, // 3: cmp $512, %r8d
			0x7d, 0x74,                               // 10: jge 80 <matmul_locality+0x80>
			0x45, 0x31, 0xc9,                         // 12: xor %r9d, %r9d
			0x41, 0x81, 0xf9, 0x00, 0x02, 0x00, 0x00, // 15: cmp $512, %r9d
			0x7d, 0x63,                               // 22: jge 7b <matmul_locality+0x7b>
			0x44, 0x89, 0xc0,                         // 24: mov %r8d, %eax
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 27: imul $512, %eax, %eax
			0x44, 0x01, 0xc8,                         // 33: add %r9d, %eax
			0xc4, 0xe2, 0x7d, 0x58, 0x04, 0x87,       // 36: vpbroadcastd (%rdi,%rax,4),%ymm0
			0x45, 0x31, 0xd2,                         // 42: xor %r10d, %r10d
			0x41, 0x81, 0xfa, 0x00, 0x02, 0x00, 0x00, // 45: cmp $512, %r10d
			0x7d, 0x40,                               // 52: jge 76 <matmul_locality+0x76>
			0x44, 0x89, 0xc8,                         // 54: mov %r9d, %eax
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 57: imul $512, %eax, %eax
			0x44, 0x01, 0xd0,                         // 63: add %r10d, %eax
			0x0f, 0x18, 0x4c, 0x86, 0x40,             // 66: prefetcht0 0x40(%rsi,%rax,4)
			0xc5, 0xfe, 0x6f, 0x0c, 0x86,             // 71: vmovdqu (%rsi,%rax,4), %ymm1
			0xc4, 0xe2, 0x75, 0x40, 0xc8,             // 76: vpmulld %ymm0, %ymm1, %ymm1
			0x44, 0x89, 0xc0,                         // 81: mov %r8d, %eax
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,       // 84: imul $512, %eax, %eax
			0x44, 0x01, 0xd0,                         // 90: add %r10d, %eax
			0x0f, 0x18, 0x4c, 0x82, 0x40,             // 93: prefetcht0 0x40(%rdx,%rax,4)
			0xc5, 0xfe, 0x6f, 0x14, 0x82,             // 98: vmovdqu (%rdx,%rax,4), %ymm2
			0xc5, 0xed, 0xfe, 0xd1,                   // 103: vpaddd %ymm1, %ymm2, %ymm2
			0xc5, 0xfe, 0x7f, 0x14, 0x82,             // 107: vmovdqu %ymm2, (%rdx,%rax,4)
			0x41, 0x83, 0xc2, 0x08,                   // 112: add $8, %r10d
			0xeb, 0xb7,                               // 116: jmp 2d <matmul_locality+0x2d>
			0x41, 0xff, 0xc1,                         // 118: inc %r9d
			0xeb, 0x94,                               // 121: jmp f <matmul_locality+0xf>
			0x41, 0xff, 0xc0,                         // 123: inc %r8d
			0xeb, 0x83,                               // 126: jmp 3 <matmul_locality+0x3>
			0xc3,                                     // 128: ret
		}

		code, err := mmapJIT(len(template))
		if err != nil {
			return nil, err
		}
		copy(code, template)

		val := uint32(n)
		writeUint32(code, 6, val)
		writeUint32(code, 18, val)
		writeUint32(code, 29, val)
		writeUint32(code, 48, val)
		writeUint32(code, 59, val)
		writeUint32(code, 86, val)

		err = mprotectRX(code)
		if err != nil {
			_ = munmapJIT(code)
			return nil, err
		}

		return &amd64Kernel{code: code}, nil
	} else {
		// Emit optimized scalar locality kernel
		template := []byte{
			0x45, 0x31, 0xc0,
			0x41, 0x81, 0xf8, 0x00, 0x02, 0x00, 0x00,
			0x7d, 0x7e,
			0x45, 0x31, 0xc9,
			0x41, 0x81, 0xf9, 0x00, 0x02, 0x00, 0x00,
			0x7d, 0x6a,
			0x44, 0x89, 0xc0,
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,
			0x44, 0x01, 0xc8,
			0x44, 0x8b, 0x1c, 0x87,
			0x45, 0x31, 0xd2,
			0x41, 0x81, 0xfa, 0x00, 0x02, 0x00, 0x00,
			0x7d, 0x49,
			0x44, 0x89, 0xc8,
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,
			0x44, 0x01, 0xd0,
			0x0f, 0x18, 0x4c, 0x86, 0x40,
			0x44, 0x89, 0xc1,
			0x69, 0xc9, 0x00, 0x02, 0x00, 0x00,
			0x44, 0x01, 0xd1,
			0x0f, 0x18, 0x4c, 0x8a, 0x40,
			0x44, 0x89, 0xc8,
			0x69, 0xc0, 0x00, 0x02, 0x00, 0x00,
			0x44, 0x01, 0xd0,
			0x8b, 0x04, 0x86,
			0x41, 0x0f, 0xaf, 0xc3,
			0x44, 0x89, 0xc1,
			0x69, 0xc9, 0x00, 0x02, 0x00, 0x00,
			0x44, 0x01, 0xd1,
			0x01, 0x04, 0x8a,
			0x41, 0xff, 0xc2,
			0xeb, 0xae,
			0x41, 0xff, 0xc1,
			0xeb, 0x8d,
			0x41, 0xff, 0xc0,
			0xe9, 0x79, 0xff, 0xff, 0xff,
			0xc3,
		}

		code, err := mmapJIT(len(template))
		if err != nil {
			return nil, err
		}
		copy(code, template)

		val := uint32(n)
		writeUint32(code, 6, val)
		writeUint32(code, 18, val)
		writeUint32(code, 29, val)
		writeUint32(code, 46, val)
		writeUint32(code, 57, val)
		writeUint32(code, 74, val)
		writeUint32(code, 91, val)
		writeUint32(code, 110, val)

		err = mprotectRX(code)
		if err != nil {
			_ = munmapJIT(code)
			return nil, err
		}

		return &amd64Kernel{code: code}, nil
	}
}

/**
 * @brief Helper to write a 32-bit unsigned integer in little-endian order to a byte slice.
 * 
 * @param code The destination byte slice.
 * @param index Starting index offset inside the byte slice.
 * @param val The 32-bit unsigned integer value to write.
 */
func writeUint32(code []byte, index int, val uint32) {
	code[index] = byte(val)
	code[index+1] = byte(val >> 8)
	code[index+2] = byte(val >> 16)
	code[index+3] = byte(val >> 24)
}
