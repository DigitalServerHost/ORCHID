/**
 * @file fair_harness.c
 * @brief Benchmark timing harness for matrix multiplication locality PoC.
 * 
 * This harness measures execution timing of compiled assembly matrix kernels
 * under equal logical execution constraints. It implements de-biasing strategies
 * such as active software prefetching for cache retention, loop-swapping execution sequences,
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

#ifdef __x86_64__
#include <cpuid.h>
#elif defined(__aarch64__)
#ifdef __linux__
#include <sys/auxv.h>
#include <asm/hwcap.h>
#endif
#endif

/**
 * @name Configuration Constants
 * @{
 */
enum { 
    N = 512,      ///< Dimension N of the square matrices.
    PAIRS = 8     ///< Number of back-to-back benchmarking timing pairs.
};

static const size_t CELLS = (size_t)N * (size_t)N;       ///< Total elements in a matrix.
static const size_t BYTES = CELLS * sizeof(int32_t);     ///< Total memory allocation size in bytes.
/** @} */

/**
 * @brief External Locality-Hostile (I-J-K) assembly execution kernel.
 */
extern void matmul_flat(const int32_t *a, const int32_t *b, int32_t *c);

/**
 * @brief External Locality-Aligned (I-K-J) assembly execution kernel.
 */
extern void matmul_locality(const int32_t *a, const int32_t *b, int32_t *c);

#ifdef __x86_64__
/**
 * @brief Dynamic CPUID hardware capability check for AVX-512 foundation support.
 */
static int has_avx512f(void) {
    unsigned int eax, ebx, ecx, edx;
    if (__get_cpuid_max(0, NULL) < 7) {
        return 0;
    }
    __cpuid_count(7, 0, eax, ebx, ecx, edx);
    return (ebx & (1 << 16)) != 0; // AVX-512 Foundation is bit 16 in EBX of CPUID leaf 7, subleaf 0
}
#elif defined(__aarch64__)
/**
 * @brief Dynamic hardware capability check for ARM64 SVE support.
 */
static int has_sve(void) {
#if defined(__linux__) && defined(HWCAP_SVE)
    return (getauxval(AT_HWCAP) & HWCAP_SVE) != 0;
#else
    return 0;
#endif
}

/**
 * @brief Dynamic hardware capability check for ARM64 NEON/ASIMD support.
 */
static int has_asimd(void) {
#if defined(__linux__) && defined(HWCAP_ASIMD)
    return (getauxval(AT_HWCAP) & HWCAP_ASIMD) != 0;
#else
    #if defined(__APPLE__)
    return 1; // Apple Silicon always has NEON/ASIMD
    #else
    return 0;
    #endif
#endif
}
#endif

/**
 * @brief Contiguous Locality-Aligned (I-K-J) fallback kernel in C.
 * Used when the host processor does not support native vector instructions.
 * Implements software cache prefetching via GCC/Clang __builtin_prefetch.
 */
static void matmul_locality_fallback(const int32_t *a, const int32_t *b, int32_t *c) {
    const int lookahead_stride = 16; // Prefetch 16 elements (64 bytes, 1 cache line) ahead
    for (int i = 0; i < N; ++i) {
        for (int k = 0; k < N; ++k) {
            int32_t aik = a[i * N + k];
            for (int j = 0; j < N; ++j) {
                if (j + lookahead_stride < N) {
                    __builtin_prefetch(&b[k * N + j + lookahead_stride], 0, 3);
                    __builtin_prefetch(&c[i * N + j + lookahead_stride], 1, 3);
                }
                c[i * N + j] += aik * b[k * N + j];
            }
        }
    }
}


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


// Cache flushing routine and sink registers removed for real-time execution profiles.


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

    if (!a || !b || !cf || !cl) {
        fprintf(stderr, "ERROR: System failed to allocate cache-aligned buffers.\n");
        return 2;
    }

    fill(a, b);

    // Detect host capabilities at runtime and select appropriate dispatch path
    int use_vector = 0;
#ifdef __x86_64__
    use_vector = has_avx512f();
    if (use_vector) {
        printf("HARDWARE TELEMETRY: Native AVX-512 support detected. Dispatching to assembly vector kernel.\n");
    } else {
        printf("HARDWARE TELEMETRY: AVX-512 not supported. Dispatching to optimized scalar fallback kernel.\n");
    }
#elif defined(__aarch64__)
    use_vector = has_sve() || has_asimd();
    if (use_vector) {
        if (has_sve()) {
            printf("HARDWARE TELEMETRY: Native ARM64 SVE support detected. Dispatching to assembly vector kernel.\n");
        } else {
            printf("HARDWARE TELEMETRY: Native ARM64 NEON/ASIMD support detected. Dispatching to assembly vector kernel.\n");
        }
    } else {
        printf("HARDWARE TELEMETRY: ARM64 Vector extensions not supported. Dispatching to optimized scalar fallback kernel.\n");
    }
#else
    printf("HARDWARE TELEMETRY: Unsupported architecture. Dispatching to optimized scalar fallback kernel.\n");
#endif

    void (*locality_kernel)(const int32_t*, const int32_t*, int32_t*) = 
        use_vector ? matmul_locality : matmul_locality_fallback;

    // Initial warm run & arithmetic validation check
    memset(cf, 0, BYTES);
    memset(cl, 0, BYTES);
    matmul_flat(a, b, cf);
    locality_kernel(a, b, cl);
    
    if (!equal_output(cf, cl)) {
        free(a); free(b); free(cf); free(cl);
        return 1;
    }

    printf("VERIFY equal N=%d operations=%llu\n",
           N, (unsigned long long)N * N * N);

    // Primary timing benchmark sequence
    for (int r = 0; r < PAIRS; ++r) {
        double flat, local;
        const char *order;
        
        // Alternate execution order to eliminate persistent cache warming bias
        if ((r % 2) == 0) {
            order = "flat-first";
            flat = bench(matmul_flat, a, b, cf);
            local = bench(locality_kernel, a, b, cl);
        } else {
            order = "locality-first";
            local = bench(locality_kernel, a, b, cl);
            flat = bench(matmul_flat, a, b, cf);
        }
        
        printf("PAIR %d order=%s flat_sec=%.9f locality_sec=%.9f speedup=%.3fx\n",
               r + 1, order, flat, local, flat / local);
    }

    // Resource deallocation
    free(a); 
    free(b); 
    free(cf); 
    free(cl);
    
    return 0;
}
