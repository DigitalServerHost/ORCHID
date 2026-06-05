/**
 * @file jit_test.go
 * @brief Correctness and latency benchmarks for ORCHID JIT compiler.
 * 
 * License: GNU GPLv3
 */

package jit

import (
	"math/rand"
	"testing"
	"time"
	"unsafe"
)

/**
 * @brief Generates random test matrices A and B, and allocates buffer C.
 * 
 * @param n Size of the matrix (N x N).
 * @return Slice holding matrix A, slice holding matrix B, and output buffer slice C.
 */
func generateMatrices(n int) ([]int32, []int32, []int32) {
	a := make([]int32, n*n)
	b := make([]int32, n*n)
	c := make([]int32, n*n)
	r := rand.New(rand.NewSource(42))
	for i := 0; i < n*n; i++ {
		a[i] = int32(r.Intn(100) - 50)
		b[i] = int32(r.Intn(100) - 50)
	}
	return a, b, c
}

/**
 * @brief Validates mathematical parity of Flat and Locality JIT execution targets.
 * 
 * Compiles and executes JIT kernels, verifying results against Go reference matrix operations.
 * 
 * @param t Go testing state handle.
 */
func TestJITCorrectness(t *testing.T) {
	sizes := []int{64, 128}
	for _, n := range sizes {
		a, b, cRef := generateMatrices(n)
		_, _, cJitFlat := generateMatrices(n)
		_, _, cJitLoc := generateMatrices(n)

		// Reference computation
		ref := GoFallbackKernel{N: n, Locality: false}
		ref.Execute(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&cRef[0]))

		// JIT Flat
		kFlat, err := CompileFlat(n)
		if err != nil {
			t.Fatalf("CompileFlat failed for N=%d: %v", n, err)
		}
		kFlat.Execute(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&cJitFlat[0]))
		_ = kFlat.Free()

		// JIT Locality
		kLoc, err := CompileLocality(n)
		if err != nil {
			t.Fatalf("CompileLocality failed for N=%d: %v", n, err)
		}
		kLoc.Execute(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&cJitLoc[0]))
		_ = kLoc.Free()

		// Compare outputs
		for i := 0; i < n*n; i++ {
			if cJitFlat[i] != cRef[i] {
				t.Fatalf("N=%d: Flat JIT mismatch at index %d: expected %d, got %d", n, i, cRef[i], cJitFlat[i])
			}
			if cJitLoc[i] != cRef[i] {
				t.Fatalf("N=%d: Locality JIT mismatch at index %d: expected %d, got %d", n, i, cRef[i], cJitLoc[i])
			}
		}
		t.Logf("N=%d: JIT math successfully validated against Go reference model.", n)
	}
}

/**
 * @brief Benchmarks compilation overhead of the dynamic JIT compiler.
 * 
 * Ensures page allocations, instruction writing, and page transitions
 * happen within acceptable microsecond limits.
 * 
 * @param t Go testing state handle.
 */
func TestJITCompilationTime(t *testing.T) {
	start := time.Now()
	k, err := CompileLocality(256)
	if err != nil {
		t.Fatalf("CompileLocality compilation failed: %v", err)
	}
	elapsed := time.Since(start)
	_ = k.Free()
	t.Logf("JIT emission overhead for 256x256 target: %s", elapsed)
	if elapsed > 50*time.Millisecond {
		t.Errorf("JIT compiler overhead exceeded performance threshold: %s", elapsed)
	}
}

/**
 * @struct mockTraceHook
 * @brief Simple mock implementation of TraceHook to test verification routing.
 */
type mockTraceHook struct {
	called bool              ///< Flag indicating if OnExecute was invoked
	meta   ExecutionMetadata ///< Metadata captured during the execution callback
}

/**
 * @brief Callback method to record JIT execution details.
 * 
 * @param meta The runtime execution details.
 */
func (m *mockTraceHook) OnExecute(meta ExecutionMetadata) {
	m.called = true
	m.meta = meta
}

/**
 * @brief Verifies that registering a trace hook captures execute pointer data.
 * 
 * @param t Go testing state handle.
 */
func TestJITTraceHook(t *testing.T) {
	hook := &mockTraceHook{}
	RegisterTraceHook(hook)
	defer RegisterTraceHook(nil)

	n := 64
	a, b, c := generateMatrices(n)

	k, err := CompileLocality(n)
	if err != nil {
		t.Fatalf("Failed to compile kernel: %v", err)
	}
	defer k.Free()

	k.Execute(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), unsafe.Pointer(&c[0]))

	if !hook.called {
		t.Fatalf("Expected trace hook to be called, but it was not")
	}

	if hook.meta.N != n {
		t.Errorf("Expected N=%d in metadata, got %d", n, hook.meta.N)
	}

	if !hook.meta.Locality {
		t.Errorf("Expected Locality=true in metadata, got false")
	}

	if hook.meta.APtr != unsafe.Pointer(&a[0]) {
		t.Errorf("Expected APtr=%p in metadata, got %p", unsafe.Pointer(&a[0]), hook.meta.APtr)
	}
}

