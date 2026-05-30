#define _POSIX_C_SOURCE 200809L
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

enum { N = 512, PAIRS = 8 };
#define FLUSH_BYTES ((size_t)64 * 1024 * 1024)
static const size_t CELLS = (size_t)N * (size_t)N;
static const size_t BYTES = CELLS * sizeof(int32_t);
static volatile uint64_t flush_sink = 0;

extern void matmul_flat(const int32_t *a, const int32_t *b, int32_t *c);
extern void matmul_locality(const int32_t *a, const int32_t *b, int32_t *c);

static double now_sec(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC_RAW, &ts);
    return (double)ts.tv_sec + (double)ts.tv_nsec / 1000000000.0;
}
static void fill(int32_t *a, int32_t *b) {
    for (size_t i = 0; i < CELLS; ++i) {
        a[i] = (int32_t)((i * 17u + 3u) % 7u) - 3;
        b[i] = (int32_t)((i * 13u + 5u) % 7u) - 3;
    }
}
static int equal_output(const int32_t *x, const int32_t *y) {
    for (size_t i = 0; i < CELLS; ++i) {
        if (x[i] != y[i]) {
            fprintf(stderr, "MISMATCH index=%zu flat=%d locality=%d\n", i, x[i], y[i]);
            return 0;
        }
    }
    return 1;
}
static void flush_cache(uint8_t *buf) {
    uint64_t local = 0;
    for (size_t i = 0; i < FLUSH_BYTES; i += 64) {
        buf[i] = (uint8_t)(buf[i] + 1u);
        local += buf[i];
    }
    flush_sink += local;
}
static double bench(void (*fn)(const int32_t*, const int32_t*, int32_t*),
                    const int32_t *a, const int32_t *b, int32_t *out) {
    memset(out, 0, BYTES);
    double t0 = now_sec();
    fn(a, b, out);
    double t1 = now_sec();
    return t1 - t0;
}
int main(void) {
    int32_t *a = aligned_alloc(64, BYTES);
    int32_t *b = aligned_alloc(64, BYTES);
    int32_t *cf = aligned_alloc(64, BYTES);
    int32_t *cl = aligned_alloc(64, BYTES);
    uint8_t *flush = aligned_alloc(64, FLUSH_BYTES);
    if (!a || !b || !cf || !cl || !flush) return 2;
    memset(flush, 1, FLUSH_BYTES);
    fill(a, b);

    memset(cf, 0, BYTES);
    memset(cl, 0, BYTES);
    matmul_flat(a, b, cf);
    matmul_locality(a, b, cl);
    if (!equal_output(cf, cl)) return 1;

    printf("VERIFY equal N=%d operations=%llu cache_flush_bytes=%llu\n",
           N, (unsigned long long)N * N * N,
           (unsigned long long)FLUSH_BYTES);

    for (int r = 0; r < PAIRS; ++r) {
        double flat, local;
        const char *order;
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
    free(flush); free(a); free(b); free(cf); free(cl);
    return 0;
}
