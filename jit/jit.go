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
 * @struct ExecutionMetadata
 * @brief Holds runtime details of a completed JIT kernel execution for auditing.
 */
type ExecutionMetadata struct {
	N        int            ///< Matrix size (N x N)
	APtr     unsafe.Pointer ///< Pointer to input matrix A
	BPtr     unsafe.Pointer ///< Pointer to input matrix B
	CPtr     unsafe.Pointer ///< Pointer to output matrix C
	Locality bool           ///< True if locality optimization was used
}

/**
 * @interface TraceHook
 * @brief Interface for registering execution tracing and verification auditing tools.
 */
type TraceHook interface {
	// OnExecute is invoked after a JIT kernel completes computation.
	OnExecute(meta ExecutionMetadata)
}

// Global active trace hook registration
var activeTraceHook TraceHook

/**
 * @brief Registers a global trace hook to capture JIT kernel execution details.
 * 
 * @param hook The TraceHook to register.
 */
func RegisterTraceHook(hook TraceHook) {
	activeTraceHook = hook
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
 * @brief Performs a locality-optimized matrix multiplication traversal in Go.
 * 
 * @param n Size of the matrix (N x N).
 * @param a Flat slice containing matrix A data.
 * @param b Flat slice containing matrix B data.
 * @param c Flat slice containing matrix C output data.
 */
func executeLocality(n int, a, b, c []int32) {
	for i := 0; i < n; i++ {
		for kv := 0; kv < n; kv++ {
			r := a[i*n+kv]
			for j := 0; j < n; j++ {
				c[i*n+j] += r * b[kv*n+j]
			}
		}
	}
}

/**
 * @brief Performs a flat triple-loop matrix multiplication traversal in Go.
 * 
 * @param n Size of the matrix (N x N).
 * @param a Flat slice containing matrix A data.
 * @param b Flat slice containing matrix B data.
 * @param c Flat slice containing matrix C output data.
 */
func executeFlat(n int, a, b, c []int32) {
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			var sum int32
			for kv := 0; kv < n; kv++ {
				sum += a[i*n+kv] * b[kv*n+j]
			}
			c[i*n+j] = sum
		}
	}
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
		executeLocality(n, aSlice, bSlice, cSlice)
	} else {
		executeFlat(n, aSlice, bSlice, cSlice)
	}

	if activeTraceHook != nil {
		activeTraceHook.OnExecute(ExecutionMetadata{
			N:        n,
			APtr:     a,
			BPtr:     b,
			CPtr:     c,
			Locality: k.Locality,
		})
	}
}
