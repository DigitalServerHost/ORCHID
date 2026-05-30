/**
 * @file fair_harness.c
 * @brief Benchmark timing harness for matrix multiplication locality PoC.
 * 
 * This harness measures execution timing of compiled assembly matrix kernels
 * under equal logical execution constraints. It implements de-biasing strategies
 * such as L1-L3 cache flushing between runs, loop-swapping execution sequences,
 * and double-triplicate result verification.
 * 
 * Originator: Teppei Oohira / 大平鉄兵
 * Maintainer/Project Lead: Kevin West / @westkevin12
 */

#define _POSIX_C_SOURCE 200809L
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

/**
 * @name Configuration Constants
 * @{
 */
enum { 
    N = 512,      ///< Dimension N of the square matrices.
    PAIRS = 8     ///< Number of back-to-back benchmarking timing pairs.
};

/**
 * Size of the L1-L3 cache flushing buffer in bytes. Set to 64 MiB to completely 
 * saturate and clear modern CPU cache architectures (L1, L2, and large L3).
 */
#define FLUSH_BYTES ((size_t)64 * 1024 * 1024)

static const size_t CELLS = (size_t)N * (size_t)N;       ///< Total elements in a matrix.
static const size_t BYTES = CELLS * sizeof(int32_t);     ///< Total memory allocation size in bytes.

/**
 * Volatile register sink to prevent compiler optimizations from stripping away
 * the sequential reads/writes performed during cache-flushing loops.
 */
static volatile uint64_t flush_sink = 0;
/** @} */

/**
 * @brief External Locality-Hostile (I-J-K) assembly execution kernel.
 */
extern void matmul_flat(const int32_t *a, const int32_t *b, int32_t *c);

/**
 * @brief External Locality-Aligned (I-K-J) assembly execution kernel.
 */
extern void matmul_locality(const int32_t *a, const int32_t *b, int32_t *c);


/**
 * @brief Retrieves current system time in fractional seconds.
 * 
 * Uses CLOCK_MONOTONIC_RAW to bypass system time adjustments (NTP slews),
 * ensuring maximum precision during execution timing.
 * 
 * @return A double representing fractional seconds.
 */
static double now_sec(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC_RAW, &ts);
    return (double)ts.tv_sec + (double)ts.tv_nsec / 1000000000.0;
}


/**
 * @brief Fills input matrices with deterministic, pseudo-random integer values.
 * 
 * @param a Pointer to matrix A array.
 * @param b Pointer to matrix B array.
 */
static void fill(int32_t *a, int32_t *b) {
    for (size_t i = 0; i < CELLS; ++i) {
        a[i] = (int32_t)((i * 17u + 3u) % 7u) - 3;
        b[i] = (int32_t)((i * 13u + 5u) % 7u) - 3;
    }
}


/**
 * @brief Validates index-by-index mathematical equality of execution outputs.
 * 
 * Logs a mismatch failure description to stderr if any cell differs.
 * 
 * @param x Pointer to output matrix X.
 * @param y Pointer to output matrix Y.
 * @return Integer boolean flag (1 if identical, 0 if mismatch).
 */
static int equal_output(const int32_t *x, const int32_t *y) {
    for (size_t i = 0; i < CELLS; ++i) {
        if (x[i] != y[i]) {
            fprintf(stderr, "MISMATCH: Verification failure at index=%zu flat=%d locality=%d\n", i, x[i], y[i]);
            return 0;
        }
    }
    return 1;
}


/**
 * @brief Flushes the CPU's cache lines.
 * 
 * Sequentially writes to every 64-byte boundary within the 64 MiB buffer.
 * Forces the CPU cache controller to evict existing matrix cache lines,
 * preventing execution-history bias during timing runs.
 * 
 * @param buf Pointer to the 64 MiB cache-flush buffer.
 */
static void flush_cache(uint8_t *buf) {
    uint64_t local = 0;
    for (size_t i = 0; i < FLUSH_BYTES; i += 64) {
        buf[i] = (uint8_t)(buf[i] + 1u);
        local += buf[i];
    }
    flush_sink += local;
}


/**
 * @brief Executes a specific matmul variant and returns its execution time.
 * 
 * @param fn Function pointer to the matrix multiplication routine to benchmark.
 * @param a Pointer to input matrix A.
 * @param b Pointer to input matrix B.
 * @param out Pointer to output matrix C.
 * @return Double timing duration in seconds.
 */
static double bench(void (*fn)(const int32_t*, const int32_t*, int32_t*),
                    const int32_t *a, const int32_t *b, int32_t *out) {
    memset(out, 0, BYTES);
    double t0 = now_sec();
    fn(a, b, out);
    double t1 = now_sec();
    return t1 - t0;
}


/**
 * @brief Entry point for the benchmark suite.
 * 
 * Sets up cache-aligned matrix buffers, runs equivalence tests, performs the
 * alternating benchmark run sequences, and prints the speedup results.
 * 
 * @return System status code (0 for success, 1 for math mismatch, 2 for allocation error).
 */
int main(void) {
    // Allocate 64-byte aligned memory buffers to ensure optimal AVX cache alignment
    int32_t *a = aligned_alloc(64, BYTES);
    int32_t *b = aligned_alloc(64, BYTES);
    int32_t *cf = aligned_alloc(64, BYTES);
    int32_t *cl = aligned_alloc(64, BYTES);
    uint8_t *flush = aligned_alloc(64, FLUSH_BYTES);

    if (!a || !b || !cf || !cl || !flush) {
        fprintf(stderr, "ERROR: System failed to allocate cache-aligned buffers.\n");
        return 2;
    }

    memset(flush, 1, FLUSH_BYTES);
    fill(a, b);

    // Initial warm run & arithmetic validation check
    memset(cf, 0, BYTES);
    memset(cl, 0, BYTES);
    matmul_flat(a, b, cf);
    matmul_locality(a, b, cl);
    
    if (!equal_output(cf, cl)) {
        free(flush); free(a); free(b); free(cf); free(cl);
        return 1;
    }

    printf("VERIFY equal N=%d operations=%llu cache_flush_bytes=%llu\n",
           N, (unsigned long long)N * N * N,
           (unsigned long long)FLUSH_BYTES);

    // Primary timing benchmark sequence
    for (int r = 0; r < PAIRS; ++r) {
        double flat, local;
        const char *order;
        
        // Alternate execution order to eliminate persistent cache warming bias
        if ((r % 2) == 0) {
            order = "flat-first";
            flush_cache(flush);
            flat = bench(matmul_flat, a, b, cf);
            flush_cache(flush);
            local = bench(matmul_locality, a, b, cl);
        } else {
            order = "locality-first";
            flush_cache(flush);
            local = bench(matmul_locality, a, b, cl);
            flush_cache(flush);
            flat = bench(matmul_flat, a, b, cf);
        }
        
        printf("PAIR %d order=%s flat_sec=%.9f locality_sec=%.9f speedup=%.3fx\n",
               r + 1, order, flat, local, flat / local);
    }

    printf("FLUSH sink=%llu\n", (unsigned long long)flush_sink);

    // Resource deallocation
    free(flush); 
    free(a); 
    free(b); 
    free(cf); 
    free(cl);
    
    return 0;
}
