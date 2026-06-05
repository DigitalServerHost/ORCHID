/**
 * @file jit.go
 * @brief Memory management and W^X memory page allocation wrappers for ORCHID JIT compiler.
 * 
 * License: GNU GPLv3
 */

package jit

import (
	"fmt"
	"syscall"
	"unsafe"
)

/**
 * @interface Kernel
 * @brief Represents an executable JIT-compiled matrix multiplication block.
 */
type Kernel interface {
	// Execute dispatches the compiled block using pointers to input/output buffers.
	Execute(a, b, c unsafe.Pointer)
	// Free releases the allocated memory segment.
	Free() error
}

/**
 * @brief Allocates memory using syscall.Mmap with read-write protections.
 * 
 * @param size The size of the memory segment to allocate in bytes.
 * @return The allocated byte slice or an error.
 */
func mmapJIT(size int) ([]byte, error) {
	data, err := syscall.Mmap(
		-1,
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANON|syscall.MAP_PRIVATE,
	)
	if err != nil {
		return nil, fmt.Errorf("syscall mmap failed: %w", err)
	}
	return data, nil
}

/**
 * @brief Transitions the memory protections of a segment to read-execute.
 * 
 * @param data The byte slice representing the memory segment to protect.
 * @return nil on success, or error if syscall failed.
 */
func mprotectRX(data []byte) error {
	err := syscall.Mprotect(data, syscall.PROT_READ|syscall.PROT_EXEC)
	if err != nil {
		return fmt.Errorf("syscall mprotect RX failed: %w", err)
	}
	return nil
}

/**
 * @brief Frees memory allocated using syscall.Mmap.
 * 
 * @param data The byte slice to release.
 * @return nil on success, or error if munmap failed.
 */
func munmapJIT(data []byte) error {
	err := syscall.Munmap(data)
	if err != nil {
		return fmt.Errorf("syscall munmap failed: %w", err)
	}
	return nil
}

/**
 * @struct GoFallbackKernel
 * @brief Implements Kernel by executing a standard math calculation loop in Go.
 */
type GoFallbackKernel struct {
	N        int  ///< Size of the matrix (N x N)
	Locality bool ///< Flag indicating if locality-aware access loop order should be used
}

/**
 * @brief Releases the memory page for the fallback kernel (noop).
 * 
 * @return nil always.
 */
func (k *GoFallbackKernel) Free() error {
	return nil
}

/**
 * @brief Executes matrix multiplication using Go fallback loops.
 * 
 * @param a Pointer to matrix A.
 * @param b Pointer to matrix B.
 * @param c Pointer to output matrix C.
 */
func (k *GoFallbackKernel) Execute(a, b, c unsafe.Pointer) {
	n := k.N
	cells := n * n
	aSlice := (*[1 << 28]int32)(a)[:cells:cells]
	bSlice := (*[1 << 28]int32)(b)[:cells:cells]
	cSlice := (*[1 << 28]int32)(c)[:cells:cells]

	if k.Locality {
		for i := 0; i < n; i++ {
			for kv := 0; kv < n; kv++ {
				r := aSlice[i*n+kv]
				for j := 0; j < n; j++ {
					cSlice[i*n+j] += r * bSlice[kv*n+j]
				}
			}
		}
	} else {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				var sum int32
				for kv := 0; kv < n; kv++ {
					sum += aSlice[i*n+kv] * bSlice[kv*n+j]
				}
				cSlice[i*n+j] = sum
			}
		}
	}
}
