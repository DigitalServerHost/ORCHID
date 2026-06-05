/**
 * @file matmul_wrapper.go
 * @brief Go wrapper linking JIT compilation and executing locality timing benchmarks.
 * 
 * Coordinate JIT execution, physical memory alignment allocations,
 * CPU cache flushes, statistical speedup analysis, and timing files creation.
 * 
 * Originator: Teppei Oohira (@gatchimuchio) / 大平鉄兵
 * Project Lead & Maintainer: Kevin West (@westkevin12)
 * License: GNU GPLv3
 */

package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

void flush_cache_c(uint8_t *buf, size_t size);
uint64_t get_flush_sink(void);
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unsafe"

	"ORCHID/jit"
)

const (
	N          = 512
	Cells      = N * N
	Bytes      = Cells * 4
	FlushBytes = 64 * 1024 * 1024
)

/**
 * @struct LocalityResult
 * @brief Stores timing speedup statistics and correctness verification checksums.
 */
type LocalityResult struct {
	Min      float64 `json:"min"`
	Median   float64 `json:"median"`
	Max      float64 `json:"max"`
	Mean     float64 `json:"mean"`
	Checksum int64   `json:"checksum"`
}

/**
 * @brief Computes the median value of a slice of floats.
 * 
 * @param values Slice of floats to compute median for.
 * @return The calculated median value.
 */
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	sort.Float64s(values)
	n := len(values)
	if n%2 == 1 {
		return values[n/2]
	}
	return (values[n/2-1] + values[n/2]) / 2.0
}

/**
 * @struct benchmarkConfig
 * @brief Bundles variables and pointers passed to benchmark executors.
 */
type benchmarkConfig struct {
	aPtr     unsafe.Pointer
	bPtr     unsafe.Pointer
	cfPtr    unsafe.Pointer
	clPtr    unsafe.Pointer
	flushPtr unsafe.Pointer
	kFlat    jit.Kernel
	kLoc     jit.Kernel
}

/**
 * @brief Executes pairs of flat vs locality benchmarks to measure cache speedups.
 * 
 * @param repeats Number of benchmark iterations to perform.
 * @param cfg Pointer to benchmarkConfig payload.
 * @return Speedup values slice and printed log lines slice.
 */
func runBenchmarkPairs(repeats int, cfg *benchmarkConfig) ([]float64, []string) {
	var speedups []float64
	var timingLines []string

	for r := 0; r < repeats; r++ {
		var flatSec, localSec float64
		var order string

		if r%2 == 0 {
			order = "flat-first"
			C.flush_cache_c((*C.uint8_t)(cfg.flushPtr), C.size_t(FlushBytes))
			C.memset(cfg.cfPtr, 0, C.size_t(Bytes))
			t0 := time.Now()
			cfg.kFlat.Execute(cfg.aPtr, cfg.bPtr, cfg.cfPtr)
			flatSec = time.Since(t0).Seconds()

			C.flush_cache_c((*C.uint8_t)(cfg.flushPtr), C.size_t(FlushBytes))
			C.memset(cfg.clPtr, 0, C.size_t(Bytes))
			t0 = time.Now()
			cfg.kLoc.Execute(cfg.aPtr, cfg.bPtr, cfg.clPtr)
			localSec = time.Since(t0).Seconds()
		} else {
			order = "locality-first"
			C.flush_cache_c((*C.uint8_t)(cfg.flushPtr), C.size_t(FlushBytes))
			C.memset(cfg.clPtr, 0, C.size_t(Bytes))
			t0 := time.Now()
			cfg.kLoc.Execute(cfg.aPtr, cfg.bPtr, cfg.clPtr)
			localSec = time.Since(t0).Seconds()

			C.flush_cache_c((*C.uint8_t)(cfg.flushPtr), C.size_t(FlushBytes))
			C.memset(cfg.cfPtr, 0, C.size_t(Bytes))
			t0 = time.Now()
			cfg.kFlat.Execute(cfg.aPtr, cfg.bPtr, cfg.cfPtr)
			flatSec = time.Since(t0).Seconds()
		}

		speedup := flatSec / localSec
		speedups = append(speedups, speedup)

		pairMsg := fmt.Sprintf("PAIR %d order=%s flat_sec=%.9f locality_sec=%.9f speedup=%.3fx",
			r+1, order, flatSec, localSec, speedup)
		fmt.Println(pairMsg)
		timingLines = append(timingLines, pairMsg)
	}

	return speedups, timingLines
}

type benchmarkTraceHook struct{}

func (b *benchmarkTraceHook) OnExecute(meta jit.ExecutionMetadata) {
	// This empty callback is intentional to measure trace callback dispatch overhead.
}

/**
 * @struct BenchmarkOutputs
 * @brief Groups together the benchmark output metrics and directory configurations.
 */
