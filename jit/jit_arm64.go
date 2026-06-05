//go:build arm64
/**
 * @file jit_arm64.go
 * @brief ARM64 portable fallback routines for ORCHID matrix kernels.
 * 
 * License: GNU GPLv3
 */

package jit

/**
 * @brief Compiles flat matrix multiplication for target size n on ARM64.
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
 * @brief Compiles locality-optimized matrix multiplication for target size n on ARM64.
 * 
 * Falls back to Go reference model to maintain correctness.
 * 
 * @param n Size of the matrix (N x N).
 * @return Compiled Kernel fallback object or error.
 */
func CompileLocality(n int) (Kernel, error) {
	return &GoFallbackKernel{N: n, Locality: true}, nil
}
