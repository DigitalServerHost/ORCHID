//go:build !amd64 && !arm64
/**
 * @file jit_other.go
 * @brief Generic platform-independent fallback routines for ORCHID matrix kernels.
 * 
 * License: GNU GPLv3
 */

package jit

/**
 * @brief Compiles flat matrix multiplication for target size n on other platforms.
 * 
 * Falls back to Go reference model to maintain correctness.
 * 
 * @param n Size of the matrix (N x N).
 * @return Compiled Kernel fallback object or error.
 */
func CompileFlat(n int) (Kernel, error) {
	return &GoFallbackKernel{N: n, Locality: false}, nil
}

/**
 * @brief Compiles locality-optimized matrix multiplication for target size n on other platforms.
 * 
 * Falls back to Go reference model to maintain correctness.
 * 
 * @param n Size of the matrix (N x N).
 * @return Compiled Kernel fallback object or error.
 */
func CompileLocality(n int) (Kernel, error) {
	return &GoFallbackKernel{N: n, Locality: true}, nil
}