type BenchmarkOutputs struct {
	OutDir       string
	TelemetryMsg string
	VerifyMsg    string
	FlushSinkMsg string
	TimingLines  []string
	Min          float64
	Median       float64
	Max          float64
	Mean         float64
	TraceMin     float64
	TraceMedian  float64
	TraceMax     float64
	TraceMean    float64
}

/**
 * @brief Outputs the timing log, statistical summaries, and JSON speedups to the file system.
 * 
 * @param cfg Pointer to BenchmarkOutputs configuration.
 * @return error if any directory creation or write operation fails.
 */
func writeBenchmarkOutputs(cfg *BenchmarkOutputs) error {
	if cfg.OutDir == "" {
		return nil
	}

	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return err
	}

	// 1. Write fair_timing_result_current_environment.txt
	timingContent := cfg.TelemetryMsg + "\n" + cfg.VerifyMsg + "\n"
	for _, line := range cfg.TimingLines {
		timingContent += line + "\n"
	}
	timingContent += cfg.FlushSinkMsg + "\n"
	err := os.WriteFile(filepath.Join(cfg.OutDir, "fair_timing_result_current_environment.txt"), []byte(timingContent), 0644)
	if err != nil {
		return err
	}

	// 2. Write fair_summary_current_environment.txt
	summaryContent := fmt.Sprintf("speedup_min=%.3fx\nspeedup_median=%.3fx\nspeedup_max=%.3fx\nspeedup_mean=%.3fx\n",
		cfg.Min, cfg.Median, cfg.Max, cfg.Mean)
	err = os.WriteFile(filepath.Join(cfg.OutDir, "fair_summary_current_environment.txt"), []byte(summaryContent), 0644)
	if err != nil {
		return err
	}

	// 3. Write speedups.json (Standard vs Trace comparative format)
	speedupMap := map[string]map[string]string{
		"standard": {
			"min":    fmt.Sprintf("%.3fx", cfg.Min),
			"median": fmt.Sprintf("%.3fx", cfg.Median),
			"max":    fmt.Sprintf("%.3fx", cfg.Max),
			"mean":   fmt.Sprintf("%.3fx", cfg.Mean),
		},
		"trace": {
			"min":    fmt.Sprintf("%.3fx", cfg.TraceMin),
			"median": fmt.Sprintf("%.3fx", cfg.TraceMedian),
			"max":    fmt.Sprintf("%.3fx", cfg.TraceMax),
			"mean":   fmt.Sprintf("%.3fx", cfg.TraceMean),
		},
	}
	speedupJSON, err := json.MarshalIndent(speedupMap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.OutDir, "speedups.json"), append(speedupJSON, '\n'), 0644)
}

/**
 * @brief Computes summary statistics from a speedup values slice.
 * 
 * @param speedups Slice of floating-point speedups.
 * @return min, median, max, and mean speedups.
 */
func computeStats(speedups []float64) (float64, float64, float64, float64) {
	minVal := speedups[0]
	maxVal := speedups[0]
	sumVal := 0.0
	for _, v := range speedups {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sumVal += v
	}
	meanVal := sumVal / float64(len(speedups))
	medianVal := median(speedups)
	return minVal, medianVal, maxVal, meanVal
}

/**
 * @brief Compares two result slices for structural equality.
 * 
 * @param cf Reference flat result slice.
 * @param cl Locality-optimized result slice.
 * @return error if any value mismatch is found.
 */
func verifyEquivalence(cf, cl []int32) error {
	for i := 0; i < len(cf); i++ {
		if cf[i] != cl[i] {
			return fmt.Errorf("MISMATCH: Verification failure at index=%d flat=%d locality=%d", i, cf[i], cl[i])
		}
	}
	return nil
}

/**
 * @brief Entry point for running the matrix cache-locality timing benchmark.
 * 
 * Coordinates memory allocations, registers inputs, validates flat/locality equivalence,
 * runs the benchmarking sweeps, and writes outputs to the output directory.
 * 
 * @param repeats Number of benchmark pairs to execute.
 * @param outDir Target directory to save results files.
 * @return LocalityResult with statistics and checksum or an error.
 */
