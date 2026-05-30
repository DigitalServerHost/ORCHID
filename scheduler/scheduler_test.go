/**
 * @file scheduler_test.go
 * @brief High-performance unit tests and benchmarks for the banked memory scheduler.
 * 
 * Verifies mathematical calculation consistency and benchmarks scheduling cycles
 * under serialized single-bank vs. parallel role-split memory layouts.
 * 
 * Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
 * Project Lead & Maintainer: Kevin West (@westkevin12)
 * License: GNU GPLv3
 */

package scheduler

import (
	"sync"
	"testing"
)

/**
 * @brief Generates deterministic inputs stream vectors for B and C.
 */
func generateInputVectors(n int) ([]int32, []int32) {
	b := make([]int32, n)
	c := make([]int32, n)
	for i := 0; i < n; i++ {
		b[i] = int32(((i*17 + 3) % 97) - 48)
		c[i] = int32(((i*29 + 11) % 89) - 44)
	}
	return b, c
}

/**
 * @brief Simulates the STREAM-Triad scheduler logic concurrently using goroutines.
 */
func runTriadSimulation(
	n int,
	scalar int32,
	bankCount int,
	mapping map[string]int,
	serviceCycles uint64,
	computeCycles uint64,
) (uint64, []int32, int64, error) {
	b, c := generateInputVectors(n)
	a := make([]int32, n)

	scheduler, err := NewMemoryScheduler(bankCount, serviceCycles, 10)
	if err != nil {
		return 0, nil, 0, err
	}

	var wg sync.WaitGroup
	wg.Add(n)

	// Execute each triad task element in a concurrent goroutine
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()

			// Thread-safe concurrent memory reads for streams B and C
			bDone, _ := scheduler.Access("B", "READ", idx, mapping["B"], 0)
			cDone, _ := scheduler.Access("C", "READ", idx, mapping["C"], 0)

			// Arithmetic computation delay
			readyCycle := bDone
			if cDone > readyCycle {
				readyCycle = cDone
			}
			computedCycle := readyCycle + computeCycles

			a[idx] = b[idx] + scalar*c[idx] // Logical mathematical work

			// Thread-safe concurrent memory write back for output stream A
			_, _ = scheduler.Access("A", "WRITE", idx, mapping["A"], computedCycle)
		}(i)
	}

	wg.Wait()

	// Calculate check checksum
	var checksum int64
	for i, v := range a {
		checksum += int64(i+1) * int64(v)
	}

	return scheduler.TotalCycles(), a, checksum, nil
}

/**
 * @brief Tests the Go scheduler with both serialized and parallel memory splits.
 */
func TestBankedSchedulerTriad(t *testing.T) {
	n := 16384
	var scalar int32 = 3
	var serviceCycles uint64 = 100
	var computeCycles uint64 = 1

	// Setup Case 1: Single Serialized Memory Bank (B=0, C=0, A=0)
	serialMapping := map[string]int{"B": 0, "C": 0, "A": 0}
	serialCycles, serialA, serialChecksum, err := runTriadSimulation(n, scalar, 1, serialMapping, serviceCycles, computeCycles)
	if err != nil {
		t.Fatalf("Serial simulation failed: %v", err)
	}

	// Setup Case 2: Three-Bank Ideal Role-Split (B=0, C=1, A=2)
	parallelMapping := map[string]int{"B": 0, "C": 1, "A": 2}
	parallelCycles, parallelA, parallelChecksum, err := runTriadSimulation(n, scalar, 3, parallelMapping, serviceCycles, computeCycles)
	if err != nil {
		t.Fatalf("Parallel simulation failed: %v", err)
	}

	// 1. Assert mathematical output arrays are identical
	if len(serialA) != len(parallelA) {
		t.Errorf("Output length mismatch: serial=%d parallel=%d", len(serialA), len(parallelA))
	}
	for i := 0; i < len(serialA); i++ {
		if serialA[i] != parallelA[i] {
			t.Fatalf("Mathematical cell mismatch at index %d: serial=%d parallel=%d", i, serialA[i], parallelA[i])
		}
	}

	// 2. Assert checksums are identical
	if serialChecksum != parallelChecksum {
		t.Errorf("Checksum mismatch: serial=%d parallel=%d", serialChecksum, parallelChecksum)
	}

	// 3. Log speedup results to verify theoretical multi-bank limits
	speedup := float64(serialCycles) / float64(parallelCycles)
	t.Logf("VERIFY: Mathematical calculations are 100%% identical!")
	t.Logf("Deterministic Serial Cycles: %d", serialCycles)
	t.Logf("Deterministic Parallel Cycles: %d", parallelCycles)
	t.Logf("Theoretical Parallel Speedup achieved in Go: %.3fx", speedup)

	// Ensure the parallel speedup is > 1.5x (theoretical limit is near 3.0x)
	if speedup < 1.5 {
		t.Errorf("Insufficient parallel speedup: %.3fx (expected > 1.5x)", speedup)
	}
}
