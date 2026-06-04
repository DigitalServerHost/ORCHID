/**
 * @file matmul_fallback.c
 * @brief C fallbacks and CPU capability checking for ORCHID matrix kernels.
 */

#include <stdint.h>
#include <stddef.h>
#include <cpuid.h>

#define N 512

static volatile uint64_t flush_sink = 0;

/**
 * @brief Dynamic CPUID hardware capability check for AVX-512 foundation support.
 */
int has_avx512f(void) {
    unsigned int eax, ebx, ecx, edx;
    if (__get_cpuid_max(0, NULL) < 7) {
        return 0;
    }
    __cpuid_count(7, 0, eax, ebx, ecx, edx);
    return (ebx & (1 << 16)) != 0; // AVX-512 Foundation is bit 16 in EBX of CPUID leaf 7, subleaf 0
}

/**
 * @brief Contiguous Locality-Aligned (I-K-J) fallback kernel in C.
 * Used when the host processor does not support native AVX-512 vector instructions.
 */
void matmul_locality_fallback(const int32_t *a, const int32_t *b, int32_t *c) {
    for (int i = 0; i < N; ++i) {
        for (int k = 0; k < N; ++k) {
            int32_t aik = a[i * N + k];
            for (int j = 0; j < N; ++j) {
                c[i * N + j] += aik * b[k * N + j];
            }
        }
    }
}

/**
 * @brief Flushes cache lines from L1-L3 by writing to contiguous pages.
 */
void flush_cache_c(uint8_t *buf, size_t size) {
    uint64_t local = 0;
    for (size_t i = 0; i < size; i += 64) {
        buf[i] = (uint8_t)(buf[i] + 1u);
        local += buf[i];
    }
    flush_sink += local;
}

/**
 * @brief Retrieves the flush sink accumulator to prevent optimization.
 */
uint64_t get_flush_sink(void) {
    return flush_sink;
}
