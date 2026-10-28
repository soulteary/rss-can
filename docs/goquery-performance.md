# goquery fork performance comparison

Reviewed on 2026-10-28 with Go 1.27.3.

This document records the local performance comparison used when replacing
`github.com/PuerkitoBio/goquery` v1.13.0 with
`github.com/soulteary/goquery` v1.14.0. It is intended as a reproducible
baseline for future dependency reviews rather than a guarantee for every RSS
Can workload.

## Test environment

- CPU: Apple M5
- OS and architecture: macOS (`darwin/arm64`)
- Go: 1.27.3
- Benchmark workers: 10 (`Benchmark...-10`)
- Upstream module: `github.com/PuerkitoBio/goquery` v1.13.0
- Fork module: `github.com/soulteary/goquery` v1.14.0

The same Go binary and build cache were used for both modules. The benchmarks
were executed sequentially on the same machine.

## Optimized node-set lookup paths

The fork specifically optimizes `FindNodes` and `FindSelection`. Each benchmark
was run 10 times with a 500 ms benchmark duration. The table reports the median
of those runs; lower values are better.

| Benchmark | Upstream median | Fork median | Latency reduction | Speed ratio |
| --- | ---: | ---: | ---: | ---: |
| `FindSelection` | 67.215 µs/op | 32.241 µs/op | 52.0% | 2.08x |
| `FindNodes` | 71.109 µs/op | 31.985 µs/op | 55.0% | 2.22x |

Both benchmarks reported the same memory profile:

| Metric | Upstream | Fork | Reduction |
| --- | ---: | ---: | ---: |
| Allocated bytes | 12,312 B/op | 6,240 B/op | 49.3% |
| Allocations | 85 allocs/op | 11 allocs/op | 87.1% |

These results reproduce the improvement described by the fork for large
document node-set lookup workloads.

### Commands

Run the following command from each downloaded module directory, using the same
Go binary for both runs:

```sh
go test \
  -run '^$' \
  -bench 'Benchmark(FindNodes|FindSelection)$' \
  -benchmem \
  -benchtime=500ms \
  -count=10
```

The reported reduction is calculated as:

```text
latency reduction = 1 - fork median / upstream median
speed ratio       = upstream median / fork median
```

## Representative general operations

To check whether the improvement applies across the package, six shared
benchmarks were run five times with a 300 ms benchmark duration. Memory and
allocation counts were unchanged in these cases.

| Benchmark | Upstream median | Fork median | Fork latency change |
| --- | ---: | ---: | ---: |
| `Text` | 403.0 ns/op | 404.5 ns/op | +0.4% |
| `Filter` | 2,071 ns/op | 2,140 ns/op | +3.3% |
| `Find` | 3,632 ns/op | 3,779 ns/op | +4.0% |
| `Children` | 70.55 ns/op | 74.60 ns/op | +5.7% |
| `Closest` | 259.1 ns/op | 277.5 ns/op | +7.1% |
| `Is` | 236.8 ns/op | 271.7 ns/op | +14.7% |

```sh
go test \
  -run '^$' \
  -bench 'Benchmark(Find|Filter|Is|Closest|Children|Text)$' \
  -benchmem \
  -benchtime=300ms \
  -count=5
```

The smaller benchmarks are more sensitive to CPU frequency, temperature and
scheduler noise. These sequential measurements show that the fork does not
provide a general speedup across all APIs; they should not be treated as proof
of a stable regression without interleaved `benchstat` runs on an otherwise
idle machine.

## Relevance to RSS Can

The measured 2.08x-2.22x speed ratio applies to the optimized node-set lookup
benchmarks, not to RSS Can end-to-end throughput. Workloads that repeatedly use
`FindNodes` or `FindSelection` on large documents should benefit most. Ordinary
`Find`, `Text` and attribute extraction chains should not be assumed to gain the
same improvement.

For an application-level result, add benchmarks around
`ParseDataAndConfigBySSR` using representative HTML pages and production-style
rules. Compare both dependency versions with interleaved runs and `benchstat`,
and record document size, selector count and result count alongside the timing.

## Dependency boundary

RSS Can imports `github.com/soulteary/goquery` directly, and that fork uses
`github.com/soulteary/cascadia` as its selector engine. The separate
HTML-to-Markdown dependency still uses upstream cascadia transitively; its
performance is outside the scope of this comparison.