func RunLocalityBenchmark(repeats int, outDir string) (*LocalityResult, error) {
	if repeats < 1 {
		repeats = 8
	}

	// 1. Allocate 64-byte aligned memory buffers to ensure optimal AVX cache alignment
	aPtr := C.aligned_alloc(64, C.size_t(Bytes))
	bPtr := C.aligned_alloc(64, C.size_t(Bytes))
	cfPtr := C.aligned_alloc(64, C.size_t(Bytes))
	clPtr := C.aligned_alloc(64, C.size_t(Bytes))
	flushPtr := C.aligned_alloc(64, C.size_t(FlushBytes))

	if aPtr == nil || bPtr == nil || cfPtr == nil || clPtr == nil || flushPtr == nil {
		return nil, fmt.Errorf("system failed to allocate cache-aligned buffers")
	}

	defer C.free(aPtr)
	defer C.free(bPtr)
	defer C.free(cfPtr)
	defer C.free(clPtr)
	defer C.free(flushPtr)

	C.memset(flushPtr, 1, C.size_t(FlushBytes))

	// Cast C pointers to Go slices for easy initialization and checksum
	aSlice := (*[1 << 28]int32)(unsafe.Pointer(aPtr))[:Cells:Cells]
	bSlice := (*[1 << 28]int32)(unsafe.Pointer(bPtr))[:Cells:Cells]
	cfSlice := (*[1 << 28]int32)(unsafe.Pointer(cfPtr))[:Cells:Cells]
	clSlice := (*[1 << 28]int32)(unsafe.Pointer(clPtr))[:Cells:Cells]

	// Fill inputs with deterministic pseudo-random values
	for i := 0; i < Cells; i++ {
		aSlice[i] = int32((uint32(i)*17 + 3) % 7) - 3
		bSlice[i] = int32((uint32(i)*13 + 5) % 7) - 3
	}

	// Dynamic compile dynamic JIT kernels and measure compilation latency
	tJitStart := time.Now()
	kFlat, err := jit.CompileFlat(N)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JIT flat kernel: %w", err)
	}
	defer kFlat.Free()

	kLoc, err := jit.CompileLocality(N)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JIT locality kernel: %w", err)
	}
	defer kLoc.Free()
	jitElapsed := time.Since(tJitStart)

	telemetryMsg := fmt.Sprintf("HARDWARE TELEMETRY: JIT compiled kernels in %s. Executing bare-metal blocks via W^X function pointers.", jitElapsed)
	fmt.Println(telemetryMsg)

	// Initial warm run & arithmetic validation check
	C.memset(cfPtr, 0, C.size_t(Bytes))
	C.memset(clPtr, 0, C.size_t(Bytes))

	kFlat.Execute(aPtr, bPtr, cfPtr)
	kLoc.Execute(aPtr, bPtr, clPtr)

	// Verify equal outputs
	if err := verifyEquivalence(cfSlice, clSlice); err != nil {
		return nil, err
	}

	// Calculate checksum of results
	var checksum int64
	for i, v := range cfSlice {
		checksum += int64(i+1) * int64(v)
	}

	verifyMsg := fmt.Sprintf("VERIFY equal N=%d operations=%d cache_flush_bytes=%d", N, N*N*N, FlushBytes)
	fmt.Println(verifyMsg)

	benchCfg := &benchmarkConfig{
		aPtr:     aPtr,
		bPtr:     bPtr,
		cfPtr:    cfPtr,
		clPtr:    clPtr,
		flushPtr: flushPtr,
		kFlat:    kFlat,
		kLoc:     kLoc,
	}

	// Collect standard timing pairs
	speedups, timingLines := runBenchmarkPairs(repeats, benchCfg)

	// Register trace hook for trace mode benchmarking
	fmt.Println("\n--- ENABLING TRACE MODE ---")
	jit.RegisterTraceHook(&benchmarkTraceHook{})

	// Collect trace timing pairs
	traceSpeedups, traceTimingLines := runBenchmarkPairs(repeats, benchCfg)

	// Clean up trace hook registration
	jit.RegisterTraceHook(nil)
	fmt.Println("--- TRACE MODE DISABLED ---")
	fmt.Println()

	flushSinkMsg := fmt.Sprintf("FLUSH sink=%d", C.get_flush_sink())
	fmt.Println(flushSinkMsg)

	// Compute statistics
	minVal, medianVal, maxVal, meanVal := computeStats(speedups)
	traceMinVal, traceMedianVal, traceMaxVal, traceMeanVal := computeStats(traceSpeedups)

	// Merge timing lines for logging
	allTimingLines := append([]string{"=== STANDARD MODE TIMINGS ==="}, timingLines...)
	allTimingLines = append(allTimingLines, "=== TRACE MODE TIMINGS ===")
	allTimingLines = append(allTimingLines, traceTimingLines...)

	// Write output files
	cfg := &BenchmarkOutputs{
		OutDir:       outDir,
		TelemetryMsg: telemetryMsg,
		VerifyMsg:    verifyMsg,
		FlushSinkMsg: flushSinkMsg,
		TimingLines:  allTimingLines,
		Min:          minVal,
		Median:       medianVal,
		Max:          maxVal,
		Mean:         meanVal,
		TraceMin:     traceMinVal,
		TraceMedian:  traceMedianVal,
		TraceMax:     traceMaxVal,
		TraceMean:    traceMeanVal,
	}
	if err := writeBenchmarkOutputs(cfg); err != nil {
		return nil, err
	}

	return &LocalityResult{
		Min:      minVal,
		Median:   medianVal,
		Max:      maxVal,
		Mean:     meanVal,
		Checksum: checksum,
	}, nil
}
